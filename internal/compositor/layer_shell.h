/* Native wlroots bridge for the HatWM compositor package. */
#ifndef HATWM_LAYER_SHELL_H
#define HATWM_LAYER_SHELL_H

#include <stdbool.h>
#include <stdint.h>
#include <wayland-server-core.h>
#include <wlr/types/wlr_layer_shell_v1.h>
#include <wlr/types/wlr_scene.h>
#include <wlr/types/wlr_output_layout.h>
#include <wlr/types/wlr_xdg_output_v1.h>
#include <wlr/types/wlr_xdg_shell.h>

struct hatwm_layer_shell;

struct hatwm_layer_shell *hatwm_layer_shell_create(struct wl_display *display);
struct wlr_xdg_output_manager_v1 *hatwm_xdg_output_manager_create(struct wl_display *display, struct wlr_output_layout *layout);
void hatwm_layer_shell_destroy(struct hatwm_layer_shell *shell);

struct wlr_surface *hatwm_layer_surface_surface(struct wlr_layer_surface_v1 *layer);
struct wlr_output *hatwm_layer_surface_output(struct wlr_layer_surface_v1 *layer);
void hatwm_layer_surface_set_output(struct wlr_layer_surface_v1 *layer, struct wlr_output *output);
uint32_t hatwm_layer_surface_layer(struct wlr_layer_surface_v1 *layer);
uint32_t hatwm_layer_surface_anchor(struct wlr_layer_surface_v1 *layer);
uint32_t hatwm_layer_surface_desired_width(struct wlr_layer_surface_v1 *layer);
uint32_t hatwm_layer_surface_desired_height(struct wlr_layer_surface_v1 *layer);
int32_t hatwm_layer_surface_exclusive_zone(struct wlr_layer_surface_v1 *layer);
int32_t hatwm_layer_surface_margin_top(struct wlr_layer_surface_v1 *layer);
int32_t hatwm_layer_surface_margin_right(struct wlr_layer_surface_v1 *layer);
int32_t hatwm_layer_surface_margin_bottom(struct wlr_layer_surface_v1 *layer);
int32_t hatwm_layer_surface_margin_left(struct wlr_layer_surface_v1 *layer);
uint32_t hatwm_layer_surface_keyboard_interactive(struct wlr_layer_surface_v1 *layer);
bool hatwm_layer_surface_mapped(struct wlr_layer_surface_v1 *layer);
uint32_t hatwm_layer_surface_configure(struct wlr_layer_surface_v1 *layer, uint32_t width, uint32_t height);

struct wlr_scene_tree *hatwm_layer_scene_create(struct wlr_scene_tree *parent, struct wlr_surface *surface);
void hatwm_layer_surface_set_scene_tree(struct wlr_layer_surface_v1 *layer, struct wlr_scene_tree *tree);
void hatwm_scene_tree_set_position(struct wlr_scene_tree *tree, int x, int y);
void hatwm_scene_tree_set_enabled(struct wlr_scene_tree *tree, bool enabled);
void hatwm_scene_tree_destroy(struct wlr_scene_tree *tree);

#endif
