#include "cursor_image.h"

#include <string.h>
#include <wlr/types/wlr_cursor.h>
#include <wlr/types/wlr_xcursor_manager.h>

bool hatwm_cursor_manager_load(
        struct wlr_xcursor_manager *manager, float scale) {
    return manager != NULL && wlr_xcursor_manager_load(manager, scale);
}

static const char *find_cursor_name(
        struct wlr_xcursor_manager *manager,
        const char *name,
        float scale) {
    // Adwaita provides "move", but aliases it to the ordinary arrow. Prefer
    // the traditional four-direction cursor so window movement is visible.
    if (strcmp(name, "move") == 0) {
        static const char *move_names[] = {
            "fleur", "all-scroll", "grabbing", "grab", "move",
        };
        for (size_t i = 0; i < sizeof(move_names) / sizeof(move_names[0]); ++i) {
            if (wlr_xcursor_manager_get_xcursor(
                    manager, move_names[i], scale) != NULL) {
                return move_names[i];
            }
        }
        return NULL;
    }
    if (wlr_xcursor_manager_get_xcursor(manager, name, scale) != NULL) {
        return name;
    }
    const char *fallback = NULL;
    if (strcmp(name, "nwse-resize") == 0) {
        fallback = "se-resize";
    } else if (strcmp(name, "nesw-resize") == 0) {
        fallback = "ne-resize";
    }
    if (fallback != NULL &&
        wlr_xcursor_manager_get_xcursor(manager, fallback, scale) != NULL) {
        return fallback;
    }
    return NULL;
}

bool hatwm_cursor_set_named(
        struct wlr_cursor *cursor,
        struct wlr_xcursor_manager *manager,
        const char *name,
        float scale) {
    if (cursor == NULL || manager == NULL || name == NULL) {
        return false;
    }
    const char *resolved = find_cursor_name(manager, name, scale);
    if (resolved == NULL) {
        return false;
    }
    // Explicitly detach a client-provided cursor surface before installing a
    // compositor-owned themed cursor for move/resize grabs.
    wlr_cursor_set_surface(cursor, NULL, 0, 0);
    wlr_cursor_set_xcursor(cursor, manager, resolved);
    return true;
}
