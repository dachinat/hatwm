package compositor

/*
#cgo pkg-config: wlroots-0.18 wayland-server
#cgo CFLAGS: -D_GNU_SOURCE -DWLR_USE_UNSTABLE -I${SRCDIR}/protocols
#include "layer_shell.h"
*/
import "C"

import (
	"log/slog"
	"unsafe"

	"github.com/swaywm/go-wlroots/wlroots"
)

const (
	layerBackground = 0
	layerBottom     = 1
	layerTop        = 2
	layerOverlay    = 3

	anchorTop    = 1
	anchorBottom = 2
	anchorLeft   = 4
	anchorRight  = 8

	keyboardNone      = 0
	keyboardExclusive = 1
	keyboardOnDemand  = 2
)

type LayerSurface struct {
	ptr        *C.struct_wlr_layer_surface_v1
	scene      *C.struct_wlr_scene_tree
	mapped     bool
	layer      uint32
	lastX      int
	lastY      int
	lastW      uint32
	lastH      uint32
	configured bool
}

type usableBox struct {
	x, y, width, height int
}

var activeServer *Server

func displayPointer(display wlroots.Display) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&display))
}

func outputPointer(output wlroots.Output) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&output))
}

func outputLayoutPointer(layout wlroots.OutputLayout) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&layout))
}

func surfacePointer(surface wlroots.Surface) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&surface))
}

func (s *Server) initLayerShell() error {
	activeServer = s
	s.layerShell = C.hatwm_layer_shell_create((*C.struct_wl_display)(displayPointer(s.display)))
	if s.layerShell == nil {
		return errLayerShellInit
	}
	return nil
}

var errLayerShellInit = &layerShellError{}

type layerShellError struct{}

func (*layerShellError) Error() string { return "failed to create wlr-layer-shell-v1 global" }

func (s *Server) layerTree(layer uint32) wlroots.SceneTree {
	switch layer {
	case layerBackground:
		return s.backgroundTree
	case layerBottom:
		return s.bottomTree
	case layerTop:
		return s.topTree
	case layerOverlay:
		return s.overlayTree
	default:
		return s.topTree
	}
}

func (s *Server) handleLayerNew(ptr *C.struct_wlr_layer_surface_v1) {
	if ptr == nil {
		return
	}
	if len(s.outputs) > 0 && C.hatwm_layer_surface_output(ptr) == nil {
		C.hatwm_layer_surface_set_output(ptr, (*C.struct_wlr_output)(outputPointer(s.outputs[0])))
	}
	layer := uint32(C.hatwm_layer_surface_layer(ptr))
	parent := s.layerTree(layer)
	scene := C.hatwm_layer_scene_create((*C.struct_wlr_scene_tree)(sceneTreePointer(parent)), C.hatwm_layer_surface_surface(ptr))
	if scene == nil {
		slog.Error("failed to create layer surface scene tree")
		return
	}
	C.hatwm_layer_surface_set_scene_tree(ptr, scene)
	ls := &LayerSurface{ptr: ptr, scene: scene, layer: layer}
	C.hatwm_scene_tree_set_enabled(scene, false)
	s.layerSurfaces = append(s.layerSurfaces, ls)
	slog.Info("layer surface created", "layer", layer)
	// Do not configure here. The client has not committed its initial pending
	// layer-shell state yet, so anchors/desired size may still be incomplete.
	// The first surface commit will call handleLayerCommit, which sends exactly
	// one initial configure.
}

func (s *Server) handleLayerMap(ptr *C.struct_wlr_layer_surface_v1) {
	ls := s.findLayerSurface(ptr)
	if ls == nil {
		return
	}
	ls.mapped = true
	slog.Info("layer surface mapped", "layer", ls.layer)
	C.hatwm_scene_tree_set_enabled(ls.scene, true)
	s.arrangeLayers()
	if uint32(C.hatwm_layer_surface_keyboard_interactive(ptr)) == keyboardExclusive {
		s.focusLayerSurface(ls)
	}
}

func (s *Server) handleLayerUnmap(ptr *C.struct_wlr_layer_surface_v1) {
	ls := s.findLayerSurface(ptr)
	if ls == nil {
		return
	}
	ls.mapped = false
	C.hatwm_scene_tree_set_enabled(ls.scene, false)
	s.arrangeLayers()
}

func (s *Server) handleLayerCommit(ptr *C.struct_wlr_layer_surface_v1) {
	ls := s.findLayerSurface(ptr)
	if ls == nil {
		return
	}
	newLayer := uint32(C.hatwm_layer_surface_layer(ptr))
	if newLayer != ls.layer {
		// Layer is effectively immutable for normal clients; recreating the scene
		// tree here would complicate ownership, so retain the creation layer.
		ls.layer = newLayer
	}
	s.arrangeLayers()
}

func (s *Server) handleLayerDestroy(ptr *C.struct_wlr_layer_surface_v1) {
	for i, ls := range s.layerSurfaces {
		if ls.ptr != ptr {
			continue
		}
		if ls.scene != nil {
			C.hatwm_scene_tree_destroy(ls.scene)
			ls.scene = nil
		}
		s.layerSurfaces = append(s.layerSurfaces[:i], s.layerSurfaces[i+1:]...)
		break
	}
	s.arrangeLayers()
}

func (s *Server) findLayerSurface(ptr *C.struct_wlr_layer_surface_v1) *LayerSurface {
	for _, ls := range s.layerSurfaces {
		if ls.ptr == ptr {
			return ls
		}
	}
	return nil
}

func (s *Server) layerSurfaceForSurface(surface wlroots.Surface) *LayerSurface {
	sp := surfacePointer(surface)
	for _, ls := range s.layerSurfaces {
		if !ls.mapped || ls.ptr == nil {
			continue
		}
		if unsafe.Pointer(C.hatwm_layer_surface_surface(ls.ptr)) == sp {
			return ls
		}
	}
	return nil
}

func (s *Server) exclusiveKeyboardLayer() *LayerSurface {
	for i := len(s.layerSurfaces) - 1; i >= 0; i-- {
		ls := s.layerSurfaces[i]
		if ls.mapped && ls.ptr != nil && uint32(C.hatwm_layer_surface_keyboard_interactive(ls.ptr)) == keyboardExclusive {
			return ls
		}
	}
	return nil
}

func (s *Server) focusLayerSurface(ls *LayerSurface) {
	if s.sessionLocked || ls == nil || !ls.mapped || ls.ptr == nil {
		return
	}
	mode := uint32(C.hatwm_layer_surface_keyboard_interactive(ls.ptr))
	if mode == keyboardNone {
		return
	}
	sp := C.hatwm_layer_surface_surface(ls.ptr)
	if sp == nil {
		return
	}
	var surface wlroots.Surface
	*(*unsafe.Pointer)(unsafe.Pointer(&surface)) = unsafe.Pointer(sp)
	s.seat.NotifyKeyboardEnter(surface, s.seat.Keyboard())
}

func (s *Server) arrangeLayers() {
	if len(s.outputs) == 0 {
		return
	}
	outW, outH := s.outputs[0].EffectiveResolution()
	full := usableBox{width: outW, height: outH}
	usable := full

	// A layer-shell client cannot map until the compositor sends an initial
	// configure. Configure unmapped surfaces too; they must not reserve usable
	// space until they are actually mapped.
	for _, ls := range s.layerSurfaces {
		if !ls.mapped {
			s.arrangeOneLayer(ls, full, nil)
		}
	}

	// Exclusive mapped surfaces reserve space before normal/non-exclusive surfaces are placed.
	for _, layer := range []uint32{layerOverlay, layerTop, layerBottom, layerBackground} {
		for _, ls := range s.layerSurfaces {
			if !ls.mapped || ls.layer != layer {
				continue
			}
			anchor := uint32(C.hatwm_layer_surface_anchor(ls.ptr))
			zone := effectiveExclusiveZone(
				anchor, int(C.hatwm_layer_surface_exclusive_zone(ls.ptr)))
			if zone > 0 {
				s.arrangeOneLayer(ls, full, &usable)
			}
		}
	}
	for _, layer := range []uint32{layerBackground, layerBottom, layerTop, layerOverlay} {
		for _, ls := range s.layerSurfaces {
			if !ls.mapped || ls.layer != layer {
				continue
			}
			anchor := uint32(C.hatwm_layer_surface_anchor(ls.ptr))
			zone := effectiveExclusiveZone(
				anchor, int(C.hatwm_layer_surface_exclusive_zone(ls.ptr)))
			if zone <= 0 {
				s.arrangeOneLayer(ls, layerPlacementBounds(full, usable, zone), nil)
			}
		}
	}

	s.usable = usable
	s.arrangeViewsIn(usable)
}

// effectiveExclusiveZone implements the anchor validation required by the
// layer-shell protocol. A positive zone is meaningful only for a surface
// anchored to exactly one edge, optionally with both perpendicular edges.
// Invalid positive zones behave like zero and therefore avoid valid exclusive
// surfaces such as panels.
func effectiveExclusiveZone(anchor uint32, zone int) int {
	if zone > 0 && exclusiveAnchorEdge(anchor) == 0 {
		return 0
	}
	return zone
}

func exclusiveAnchorEdge(anchor uint32) uint32 {
	horizontal := anchor & (anchorLeft | anchorRight)
	vertical := anchor & (anchorTop | anchorBottom)
	allHorizontal := uint32(anchorLeft | anchorRight)
	allVertical := uint32(anchorTop | anchorBottom)

	switch {
	case vertical == anchorTop && (horizontal == 0 || horizontal == allHorizontal):
		return anchorTop
	case vertical == anchorBottom && (horizontal == 0 || horizontal == allHorizontal):
		return anchorBottom
	case horizontal == anchorLeft && (vertical == 0 || vertical == allVertical):
		return anchorLeft
	case horizontal == anchorRight && (vertical == 0 || vertical == allVertical):
		return anchorRight
	default:
		return 0
	}
}

func layerPlacementBounds(full, usable usableBox, zone int) usableBox {
	if zone == 0 {
		return usable
	}
	return full
}

func (s *Server) arrangeOneLayer(ls *LayerSurface, bounds usableBox, usable *usableBox) {
	p := ls.ptr
	anchor := uint32(C.hatwm_layer_surface_anchor(p))
	w := int(C.hatwm_layer_surface_desired_width(p))
	h := int(C.hatwm_layer_surface_desired_height(p))
	mt := int(C.hatwm_layer_surface_margin_top(p))
	mr := int(C.hatwm_layer_surface_margin_right(p))
	mb := int(C.hatwm_layer_surface_margin_bottom(p))
	ml := int(C.hatwm_layer_surface_margin_left(p))

	if w == 0 {
		if anchor&anchorLeft != 0 && anchor&anchorRight != 0 {
			w = bounds.width - ml - mr
		} else {
			w = bounds.width
		}
	}
	if h == 0 {
		if anchor&anchorTop != 0 && anchor&anchorBottom != 0 {
			h = bounds.height - mt - mb
		} else {
			h = bounds.height
		}
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}

	x := bounds.x + (bounds.width-w)/2
	y := bounds.y + (bounds.height-h)/2
	if anchor&anchorLeft != 0 {
		x = bounds.x + ml
	} else if anchor&anchorRight != 0 {
		x = bounds.x + bounds.width - w - mr
	}
	if anchor&anchorTop != 0 {
		y = bounds.y + mt
	} else if anchor&anchorBottom != 0 {
		y = bounds.y + bounds.height - h - mb
	}

	// Sending a configure for every client commit creates a feedback loop:
	// configure -> ack/commit -> configure -> ... . Only configure the first
	// time, or when HatWM actually computes a different size.
	newW, newH := uint32(w), uint32(h)
	if !ls.configured || ls.lastW != newW || ls.lastH != newH {
		C.hatwm_layer_surface_configure(p, C.uint32_t(newW), C.uint32_t(newH))
		ls.configured = true
	}

	if ls.lastX != x || ls.lastY != y {
		C.hatwm_scene_tree_set_position(ls.scene, C.int(x), C.int(y))
	}
	ls.lastX, ls.lastY, ls.lastW, ls.lastH = x, y, newW, newH

	// Unmapped surfaces are configured so that they can map, but only mapped
	// surfaces are allowed to consume an exclusive zone.
	if usable == nil {
		return
	}
	zone := int(C.hatwm_layer_surface_exclusive_zone(p))
	if zone <= 0 {
		return
	}
	// Reserve only surfaces with a protocol-valid exclusive anchor.
	switch exclusiveAnchorEdge(anchor) {
	case anchorTop:
		amount := zone + mt
		if amount > usable.height {
			amount = usable.height
		}
		usable.y += amount
		usable.height -= amount
	case anchorBottom:
		amount := zone + mb
		if amount > usable.height {
			amount = usable.height
		}
		usable.height -= amount
	case anchorLeft:
		amount := zone + ml
		if amount > usable.width {
			amount = usable.width
		}
		usable.x += amount
		usable.width -= amount
	case anchorRight:
		amount := zone + mr
		if amount > usable.width {
			amount = usable.width
		}
		usable.width -= amount
	}
}

//export hatwmGoLayerNew
func hatwmGoLayerNew(layer unsafe.Pointer) {
	if activeServer != nil {
		activeServer.handleLayerNew((*C.struct_wlr_layer_surface_v1)(layer))
	}
}

//export hatwmGoLayerMap
func hatwmGoLayerMap(layer unsafe.Pointer) {
	if activeServer != nil {
		activeServer.handleLayerMap((*C.struct_wlr_layer_surface_v1)(layer))
	}
}

//export hatwmGoLayerUnmap
func hatwmGoLayerUnmap(layer unsafe.Pointer) {
	if activeServer != nil {
		activeServer.handleLayerUnmap((*C.struct_wlr_layer_surface_v1)(layer))
	}
}

//export hatwmGoLayerCommit
func hatwmGoLayerCommit(layer unsafe.Pointer) {
	if activeServer != nil {
		activeServer.handleLayerCommit((*C.struct_wlr_layer_surface_v1)(layer))
	}
}

//export hatwmGoLayerDestroy
func hatwmGoLayerDestroy(layer unsafe.Pointer) {
	if activeServer != nil {
		activeServer.handleLayerDestroy((*C.struct_wlr_layer_surface_v1)(layer))
	}
}
