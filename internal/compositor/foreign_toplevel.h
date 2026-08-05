/* Native wlroots bridge for the HatWM compositor package. */
#ifndef HATWM_FOREIGN_TOPLEVEL_H
#define HATWM_FOREIGN_TOPLEVEL_H

struct wl_display;
struct wlr_ext_foreign_toplevel_handle_v1;

struct hatwm_foreign_toplevels;

struct hatwm_foreign_toplevels *hatwm_foreign_toplevels_create(
    struct wl_display *display);
void hatwm_foreign_toplevels_destroy(struct hatwm_foreign_toplevels *state);

struct wlr_ext_foreign_toplevel_handle_v1 *hatwm_foreign_toplevel_create(
    struct hatwm_foreign_toplevels *state,
    const char *title,
    const char *app_id);
void hatwm_foreign_toplevel_update(
    struct wlr_ext_foreign_toplevel_handle_v1 *handle,
    const char *title,
    const char *app_id);
void hatwm_foreign_toplevel_destroy(
    struct wlr_ext_foreign_toplevel_handle_v1 *handle);

#endif
