#define _POSIX_C_SOURCE 200809L

#include "foreign_toplevel.h"

#include <stdlib.h>
#include <wlr/types/wlr_ext_foreign_toplevel_list_v1.h>

struct hatwm_foreign_toplevels {
    struct wlr_ext_foreign_toplevel_list_v1 *list;
};

struct hatwm_foreign_toplevels *hatwm_foreign_toplevels_create(
        struct wl_display *display) {
    if (display == NULL) {
        return NULL;
    }
    struct hatwm_foreign_toplevels *state = calloc(1, sizeof(*state));
    if (state == NULL) {
        return NULL;
    }
    state->list = wlr_ext_foreign_toplevel_list_v1_create(display, 1);
    if (state->list == NULL) {
        free(state);
        return NULL;
    }
    return state;
}

void hatwm_foreign_toplevels_destroy(struct hatwm_foreign_toplevels *state) {
    free(state);
}

struct wlr_ext_foreign_toplevel_handle_v1 *hatwm_foreign_toplevel_create(
        struct hatwm_foreign_toplevels *state,
        const char *title,
        const char *app_id) {
    if (state == NULL) {
        return NULL;
    }
    const struct wlr_ext_foreign_toplevel_handle_v1_state toplevel_state = {
        .title = title != NULL ? title : "",
        .app_id = app_id != NULL ? app_id : "",
    };
    return wlr_ext_foreign_toplevel_handle_v1_create(
        state->list, &toplevel_state);
}

void hatwm_foreign_toplevel_update(
        struct wlr_ext_foreign_toplevel_handle_v1 *handle,
        const char *title,
        const char *app_id) {
    if (handle == NULL) {
        return;
    }
    const struct wlr_ext_foreign_toplevel_handle_v1_state state = {
        .title = title != NULL ? title : "",
        .app_id = app_id != NULL ? app_id : "",
    };
    wlr_ext_foreign_toplevel_handle_v1_update_state(handle, &state);
}

void hatwm_foreign_toplevel_destroy(
        struct wlr_ext_foreign_toplevel_handle_v1 *handle) {
    if (handle != NULL) {
        wlr_ext_foreign_toplevel_handle_v1_destroy(handle);
    }
}
