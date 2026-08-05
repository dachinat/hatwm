/* Native wlroots bridge for the HatWM compositor package. */
#ifndef HATWM_XDG_ACTIVATION_H
#define HATWM_XDG_ACTIVATION_H

#include <wayland-server-core.h>

struct hatwm_xdg_activation;

struct hatwm_xdg_activation *hatwm_xdg_activation_create(
    struct wl_display *display);
void hatwm_xdg_activation_destroy(
    struct hatwm_xdg_activation *activation);

#endif
