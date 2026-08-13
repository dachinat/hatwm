/* Native wlroots bridge for the HatWM compositor package. */
#include "maximize.h"

#include <stdlib.h>
#include <wayland-server-core.h>

extern void hatwmGoRequestMaximize(uintptr_t token, bool maximized);
extern void hatwmGoRequestFullscreen(uintptr_t token, bool fullscreen);
extern void hatwmGoRequestMinimize(uintptr_t token);
extern void hatwmGoWindowMetadataChanged(uintptr_t token);
extern void hatwmGoWindowParentChanged(uintptr_t token);
extern void hatwmGoMaximizeListenerDestroy(uintptr_t token);

struct hatwm_maximize_listener {
    struct wl_listener request_maximize;
    struct wl_listener request_fullscreen;
    struct wl_listener request_minimize;
    struct wl_listener set_title;
    struct wl_listener set_app_id;
    struct wl_listener set_parent;
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

static void handle_request_fullscreen(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_maximize_listener *state =
        wl_container_of(listener, state, request_fullscreen);
    hatwmGoRequestFullscreen(
        state->token, state->toplevel->requested.fullscreen);
}

static void handle_request_minimize(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_maximize_listener *state =
        wl_container_of(listener, state, request_minimize);
    hatwmGoRequestMinimize(state->token);
}

static void handle_metadata_changed(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_maximize_listener *state =
        wl_container_of(listener, state, set_title);
    hatwmGoWindowMetadataChanged(state->token);
}

static void handle_app_id_changed(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_maximize_listener *state =
        wl_container_of(listener, state, set_app_id);
    hatwmGoWindowMetadataChanged(state->token);
}

static void handle_parent_changed(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_maximize_listener *state =
        wl_container_of(listener, state, set_parent);
    hatwmGoWindowParentChanged(state->token);
}

static void handle_toplevel_destroy(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_maximize_listener *state =
        wl_container_of(listener, state, destroy);
    wl_list_remove(&state->request_maximize.link);
    wl_list_remove(&state->request_fullscreen.link);
    wl_list_remove(&state->request_minimize.link);
    wl_list_remove(&state->set_title.link);
    wl_list_remove(&state->set_app_id.link);
    wl_list_remove(&state->set_parent.link);
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
    state->request_fullscreen.notify = handle_request_fullscreen;
    wl_signal_add(
        &toplevel->events.request_fullscreen, &state->request_fullscreen);
    state->request_minimize.notify = handle_request_minimize;
    wl_signal_add(&toplevel->events.request_minimize, &state->request_minimize);
    state->set_title.notify = handle_metadata_changed;
    wl_signal_add(&toplevel->events.set_title, &state->set_title);
    state->set_app_id.notify = handle_app_id_changed;
    wl_signal_add(&toplevel->events.set_app_id, &state->set_app_id);
    state->set_parent.notify = handle_parent_changed;
    wl_signal_add(&toplevel->events.set_parent, &state->set_parent);
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
    wl_list_remove(&listener->request_fullscreen.link);
    wl_list_remove(&listener->request_minimize.link);
    wl_list_remove(&listener->set_title.link);
    wl_list_remove(&listener->set_app_id.link);
    wl_list_remove(&listener->set_parent.link);
    wl_list_remove(&listener->destroy.link);
    free(listener);
}

void hatwm_xdg_toplevel_set_window_state(
    struct wlr_xdg_toplevel *toplevel,
    bool maximized,
    bool fullscreen) {
    if (toplevel == NULL) {
        return;
    }
    wlr_xdg_toplevel_set_maximized(toplevel, maximized);
    wlr_xdg_toplevel_set_fullscreen(toplevel, fullscreen);
}

void hatwm_xdg_toplevel_set_supported_capabilities(
        struct wlr_xdg_toplevel *toplevel) {
    if (toplevel == NULL) {
        return;
    }
    // Minimize requests are represented by stashing the toplevel in The Hat.
    wlr_xdg_toplevel_set_wm_capabilities(toplevel,
        WLR_XDG_TOPLEVEL_WM_CAPABILITIES_MAXIMIZE |
        WLR_XDG_TOPLEVEL_WM_CAPABILITIES_MINIMIZE |
        WLR_XDG_TOPLEVEL_WM_CAPABILITIES_FULLSCREEN);
}
