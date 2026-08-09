package compositor

/*
#cgo pkg-config: wlroots-0.18 wayland-server
#cgo CFLAGS: -D_GNU_SOURCE -DWLR_USE_UNSTABLE
#include <wlr/types/wlr_scene.h>

static inline struct wlr_scene_tree *hatwm_scene_tree(void *p) {
    return (struct wlr_scene_tree *)p;
}
static inline void hatwm_rect_size(struct wlr_scene_rect *r, int w, int h) {
    wlr_scene_rect_set_size(r, w, h);
}
static inline void hatwm_rect_color(struct wlr_scene_rect *r, float *color) {
    wlr_scene_rect_set_color(r, color);
}
static inline void hatwm_node_position(struct wlr_scene_node *n, int x, int y) {
    wlr_scene_node_set_position(n, x, y);
}
static inline void hatwm_node_enabled(struct wlr_scene_node *n, bool enabled) {
    wlr_scene_node_set_enabled(n, enabled);
}
static inline void hatwm_node_destroy(struct wlr_scene_node *n) {
    wlr_scene_node_destroy(n);
}
static inline void hatwm_node_raise(struct wlr_scene_node *n) {
    wlr_scene_node_raise_to_top(n);
}
static inline void hatwm_node_lower(struct wlr_scene_node *n) {
    wlr_scene_node_lower_to_bottom(n);
}
*/
import "C"

import (
	"math"
	"unsafe"

	"github.com/swaywm/go-wlroots/wlroots"
)

type WindowDecoration struct {
	// Edges are top, bottom, left, right. They persist for the life of the view.
	Edges [4]*C.struct_wlr_scene_rect

	// Corners contains four 1px-high slices per radius row (TL, TR, BL, BR).
	// Slices are allocated lazily and then reused; updateDecoration never
	// destroys/recreates them during normal commits or resizes.
	Corners []*C.struct_wlr_scene_rect

	// CornerFills are outer-circle slices below the client surface. CSD clients
	// often leave their rounded corners transparent with a radius that differs
	// from HatWM's border radius. These prevent the wallpaper from showing
	// through the inside of the border without covering client content.
	CornerFills []*C.struct_wlr_scene_rect

	ready bool
}

func sceneTreePointer(tree wlroots.SceneTree) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&tree))
}

func sceneRect(parent *C.struct_wlr_scene_tree, color [4]C.float) *C.struct_wlr_scene_rect {
	r := C.wlr_scene_rect_create(parent, 1, 1, &color[0])
	if r != nil {
		// Rounded corner slices overlap the square client surface along their
		// inner edge. Keep decorations above the surface so the client cannot
		// cover that part of the border at larger radii.
		C.wlr_scene_node_raise_to_top(&r.node)
	}
	return r
}

func sceneRectBelow(parent *C.struct_wlr_scene_tree, color [4]C.float) *C.struct_wlr_scene_rect {
	r := C.wlr_scene_rect_create(parent, 1, 1, &color[0])
	if r != nil {
		C.hatwm_node_lower(&r.node)
	}
	return r
}

func (s *Server) ensureDecoration(v *View) {
	if v == nil || v.Decor.ready || s.viewBorderSize(v) <= 0 {
		return
	}
	parent := C.hatwm_scene_tree(sceneTreePointer(v.RootTree))
	color := s.config.InactiveBorderColor
	c := [4]C.float{C.float(color[0]), C.float(color[1]), C.float(color[2]), C.float(color[3])}
	for i := range v.Decor.Edges {
		v.Decor.Edges[i] = sceneRect(parent, c)
	}
	v.Decor.ready = true
}

func (s *Server) ensureCornerCapacity(v *View, rows int, color [4]C.float) {
	if v == nil || rows <= 0 {
		return
	}
	needed := rows * 4
	if len(v.Decor.Corners) >= needed {
		return
	}

	parent := C.hatwm_scene_tree(sceneTreePointer(v.RootTree))
	for len(v.Decor.Corners) < needed {
		v.Decor.Corners = append(v.Decor.Corners, sceneRect(parent, color))
	}
	for len(v.Decor.CornerFills) < needed {
		v.Decor.CornerFills = append(
			v.Decor.CornerFills, sceneRectBelow(parent, color))
	}
}

func (s *Server) destroyDecoration(v *View) {
	if v == nil || !v.Decor.ready {
		return
	}
	for i, rect := range v.Decor.Edges {
		if rect != nil {
			C.hatwm_node_destroy(&rect.node)
			v.Decor.Edges[i] = nil
		}
	}
	for i, rect := range v.Decor.Corners {
		if rect != nil {
			C.hatwm_node_destroy(&rect.node)
			v.Decor.Corners[i] = nil
		}
	}
	for i, rect := range v.Decor.CornerFills {
		if rect != nil {
			C.hatwm_node_destroy(&rect.node)
			v.Decor.CornerFills[i] = nil
		}
	}
	v.Decor.Corners = nil
	v.Decor.CornerFills = nil
	v.Decor.ready = false
}

func setRect(rect *C.struct_wlr_scene_rect, x, y, w, h int, color [4]C.float, enabled bool) {
	if rect == nil {
		return
	}
	if !enabled || w <= 0 || h <= 0 {
		C.hatwm_node_enabled(&rect.node, false)
		return
	}
	C.hatwm_rect_size(rect, C.int(w), C.int(h))
	C.hatwm_rect_color(rect, &color[0])
	C.hatwm_node_position(&rect.node, C.int(x), C.int(y))
	C.hatwm_node_enabled(&rect.node, true)
}

func (s *Server) hideDecoration(v *View) {
	if v == nil || !v.Decor.ready {
		return
	}
	for _, r := range v.Decor.Edges {
		if r != nil {
			C.hatwm_node_enabled(&r.node, false)
		}
	}
	for _, r := range v.Decor.Corners {
		if r != nil {
			C.hatwm_node_enabled(&r.node, false)
		}
	}
	for _, r := range v.Decor.CornerFills {
		if r != nil {
			C.hatwm_node_enabled(&r.node, false)
		}
	}
}

// raiseDecoration restores the border's stacking after an XWayland surface
// tree is recreated. Persistent decoration nodes predate the replacement
// surface tree and would otherwise remain underneath it.
func (s *Server) raiseDecoration(v *View) {
	if v == nil || !v.Decor.ready {
		return
	}
	for _, rect := range v.Decor.Edges {
		if rect != nil {
			C.hatwm_node_raise(&rect.node)
		}
	}
	for _, rect := range v.Decor.Corners {
		if rect != nil {
			C.hatwm_node_raise(&rect.node)
		}
	}
}

func (s *Server) updateDecoration(v *View) {
	if v == nil {
		return
	}
	v.updateTiledClip()
	if !v.Managed || !v.Mapped {
		s.hideDecoration(v)
		return
	}

	b := s.viewBorderSize(v)
	if b <= 0 || s.viewFullscreen(v) {
		s.hideDecoration(v)
		v.SurfaceTree.Node().SetPosition(0, 0)
		v.setXWaylandRoundedClip(0)
		return
	}

	s.ensureDecoration(v)
	v.SurfaceTree.Node().SetPosition(float64(b), float64(b))

	g := v.geometry()
	w, h := g.Width, g.Height
	if v.TileWidth > 0 && v.TileHeight > 0 {
		w, h = v.TileWidth, v.TileHeight
	}
	if w < 1 || h < 1 {
		s.hideDecoration(v)
		return
	}

	totalW, totalH := w+2*b, h+2*b
	active := s.seat.KeyboardState().FocusedSurface() == v.clientSurface()
	color := s.config.InactiveBorderColor
	if active {
		color = s.config.ActiveBorderColor
	}
	c := [4]C.float{C.float(color[0]), C.float(color[1]), C.float(color[2]), C.float(color[3])}

	radius := s.viewBorderRounding(v)
	maxRadius := totalW / 2
	if totalH/2 < maxRadius {
		maxRadius = totalH / 2
	}
	if radius > maxRadius {
		radius = maxRadius
	}
	if radius < 0 {
		radius = 0
	}

	// X11 clients usually submit an opaque rectangular buffer. Shape it to the
	// border's inner curve; drawing a rounded ring alone leaves square client
	// pixels visible through the outside of that ring.
	clientRadius := radius - b
	if clientRadius < 0 {
		clientRadius = 0
	}
	v.setXWaylandRoundedClip(clientRadius)

	if radius == 0 {
		// Classic four-edge border.
		setRect(v.Decor.Edges[0], 0, 0, totalW, b, c, true)
		setRect(v.Decor.Edges[1], 0, totalH-b, totalW, b, c, true)
		setRect(v.Decor.Edges[2], 0, b, b, h, c, true)
		setRect(v.Decor.Edges[3], totalW-b, b, b, h, c, true)
		for _, r := range v.Decor.Corners {
			if r != nil {
				C.hatwm_node_enabled(&r.node, false)
			}
		}
		for _, r := range v.Decor.CornerFills {
			if r != nil {
				C.hatwm_node_enabled(&r.node, false)
			}
		}
		return
	}

	// Straight portions stop before the rounded corners.
	setRect(v.Decor.Edges[0], radius, 0, totalW-2*radius, b, c, true)
	setRect(v.Decor.Edges[1], radius, totalH-b, totalW-2*radius, b, c, true)
	setRect(v.Decor.Edges[2], 0, radius, b, totalH-2*radius, c, true)
	setRect(v.Decor.Edges[3], totalW-b, radius, b, totalH-2*radius, c, true)

	s.ensureCornerCapacity(v, radius, c)
	used := radius * 4

	outerR := float64(radius)
	innerRadius := radius - b
	if innerRadius < 0 {
		innerRadius = 0
	}
	innerR := float64(innerRadius)

	for dy := 0; dy < radius; dy++ {
		// Pixel-center sampling makes small radii look less jagged than using
		// the integer row edge directly.
		y := float64(dy) + 0.5
		fromCenter := outerR - y
		outSq := outerR*outerR - fromCenter*fromCenter
		if outSq < 0 {
			outSq = 0
		}
		xOuter := outerR - math.Sqrt(outSq)

		xInner := outerR
		if innerRadius > 0 && fromCenter < innerR {
			inSq := innerR*innerR - fromCenter*fromCenter
			if inSq < 0 {
				inSq = 0
			}
			// Inner circle shares the same center as the outer circle.
			xInner = outerR - math.Sqrt(inSq)
		}

		start := int(math.Floor(xOuter))
		end := int(math.Ceil(xInner))
		sliceW := end - start
		base := dy * 4
		fillW := radius - start
		if fillW > 0 {
			setRect(v.Decor.CornerFills[base+0], start, dy, fillW, 1, c, true)
			setRect(v.Decor.CornerFills[base+1], totalW-radius, dy, fillW, 1, c, true)
			setRect(v.Decor.CornerFills[base+2], start, totalH-1-dy, fillW, 1, c, true)
			setRect(v.Decor.CornerFills[base+3], totalW-radius, totalH-1-dy, fillW, 1, c, true)
		} else {
			for j := 0; j < 4; j++ {
				setRect(v.Decor.CornerFills[base+j], 0, 0, 0, 0, c, false)
			}
		}
		if sliceW <= 0 {
			for j := 0; j < 4; j++ {
				setRect(v.Decor.Corners[base+j], 0, 0, 0, 0, c, false)
			}
			continue
		}

		setRect(v.Decor.Corners[base+0], start, dy, sliceW, 1, c, true)
		setRect(v.Decor.Corners[base+1], totalW-end, dy, sliceW, 1, c, true)
		setRect(v.Decor.Corners[base+2], start, totalH-1-dy, sliceW, 1, c, true)
		setRect(v.Decor.Corners[base+3], totalW-end, totalH-1-dy, sliceW, 1, c, true)
	}

	// Radius may have been reduced on config reload. Keep the already-created
	// nodes for reuse but disable the surplus instead of destroying them.
	for i := used; i < len(v.Decor.Corners); i++ {
		if v.Decor.Corners[i] != nil {
			C.hatwm_node_enabled(&v.Decor.Corners[i].node, false)
		}
	}
	for i := used; i < len(v.Decor.CornerFills); i++ {
		if v.Decor.CornerFills[i] != nil {
			C.hatwm_node_enabled(&v.Decor.CornerFills[i].node, false)
		}
	}
}

func (s *Server) updateAllDecorations() {
	for _, v := range s.views {
		s.updateDecoration(v)
	}
}
