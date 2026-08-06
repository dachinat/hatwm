/* Native wlroots bridge for the HatWM compositor package. */
#ifndef HATWM_MAXIMIZE_H
#define HATWM_MAXIMIZE_H

#include <stdbool.h>
#include <stdint.h>
#include <wlr/types/wlr_xdg_shell.h>

struct hatwm_maximize_listener;

struct hatwm_maximize_listener *hatwm_xdg_toplevel_listen_maximize(
    struct wlr_xdg_toplevel *toplevel,
    uintptr_t token);
void hatwm_xdg_toplevel_unlisten_maximize(
    struct hatwm_maximize_listener *listener);
void hatwm_xdg_toplevel_set_window_state(
    struct wlr_xdg_toplevel *toplevel,
    bool maximized,
    bool fullscreen);
void hatwm_xdg_toplevel_set_supported_capabilities(
    struct wlr_xdg_toplevel *toplevel);

#endif
