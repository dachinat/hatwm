#define _POSIX_C_SOURCE 200809L

#include "input_protocols.h"

#include <stdlib.h>
#include <wayland-server-core.h>
#include <wlr/types/wlr_cursor.h>
#include <wlr/types/wlr_cursor_shape_v1.h>
#include <wlr/types/wlr_data_control_v1.h>
#include <wlr/types/wlr_idle_inhibit_v1.h>
#include <wlr/types/wlr_idle_notify_v1.h>
#include <wlr/types/wlr_input_device.h>
#include <wlr/types/wlr_keyboard_shortcuts_inhibit_v1.h>
#include <wlr/types/wlr_pointer_constraints_v1.h>
#include <wlr/types/wlr_pointer_gestures_v1.h>
#include <wlr/types/wlr_primary_selection_v1.h>
#include <wlr/types/wlr_relative_pointer_v1.h>
#include <wlr/types/wlr_seat.h>
#include <wlr/types/wlr_virtual_pointer_v1.h>
#include <wlr/types/wlr_xcursor_manager.h>
#include <wlr/util/region.h>

struct hatwm_idle_inhibitor {
    struct hatwm_input_protocols *protocols;
    struct wl_listener destroy;
};

struct hatwm_shortcut_inhibitor {
    struct wl_listener destroy;
};

struct hatwm_constraint {
    struct hatwm_input_protocols *protocols;
    struct wlr_pointer_constraint_v1 *constraint;
    struct wl_listener destroy;
};

struct hatwm_input_protocols {
    struct wlr_cursor *cursor;
    struct wlr_xcursor_manager *cursor_manager;
    struct wlr_seat *seat;
    struct wlr_idle_notifier_v1 *idle_notifier;
    struct wlr_idle_inhibit_manager_v1 *idle_inhibit;
    struct wlr_relative_pointer_manager_v1 *relative_pointer;
    struct wlr_pointer_constraints_v1 *pointer_constraints;
    struct wlr_pointer_constraint_v1 *active_constraint;
    double constraint_sx, constraint_sy;
    struct wlr_cursor_shape_manager_v1 *cursor_shape;
    struct wlr_pointer_gestures_v1 *pointer_gestures;
    struct wlr_virtual_pointer_manager_v1 *virtual_pointer;
    struct wlr_keyboard_shortcuts_inhibit_manager_v1 *shortcut_inhibit;
    bool compositor_cursor_override;
    struct wl_listener new_idle_inhibitor;
    struct wl_listener new_constraint;
    struct wl_listener request_cursor_shape;
    struct wl_listener new_virtual_pointer;
    struct wl_listener new_shortcut_inhibitor;
    struct wl_listener swipe_begin, swipe_update, swipe_end;
    struct wl_listener pinch_begin, pinch_update, pinch_end;
    struct wl_listener hold_begin, hold_end;
};

static void update_idle_inhibition(struct hatwm_input_protocols *protocols) {
    wlr_idle_notifier_v1_set_inhibited(protocols->idle_notifier,
        !wl_list_empty(&protocols->idle_inhibit->inhibitors));
}

static void handle_idle_inhibitor_destroy(
        struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_idle_inhibitor *item =
        wl_container_of(listener, item, destroy);
    struct hatwm_input_protocols *protocols = item->protocols;
    wl_list_remove(&item->destroy.link);
    free(item);
    update_idle_inhibition(protocols);
}

static void handle_new_idle_inhibitor(
        struct wl_listener *listener, void *data) {
    struct hatwm_input_protocols *protocols =
        wl_container_of(listener, protocols, new_idle_inhibitor);
    struct wlr_idle_inhibitor_v1 *inhibitor = data;
    struct hatwm_idle_inhibitor *item = calloc(1, sizeof(*item));
    if (item != NULL) {
        item->protocols = protocols;
        item->destroy.notify = handle_idle_inhibitor_destroy;
        wl_signal_add(&inhibitor->events.destroy, &item->destroy);
    }
    update_idle_inhibition(protocols);
}

static void handle_constraint_destroy(
        struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_constraint *item = wl_container_of(listener, item, destroy);
    if (item->protocols->active_constraint == item->constraint) {
        item->protocols->active_constraint = NULL;
    }
    wl_list_remove(&item->destroy.link);
    free(item);
}

static void handle_new_constraint(struct wl_listener *listener, void *data) {
    struct hatwm_input_protocols *protocols =
        wl_container_of(listener, protocols, new_constraint);
    struct wlr_pointer_constraint_v1 *constraint = data;
    struct hatwm_constraint *item = calloc(1, sizeof(*item));
    if (item != NULL) {
        item->protocols = protocols;
        item->constraint = constraint;
        item->destroy.notify = handle_constraint_destroy;
        wl_signal_add(&constraint->events.destroy, &item->destroy);
    }
    if (protocols->seat->pointer_state.focused_surface == constraint->surface) {
        if (protocols->active_constraint != NULL) {
            wlr_pointer_constraint_v1_send_deactivated(
                protocols->active_constraint);
        }
        protocols->active_constraint = constraint;
        wlr_pointer_constraint_v1_send_activated(constraint);
    }
}

static void handle_request_cursor_shape(
        struct wl_listener *listener, void *data) {
    struct hatwm_input_protocols *protocols =
        wl_container_of(listener, protocols, request_cursor_shape);
    struct wlr_cursor_shape_manager_v1_request_set_shape_event *event = data;
    if (protocols->compositor_cursor_override ||
        event->device_type != WLR_CURSOR_SHAPE_MANAGER_V1_DEVICE_TYPE_POINTER ||
        event->seat_client != protocols->seat->pointer_state.focused_client) {
        return;
    }
    const char *name = wlr_cursor_shape_v1_name(event->shape);
    if (name != NULL) {
        wlr_cursor_set_xcursor(protocols->cursor, protocols->cursor_manager, name);
    }
}

static void handle_new_virtual_pointer(
        struct wl_listener *listener, void *data) {
    struct hatwm_input_protocols *protocols =
        wl_container_of(listener, protocols, new_virtual_pointer);
    struct wlr_virtual_pointer_v1_new_pointer_event *event = data;
    if (event != NULL && event->new_pointer != NULL &&
        (event->suggested_seat == NULL || event->suggested_seat == protocols->seat)) {
        wlr_cursor_attach_input_device(
            protocols->cursor, &event->new_pointer->pointer.base);
    }
}

static void handle_shortcut_inhibitor_destroy(
        struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_shortcut_inhibitor *item =
        wl_container_of(listener, item, destroy);
    wl_list_remove(&item->destroy.link);
    free(item);
}

static void handle_new_shortcut_inhibitor(
        struct wl_listener *listener, void *data) {
    (void)listener;
    struct wlr_keyboard_shortcuts_inhibitor_v1 *inhibitor = data;
    struct hatwm_shortcut_inhibitor *item = calloc(1, sizeof(*item));
    if (item == NULL) {
        return;
    }
    item->destroy.notify = handle_shortcut_inhibitor_destroy;
    wl_signal_add(&inhibitor->events.destroy, &item->destroy);
    wlr_keyboard_shortcuts_inhibitor_v1_activate(inhibitor);
}

static void handle_swipe_begin(struct wl_listener *listener, void *data) {
    struct hatwm_input_protocols *p = wl_container_of(listener, p, swipe_begin);
    struct wlr_pointer_swipe_begin_event *e = data;
    wlr_idle_notifier_v1_notify_activity(p->idle_notifier, p->seat);
    wlr_pointer_gestures_v1_send_swipe_begin(
        p->pointer_gestures, p->seat, e->time_msec, e->fingers);
}

static void handle_swipe_update(struct wl_listener *listener, void *data) {
    struct hatwm_input_protocols *p = wl_container_of(listener, p, swipe_update);
    struct wlr_pointer_swipe_update_event *e = data;
    wlr_idle_notifier_v1_notify_activity(p->idle_notifier, p->seat);
    wlr_pointer_gestures_v1_send_swipe_update(
        p->pointer_gestures, p->seat, e->time_msec, e->dx, e->dy);
}

static void handle_swipe_end(struct wl_listener *listener, void *data) {
    struct hatwm_input_protocols *p = wl_container_of(listener, p, swipe_end);
    struct wlr_pointer_swipe_end_event *e = data;
    wlr_pointer_gestures_v1_send_swipe_end(
        p->pointer_gestures, p->seat, e->time_msec, e->cancelled);
}

static void handle_pinch_begin(struct wl_listener *listener, void *data) {
    struct hatwm_input_protocols *p = wl_container_of(listener, p, pinch_begin);
    struct wlr_pointer_pinch_begin_event *e = data;
    wlr_idle_notifier_v1_notify_activity(p->idle_notifier, p->seat);
    wlr_pointer_gestures_v1_send_pinch_begin(
        p->pointer_gestures, p->seat, e->time_msec, e->fingers);
}

static void handle_pinch_update(struct wl_listener *listener, void *data) {
    struct hatwm_input_protocols *p = wl_container_of(listener, p, pinch_update);
    struct wlr_pointer_pinch_update_event *e = data;
    wlr_idle_notifier_v1_notify_activity(p->idle_notifier, p->seat);
    wlr_pointer_gestures_v1_send_pinch_update(
        p->pointer_gestures, p->seat, e->time_msec,
        e->dx, e->dy, e->scale, e->rotation);
}

static void handle_pinch_end(struct wl_listener *listener, void *data) {
    struct hatwm_input_protocols *p = wl_container_of(listener, p, pinch_end);
    struct wlr_pointer_pinch_end_event *e = data;
    wlr_pointer_gestures_v1_send_pinch_end(
        p->pointer_gestures, p->seat, e->time_msec, e->cancelled);
}

static void handle_hold_begin(struct wl_listener *listener, void *data) {
    struct hatwm_input_protocols *p = wl_container_of(listener, p, hold_begin);
    struct wlr_pointer_hold_begin_event *e = data;
    wlr_idle_notifier_v1_notify_activity(p->idle_notifier, p->seat);
    wlr_pointer_gestures_v1_send_hold_begin(
        p->pointer_gestures, p->seat, e->time_msec, e->fingers);
}

static void handle_hold_end(struct wl_listener *listener, void *data) {
    struct hatwm_input_protocols *p = wl_container_of(listener, p, hold_end);
    struct wlr_pointer_hold_end_event *e = data;
    wlr_pointer_gestures_v1_send_hold_end(
        p->pointer_gestures, p->seat, e->time_msec, e->cancelled);
}

struct hatwm_input_protocols *hatwm_input_protocols_create(
        struct wl_display *display,
        struct wlr_cursor *cursor,
        struct wlr_xcursor_manager *cursor_manager,
        struct wlr_seat *seat) {
    if (display == NULL || cursor == NULL || cursor_manager == NULL || seat == NULL) {
        return NULL;
    }
    struct hatwm_input_protocols *p = calloc(1, sizeof(*p));
    if (p == NULL) {
        return NULL;
    }
    p->cursor = cursor;
    p->cursor_manager = cursor_manager;
    p->seat = seat;
    p->idle_notifier = wlr_idle_notifier_v1_create(display);
    p->idle_inhibit = wlr_idle_inhibit_v1_create(display);
    p->relative_pointer = wlr_relative_pointer_manager_v1_create(display);
    p->pointer_constraints = wlr_pointer_constraints_v1_create(display);
    p->cursor_shape = wlr_cursor_shape_manager_v1_create(display, 1);
    p->pointer_gestures = wlr_pointer_gestures_v1_create(display);
    p->virtual_pointer = wlr_virtual_pointer_manager_v1_create(display);
    p->shortcut_inhibit = wlr_keyboard_shortcuts_inhibit_v1_create(display);
    bool failed = p->idle_notifier == NULL || p->idle_inhibit == NULL ||
        p->relative_pointer == NULL || p->pointer_constraints == NULL ||
        p->cursor_shape == NULL || p->pointer_gestures == NULL ||
        p->virtual_pointer == NULL || p->shortcut_inhibit == NULL ||
        wlr_primary_selection_v1_device_manager_create(display) == NULL ||
        wlr_data_control_manager_v1_create(display) == NULL;
    if (failed) {
        free(p);
        return NULL;
    }

#define LISTEN(object, signal, member, handler) \
    do { p->member.notify = handler; wl_signal_add(&object->events.signal, &p->member); } while (0)
    LISTEN(p->idle_inhibit, new_inhibitor, new_idle_inhibitor, handle_new_idle_inhibitor);
    LISTEN(p->pointer_constraints, new_constraint, new_constraint, handle_new_constraint);
    LISTEN(p->cursor_shape, request_set_shape, request_cursor_shape, handle_request_cursor_shape);
    LISTEN(p->virtual_pointer, new_virtual_pointer, new_virtual_pointer, handle_new_virtual_pointer);
    LISTEN(p->shortcut_inhibit, new_inhibitor, new_shortcut_inhibitor, handle_new_shortcut_inhibitor);
    LISTEN(p->cursor, swipe_begin, swipe_begin, handle_swipe_begin);
    LISTEN(p->cursor, swipe_update, swipe_update, handle_swipe_update);
    LISTEN(p->cursor, swipe_end, swipe_end, handle_swipe_end);
    LISTEN(p->cursor, pinch_begin, pinch_begin, handle_pinch_begin);
    LISTEN(p->cursor, pinch_update, pinch_update, handle_pinch_update);
    LISTEN(p->cursor, pinch_end, pinch_end, handle_pinch_end);
    LISTEN(p->cursor, hold_begin, hold_begin, handle_hold_begin);
    LISTEN(p->cursor, hold_end, hold_end, handle_hold_end);
#undef LISTEN
    return p;
}

void hatwm_input_protocols_destroy(struct hatwm_input_protocols *p) {
    if (p == NULL) {
        return;
    }
    wl_list_remove(&p->new_idle_inhibitor.link);
    wl_list_remove(&p->new_constraint.link);
    wl_list_remove(&p->request_cursor_shape.link);
    wl_list_remove(&p->new_virtual_pointer.link);
    wl_list_remove(&p->new_shortcut_inhibitor.link);
    wl_list_remove(&p->swipe_begin.link);
    wl_list_remove(&p->swipe_update.link);
    wl_list_remove(&p->swipe_end.link);
    wl_list_remove(&p->pinch_begin.link);
    wl_list_remove(&p->pinch_update.link);
    wl_list_remove(&p->pinch_end.link);
    wl_list_remove(&p->hold_begin.link);
    wl_list_remove(&p->hold_end.link);
    free(p);
}

void hatwm_input_protocols_notify_activity(struct hatwm_input_protocols *p) {
    if (p != NULL) {
        wlr_idle_notifier_v1_notify_activity(p->idle_notifier, p->seat);
    }
}

void hatwm_input_protocols_set_cursor_override(
        struct hatwm_input_protocols *p, bool enabled) {
    if (p != NULL) {
        p->compositor_cursor_override = enabled;
    }
}

void hatwm_input_protocols_set_cursor_manager(
        struct hatwm_input_protocols *p,
        struct wlr_xcursor_manager *manager) {
    if (p != NULL && manager != NULL) {
        p->cursor_manager = manager;
    }
}

bool hatwm_input_protocols_handle_relative_motion(
        struct hatwm_input_protocols *p,
        struct wlr_input_device *device,
        uint64_t time_usec,
        double dx,
        double dy,
        double dx_unaccel,
        double dy_unaccel) {
    if (p == NULL) {
        return false;
    }
    wlr_relative_pointer_manager_v1_send_relative_motion(
        p->relative_pointer, p->seat, time_usec,
        dx, dy, dx_unaccel, dy_unaccel);
    if (p->active_constraint == NULL) {
        return false;
    }
    if (p->active_constraint->type == WLR_POINTER_CONSTRAINT_V1_LOCKED) {
        return true;
    }
    double x = p->constraint_sx;
    double y = p->constraint_sy;
    if (wlr_region_confine(&p->active_constraint->region,
            p->constraint_sx, p->constraint_sy,
            p->constraint_sx + dx, p->constraint_sy + dy, &x, &y)) {
        wlr_cursor_move(p->cursor, device,
            x - p->constraint_sx, y - p->constraint_sy);
        p->constraint_sx = x;
        p->constraint_sy = y;
    }
    return true;
}

void hatwm_input_protocols_pointer_focus(
        struct hatwm_input_protocols *p,
        struct wlr_surface *surface,
        double sx,
        double sy) {
    if (p == NULL) {
        return;
    }
    struct wlr_pointer_constraint_v1 *next = surface == NULL ? NULL :
        wlr_pointer_constraints_v1_constraint_for_surface(
            p->pointer_constraints, surface, p->seat);
    if (next != p->active_constraint) {
        if (p->active_constraint != NULL) {
            wlr_pointer_constraint_v1_send_deactivated(p->active_constraint);
        }
        p->active_constraint = next;
        if (next != NULL) {
            wlr_pointer_constraint_v1_send_activated(next);
        }
    }
    p->constraint_sx = sx;
    p->constraint_sy = sy;
}

bool hatwm_input_protocols_pointer_locked(struct hatwm_input_protocols *p) {
    return p != NULL && p->active_constraint != NULL &&
        p->active_constraint->type == WLR_POINTER_CONSTRAINT_V1_LOCKED;
}

bool hatwm_input_protocols_shortcuts_inhibited(struct hatwm_input_protocols *p) {
    if (p == NULL || p->seat->keyboard_state.focused_surface == NULL) {
        return false;
    }
    struct wlr_keyboard_shortcuts_inhibitor_v1 *inhibitor;
    wl_list_for_each(inhibitor, &p->shortcut_inhibit->inhibitors, link) {
        if (inhibitor->active && inhibitor->seat == p->seat &&
            inhibitor->surface == p->seat->keyboard_state.focused_surface) {
            return true;
        }
    }
    return false;
}
