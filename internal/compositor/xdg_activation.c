/* Native wlroots bridge for the HatWM compositor package. */
#include "xdg_activation.h"

#include <stdlib.h>
#include <wlr/types/wlr_xdg_activation_v1.h>

extern void hatwmGoXDGRequestActivate(void *surface);

struct hatwm_xdg_activation {
    struct wlr_xdg_activation_v1 *manager;
    struct wl_listener request_activate;
};

static void handle_request_activate(
        struct wl_listener *listener, void *data) {
    (void)listener;
    struct wlr_xdg_activation_v1_request_activate_event *event = data;
    if (event != NULL && event->surface != NULL) {
        hatwmGoXDGRequestActivate(event->surface);
    }
}

struct hatwm_xdg_activation *hatwm_xdg_activation_create(
        struct wl_display *display) {
    struct hatwm_xdg_activation *state = calloc(1, sizeof(*state));
    if (state == NULL) {
        return NULL;
    }
    state->manager = wlr_xdg_activation_v1_create(display);
    if (state->manager == NULL) {
        free(state);
        return NULL;
    }
    state->request_activate.notify = handle_request_activate;
    wl_signal_add(
        &state->manager->events.request_activate,
        &state->request_activate);
    return state;
}

void hatwm_xdg_activation_destroy(
        struct hatwm_xdg_activation *state) {
    if (state == NULL) {
        return;
    }
    wl_list_remove(&state->request_activate.link);
    free(state);
}
