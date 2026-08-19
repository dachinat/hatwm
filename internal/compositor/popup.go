package compositor

/*
#cgo pkg-config: wlroots-0.18 wayland-server
#cgo CFLAGS: -D_GNU_SOURCE -DWLR_USE_UNSTABLE
#include <wlr/types/wlr_scene.h>
#include <wlr/types/wlr_xdg_shell.h>
#include "popup_listener.h"
*/
import "C"

import (
	"sync"
	"unsafe"

	"github.com/swaywm/go-wlroots/wlroots"
)

type popupConstraintTarget struct {
	server     *Server
	popup      wlroots.XDGPopup
	owner      *View
	layerOwner *LayerSurface
}

var popupConstraintRegistry = struct {
	sync.RWMutex
	next    uintptr
	targets map[uintptr]popupConstraintTarget
}{
	next:    1,
	targets: make(map[uintptr]popupConstraintTarget),
}

func xdgPopupPointer(popup wlroots.XDGPopup) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&popup))
}

func sceneTreeLayoutCoords(tree wlroots.SceneTree) (int, int, bool) {
	ptr := (*C.struct_wlr_scene_tree)(sceneTreePointer(tree))
	if ptr == nil {
		return 0, 0, false
	}
	var x, y C.int
	enabled := bool(C.wlr_scene_node_coords(&ptr.node, &x, &y))
	return int(x), int(y), enabled
}

// popupRootParentSurface follows xdg_popup parent links (and any subsurface
// parent links along the way) until it reaches the root desktop surface. A
// nested popup's wlr_surface root is the popup itself, so RootSurface alone is
// not enough to find the owning toplevel.
func popupRootParentSurface(popup wlroots.XDGPopup) wlroots.Surface {
	parent := popup.Parent()
	for !parent.Nil() {
		root := parent.RootSurface()
		if root.Nil() {
			return wlroots.Surface{}
		}
		parent = root
		if parent.Type() != wlroots.SurfaceTypeXDG {
			return parent
		}

		xdg := parent.XDGSurface()
		switch xdg.Role() {
		case wlroots.XDGSurfaceRoleTopLevel:
			return parent
		case wlroots.XDGSurfaceRolePopup:
			parent = xdg.Popup().Parent()
		default:
			return parent
		}
	}
	return wlroots.Surface{}
}

func (s *Server) popupOwner(popup wlroots.XDGPopup) *View {
	root := popupRootParentSurface(popup)
	if root.Nil() || root.Type() != wlroots.SurfaceTypeXDG ||
		root.XDGSurface().Role() != wlroots.XDGSurfaceRoleTopLevel {
		return nil
	}
	for _, view := range s.views {
		if view.Associated && !view.IsXWayland && view.clientSurface() == root {
			return view
		}
	}
	return nil
}

func (s *Server) popupLayerOwner(popup wlroots.XDGPopup) *LayerSurface {
	root := popupRootParentSurface(popup)
	if root.Nil() {
		return nil
	}
	return s.layerSurfaceForSurface(root)
}

// popupConstraintBox converts an output box from layout coordinates into the
// root toplevel surface coordinate system required by
// wlr_xdg_popup_unconstrain_from_box. sceneX/sceneY are the layout coordinates
// of the scene_xdg_surface tree origin, which corresponds to the top-left of
// the xdg window geometry. geometryX/geometryY convert that origin back to the
// actual wl_surface coordinate origin.
func popupConstraintBox(output usableBox, sceneX, sceneY, geometryX, geometryY int) usableBox {
	return usableBox{
		x:      output.x - sceneX + geometryX,
		y:      output.y - sceneY + geometryY,
		width:  output.width,
		height: output.height,
	}
}

func layerPopupConstraintBox(output usableBox, layerX, layerY int) usableBox {
	return popupConstraintBox(output, layerX, layerY, 0, 0)
}

func unconstrainXDGPopupFromBox(popup wlroots.XDGPopup, box usableBox) {
	ptr := (*C.struct_wlr_xdg_popup)(xdgPopupPointer(popup))
	if ptr == nil || box.width <= 0 || box.height <= 0 {
		return
	}
	cbox := C.struct_wlr_box{
		x:      C.int(box.x),
		y:      C.int(box.y),
		width:  C.int(box.width),
		height: C.int(box.height),
	}
	C.wlr_xdg_popup_unconstrain_from_box(ptr, &cbox)
}

func (s *Server) unconstrainXDGPopup(popup wlroots.XDGPopup, owner *View) {
	if owner == nil || owner.IsXWayland || owner.TopLevel.Nil() {
		return
	}
	output := s.ensureViewOutput(owner)
	if output == nil || output.Full.width <= 0 || output.Full.height <= 0 {
		return
	}

	sceneX, sceneY, _ := sceneTreeLayoutCoords(owner.SurfaceTree)
	geometry := owner.TopLevel.Base().Geometry()
	box := popupConstraintBox(output.Full, sceneX, sceneY, geometry.X, geometry.Y)
	unconstrainXDGPopupFromBox(popup, box)
}

func (s *Server) unconstrainLayerXDGPopup(popup wlroots.XDGPopup, layer *LayerSurface) {
	if layer == nil || layer.output == nil || layer.scene == nil {
		return
	}
	box := layerPopupConstraintBox(layer.output.Full, layer.lastX, layer.lastY)
	unconstrainXDGPopupFromBox(popup, box)
}

func (s *Server) listenForPopupReposition(
	popup wlroots.XDGPopup, owner *View, layerOwner *LayerSurface,
) {
	if owner == nil && layerOwner == nil {
		return
	}

	popupConstraintRegistry.Lock()
	token := popupConstraintRegistry.next
	popupConstraintRegistry.next++
	popupConstraintRegistry.targets[token] = popupConstraintTarget{
		server: s, popup: popup, owner: owner, layerOwner: layerOwner,
	}
	popupConstraintRegistry.Unlock()

	listener := C.hatwm_xdg_popup_listen(
		(*C.struct_wlr_xdg_popup)(xdgPopupPointer(popup)), C.uintptr_t(token))
	if listener == nil {
		popupConstraintRegistry.Lock()
		delete(popupConstraintRegistry.targets, token)
		popupConstraintRegistry.Unlock()
	}
}

func unconstrainPopupTarget(target popupConstraintTarget) {
	if target.server == nil {
		return
	}
	if target.owner != nil {
		target.server.unconstrainXDGPopup(target.popup, target.owner)
	} else if target.layerOwner != nil {
		target.server.unconstrainLayerXDGPopup(target.popup, target.layerOwner)
	}
}

//export hatwmGoPopupReposition
func hatwmGoPopupReposition(token C.uintptr_t) {
	popupConstraintRegistry.RLock()
	target, ok := popupConstraintRegistry.targets[uintptr(token)]
	popupConstraintRegistry.RUnlock()
	if ok {
		unconstrainPopupTarget(target)
	}
}

//export hatwmGoPopupListenerDestroy
func hatwmGoPopupListenerDestroy(token C.uintptr_t) {
	popupConstraintRegistry.Lock()
	delete(popupConstraintRegistry.targets, uintptr(token))
	popupConstraintRegistry.Unlock()
}
