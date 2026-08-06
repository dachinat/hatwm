package compositor

import (
	"unsafe"

	"github.com/swaywm/go-wlroots/wlroots"
)

type CursorMode int

const (
	CursorPassthrough CursorMode = iota
	CursorMove
	CursorResize
	CursorTilingMove
	CursorTilingResize
)

type Geometry struct {
	X, Y          float64
	Width, Height uint32
}

// View is the single owner record for either an XDG toplevel or an XWayland
// window. We never key views by wrapper addresses and never retain pointers to
// temporary Go wrapper copies.
type View struct {
	ID            uint64
	TopLevel      wlroots.XDGTopLevel
	XWayland      unsafe.Pointer
	IsXWayland    bool
	Managed       bool
	Associated    bool
	ClientSurface wlroots.Surface

	RootTree    wlroots.SceneTree
	SurfaceTree wlroots.SceneTree
	Decor       WindowDecoration
	TileWidth   int
	TileHeight  int

	Mapped           bool
	Urgent           bool
	Dialog           bool
	Modal            bool
	AutoFloating     bool
	AppID            string
	Title            string
	XWaylandClass    string
	XWaylandInstance string
	RuleActions      WindowRuleActions
	MatchedRules     string
	Workspace        int
	Saved            Geometry
	Floating         Geometry
	FloatingValid    bool
	Animation        ViewAnimation

	Server           *Server
	MaximizeToken    uintptr
	MaximizeListener unsafe.Pointer
	ForeignToplevel  unsafe.Pointer

	XClipRadius int
	XClipWidth  int
	XClipHeight int
}

type Keyboard struct {
	Device wlroots.InputDevice
}
