/* Native wlroots bridge for the HatWM compositor package. */
#ifndef HATWM_INPUT_PROTOCOLS_H
#define HATWM_INPUT_PROTOCOLS_H

#include <stdbool.h>
#include <stdint.h>

struct wl_display;
struct wlr_cursor;
struct wlr_input_device;
struct wlr_seat;
struct wlr_surface;
struct wlr_xcursor_manager;

struct hatwm_input_protocols;

struct hatwm_input_protocols *hatwm_input_protocols_create(
    struct wl_display *display,
    struct wlr_cursor *cursor,
    struct wlr_xcursor_manager *cursor_manager,
    struct wlr_seat *seat);
void hatwm_input_protocols_destroy(struct hatwm_input_protocols *protocols);
void hatwm_input_protocols_notify_activity(struct hatwm_input_protocols *protocols);
void hatwm_input_protocols_set_cursor_override(
    struct hatwm_input_protocols *protocols, bool enabled);
void hatwm_input_protocols_set_cursor_manager(
    struct hatwm_input_protocols *protocols,
    struct wlr_xcursor_manager *manager);
bool hatwm_input_protocols_handle_relative_motion(
    struct hatwm_input_protocols *protocols,
    struct wlr_input_device *device,
    uint64_t time_usec,
    double dx,
    double dy,
    double dx_unaccel,
    double dy_unaccel);
void hatwm_input_protocols_pointer_focus(
    struct hatwm_input_protocols *protocols,
    struct wlr_surface *surface,
    double sx,
    double sy);
bool hatwm_input_protocols_pointer_locked(struct hatwm_input_protocols *protocols);
bool hatwm_input_protocols_shortcuts_inhibited(
    struct hatwm_input_protocols *protocols);

#endif
