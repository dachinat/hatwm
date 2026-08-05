/* Native wlroots bridge for the HatWM compositor package. */
#ifndef HATWM_DESKTOP_PROTOCOLS_H
#define HATWM_DESKTOP_PROTOCOLS_H

struct wl_display;
struct wlr_backend;
struct wlr_surface;

struct hatwm_desktop_protocols;

struct hatwm_desktop_protocols *hatwm_desktop_protocols_create(
    struct wl_display *display, struct wlr_backend *backend);
void hatwm_desktop_protocols_destroy(struct hatwm_desktop_protocols *protocols);
void hatwm_desktop_protocols_notify_scale(
    struct hatwm_desktop_protocols *protocols,
    struct wlr_surface *surface,
    double scale);

#endif
