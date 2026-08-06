package compositor

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const ipcProtocolVersion = 1

type IPCServer struct {
	compositor *Server
	listener   net.Listener
	socketPath string
	requests   chan ipcQueuedRequest

	mu      sync.Mutex
	clients map[*ipcClient]struct{}
	closed  bool
}

type ipcClient struct {
	conn net.Conn
	send chan IPCMessage

	mu            sync.RWMutex
	subscriptions map[string]bool
	closed        bool
}

type ipcQueuedRequest struct {
	client   *ipcClient
	request  IPCRequest
	response chan IPCMessage
}

type IPCRequest struct {
	Type            string          `json:"type"`
	ID              any             `json:"id,omitempty"`
	ProtocolVersion int             `json:"protocol_version,omitempty"`
	Client          string          `json:"client,omitempty"`
	ClientVersion   string          `json:"client_version,omitempty"`
	Events          []string        `json:"events,omitempty"`
	Command         string          `json:"command,omitempty"`
	Workspace       int             `json:"workspace,omitempty"`
	Wallpaper       string          `json:"wallpaper,omitempty"`
	Arguments       json.RawMessage `json:"arguments,omitempty"`
}

type IPCMessage struct {
	Type            string   `json:"type"`
	ID              any      `json:"id,omitempty"`
	Success         *bool    `json:"success,omitempty"`
	Error           string   `json:"error,omitempty"`
	ProtocolVersion int      `json:"protocol_version,omitempty"`
	Server          string   `json:"server,omitempty"`
	ServerVersion   string   `json:"server_version,omitempty"`
	Capabilities    []string `json:"capabilities,omitempty"`
	Event           string   `json:"event,omitempty"`
	Result          any      `json:"result,omitempty"`
	Data            any      `json:"data,omitempty"`
	Timestamp       string   `json:"timestamp,omitempty"`
}

type IPCState struct {
	Workspace      int            `json:"workspace"`
	WorkspaceCount int            `json:"workspace_count"`
	Tiling         bool           `json:"tiling"`
	Fullscreen     bool           `json:"fullscreen"`
	KeyboardLayout string         `json:"keyboard_layout"`
	Wallpaper      string         `json:"wallpaper"`
	FocusedWindow  *IPCWindow     `json:"focused_window,omitempty"`
	Workspaces     []IPCWorkspace `json:"workspaces"`
	Output         IPCOutput      `json:"output"`
}

type IPCWorkspace struct {
	Number  int  `json:"number"`
	Active  bool `json:"active"`
	Focused bool `json:"focused"`
	Urgent  bool `json:"urgent"`
	Windows int  `json:"windows"`
}

type IPCOutput struct {
	X            int `json:"x"`
	Y            int `json:"y"`
	Width        int `json:"width"`
	Height       int `json:"height"`
	UsableX      int `json:"usable_x"`
	UsableY      int `json:"usable_y"`
	UsableWidth  int `json:"usable_width"`
	UsableHeight int `json:"usable_height"`
}

type IPCWindow struct {
	ID         uint64 `json:"id"`
	Title      string `json:"title,omitempty"`
	AppID      string `json:"app_id,omitempty"`
	Class      string `json:"class,omitempty"`
	Instance   string `json:"instance,omitempty"`
	Output     string `json:"output,omitempty"`
	Rules      string `json:"rules,omitempty"`
	Workspace  int    `json:"workspace"`
	Mapped     bool   `json:"mapped"`
	Focused    bool   `json:"focused"`
	Urgent     bool   `json:"urgent"`
	Dialog     bool   `json:"dialog"`
	Modal      bool   `json:"modal"`
	Floating   bool   `json:"floating"`
	Fullscreen bool   `json:"fullscreen"`
	XWayland   bool   `json:"xwayland"`
	X          int    `json:"x"`
	Y          int    `json:"y"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

func (s *Server) startIPC() error {
	if s.ipc != nil {
		return nil
	}
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		return errors.New("XDG_RUNTIME_DIR is not set")
	}
	dir := filepath.Join(runtimeDir, "hatwm")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create IPC directory: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return fmt.Errorf("secure IPC directory: %w", err)
	}
	path := filepath.Join(dir, "ipc.sock")
	if err := removeStaleIPCSocket(path); err != nil {
		return err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return fmt.Errorf("listen on IPC socket: %w", err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		_ = listener.Close()
		_ = os.Remove(path)
		return fmt.Errorf("secure IPC socket: %w", err)
	}
	ipc := &IPCServer{
		compositor: s,
		listener:   listener,
		socketPath: path,
		requests:   make(chan ipcQueuedRequest, 128),
		clients:    make(map[*ipcClient]struct{}),
	}
	s.ipc = ipc
	_ = os.Setenv("HATWM_SOCKET", path)
	go ipc.acceptLoop()
	slog.Info("HatWM IPC listening", "socket", path)
	return nil
}

func removeStaleIPCSocket(path string) error {
	if _, err := os.Lstat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	conn, err := net.DialTimeout("unix", path, 150*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return fmt.Errorf("another HatWM IPC server is already listening at %s", path)
	}
	return os.Remove(path)
}

func (ipc *IPCServer) acceptLoop() {
	for {
		conn, err := ipc.listener.Accept()
		if err != nil {
			ipc.mu.Lock()
			closed := ipc.closed
			ipc.mu.Unlock()
			if !closed {
				slog.Warn("IPC accept failed", "error", err)
			}
			return
		}
		client := &ipcClient{conn: conn, send: make(chan IPCMessage, 64), subscriptions: make(map[string]bool)}
		ipc.mu.Lock()
		ipc.clients[client] = struct{}{}
		ipc.mu.Unlock()
		go ipc.writeLoop(client)
		go ipc.readLoop(client)
	}
}

func (ipc *IPCServer) readLoop(client *ipcClient) {
	defer ipc.removeClient(client)
	scanner := bufio.NewScanner(client.conn)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		var req IPCRequest
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			client.enqueue(ipcError(nil, "invalid JSON: "+err.Error()))
			continue
		}
		response := make(chan IPCMessage, 1)
		queued := ipcQueuedRequest{client: client, request: req, response: response}
		select {
		case ipc.requests <- queued:
		case <-time.After(2 * time.Second):
			client.enqueue(ipcError(req.ID, "compositor request queue is busy"))
			continue
		}
		select {
		case msg := <-response:
			client.enqueue(msg)
		case <-time.After(5 * time.Second):
			client.enqueue(ipcError(req.ID, "compositor request timed out"))
		}
	}
}

func (ipc *IPCServer) writeLoop(client *ipcClient) {
	encoder := json.NewEncoder(client.conn)
	for message := range client.send {
		if err := encoder.Encode(message); err != nil {
			ipc.removeClient(client)
			return
		}
	}
}

func (client *ipcClient) enqueue(message IPCMessage) {
	client.mu.RLock()
	closed := client.closed
	client.mu.RUnlock()
	if closed {
		return
	}
	select {
	case client.send <- message:
	default:
		_ = client.conn.Close()
	}
}

func (ipc *IPCServer) removeClient(client *ipcClient) {
	client.mu.Lock()
	if client.closed {
		client.mu.Unlock()
		return
	}
	client.closed = true
	close(client.send)
	_ = client.conn.Close()
	client.mu.Unlock()
	ipc.mu.Lock()
	delete(ipc.clients, client)
	ipc.mu.Unlock()
}

func (ipc *IPCServer) Close() {
	if ipc == nil {
		return
	}
	ipc.mu.Lock()
	if ipc.closed {
		ipc.mu.Unlock()
		return
	}
	ipc.closed = true
	clients := make([]*ipcClient, 0, len(ipc.clients))
	for client := range ipc.clients {
		clients = append(clients, client)
	}
	ipc.mu.Unlock()
	_ = ipc.listener.Close()
	for _, client := range clients {
		ipc.removeClient(client)
	}
	_ = os.Remove(ipc.socketPath)
}

func (s *Server) processIPCRequests() {
	if s.ipc == nil {
		return
	}
	for i := 0; i < 32; i++ {
		select {
		case queued := <-s.ipc.requests:
			queued.response <- s.handleIPCRequest(queued.client, queued.request)
		default:
			return
		}
	}
}

func (s *Server) handleIPCRequest(client *ipcClient, req IPCRequest) IPCMessage {
	switch strings.ToLower(req.Type) {
	case "hello":
		if req.ProtocolVersion != 0 && req.ProtocolVersion != ipcProtocolVersion {
			return ipcError(req.ID, fmt.Sprintf("unsupported protocol version %d", req.ProtocolVersion))
		}
		ok := true
		return IPCMessage{Type: "hello", ID: req.ID, Success: &ok, ProtocolVersion: ipcProtocolVersion, Server: "HatWM", ServerVersion: "development", Capabilities: []string{"state", "workspaces", "windows", "events", "commands"}}
	case "get_state":
		return ipcSuccess(req.ID, s.ipcState())
	case "get_workspaces":
		return ipcSuccess(req.ID, s.ipcWorkspaces())
	case "get_windows":
		return ipcSuccess(req.ID, s.ipcWindows())
	case "subscribe":
		client.mu.Lock()
		client.subscriptions = make(map[string]bool)
		for _, event := range req.Events {
			client.subscriptions[strings.ToLower(event)] = true
		}
		client.mu.Unlock()
		return ipcSuccess(req.ID, map[string]any{"events": req.Events})
	case "command":
		return s.handleIPCCommand(req)
	default:
		return ipcError(req.ID, "unknown request type")
	}
}

func (s *Server) handleIPCCommand(req IPCRequest) IPCMessage {
	command := strings.ToLower(strings.TrimSpace(req.Command))
	var handled bool
	switch command {
	case "workspace":
		handled = s.switchWorkspaceArg(fmt.Sprint(req.Workspace))
	case "move_to_workspace":
		handled = s.moveFocusedToWorkspaceArg(fmt.Sprint(req.Workspace))
	case "set_wallpaper":
		if strings.TrimSpace(req.Wallpaper) == "" {
			return ipcError(req.ID, "wallpaper path is required")
		}
		if err := s.setWallpaper(req.Wallpaper); err != nil {
			return ipcError(req.ID, fmt.Sprintf("could not set wallpaper: %v", err))
		}
		s.config.Wallpaper = s.wallpaperPath
		handled = true
		s.emitIPCEvent("wallpaper_changed",
			map[string]any{"path": s.wallpaperPath})
	case "toggle_tiling", "toggle_fullscreen", "toggle_keyboard_layout",
		"cycle_focus", "close", "reload_config", "exit":
		if command == "reload_config" {
			cfg, err := LoadConfig()
			if err != nil {
				return ipcError(req.ID, err.Error())
			}
			oldConfig := s.config
			s.config = cfg
			if oldConfig.CursorTheme != cfg.CursorTheme || oldConfig.CursorSize != cfg.CursorSize {
				if err := s.configureCursorTheme(cfg.CursorTheme, cfg.CursorSize); err != nil {
					s.config = oldConfig
					return ipcError(req.ID, err.Error())
				}
			}
			if appearanceChanged(oldConfig, s.config) {
				s.applyAppearanceProfile()
			}
			s.reapplyWindowRules()
			s.applyWindowOpacityToAll()
			s.startWallpaper()
			s.updateAllDecorations()
			s.arrange()
			handled = true
			s.emitIPCEvent("config_reloaded", s.ipcState())
		} else {
			handled = s.executeAction(command, "")
		}
	default:
		return ipcError(req.ID, "unsupported command")
	}
	if !handled {
		return ipcError(req.ID, "command could not be completed")
	}
	return ipcSuccess(req.ID, s.ipcState())
}

func ipcSuccess(id any, result any) IPCMessage {
	ok := true
	return IPCMessage{Type: "response", ID: id, Success: &ok, Result: result}
}
func ipcError(id any, message string) IPCMessage {
	ok := false
	return IPCMessage{Type: "response", ID: id, Success: &ok, Error: message}
}

func (s *Server) ipcState() IPCState {
	layouts := s.config.KeyboardLayouts
	layout := ""
	if len(layouts) > 0 {
		layout = layouts[s.keyboardLayoutIndex%len(layouts)]
	}
	var focused *IPCWindow
	if view := s.focusedView(); view != nil {
		value := s.ipcWindow(view)
		focused = &value
	}
	count := s.config.WorkspaceCount
	if count < 1 {
		count = 9
	}
	return IPCState{
		Workspace:      s.currentWorkspace,
		WorkspaceCount: count,
		Tiling:         s.config.Tiling,
		Fullscreen:     s.fullscreen != nil,
		KeyboardLayout: layout,
		Wallpaper:      s.wallpaperPath,
		FocusedWindow:  focused,
		Workspaces:     s.ipcWorkspaces(),
		Output:         s.ipcOutput(),
	}
}

func (s *Server) ipcOutput() IPCOutput {
	result := IPCOutput{
		UsableX:      s.usable.x,
		UsableY:      s.usable.y,
		UsableWidth:  s.usable.width,
		UsableHeight: s.usable.height,
	}
	if len(s.outputs) > 0 {
		result.Width, result.Height = s.outputs[0].EffectiveResolution()
	}
	if result.UsableWidth <= 0 {
		result.UsableWidth = result.Width
	}
	if result.UsableHeight <= 0 {
		result.UsableHeight = result.Height
	}
	return result
}

func (s *Server) ipcWorkspaces() []IPCWorkspace {
	count := s.config.WorkspaceCount
	if count < 1 {
		count = 9
	}
	focused := s.focusedView()
	result := make([]IPCWorkspace, 0, count)
	for number := 1; number <= count; number++ {
		windows := 0
		urgent := false
		for _, view := range s.views {
			if view.Mapped && view.Workspace == number {
				windows++
				urgent = urgent || view.Urgent
			}
		}
		result = append(result, IPCWorkspace{
			Number:  number,
			Active:  number == s.currentWorkspace,
			Focused: focused != nil && focused.Workspace == number,
			Urgent:  urgent,
			Windows: windows,
		})
	}
	return result
}

func (s *Server) ipcWindows() []IPCWindow {
	result := make([]IPCWindow, 0, len(s.views))
	for _, view := range s.views {
		if view.Mapped {
			result = append(result, s.ipcWindow(view))
		}
	}
	return result
}

func (s *Server) ipcWindow(view *View) IPCWindow {
	geometry := view.geometry()
	x := view.RootTree.Node().X()
	y := view.RootTree.Node().Y()
	width := geometry.Width
	height := geometry.Height
	if view.TileWidth > 0 && view.TileHeight > 0 {
		width = view.TileWidth
		height = view.TileHeight
	}
	border := s.viewBorderSize(view)
	if view.Managed && s.fullscreen != view && border > 0 {
		width += 2 * border
		height += 2 * border
	}
	return IPCWindow{
		ID:         view.ID,
		Title:      view.Title,
		AppID:      view.AppID,
		Class:      view.XWaylandClass,
		Instance:   view.XWaylandInstance,
		Output:     view.RuleActions.Output,
		Rules:      view.MatchedRules,
		Workspace:  view.Workspace,
		Mapped:     view.Mapped,
		Focused:    view == s.focusedView(),
		Urgent:     view.Urgent,
		Dialog:     view.Dialog,
		Modal:      view.Modal,
		Floating:   s.isFloatingView(view),
		Fullscreen: view == s.fullscreen,
		XWayland:   view.IsXWayland,
		X:          x,
		Y:          y,
		Width:      width,
		Height:     height,
	}
}

func (s *Server) emitIPCEvent(event string, data any) {
	if s.ipc == nil {
		return
	}
	message := IPCMessage{Type: "event", Event: event, Data: data, Timestamp: time.Now().UTC().Format(time.RFC3339Nano)}
	s.ipc.mu.Lock()
	clients := make([]*ipcClient, 0, len(s.ipc.clients))
	for client := range s.ipc.clients {
		clients = append(clients, client)
	}
	s.ipc.mu.Unlock()
	event = strings.ToLower(event)
	for _, client := range clients {
		client.mu.RLock()
		subscribed := client.subscriptions[event] || client.subscriptions["*"]
		client.mu.RUnlock()
		if subscribed {
			client.enqueue(message)
		}
	}
}
