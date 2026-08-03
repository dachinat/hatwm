#include "xdg_dialog.h"

#include <stdlib.h>
#include "xdg-dialog-v1-protocol.h"

extern void hatwmGoXDGDialogChanged(
    struct wlr_xdg_toplevel *toplevel, bool is_dialog, bool is_modal);

struct hatwm_xdg_dialog_manager {
    struct wl_global *global;
    struct wl_list dialogs;
};

struct hatwm_xdg_dialog {
    struct hatwm_xdg_dialog_manager *manager;
    struct wl_resource *resource;
    struct wlr_xdg_toplevel *toplevel;
    struct wl_listener toplevel_destroy;
    struct wl_list link;
    bool modal;
};

static struct hatwm_xdg_dialog *find_dialog(
        struct hatwm_xdg_dialog_manager *manager,
        struct wlr_xdg_toplevel *toplevel) {
    struct hatwm_xdg_dialog *dialog;
    wl_list_for_each(dialog, &manager->dialogs, link) {
        if (dialog->toplevel == toplevel) {
            return dialog;
        }
    }
    return NULL;
}

static void dialog_resource_destroy(struct wl_resource *resource) {
    struct hatwm_xdg_dialog *dialog = wl_resource_get_user_data(resource);
    if (dialog == NULL) {
        return;
    }
    if (dialog->toplevel != NULL) {
        struct wlr_xdg_toplevel *toplevel = dialog->toplevel;
        wl_list_remove(&dialog->toplevel_destroy.link);
        dialog->toplevel = NULL;
        hatwmGoXDGDialogChanged(toplevel, false, false);
    }
    wl_list_remove(&dialog->link);
    free(dialog);
}

static void handle_toplevel_destroy(
        struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_xdg_dialog *dialog =
        wl_container_of(listener, dialog, toplevel_destroy);
    struct wlr_xdg_toplevel *toplevel = dialog->toplevel;
    wl_list_remove(&dialog->toplevel_destroy.link);
    dialog->toplevel = NULL;
    dialog->modal = false;
    hatwmGoXDGDialogChanged(toplevel, false, false);
}

static void dialog_destroy(
        struct wl_client *client, struct wl_resource *resource) {
    (void)client;
    wl_resource_destroy(resource);
}

static void dialog_set_modal(
        struct wl_client *client, struct wl_resource *resource) {
    (void)client;
    struct hatwm_xdg_dialog *dialog = wl_resource_get_user_data(resource);
    if (dialog == NULL || dialog->toplevel == NULL || dialog->modal) {
        return;
    }
    dialog->modal = true;
    hatwmGoXDGDialogChanged(dialog->toplevel, true, true);
}

static void dialog_unset_modal(
        struct wl_client *client, struct wl_resource *resource) {
    (void)client;
    struct hatwm_xdg_dialog *dialog = wl_resource_get_user_data(resource);
    if (dialog == NULL || dialog->toplevel == NULL || !dialog->modal) {
        return;
    }
    dialog->modal = false;
    hatwmGoXDGDialogChanged(dialog->toplevel, true, false);
}

static const struct xdg_dialog_v1_interface dialog_implementation = {
    .destroy = dialog_destroy,
    .set_modal = dialog_set_modal,
    .unset_modal = dialog_unset_modal,
};

static void manager_destroy(
        struct wl_client *client, struct wl_resource *resource) {
    (void)client;
    wl_resource_destroy(resource);
}

static void manager_get_xdg_dialog(
        struct wl_client *client,
        struct wl_resource *resource,
        uint32_t id,
        struct wl_resource *toplevel_resource) {
    struct hatwm_xdg_dialog_manager *manager =
        wl_resource_get_user_data(resource);
    struct wlr_xdg_toplevel *toplevel =
        wlr_xdg_toplevel_from_resource(toplevel_resource);
    if (toplevel == NULL) {
        return;
    }
    if (find_dialog(manager, toplevel) != NULL) {
        wl_resource_post_error(resource,
            XDG_WM_DIALOG_V1_ERROR_ALREADY_USED,
            "xdg_toplevel already has an xdg_dialog_v1 object");
        return;
    }

    struct hatwm_xdg_dialog *dialog = calloc(1, sizeof(*dialog));
    if (dialog == NULL) {
        wl_resource_post_no_memory(resource);
        return;
    }
    dialog->resource = wl_resource_create(client, &xdg_dialog_v1_interface,
        wl_resource_get_version(resource), id);
    if (dialog->resource == NULL) {
        free(dialog);
        wl_resource_post_no_memory(resource);
        return;
    }
    dialog->manager = manager;
    dialog->toplevel = toplevel;
    dialog->toplevel_destroy.notify = handle_toplevel_destroy;
    wl_signal_add(&toplevel->events.destroy, &dialog->toplevel_destroy);
    wl_list_insert(manager->dialogs.prev, &dialog->link);
    wl_resource_set_implementation(dialog->resource,
        &dialog_implementation, dialog, dialog_resource_destroy);
    hatwmGoXDGDialogChanged(toplevel, true, false);
}

static const struct xdg_wm_dialog_v1_interface manager_implementation = {
    .destroy = manager_destroy,
    .get_xdg_dialog = manager_get_xdg_dialog,
};

static void manager_bind(
        struct wl_client *client, void *data,
        uint32_t version, uint32_t id) {
    struct hatwm_xdg_dialog_manager *manager = data;
    struct wl_resource *resource = wl_resource_create(client,
        &xdg_wm_dialog_v1_interface, version, id);
    if (resource == NULL) {
        wl_client_post_no_memory(client);
        return;
    }
    wl_resource_set_implementation(resource,
        &manager_implementation, manager, NULL);
}

struct hatwm_xdg_dialog_manager *hatwm_xdg_dialog_manager_create(
        struct wl_display *display) {
    if (display == NULL) {
        return NULL;
    }
    struct hatwm_xdg_dialog_manager *manager = calloc(1, sizeof(*manager));
    if (manager == NULL) {
        return NULL;
    }
    wl_list_init(&manager->dialogs);
    manager->global = wl_global_create(display,
        &xdg_wm_dialog_v1_interface, 1, manager, manager_bind);
    if (manager->global == NULL) {
        free(manager);
        return NULL;
    }
    return manager;
}

void hatwm_xdg_dialog_manager_destroy(
        struct hatwm_xdg_dialog_manager *manager) {
    if (manager == NULL) {
        return;
    }
    while (!wl_list_empty(&manager->dialogs)) {
        struct hatwm_xdg_dialog *dialog =
            wl_container_of(manager->dialogs.next, dialog, link);
        wl_resource_destroy(dialog->resource);
    }
    wl_global_destroy(manager->global);
    free(manager);
}

bool hatwm_xdg_dialog_state(
        struct hatwm_xdg_dialog_manager *manager,
        struct wlr_xdg_toplevel *toplevel,
        bool *is_dialog,
        bool *is_modal) {
    if (is_dialog != NULL) {
        *is_dialog = false;
    }
    if (is_modal != NULL) {
        *is_modal = false;
    }
    if (manager == NULL || toplevel == NULL) {
        return false;
    }
    struct hatwm_xdg_dialog *dialog = find_dialog(manager, toplevel);
    if (dialog == NULL) {
        return false;
    }
    if (is_dialog != NULL) {
        *is_dialog = true;
    }
    if (is_modal != NULL) {
        *is_modal = dialog->modal;
    }
    return true;
}
