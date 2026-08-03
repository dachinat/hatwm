package main

/*
#cgo pkg-config: wlroots-0.18
#cgo CFLAGS: -D_GNU_SOURCE -DWLR_USE_UNSTABLE
#include <wlr/types/wlr_scene.h>

static void hatwm_set_buffer_opacity(
		struct wlr_scene_buffer *buffer, int sx, int sy, void *data) {
	(void)sx;
	(void)sy;
	float opacity = *(float *)data;
	if (buffer->opacity != opacity) {
		wlr_scene_buffer_set_opacity(buffer, opacity);
	}
}

static void hatwm_scene_tree_set_opacity(
		struct wlr_scene_tree *tree, float opacity) {
	if (tree == NULL) {
		return;
	}
	wlr_scene_node_for_each_buffer(
		&tree->node, hatwm_set_buffer_opacity, &opacity);
}
*/
import "C"

func (s *Server) applyWindowOpacity(view *View) {
	if s == nil || view == nil || view.RootTree.Nil() {
		return
	}
	C.hatwm_scene_tree_set_opacity(
		(*C.struct_wlr_scene_tree)(sceneTreePointer(view.RootTree)),
		C.float(s.config.WindowOpacity),
	)
}

func (s *Server) applyWindowOpacityToAll() {
	for _, view := range s.views {
		s.applyWindowOpacity(view)
	}
}
