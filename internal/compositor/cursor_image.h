/* Native wlroots bridge for the HatWM compositor package. */
#ifndef HATWM_CURSOR_IMAGE_H
#define HATWM_CURSOR_IMAGE_H

#include <stdbool.h>

struct wlr_cursor;
struct wlr_xcursor_manager;

bool hatwm_cursor_manager_load(
    struct wlr_xcursor_manager *manager, float scale);
bool hatwm_cursor_set_named(
    struct wlr_cursor *cursor,
    struct wlr_xcursor_manager *manager,
    const char *name,
    float scale);

#endif
