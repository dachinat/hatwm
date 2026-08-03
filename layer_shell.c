#include "layer_shell.h"

#include <stdlib.h>
#include <stddef.h>

extern void hatwmGoLayerNew(void *layer);
extern void hatwmGoLayerMap(void *layer);
extern void hatwmGoLayerUnmap(void *layer);
extern void hatwmGoLayerCommit(void *layer);
extern void hatwmGoLayerDestroy(void *layer);

struct hatwm_layer_surface_listener {
    struct wlr_layer_surface_v1 *layer;
    struct wl_listener map;
    struct wl_listener unmap;
    struct wl_listener commit;
    struct wl_listener destroy;
    struct wl_listener new_popup;
    struct wlr_scene_tree *scene_tree;
};

struct hatwm_layer_shell {
    struct wlr_layer_shell_v1 *shell;
    struct wl_listener new_surface;
    struct wl_listener display_destroy;
};

static void handle_layer_map(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_layer_surface_listener *l = wl_container_of(listener, l, map);
    hatwmGoLayerMap(l->layer);
}

static void handle_layer_unmap(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_layer_surface_listener *l = wl_container_of(listener, l, unmap);
    hatwmGoLayerUnmap(l->layer);
}

static void handle_layer_commit(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_layer_surface_listener *l = wl_container_of(listener, l, commit);
    hatwmGoLayerCommit(l->layer);
}

static void handle_layer_new_popup(struct wl_listener *listener, void *data) {
    struct hatwm_layer_surface_listener *l = wl_container_of(listener, l, new_popup);
    struct wlr_xdg_popup *popup = data;
    if (l->scene_tree != NULL && popup != NULL) {
        /*
         * xdg-positioner coordinates are relative to the layer surface.
         * Translate the output bounds into that coordinate space before
         * asking wlroots to flip/slide/resize the popup. Without this, a
         * menu opened from an item at the left edge can remain partially
         * outside the output.
         */
        int layer_x = 0, layer_y = 0;
        int output_width = 0, output_height = 0;
        wlr_scene_node_coords(&l->scene_tree->node, &layer_x, &layer_y);
        if (l->layer->output != NULL) {
            wlr_output_effective_resolution(
                l->layer->output, &output_width, &output_height);
        }
        if (output_width > 0 && output_height > 0) {
            struct wlr_box output_box = {
                .x = -layer_x,
                .y = -layer_y,
                .width = output_width,
                .height = output_height,
            };
            wlr_xdg_popup_unconstrain_from_box(popup, &output_box);
        }

        struct wlr_scene_tree *popup_tree =
            wlr_scene_xdg_surface_create(l->scene_tree, popup->base);
        if (popup_tree != NULL) {
            /* Needed as the parent tree for nested xdg popups. */
            popup->base->data = popup_tree;
        }
    }
}

static void handle_layer_destroy(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_layer_surface_listener *l = wl_container_of(listener, l, destroy);
    hatwmGoLayerDestroy(l->layer);
    if (l->layer->data == l) {
        l->layer->data = NULL;
    }
    wl_list_remove(&l->map.link);
    wl_list_remove(&l->unmap.link);
    wl_list_remove(&l->commit.link);
    wl_list_remove(&l->destroy.link);
    wl_list_remove(&l->new_popup.link);
    free(l);
}

static void handle_new_surface(struct wl_listener *listener, void *data) {
    (void)listener;
    struct wlr_layer_surface_v1 *layer = data;
    struct hatwm_layer_surface_listener *l = calloc(1, sizeof(*l));
    if (l == NULL) {
        return;
    }
    l->layer = layer;
    layer->data = l;
    l->map.notify = handle_layer_map;
    l->unmap.notify = handle_layer_unmap;
    l->commit.notify = handle_layer_commit;
    l->destroy.notify = handle_layer_destroy;
    l->new_popup.notify = handle_layer_new_popup;
    wl_signal_add(&layer->surface->events.map, &l->map);
    wl_signal_add(&layer->surface->events.unmap, &l->unmap);
    wl_signal_add(&layer->surface->events.commit, &l->commit);
    wl_signal_add(&layer->events.destroy, &l->destroy);
    wl_signal_add(&layer->events.new_popup, &l->new_popup);
    hatwmGoLayerNew(layer);
}

static void handle_display_destroy(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_layer_shell *shell = wl_container_of(listener, shell, display_destroy);
    wl_list_remove(&shell->new_surface.link);
    wl_list_remove(&shell->display_destroy.link);
    free(shell);
}

struct wlr_xdg_output_manager_v1 *hatwm_xdg_output_manager_create(
        struct wl_display *display, struct wlr_output_layout *layout) {
    if (display == NULL || layout == NULL) {
        return NULL;
    }
    return wlr_xdg_output_manager_v1_create(display, layout);
}

struct hatwm_layer_shell *hatwm_layer_shell_create(struct wl_display *display) {
    struct hatwm_layer_shell *result = calloc(1, sizeof(*result));
    if (result == NULL) {
        return NULL;
    }
    result->shell = wlr_layer_shell_v1_create(display, 4);
    if (result->shell == NULL) {
        free(result);
        return NULL;
    }
    result->new_surface.notify = handle_new_surface;
    wl_signal_add(&result->shell->events.new_surface, &result->new_surface);
    result->display_destroy.notify = handle_display_destroy;
    wl_display_add_destroy_listener(display, &result->display_destroy);
    return result;
}

void hatwm_layer_shell_destroy(struct hatwm_layer_shell *shell) {
    /* The wlroots global is display-owned. This helper only detaches our listeners. */
    if (shell == NULL) {
        return;
    }
    wl_list_remove(&shell->new_surface.link);
    wl_list_remove(&shell->display_destroy.link);
    free(shell);
}

struct wlr_surface *hatwm_layer_surface_surface(struct wlr_layer_surface_v1 *layer) { return layer ? layer->surface : NULL; }
struct wlr_output *hatwm_layer_surface_output(struct wlr_layer_surface_v1 *layer) { return layer ? layer->output : NULL; }
void hatwm_layer_surface_set_output(struct wlr_layer_surface_v1 *layer, struct wlr_output *output) { if (layer && layer->output == NULL) layer->output = output; }
static const struct wlr_layer_surface_v1_state *hatwm_layer_surface_state(
        struct wlr_layer_surface_v1 *layer) {
    if (layer == NULL) {
        return NULL;
    }
    /*
     * Before the first commit, the client's requested state lives in pending.
     * Once wlroots has initialized the role, current is authoritative.
     */
    return layer->initialized ? &layer->current : &layer->pending;
}

uint32_t hatwm_layer_surface_layer(struct wlr_layer_surface_v1 *layer) {
    const struct wlr_layer_surface_v1_state *state = hatwm_layer_surface_state(layer);
    return state ? state->layer : 0;
}
uint32_t hatwm_layer_surface_anchor(struct wlr_layer_surface_v1 *layer) {
    const struct wlr_layer_surface_v1_state *state = hatwm_layer_surface_state(layer);
    return state ? state->anchor : 0;
}
uint32_t hatwm_layer_surface_desired_width(struct wlr_layer_surface_v1 *layer) {
    const struct wlr_layer_surface_v1_state *state = hatwm_layer_surface_state(layer);
    return state ? state->desired_width : 0;
}
uint32_t hatwm_layer_surface_desired_height(struct wlr_layer_surface_v1 *layer) {
    const struct wlr_layer_surface_v1_state *state = hatwm_layer_surface_state(layer);
    return state ? state->desired_height : 0;
}
int32_t hatwm_layer_surface_exclusive_zone(struct wlr_layer_surface_v1 *layer) {
    const struct wlr_layer_surface_v1_state *state = hatwm_layer_surface_state(layer);
    return state ? state->exclusive_zone : 0;
}
int32_t hatwm_layer_surface_margin_top(struct wlr_layer_surface_v1 *layer) {
    const struct wlr_layer_surface_v1_state *state = hatwm_layer_surface_state(layer);
    return state ? state->margin.top : 0;
}
int32_t hatwm_layer_surface_margin_right(struct wlr_layer_surface_v1 *layer) {
    const struct wlr_layer_surface_v1_state *state = hatwm_layer_surface_state(layer);
    return state ? state->margin.right : 0;
}
int32_t hatwm_layer_surface_margin_bottom(struct wlr_layer_surface_v1 *layer) {
    const struct wlr_layer_surface_v1_state *state = hatwm_layer_surface_state(layer);
    return state ? state->margin.bottom : 0;
}
int32_t hatwm_layer_surface_margin_left(struct wlr_layer_surface_v1 *layer) {
    const struct wlr_layer_surface_v1_state *state = hatwm_layer_surface_state(layer);
    return state ? state->margin.left : 0;
}
uint32_t hatwm_layer_surface_keyboard_interactive(struct wlr_layer_surface_v1 *layer) {
    const struct wlr_layer_surface_v1_state *state = hatwm_layer_surface_state(layer);
    return state ? state->keyboard_interactive : 0;
}
bool hatwm_layer_surface_mapped(struct wlr_layer_surface_v1 *layer) { return layer && layer->surface && layer->surface->mapped; }
uint32_t hatwm_layer_surface_configure(struct wlr_layer_surface_v1 *layer, uint32_t width, uint32_t height) { return layer ? wlr_layer_surface_v1_configure(layer, width, height) : 0; }

struct wlr_scene_tree *hatwm_layer_scene_create(struct wlr_scene_tree *parent, struct wlr_surface *surface) {
    if (parent == NULL || surface == NULL) return NULL;
    return wlr_scene_subsurface_tree_create(parent, surface);
}
void hatwm_layer_surface_set_scene_tree(
        struct wlr_layer_surface_v1 *layer, struct wlr_scene_tree *tree) {
    if (layer == NULL || layer->data == NULL) return;
    struct hatwm_layer_surface_listener *l = layer->data;
    l->scene_tree = tree;
}
void hatwm_scene_tree_set_position(struct wlr_scene_tree *tree, int x, int y) { if (tree) wlr_scene_node_set_position(&tree->node, x, y); }
void hatwm_scene_tree_set_enabled(struct wlr_scene_tree *tree, bool enabled) { if (tree) wlr_scene_node_set_enabled(&tree->node, enabled); }
void hatwm_scene_tree_destroy(struct wlr_scene_tree *tree) { if (tree) wlr_scene_node_destroy(&tree->node); }
