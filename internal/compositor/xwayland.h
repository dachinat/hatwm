/* Native wlroots bridge for the HatWM compositor package. */
#ifndef HATWM_XWAYLAND_H
#define HATWM_XWAYLAND_H

#include <stdbool.h>
#include <stdint.h>
#include <wayland-server-core.h>
#include <wlr/types/wlr_compositor.h>
#include <wlr/types/wlr_scene.h>
#include <wlr/types/wlr_seat.h>
#include <wlr/types/wlr_xcursor_manager.h>
#include <wlr/xwayland.h>

struct hatwm_xwayland;

struct hatwm_xwayland *hatwm_xwayland_create(
    struct wl_display *display,
    struct wlr_compositor *compositor,
    bool lazy);
void hatwm_xwayland_destroy(struct hatwm_xwayland *xwayland);
void hatwm_xwayland_set_seat(
    struct hatwm_xwayland *xwayland,
    struct wlr_seat *seat);
void hatwm_xwayland_set_cursor(
    struct hatwm_xwayland *xwayland,
    struct wlr_xcursor_manager *manager,
    const char *name,
    float scale);
const char *hatwm_xwayland_display_name(
    struct hatwm_xwayland *xwayland);

struct wlr_surface *hatwm_xwayland_surface_surface(
    struct wlr_xwayland_surface *surface);
bool hatwm_xwayland_surface_override_redirect(
    struct wlr_xwayland_surface *surface);
bool hatwm_xwayland_surface_wants_focus(
    struct wlr_xwayland_surface *surface);
int hatwm_xwayland_surface_x(struct wlr_xwayland_surface *surface);
int hatwm_xwayland_surface_y(struct wlr_xwayland_surface *surface);
int hatwm_xwayland_surface_width(struct wlr_xwayland_surface *surface);
int hatwm_xwayland_surface_height(struct wlr_xwayland_surface *surface);

void hatwm_xwayland_surface_configure(
    struct wlr_xwayland_surface *surface,
    int x, int y, int width, int height);
void hatwm_xwayland_surface_activate(
    struct wlr_xwayland_surface *surface,
    bool activated);
void hatwm_xwayland_surface_close(
    struct wlr_xwayland_surface *surface);
void hatwm_xwayland_surface_set_window_state(
    struct wlr_xwayland_surface *surface,
    bool maximized,
    bool fullscreen);
void hatwm_xwayland_surface_set_rounded_clip(
    struct hatwm_xwayland *xwayland,
    struct wlr_xwayland_surface *surface,
    int radius);

#endif
