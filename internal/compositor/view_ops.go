package compositor

/*
#cgo pkg-config: wlroots-0.18 wayland-server
#cgo CFLAGS: -D_GNU_SOURCE -DWLR_USE_UNSTABLE
#include "xwayland.h"
#include <wlr/types/wlr_scene.h>
#include <wlr/types/wlr_xdg_shell.h>

static inline void hatwm_scene_tree_clip(
        struct wlr_scene_tree *tree, int x, int y, int width, int height) {
    if (tree == NULL) {
        return;
    }
    if (width <= 0 || height <= 0) {
        wlr_scene_subsurface_tree_set_clip(&tree->node, NULL);
        return;
    }
    struct wlr_box clip = { .x = x, .y = y, .width = width, .height = height };
    wlr_scene_subsurface_tree_set_clip(&tree->node, &clip);
}

static inline int hatwm_xdg_toplevel_min_width(struct wlr_xdg_toplevel *top) {
    return top != NULL ? top->current.min_width : 0;
}

static inline int hatwm_xdg_toplevel_min_height(struct wlr_xdg_toplevel *top) {
    return top != NULL ? top->current.min_height : 0;
}

static inline int hatwm_xdg_toplevel_max_width(struct wlr_xdg_toplevel *top) {
    return top != NULL ? top->current.max_width : 0;
}

static inline int hatwm_xdg_toplevel_max_height(struct wlr_xdg_toplevel *top) {
    return top != NULL ? top->current.max_height : 0;
}
*/
import "C"

import (
	"unsafe"

	"github.com/swaywm/go-wlroots/wlroots"
)

func (v *View) clientSurface() wlroots.Surface {
	if v == nil {
		return wlroots.Surface{}
	}
	return v.ClientSurface
}

func (v *View) geometry() wlroots.GeoBox {
	if v == nil {
		return wlroots.GeoBox{}
	}
	if !v.IsXWayland {
		return v.TopLevel.Base().Geometry()
	}
	surface := (*C.struct_wlr_xwayland_surface)(v.XWayland)
	return wlroots.GeoBox{
		Width:  int(C.hatwm_xwayland_surface_width(surface)),
		Height: int(C.hatwm_xwayland_surface_height(surface)),
	}
}

func (v *View) setActivated(activated bool) {
	if v == nil {
		return
	}
	if v.IsXWayland {
		C.hatwm_xwayland_surface_activate(
			(*C.struct_wlr_xwayland_surface)(v.XWayland),
			C.bool(activated),
		)
		return
	}
	v.TopLevel.Base().TopLevelSetActivated(activated)
}

func (v *View) close() {
	if v == nil {
		return
	}
	if v.IsXWayland {
		C.hatwm_xwayland_surface_close(
			(*C.struct_wlr_xwayland_surface)(v.XWayland),
		)
		return
	}
	v.TopLevel.Base().SendClose()
}

func (v *View) setSize(width, height uint32) {
	if v == nil {
		return
	}
	if !v.IsXWayland {
		v.TopLevel.Base().TopLevelSetSize(width, height)
		return
	}
	x, y := v.targetRootPosition()
	v.configureXWayland(x, y, int(width), int(height))
}

func (v *View) setTiledContentSize(width, height uint32) {
	if v == nil {
		return
	}
	v.TileWidth = int(width)
	v.TileHeight = int(height)
	v.updateTiledClip()
}

func (v *View) clearTiledContentSize() {
	if v == nil {
		return
	}
	v.TileWidth = 0
	v.TileHeight = 0
	if !v.IsXWayland && !v.SurfaceTree.Nil() {
		C.hatwm_scene_tree_clip(
			(*C.struct_wlr_scene_tree)(sceneTreePointer(v.SurfaceTree)), 0, 0, 0, 0)
	}
}

func (v *View) updateTiledClip() {
	if v == nil || v.IsXWayland || v.SurfaceTree.Nil() {
		return
	}
	geometry := v.geometry()
	if v.TileWidth <= 0 || v.TileHeight <= 0 ||
		(geometry.Width <= v.TileWidth && geometry.Height <= v.TileHeight) {
		C.hatwm_scene_tree_clip(
			(*C.struct_wlr_scene_tree)(sceneTreePointer(v.SurfaceTree)), 0, 0, 0, 0)
		return
	}
	C.hatwm_scene_tree_clip(
		(*C.struct_wlr_scene_tree)(sceneTreePointer(v.SurfaceTree)),
		C.int(geometry.X), C.int(geometry.Y),
		C.int(v.TileWidth), C.int(v.TileHeight))
}

func (v *View) minimumSize() (int, int) {
	if v == nil || v.IsXWayland || v.TopLevel.Nil() {
		return 0, 0
	}
	top := (*C.struct_wlr_xdg_toplevel)(
		*(*unsafe.Pointer)(unsafe.Pointer(&v.TopLevel)))
	return int(C.hatwm_xdg_toplevel_min_width(top)),
		int(C.hatwm_xdg_toplevel_min_height(top))
}

func (v *View) sizeConstraints() (int, int, int, int) {
	if v == nil || v.IsXWayland || v.TopLevel.Nil() {
		return 0, 0, 0, 0
	}
	top := (*C.struct_wlr_xdg_toplevel)(
		*(*unsafe.Pointer)(unsafe.Pointer(&v.TopLevel)))
	return int(C.hatwm_xdg_toplevel_min_width(top)),
		int(C.hatwm_xdg_toplevel_min_height(top)),
		int(C.hatwm_xdg_toplevel_max_width(top)),
		int(C.hatwm_xdg_toplevel_max_height(top))
}

func (v *View) setXWaylandWindowState(fullscreen bool) {
	if v == nil || !v.IsXWayland {
		return
	}
	C.hatwm_xwayland_surface_set_window_state(
		(*C.struct_wlr_xwayland_surface)(v.XWayland),
		C.bool(fullscreen),
	)
}

func (v *View) setXWaylandRoundedClip(radius int) {
	if v == nil || !v.IsXWayland || v.XWayland == nil || v.Server == nil {
		return
	}
	g := v.geometry()
	if v.XClipRadius == radius &&
		v.XClipWidth == int(g.Width) && v.XClipHeight == int(g.Height) {
		return
	}
	C.hatwm_xwayland_surface_set_rounded_clip(
		v.Server.xwayland,
		(*C.struct_wlr_xwayland_surface)(v.XWayland),
		C.int(radius),
	)
	v.XClipRadius = radius
	v.XClipWidth = int(g.Width)
	v.XClipHeight = int(g.Height)
}

func (v *View) invalidateXWaylandRoundedClip() {
	if v == nil || !v.IsXWayland {
		return
	}
	v.XClipRadius = -1
	v.XClipWidth = -1
	v.XClipHeight = -1
}

func (v *View) targetRootPosition() (float64, float64) {
	if v.Animation.Initialized {
		return v.Animation.ToX, v.Animation.ToY
	}
	return float64(v.RootTree.Node().X()), float64(v.RootTree.Node().Y())
}

func (v *View) surfaceOffset() int {
	if v == nil || !v.Managed || v.Server == nil ||
		v.Server.fullscreen == v {
		return 0
	}
	if v.Server.config.BorderSize > 0 {
		return v.Server.config.BorderSize
	}
	return 0
}

func (v *View) configureXWayland(
	rootX, rootY float64, width, height int) {
	if v == nil || !v.IsXWayland || v.XWayland == nil {
		return
	}
	offset := v.surfaceOffset()
	C.hatwm_xwayland_surface_configure(
		(*C.struct_wlr_xwayland_surface)(v.XWayland),
		C.int(int(rootX)+offset),
		C.int(int(rootY)+offset),
		C.int(width),
		C.int(height),
	)
}

func (v *View) configureXWaylandPosition(rootX, rootY float64) {
	if v == nil || !v.IsXWayland {
		return
	}
	geometry := v.geometry()
	v.configureXWayland(
		rootX, rootY, geometry.Width, geometry.Height)
}

func (v *View) xwaylandWantsFocus() bool {
	if v == nil || !v.IsXWayland || v.XWayland == nil {
		return false
	}
	return bool(C.hatwm_xwayland_surface_wants_focus(
		(*C.struct_wlr_xwayland_surface)(v.XWayland),
	))
}

func surfaceFromXWaylandPointer(
	ptr *C.struct_wlr_xwayland_surface) wlroots.Surface {
	surfacePtr := C.hatwm_xwayland_surface_surface(ptr)
	if surfacePtr == nil {
		return wlroots.Surface{}
	}
	var surface wlroots.Surface
	*(*unsafe.Pointer)(unsafe.Pointer(&surface)) = unsafe.Pointer(surfacePtr)
	return surface
}
