/* Native wlroots bridge for xdg-popup events missing from go-wlroots. */
#include "popup_listener.h"

#include <stdlib.h>
#include <wayland-server-core.h>

extern void hatwmGoPopupReposition(uintptr_t token);
extern void hatwmGoPopupListenerDestroy(uintptr_t token);

struct hatwm_popup_listener {
    struct wl_listener reposition;
    struct wl_listener destroy;
    uintptr_t token;
};

static void handle_popup_reposition(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_popup_listener *state =
        wl_container_of(listener, state, reposition);
    hatwmGoPopupReposition(state->token);
}

static void handle_popup_destroy(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_popup_listener *state =
        wl_container_of(listener, state, destroy);
    wl_list_remove(&state->reposition.link);
    wl_list_remove(&state->destroy.link);
    hatwmGoPopupListenerDestroy(state->token);
    free(state);
}

struct hatwm_popup_listener *hatwm_xdg_popup_listen(
        struct wlr_xdg_popup *popup, uintptr_t token) {
    if (popup == NULL) {
        return NULL;
    }

    struct hatwm_popup_listener *state = calloc(1, sizeof(*state));
    if (state == NULL) {
        return NULL;
    }
    state->token = token;
    state->reposition.notify = handle_popup_reposition;
    wl_signal_add(&popup->events.reposition, &state->reposition);
    state->destroy.notify = handle_popup_destroy;
    wl_signal_add(&popup->events.destroy, &state->destroy);
    return state;
}
