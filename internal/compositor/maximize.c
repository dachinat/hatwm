/* Native wlroots bridge for the HatWM compositor package. */
#include "maximize.h"

#include <stdlib.h>
#include <wayland-server-core.h>

extern void hatwmGoRequestMaximize(uintptr_t token, bool maximized);
extern void hatwmGoMaximizeListenerDestroy(uintptr_t token);

struct hatwm_maximize_listener {
    struct wl_listener request_maximize;
    struct wl_listener destroy;
    struct wlr_xdg_toplevel *toplevel;
    uintptr_t token;
};

static void handle_request_maximize(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_maximize_listener *state =
        wl_container_of(listener, state, request_maximize);
    hatwmGoRequestMaximize(state->token, state->toplevel->requested.maximized);
}

static void handle_toplevel_destroy(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_maximize_listener *state =
        wl_container_of(listener, state, destroy);
    wl_list_remove(&state->request_maximize.link);
    wl_list_remove(&state->destroy.link);
    hatwmGoMaximizeListenerDestroy(state->token);
    free(state);
}

struct hatwm_maximize_listener *hatwm_xdg_toplevel_listen_maximize(
    struct wlr_xdg_toplevel *toplevel,
    uintptr_t token) {
    if (toplevel == NULL) {
        return NULL;
    }

    struct hatwm_maximize_listener *state = calloc(1, sizeof(*state));
    if (state == NULL) {
        return NULL;
    }

    state->toplevel = toplevel;
    state->token = token;
    state->request_maximize.notify = handle_request_maximize;
    wl_signal_add(&toplevel->events.request_maximize, &state->request_maximize);
    state->destroy.notify = handle_toplevel_destroy;
    wl_signal_add(&toplevel->events.destroy, &state->destroy);
    return state;
}

void hatwm_xdg_toplevel_unlisten_maximize(
    struct hatwm_maximize_listener *listener) {
    if (listener == NULL) {
        return;
    }
    wl_list_remove(&listener->request_maximize.link);
    wl_list_remove(&listener->destroy.link);
    free(listener);
}

void hatwm_xdg_toplevel_set_window_state(
    struct wlr_xdg_toplevel *toplevel,
    bool fullscreen) {
    if (toplevel == NULL) {
        return;
    }
    wlr_xdg_toplevel_set_maximized(toplevel, fullscreen);
    wlr_xdg_toplevel_set_fullscreen(toplevel, fullscreen);
}

void hatwm_xdg_toplevel_set_supported_capabilities(
        struct wlr_xdg_toplevel *toplevel) {
    if (toplevel == NULL) {
        return;
    }
    // HatWM supports maximizing (as its fullscreen tiling state) and regular
    // fullscreen, but has no minimized-window state. Omitting MINIMIZE tells
    // clients with CSDs not to draw a non-functional minimize button.
    wlr_xdg_toplevel_set_wm_capabilities(toplevel,
        WLR_XDG_TOPLEVEL_WM_CAPABILITIES_MAXIMIZE |
        WLR_XDG_TOPLEVEL_WM_CAPABILITIES_FULLSCREEN);
}
