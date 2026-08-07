package compositor

/*
#cgo pkg-config: wlroots-0.18 wayland-server
#cgo CFLAGS: -D_GNU_SOURCE -DWLR_USE_UNSTABLE
#include <stdlib.h>
#include "layer_shell.h"
#include "clipboard.h"
#include "keyboard_layout.h"
#include "desktop_protocols.h"
#include "foreign_toplevel.h"
#include "input_protocols.h"
#include "output_protocols.h"
#include "screencast_portal.h"
#include "session_lock.h"
#include "workspace.h"
#include "xwayland.h"
#include "xdg_activation.h"
#include "xdg_dialog.h"
*/
import "C"

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/swaywm/go-wlroots/wlroots"
)

type Server struct {
	display        wlroots.Display
	backend        wlroots.Backend
	renderer       wlroots.Renderer
	allocator      wlroots.Allocator
	compositor     wlroots.Compositor
	scene          wlroots.Scene
	sceneLayout    wlroots.SceneOutputLayout
	backgroundTree wlroots.SceneTree
	bottomTree     wlroots.SceneTree
	normalTree     wlroots.SceneTree
	tiledTree      wlroots.SceneTree
	floatingTree   wlroots.SceneTree
	topTree        wlroots.SceneTree
	overlayTree    wlroots.SceneTree
	lockTree       wlroots.SceneTree

	xdgShell      wlroots.XDGShell
	xdgActivation *C.struct_hatwm_xdg_activation
	xdgDialog     *C.struct_hatwm_xdg_dialog_manager
	views         []*View

	layerShell       *C.struct_hatwm_layer_shell
	xdgOutputManager *C.struct_wlr_xdg_output_manager_v1
	clipboard        *C.struct_hatwm_clipboard
	desktopProtocols *C.struct_hatwm_desktop_protocols
	inputProtocols   *C.struct_hatwm_input_protocols
	outputProtocols  *C.struct_hatwm_output_protocols
	foreignToplevels *C.struct_hatwm_foreign_toplevels
	screencastPortal *C.struct_hatwm_screencast_portal
	layerSurfaces    []*LayerSurface

	sessionLockManager *C.struct_hatwm_session_lock_manager
	sessionLock        *C.struct_wlr_session_lock_v1
	lockBackground     *C.struct_wlr_scene_rect
	lockSurfaces       []*SessionLockSurface
	sessionLocked      bool
	xwayland           *C.struct_hatwm_xwayland

	cursor              wlroots.Cursor
	cursorMgr           wlroots.XCursorManager
	seat                wlroots.Seat
	keyboards           []*Keyboard
	inputDevices        []wlroots.InputDevice
	keyboardModifiers   wlroots.KeyboardModifier
	keyboardLayoutIndex int
	vtSwitchKeyCode     uint32
	vtSwitchKeyActive   bool

	cursorMode        CursorMode
	grabbedView       *View
	grabX, grabY      float64
	grabBox           wlroots.GeoBox
	resizeEdges       wlroots.Edges
	cursorButtonCount uint32
	grabOwnsButton    bool
	grabMasterRatio   float64
	grabStackRatio    float64
	grabGrid          tileGridResizeGrab

	outputLayout   wlroots.OutputLayout
	outputs        []*OutputState
	activeOutput   *OutputState
	fallbackOutput OutputState

	config              Config
	running             bool
	shuttingDown        bool
	wallpaperCmd        *exec.Cmd
	wallpaperPath       string
	notificationCmd     *exec.Cmd
	configModTime       time.Time
	lastScreencastFrame time.Time
	shutdownRequested   atomic.Bool
	reloadRequested     atomic.Bool

	ipc        *IPCServer
	nextViewID uint64
}

func seatPointer(seat wlroots.Seat) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&seat))
}

func NewServer() (*Server, error) {
	s := &Server{running: true}
	s.fallbackOutput.CurrentWorkspace = 1
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	if err := validateKeyboardLayouts(cfg); err != nil {
		return nil, fmt.Errorf("invalid keyboard configuration: %w", err)
	}
	s.config = cfg

	s.display = wlroots.NewDisplay()
	s.backend, err = s.display.BackendAutocreate()
	if err != nil {
		return nil, err
	}
	s.renderer, err = s.backend.RendererAutoCreate()
	if err != nil {
		return nil, err
	}
	s.renderer.InitDisplay(s.display)
	s.allocator, err = s.backend.AllocatorAutocreate(s.renderer)
	if err != nil {
		return nil, err
	}

	s.compositor = s.display.CompositorCreate(5, s.renderer)
	s.display.SubCompositorCreate()
	s.display.DataDeviceManagerCreate()

	s.outputLayout = wlroots.NewOutputLayout(s.display)
	s.xdgOutputManager = C.hatwm_xdg_output_manager_create(
		(*C.struct_wl_display)(displayPointer(s.display)),
		(*C.struct_wlr_output_layout)(outputLayoutPointer(s.outputLayout)),
	)
	if s.xdgOutputManager == nil {
		return nil, fmt.Errorf("failed to create zxdg_output_manager_v1 global")
	}
	s.backend.OnNewOutput(s.handleNewOutput)

	s.scene = wlroots.NewScene()
	s.sceneLayout = s.scene.AttachOutputLayout(s.outputLayout)
	s.backgroundTree = s.scene.Tree().NewSceneTree()
	s.bottomTree = s.scene.Tree().NewSceneTree()
	s.normalTree = s.scene.Tree().NewSceneTree()
	// Managed windows live in explicit sibling layers. Raising a tiled window
	// can no longer cover floating windows or dialogs because RaiseToTop is
	// constrained to that window's layer.
	s.tiledTree = s.normalTree.NewSceneTree()
	s.floatingTree = s.normalTree.NewSceneTree()
	s.topTree = s.scene.Tree().NewSceneTree()
	s.overlayTree = s.scene.Tree().NewSceneTree()
	// This is deliberately created last: lock surfaces must remain above every
	// normal and layer-shell client, including overlay panels.
	s.lockTree = s.scene.Tree().NewSceneTree()
	s.lockTree.Node().SetEnabled(false)
	if os.Getenv("HATWM_DISABLE_XWAYLAND") != "1" {
		if err := s.initXWayland(); err != nil {
			slog.Warn("XWayland unavailable; continuing with native Wayland support", "error", err)
		}
	}

	// xdg_toplevel.wm_capabilities was added in xdg-shell v5. Advertise v6,
	// which wlroots 0.18 supports, so clients can hide unsupported controls
	// such as minimize. Older clients continue to negotiate lower versions.
	s.xdgShell = s.display.XDGShellCreate(6)
	// go-wlroots installs per-XDG-surface listener cleanup from OnNewSurface.
	// Keep this subscription even though HatWM handles role-specific setup via
	// OnNewTopLevel below. Without it, a dismissed popup leaves
	// its commit listener registered against freed memory and a later popup can
	// crash when wlroots reuses the same allocation.
	s.xdgShell.OnNewSurface(func(wlroots.XDGSurface) {})
	s.xdgShell.OnNewTopLevel(s.handleNewXDGTopLevel)
	s.xdgShell.OnNewPopup(s.handleNewXDGPopup)
	if err := s.initXDGDialog(); err != nil {
		return nil, err
	}
	s.xdgActivation = C.hatwm_xdg_activation_create(
		(*C.struct_wl_display)(displayPointer(s.display)))
	if s.xdgActivation == nil {
		return nil, fmt.Errorf("failed to initialize xdg-activation")
	}
	if err := s.initLayerShell(); err != nil {
		return nil, err
	}
	if err := s.initSessionLock(); err != nil {
		return nil, err
	}

	s.cursor = wlroots.NewCursor()
	s.cursor.AttachOutputLayout(s.outputLayout)
	if err := s.configureCursorTheme(cfg.CursorTheme, cfg.CursorSize); err != nil {
		return nil, err
	}
	s.cursor.OnMotion(s.handleCursorMotion)
	s.cursor.OnMotionAbsolute(s.handleCursorMotionAbsolute)
	s.cursor.OnButton(s.handleCursorButton)
	s.cursor.OnAxis(s.handleCursorAxis)
	s.cursor.OnFrame(s.handleCursorFrame)

	initScreencopy(s.display, s.outputLayout)
	if !initViewporter(s.display) {
		return nil, fmt.Errorf("failed to create wp_viewporter global")
	}

	s.backend.OnNewInput(s.handleNewInput)
	s.seat = s.display.SeatCreate("seat0")
	s.seat.OnSetCursorRequest(s.handleSetCursorRequest)
	C.hatwm_xwayland_set_seat(
		s.xwayland,
		(*C.struct_wlr_seat)(seatPointer(s.seat)),
	)
	s.clipboard = C.hatwm_clipboard_create((*C.struct_wlr_seat)(seatPointer(s.seat)))
	if s.clipboard == nil {
		return nil, fmt.Errorf("failed to initialize clipboard selection handling")
	}
	if err := s.initDesktopProtocols(); err != nil {
		return nil, err
	}
	if err := s.initInputProtocols(); err != nil {
		return nil, err
	}
	if err := s.initOutputProtocols(); err != nil {
		return nil, err
	}
	if err := s.initForeignToplevels(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Server) Start(extraStartup string) error {
	socket, err := s.display.AddSocketAuto()
	if err != nil {
		return err
	}
	if err := s.backend.Start(); err != nil {
		return err
	}
	if err := os.Setenv("WAYLAND_DISPLAY", socket); err != nil {
		return err
	}
	xDisplay := s.xwaylandDisplayName()
	if xDisplay != "" {
		if err := os.Setenv("DISPLAY", xDisplay); err != nil {
			return err
		}
	} else {
		_ = os.Unsetenv("DISPLAY")
	}
	// These variables help desktop components select their Wayland backends and
	// identify the current session when HatWM is launched outside a display manager.
	_ = os.Setenv("XDG_CURRENT_DESKTOP", "HatWM")
	_ = os.Setenv("XDG_SESSION_DESKTOP", "HatWM")
	_ = os.Setenv("XDG_SESSION_TYPE", "wayland")
	s.applyAppearanceProfile()
	slog.Info("HatWM started", "WAYLAND_DISPLAY", socket)
	logXWaylandDisplay(xDisplay)
	if err := s.startScreenCastPortal(); err != nil {
		slog.Warn("ScreenCast portal unavailable; continuing without screen sharing",
			"error", err)
	}
	if err := s.startIPC(); err != nil {
		return fmt.Errorf("start IPC: %w", err)
	}
	s.startWallpaper()
	s.startNotificationDaemon()
	for _, cmd := range s.config.Autostart {
		s.spawn(cmd)
	}
	if extraStartup != "" {
		s.spawn(extraStartup)
	}
	if path := GetConfigPath(); path != "" {
		if st, err := os.Stat(path); err == nil {
			s.configModTime = st.ModTime()
		}
	}
	return nil
}

func (s *Server) Run() error {
	loop := s.display.EventLoop()
	var lastConfigCheck time.Time
	for s.running {
		if s.shutdownRequested.Load() {
			s.running = false
			break
		}
		s.display.FlushClients()
		loop.Dispatch(10 * time.Millisecond)
		wlroots.DrainRetiredListeners()
		s.processIPCRequests()
		if s.reloadRequested.Swap(false) {
			if err := s.reloadConfig(); err != nil {
				slog.Error("SIGHUP config reload failed", "error", err)
			} else {
				slog.Info("config reloaded after SIGHUP")
				s.emitIPCEvent("config_reloaded", s.ipcState())
			}
		}
		s.tickAnimations(time.Now())
		s.tickScreenCastPortal(time.Now())
		if time.Since(lastConfigCheck) >= time.Second {
			lastConfigCheck = time.Now()
			s.reloadConfigIfChanged()
		}
	}
	if s.ipc != nil {
		s.emitIPCEvent("shutdown", map[string]any{"reason": "compositor_stopped"})
		s.ipc.Close()
		s.ipc = nil
	}
	s.shuttingDown = true
	s.stopNotificationDaemon()
	s.stopWallpaper()
	s.stopScreenCastPortal()
	s.display.DestroyClients()
	if s.xwayland != nil {
		C.hatwm_xwayland_destroy(s.xwayland)
		s.xwayland = nil
	}
	if s.xdgActivation != nil {
		C.hatwm_xdg_activation_destroy(s.xdgActivation)
		s.xdgActivation = nil
	}
	if s.xdgDialog != nil {
		C.hatwm_xdg_dialog_manager_destroy(s.xdgDialog)
		s.xdgDialog = nil
	}
	if s.clipboard != nil {
		C.hatwm_clipboard_destroy(s.clipboard)
		s.clipboard = nil
	}
	if s.foreignToplevels != nil {
		C.hatwm_foreign_toplevels_destroy(s.foreignToplevels)
		s.foreignToplevels = nil
	}
	if s.outputProtocols != nil {
		C.hatwm_output_protocols_destroy(s.outputProtocols)
		s.outputProtocols = nil
	}
	if s.inputProtocols != nil {
		C.hatwm_input_protocols_destroy(s.inputProtocols)
		s.inputProtocols = nil
	}
	if s.desktopProtocols != nil {
		C.hatwm_desktop_protocols_destroy(s.desktopProtocols)
		s.desktopProtocols = nil
	}
	if s.sessionLockManager != nil {
		C.hatwm_session_lock_manager_destroy(s.sessionLockManager)
		s.sessionLockManager = nil
	}
	if s.layerShell != nil {
		C.hatwm_layer_shell_destroy(s.layerShell)
		s.layerShell = nil
	}
	// Native callbacks must not retain a Go server after all callback-owning
	// bridges have been destroyed.
	activeServer = nil
	s.scene.Tree().Node().Destroy()
	s.cursorMgr.Destroy()
	s.outputLayout.Destroy()
	s.display.Destroy()
	wlroots.DrainRetiredListeners()
	return nil
}

func (s *Server) RequestShutdown() { s.shutdownRequested.Store(true) }

func (s *Server) RequestReload() { s.reloadRequested.Store(true) }

func (s *Server) handleNewOutput(output wlroots.Output) {
	if !output.InitRender(s.allocator, s.renderer) {
		slog.Error("could not initialize output rendering; leaving output disabled",
			"output", output.Name())
		return
	}
	state := wlroots.NewOutputState()
	state.StateInit()
	state.StateSetEnabled(true)
	if mode, err := output.PreferredMode(); err == nil {
		state.SetMode(mode)
	} else {
		slog.Warn("output has no preferred mode; trying backend default",
			"output", output.Name(), "error", err)
	}
	committed := output.TestState(state) && output.CommitState(state)
	state.Finish()
	if !committed {
		slog.Error("output rejected its initial configuration; leaving output disabled",
			"output", output.Name())
		return
	}
	output.OnFrame(s.handleOutputFrame)
	output.OnRequestState(func(o wlroots.Output, st wlroots.OutputState) {
		if !o.TestState(st) || !o.CommitState(st) {
			slog.Warn("output configuration request rejected", "output", o.Name())
			return
		}
		s.refreshOutputGeometry(s.outputStateFor(o))
		s.arrangeLayers()
	})
	output.OnDestroy(s.handleOutputDestroy)
	lo := s.outputLayout.AddOutputAuto(output)
	so := s.scene.NewOutput(output)
	s.sceneLayout.AddOutput(lo, so)
	if err := output.SetTitle(fmt.Sprintf("HatWM - %s", output.Name())); err != nil {
		slog.Warn("could not set output title", "output", output.Name(), "error", err)
	}
	outputState := &OutputState{
		Output: output, LayoutOutput: lo, SceneOutput: so, CurrentWorkspace: 1,
	}
	s.outputs = append(s.outputs, outputState)
	s.refreshOutputGeometry(outputState)
	if s.activeOutput == nil {
		s.activeOutput = outputState
	}
	for _, view := range s.views {
		if view.Output == nil || view.Output == &s.fallbackOutput {
			view.Output = outputState
		}
	}
	for _, layer := range s.layerSurfaces {
		if layer.output == nil || layer.output == &s.fallbackOutput {
			layer.output = outputState
			if C.hatwm_layer_surface_output(layer.ptr) == nil {
				C.hatwm_layer_surface_set_output(layer.ptr,
					(*C.struct_wlr_output)(outputPointer(output)))
			}
		}
	}
	if s.screencastPortal != nil {
		C.hatwm_screencast_portal_add_output(s.screencastPortal,
			(*C.struct_wlr_output)(outputPointer(output)))
	}
	s.outputProtocolsAdd(output)
	s.arrangeLayers()
	s.updateSessionLockBackground()
}

func (s *Server) handleOutputFrame(output wlroots.Output) {
	state := s.outputStateFor(output)
	if state == nil {
		return
	}
	s.renderOutput(output, state.SceneOutput)
	state.SceneOutput.SendFrameDone(time.Now())
}

func (s *Server) handleOutputDestroy(output wlroots.Output) {
	if s.shuttingDown {
		for i, candidate := range s.outputs {
			if candidate.Output == output {
				s.outputs = append(s.outputs[:i], s.outputs[i+1:]...)
				break
			}
		}
		return
	}
	if s.screencastPortal != nil {
		C.hatwm_screencast_portal_remove_output(s.screencastPortal,
			(*C.struct_wlr_output)(outputPointer(output)))
	}
	s.outputProtocolsRemove(output)
	removed := s.outputStateFor(output)
	for i, candidate := range s.outputs {
		if candidate == removed {
			s.outputs = append(s.outputs[:i], s.outputs[i+1:]...)
			break
		}
	}
	var fallback *OutputState
	if len(s.outputs) > 0 {
		fallback = s.outputs[0]
	}
	if removed != nil {
		if removed.Fullscreen != nil {
			setClientPresentationState(removed.Fullscreen, presentationNone)
			removed.Fullscreen = nil
			removed.FullscreenMode = presentationNone
		}
		for _, view := range s.views {
			if view.Output == removed {
				view.Output = fallback
				view.FloatingValid = false
			}
		}
		for _, layer := range s.layerSurfaces {
			if layer.output == removed {
				layer.output = fallback
				if fallback != nil && C.hatwm_layer_surface_output(layer.ptr) == nil {
					C.hatwm_layer_surface_set_output(layer.ptr,
						(*C.struct_wlr_output)(outputPointer(fallback.Output)))
				}
			}
		}
	}
	if s.activeOutput == removed {
		s.activeOutput = fallback
	}
	if fallback != nil && fallback.Focused == nil {
		fallback.Focused = s.focusedViewForOutput(fallback)
	}
	if fallback == nil {
		slog.Warn("all outputs removed; HatWM will wait for an output to reconnect")
	}
	s.arrangeLayers()
	s.updateSessionLockBackground()
}

func (s *Server) refreshOutputGeometry(state *OutputState) {
	if state == nil {
		return
	}
	x, y := s.outputLayout.Coords(state.Output)
	width, height := state.Output.EffectiveResolution()
	state.Full = usableBox{x: int(x), y: int(y), width: width, height: height}
	if state.Usable.width <= 0 || state.Usable.height <= 0 {
		state.Usable = state.Full
	}
}

func (s *Server) spawn(command string) {
	if command == "" {
		return
	}
	cmd := exec.Command("/bin/sh", "-c", command)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		slog.Error("failed to start command", "command", command, "error", err)
		return
	}
	go func() { _ = cmd.Wait() }()
}

func (s *Server) startNotificationDaemon() {
	s.stopNotificationDaemon()
	if !s.config.Notifications {
		return
	}
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		slog.Warn("notifications need a D-Bus session bus; start HatWM from a display manager or with dbus-run-session")
	}

	command := s.config.NotificationDaemon
	if command == "" || command == "auto" {
		for _, candidate := range []string{"mako", "swaync", "dunst"} {
			if _, err := exec.LookPath(candidate); err == nil {
				command = candidate
				break
			}
		}
	}
	if command == "none" {
		return
	}
	if command == "" || command == "auto" {
		slog.Warn("notifications enabled but no notification daemon was found", "tried", "mako, swaync, dunst")
		return
	}

	s.notificationCmd = exec.Command("/bin/sh", "-c", command)
	s.notificationCmd.Stdout = os.Stdout
	s.notificationCmd.Stderr = os.Stderr
	if err := s.notificationCmd.Start(); err != nil {
		slog.Warn("failed to start notification daemon", "command", command, "error", err)
		s.notificationCmd = nil
		return
	}
	slog.Info("notification daemon started", "command", command)
	cmd := s.notificationCmd
	go func() {
		err := cmd.Wait()
		if err != nil {
			slog.Warn("notification daemon exited", "command", command, "error", err)
		}
	}()
}

func (s *Server) stopNotificationDaemon() {
	if s.notificationCmd != nil && s.notificationCmd.Process != nil {
		_ = s.notificationCmd.Process.Kill()
		s.notificationCmd = nil
	}
}

func (s *Server) reloadConfigIfChanged() {
	path := GetConfigPath()
	if path == "" {
		return
	}
	st, err := os.Stat(path)
	if err != nil || !st.ModTime().After(s.configModTime) {
		return
	}
	if err := s.reloadConfig(); err != nil {
		slog.Error("config reload failed", "error", err)
		return
	}
	slog.Info("config reloaded")
}

func (s *Server) reloadConfig() error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	if err := s.applyConfig(cfg); err != nil {
		return err
	}
	if path := GetConfigPath(); path != "" {
		if st, err := os.Stat(path); err == nil {
			s.configModTime = st.ModTime()
		}
	}
	return nil
}

// applyConfig prepares every fallible resource before publishing the new
// configuration. This keeps a failed reload from leaving s.config half-applied.
func (s *Server) applyConfig(cfg Config) error {
	oldWallpaper := s.config.Wallpaper
	oldNotifications := s.config.Notifications
	oldNotificationDaemon := s.config.NotificationDaemon
	oldKeyboardLayouts := strings.Join(s.config.KeyboardLayouts, ",")
	oldKeyboardVariants := s.config.KeyboardVariants
	oldKeyboardOptions := s.config.KeyboardOptions
	oldCursorTheme := s.config.CursorTheme
	oldCursorSize := s.config.CursorSize
	oldConfig := s.config
	keyboardChanged := oldKeyboardLayouts != strings.Join(cfg.KeyboardLayouts, ",") ||
		oldKeyboardVariants != cfg.KeyboardVariants || oldKeyboardOptions != cfg.KeyboardOptions
	if keyboardChanged {
		if err := validateKeyboardLayouts(cfg); err != nil {
			return err
		}
	}
	inputChanged := inputConfigChanged(oldConfig, cfg)
	var cursorManager wlroots.XCursorManager
	if oldCursorTheme != cfg.CursorTheme || oldCursorSize != cfg.CursorSize {
		var err error
		cursorManager, err = createCursorManager(cfg.CursorTheme, cfg.CursorSize)
		if err != nil {
			return fmt.Errorf("load cursor theme: %w", err)
		}
	}
	s.config = cfg
	if inputChanged {
		for _, device := range s.inputDevices {
			s.configureInputDevice(device)
			if device.Type() == wlroots.InputDeviceTypeKeyboard {
				device.Keyboard().SetRepeatInfo(
					int32(cfg.KeyboardRepeatRate), int32(cfg.KeyboardRepeatDelay))
			}
		}
	}
	if xcursorManagerPointer(cursorManager) != nil {
		s.installCursorManager(cursorManager)
	}
	s.reapplyWindowRules()
	s.applyWindowOpacityToAll()
	if oldWallpaper != cfg.Wallpaper {
		s.startWallpaper()
	}
	if oldNotifications != cfg.Notifications || oldNotificationDaemon != cfg.NotificationDaemon {
		s.startNotificationDaemon()
	}
	if keyboardChanged {
		s.keyboardLayoutIndex = 0
		for _, keyboard := range s.keyboards {
			s.configureKeyboardLayouts(keyboard.Device.Keyboard())
		}
	}
	if appearanceChanged(oldConfig, s.config) {
		s.applyAppearanceProfile()
		slog.Info("appearance profile updated", "profile", s.appearanceDescription())
	}
	s.updateAllDecorations()
	s.arrange()
	return nil
}
