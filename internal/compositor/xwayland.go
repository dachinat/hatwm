package compositor

/*
#cgo pkg-config: wlroots-0.18 wayland-server xcb xcb-shape
#cgo CFLAGS: -D_GNU_SOURCE -DWLR_USE_UNSTABLE
#include "xwayland.h"
*/
import "C"

import (
	"fmt"
	"log/slog"
	"unsafe"

	"github.com/swaywm/go-wlroots/wlroots"
)

func compositorPointer(compositor wlroots.Compositor) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&compositor))
}

func xcursorManagerPointer(manager wlroots.XCursorManager) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&manager))
}

func (s *Server) initXWayland() error {
	activeServer = s
	s.xwayland = C.hatwm_xwayland_create(
		(*C.struct_wl_display)(displayPointer(s.display)),
		(*C.struct_wlr_compositor)(compositorPointer(s.compositor)),
		C.bool(true),
	)
	if s.xwayland == nil {
		return fmt.Errorf(
			"failed to initialize XWayland; ensure wlroots has XWayland support")
	}
	return nil
}

func (s *Server) xwaylandDisplayName() string {
	if s.xwayland == nil {
		return ""
	}
	name := C.hatwm_xwayland_display_name(s.xwayland)
	if name == nil {
		return ""
	}
	return C.GoString(name)
}

func (s *Server) updateXWaylandCursor() {
	if s.xwayland == nil || xcursorManagerPointer(s.cursorMgr) == nil {
		return
	}
	name := C.CString("left_ptr")
	defer C.free(unsafe.Pointer(name))
	C.hatwm_xwayland_set_cursor(s.xwayland,
		(*C.struct_wlr_xcursor_manager)(xcursorManagerPointer(s.cursorMgr)),
		name, C.float(1))
}

func (s *Server) handleNewXWaylandSurface(
	ptr *C.struct_wlr_xwayland_surface) {
	if ptr == nil {
		return
	}
	managed := !bool(C.hatwm_xwayland_surface_override_redirect(ptr))
	root := s.normalTree.NewSceneTree()
	root.Node().SetEnabled(false)
	s.nextViewID++
	view := &View{
		ID:         s.nextViewID,
		XWayland:   unsafe.Pointer(ptr),
		IsXWayland: true,
		Managed:    managed,
		RootTree:   root,
		Server:     s,
		Workspace:  s.currentWorkspace,
	}
	s.views = append(s.views, view)
	view.refreshWindowIdentity()
}

func cString(value *C.char) string {
	if value == nil {
		return ""
	}
	return C.GoString(value)
}

func (v *View) refreshWindowIdentity() {
	if v == nil {
		return
	}
	if !v.IsXWayland {
		if v.TopLevel.Nil() {
			return
		}
		v.AppID = v.TopLevel.AppId()
		v.Title = v.TopLevel.Title()
		v.XWaylandClass = ""
		v.XWaylandInstance = ""
		return
	}
	ptr := (*C.struct_wlr_xwayland_surface)(v.XWayland)
	if ptr == nil {
		return
	}
	v.AppID = ""
	v.Title = cString(C.hatwm_xwayland_surface_title(ptr))
	v.XWaylandClass = cString(C.hatwm_xwayland_surface_class(ptr))
	v.XWaylandInstance = cString(C.hatwm_xwayland_surface_instance(ptr))
	v.Modal = bool(C.hatwm_xwayland_surface_modal(ptr))
	v.Dialog = v.Modal || bool(C.hatwm_xwayland_surface_has_parent(ptr))
}

func (s *Server) handleXWaylandAssociate(
	ptr *C.struct_wlr_xwayland_surface) {
	view := s.xwaylandView(ptr)
	if view == nil || view.Associated {
		return
	}
	surface := surfaceFromXWaylandPointer(ptr)
	if surface.Nil() {
		return
	}
	surfaceTree := view.RootTree.NewSceneTree()
	surfaceTree.NewSurface(surface)
	view.SurfaceTree = surfaceTree
	view.ClientSurface = surface
	view.Associated = true
	s.raiseDecoration(view)
	s.applyWindowOpacity(view)
}

func (s *Server) handleXWaylandMap(
	ptr *C.struct_wlr_xwayland_surface) {
	view := s.xwaylandView(ptr)
	if view == nil || !view.Associated {
		return
	}
	view.Mapped = true
	view.refreshWindowIdentity()
	s.applyWindowRules(view, true)
	view.Animation = ViewAnimation{}
	view.invalidateXWaylandRoundedClip()
	s.raiseDecoration(view)
	s.applyWindowOpacity(view)
	view.RootTree.Node().SetEnabled(
		view.Workspace == s.currentWorkspace)

	if !view.Managed {
		x := int(C.hatwm_xwayland_surface_x(ptr))
		y := int(C.hatwm_xwayland_surface_y(ptr))
		s.setViewPositionImmediate(view, float64(x), float64(y))
		view.RootTree.Node().RaiseToTop()
		if view.xwaylandWantsFocus() {
			s.focusView(view, &view.ClientSurface)
		}
		s.emitIPCEvent("window_opened", s.ipcWindow(view))
		s.emitIPCEvent("workspace_updated", s.ipcWorkspaces())
		return
	}

	s.moveViewFront(view)
	if view.ruleAllowsFocus() {
		s.requestViewActivation(view)
	}
	s.arrange()
	s.updateDecoration(view)
	s.emitIPCEvent("window_opened", s.ipcWindow(view))
	s.emitIPCEvent("workspace_updated", s.ipcWorkspaces())
}

func (s *Server) handleXWaylandUnmap(
	ptr *C.struct_wlr_xwayland_surface) {
	view := s.xwaylandView(ptr)
	if view == nil {
		return
	}
	s.unmapView(view)
	s.emitIPCEvent("window_closed", map[string]any{"id": view.ID, "workspace": view.Workspace})
	s.emitIPCEvent("workspace_updated", s.ipcWorkspaces())
}

func (s *Server) handleXWaylandDissociate(
	ptr *C.struct_wlr_xwayland_surface) {
	view := s.xwaylandView(ptr)
	if view == nil {
		return
	}
	if view.Mapped {
		s.unmapView(view)
	}
	if view.Associated {
		view.setXWaylandRoundedClip(0)
		view.SurfaceTree.Node().Destroy()
		view.SurfaceTree = wlroots.SceneTree{}
		view.ClientSurface = wlroots.Surface{}
		view.Associated = false
	}
}

func (s *Server) handleXWaylandCommit(
	ptr *C.struct_wlr_xwayland_surface) {
	view := s.xwaylandView(ptr)
	if view == nil || !view.Mapped {
		return
	}
	wasFloating, oldWorkspace := view.AutoFloating, view.Workspace
	view.refreshWindowIdentity()
	s.applyWindowRules(view, false)
	if wasFloating != view.AutoFloating || oldWorkspace != view.Workspace {
		s.arrange()
	}
	s.applyWindowOpacity(view)
	if !view.Managed {
		x := int(C.hatwm_xwayland_surface_x(ptr))
		y := int(C.hatwm_xwayland_surface_y(ptr))
		view.RootTree.Node().SetPosition(float64(x), float64(y))
		return
	}
	if s.isFloatingView(view) && s.fullscreen != view {
		s.rememberFloatingGeometry(view)
	}
	s.updateDecoration(view)
}

func (s *Server) handleXWaylandDestroy(
	ptr *C.struct_wlr_xwayland_surface) {
	view := s.xwaylandView(ptr)
	if view == nil {
		return
	}
	if view.Mapped {
		s.unmapView(view)
	}
	if s.grabbedView == view {
		s.cancelViewGrab()
	}
	s.destroyDecoration(view)
	s.removeView(view)
	view.RootTree.Node().Destroy()
	s.arrange()
}

func (s *Server) handleXWaylandRequestConfigure(
	ptr *C.struct_wlr_xwayland_surface,
	x, y int16, width, height uint16) {
	view := s.xwaylandView(ptr)
	if view == nil {
		return
	}

	// Before map, honor the initial geometry so the window has a usable size.
	// Floating and override-redirect windows continue to own their geometry.
	if !view.Mapped || !view.Managed || s.isFloatingView(view) {
		offset := view.surfaceOffset()
		if view.Mapped && view.Managed && s.isFloatingView(view) {
			target := clampFloatingGeometry(Geometry{
				X:     float64(int(x) - offset),
				Y:     float64(int(y) - offset),
				Width: uint32(width), Height: uint32(height),
			}, s.viewArea(view), s.viewBorderSize(view), 0, 0)
			s.setViewPositionImmediate(view, target.X, target.Y)
			view.setSize(target.Width, target.Height)
			view.Floating = target
			view.FloatingValid = true
		} else {
			s.setViewPositionImmediate(
				view, float64(int(x)-offset), float64(int(y)-offset))
			view.setSize(uint32(width), uint32(height))
		}
		return
	}

	// Tiled clients cannot move themselves. Re-send HatWM's geometry.
	rootX, rootY := view.targetRootPosition()
	geometry := view.geometry()
	view.configureXWayland(
		rootX, rootY, geometry.Width, geometry.Height)
}

func (s *Server) handleXWaylandRequestFullscreen(
	ptr *C.struct_wlr_xwayland_surface, fullscreen bool) {
	view := s.xwaylandView(ptr)
	if view == nil || !view.Managed {
		return
	}
	s.setViewFullscreen(view, fullscreen)
}

func (s *Server) handleXWaylandRequestMaximize(
	ptr *C.struct_wlr_xwayland_surface, maximized bool) {
	view := s.xwaylandView(ptr)
	if view == nil || !view.Managed {
		return
	}
	s.handleViewMaximizeRequest(view, maximized)
}

func (s *Server) handleXWaylandRequestActivate(
	ptr *C.struct_wlr_xwayland_surface) {
	view := s.xwaylandView(ptr)
	s.requestViewActivation(view)
}

func (s *Server) handleXWaylandOverrideRedirect(
	ptr *C.struct_wlr_xwayland_surface, overrideRedirect bool) {
	view := s.xwaylandView(ptr)
	if view == nil {
		return
	}
	view.Managed = !overrideRedirect
	if !view.Managed {
		s.destroyDecoration(view)
	}
	s.arrange()
}

func (s *Server) xwaylandView(
	ptr *C.struct_wlr_xwayland_surface) *View {
	target := unsafe.Pointer(ptr)
	for _, view := range s.views {
		if view.IsXWayland && view.XWayland == target {
			return view
		}
	}
	return nil
}

//export hatwmGoXWaylandNew
func hatwmGoXWaylandNew(surface unsafe.Pointer) {
	if activeServer != nil {
		activeServer.handleNewXWaylandSurface(
			(*C.struct_wlr_xwayland_surface)(surface))
	}
}

//export hatwmGoXWaylandAssociate
func hatwmGoXWaylandAssociate(surface unsafe.Pointer) {
	if activeServer != nil {
		activeServer.handleXWaylandAssociate(
			(*C.struct_wlr_xwayland_surface)(surface))
	}
}

//export hatwmGoXWaylandDissociate
func hatwmGoXWaylandDissociate(surface unsafe.Pointer) {
	if activeServer != nil {
		activeServer.handleXWaylandDissociate(
			(*C.struct_wlr_xwayland_surface)(surface))
	}
}

//export hatwmGoXWaylandMap
func hatwmGoXWaylandMap(surface unsafe.Pointer) {
	if activeServer != nil {
		activeServer.handleXWaylandMap(
			(*C.struct_wlr_xwayland_surface)(surface))
	}
}

//export hatwmGoXWaylandUnmap
func hatwmGoXWaylandUnmap(surface unsafe.Pointer) {
	if activeServer != nil {
		activeServer.handleXWaylandUnmap(
			(*C.struct_wlr_xwayland_surface)(surface))
	}
}

//export hatwmGoXWaylandCommit
func hatwmGoXWaylandCommit(surface unsafe.Pointer) {
	if activeServer != nil {
		activeServer.handleXWaylandCommit(
			(*C.struct_wlr_xwayland_surface)(surface))
	}
}

//export hatwmGoXWaylandDestroy
func hatwmGoXWaylandDestroy(surface unsafe.Pointer) {
	if activeServer != nil {
		activeServer.handleXWaylandDestroy(
			(*C.struct_wlr_xwayland_surface)(surface))
	}
}

//export hatwmGoXWaylandRequestConfigure
func hatwmGoXWaylandRequestConfigure(
	surface unsafe.Pointer,
	x C.int16_t,
	y C.int16_t,
	width C.uint16_t,
	height C.uint16_t) {
	if activeServer != nil {
		activeServer.handleXWaylandRequestConfigure(
			(*C.struct_wlr_xwayland_surface)(surface),
			int16(x), int16(y), uint16(width), uint16(height))
	}
}

//export hatwmGoXWaylandRequestMove
func hatwmGoXWaylandRequestMove(surface unsafe.Pointer) {
	if activeServer == nil {
		return
	}
	view := activeServer.xwaylandView(
		(*C.struct_wlr_xwayland_surface)(surface))
	activeServer.beginInteractive(view, CursorMove, 0)
}

//export hatwmGoXWaylandRequestResize
func hatwmGoXWaylandRequestResize(
	surface unsafe.Pointer, edges C.uint32_t) {
	if activeServer == nil {
		return
	}
	view := activeServer.xwaylandView(
		(*C.struct_wlr_xwayland_surface)(surface))
	activeServer.beginInteractive(
		view, CursorResize, wlroots.Edges(edges))
}

//export hatwmGoXWaylandRequestFullscreen
func hatwmGoXWaylandRequestFullscreen(
	surface unsafe.Pointer, fullscreen C.bool) {
	if activeServer != nil {
		activeServer.handleXWaylandRequestFullscreen(
			(*C.struct_wlr_xwayland_surface)(surface),
			bool(fullscreen))
	}
}

//export hatwmGoXWaylandRequestMaximize
func hatwmGoXWaylandRequestMaximize(
	surface unsafe.Pointer, maximized C.bool) {
	if activeServer != nil {
		activeServer.handleXWaylandRequestMaximize(
			(*C.struct_wlr_xwayland_surface)(surface),
			bool(maximized))
	}
}

//export hatwmGoXWaylandMetadataChanged
func hatwmGoXWaylandMetadataChanged(surface unsafe.Pointer) {
	if activeServer == nil {
		return
	}
	v := activeServer.xwaylandView(
		(*C.struct_wlr_xwayland_surface)(surface))
	if v == nil {
		return
	}
	wasFloating, oldWorkspace := v.AutoFloating, v.Workspace
	v.refreshWindowIdentity()
	activeServer.applyWindowRules(v, false)
	if v.Mapped && (wasFloating != v.AutoFloating || oldWorkspace != v.Workspace) {
		activeServer.arrange()
		activeServer.emitIPCEvent("window_updated", activeServer.ipcWindow(v))
	}
}

//export hatwmGoXWaylandRequestActivate
func hatwmGoXWaylandRequestActivate(surface unsafe.Pointer) {
	if activeServer != nil {
		activeServer.handleXWaylandRequestActivate(
			(*C.struct_wlr_xwayland_surface)(surface))
	}
}

//export hatwmGoXWaylandOverrideRedirect
func hatwmGoXWaylandOverrideRedirect(
	surface unsafe.Pointer, value C.bool) {
	if activeServer != nil {
		activeServer.handleXWaylandOverrideRedirect(
			(*C.struct_wlr_xwayland_surface)(surface), bool(value))
	}
}

func logXWaylandDisplay(display string) {
	if display != "" {
		slog.Info("XWayland ready", "DISPLAY", display)
	}
}
