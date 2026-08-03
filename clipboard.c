#define _POSIX_C_SOURCE 200809L
#include <stdlib.h>
#include "clipboard.h"

#include <wlr/types/wlr_data_device.h>
#include <wlr/types/wlr_primary_selection.h>
#include <wlr/types/wlr_seat.h>

struct hatwm_clipboard {
    struct wlr_seat *seat;
    struct wl_listener request_set_selection;
    struct wl_listener request_set_primary_selection;
    struct wl_listener request_start_drag;
};

static void handle_request_set_selection(struct wl_listener *listener, void *data) {
    struct hatwm_clipboard *clipboard =
        wl_container_of(listener, clipboard, request_set_selection);
    struct wlr_seat_request_set_selection_event *event = data;

    if (clipboard == NULL || clipboard->seat == NULL || event == NULL) {
        return;
    }

    /*
     * wl_data_device selection ownership is compositor-mediated. Accept the
     * client's requested source and serial as the seat's current selection.
     * wlroots then handles MIME negotiation and data transfer between clients.
     */
    wlr_seat_set_selection(clipboard->seat, event->source, event->serial);
}

static void handle_request_start_drag(
        struct wl_listener *listener, void *data) {
    struct hatwm_clipboard *clipboard =
        wl_container_of(listener, clipboard, request_start_drag);
    struct wlr_seat_request_start_drag_event *event = data;
    if (clipboard == NULL || clipboard->seat == NULL || event == NULL ||
        event->drag == NULL) {
        return;
    }
    if (wlr_seat_validate_pointer_grab_serial(
            clipboard->seat, event->origin, event->serial)) {
        wlr_seat_start_pointer_drag(
            clipboard->seat, event->drag, event->serial);
        return;
    }
    struct wlr_touch_point *point = NULL;
    if (wlr_seat_validate_touch_grab_serial(
            clipboard->seat, event->origin, event->serial, &point)) {
        wlr_seat_start_touch_drag(
            clipboard->seat, event->drag, event->serial, point);
        return;
    }
    wlr_data_source_destroy(event->drag->source);
}

static void handle_request_set_primary_selection(
        struct wl_listener *listener, void *data) {
    struct hatwm_clipboard *clipboard = wl_container_of(
        listener, clipboard, request_set_primary_selection);
    struct wlr_seat_request_set_primary_selection_event *event = data;
    if (clipboard == NULL || clipboard->seat == NULL || event == NULL) {
        return;
    }
    wlr_seat_set_primary_selection(
        clipboard->seat, event->source, event->serial);
}

struct hatwm_clipboard *hatwm_clipboard_create(struct wlr_seat *seat) {
    if (seat == NULL) {
        return NULL;
    }

    struct hatwm_clipboard *clipboard = calloc(1, sizeof(*clipboard));
    if (clipboard == NULL) {
        return NULL;
    }

    clipboard->seat = seat;
    clipboard->request_set_selection.notify = handle_request_set_selection;
    wl_signal_add(&seat->events.request_set_selection,
        &clipboard->request_set_selection);
    clipboard->request_set_primary_selection.notify =
        handle_request_set_primary_selection;
    wl_signal_add(&seat->events.request_set_primary_selection,
        &clipboard->request_set_primary_selection);
    clipboard->request_start_drag.notify = handle_request_start_drag;
    wl_signal_add(&seat->events.request_start_drag,
        &clipboard->request_start_drag);

    return clipboard;
}

void hatwm_clipboard_destroy(struct hatwm_clipboard *clipboard) {
    if (clipboard == NULL) {
        return;
    }

    wl_list_remove(&clipboard->request_set_selection.link);
    wl_list_remove(&clipboard->request_set_primary_selection.link);
    wl_list_remove(&clipboard->request_start_drag.link);
    free(clipboard);
}
