#define _POSIX_C_SOURCE 200809L

#include "desktop_protocols.h"

#include <stdlib.h>
#include <wayland-server-core.h>
#include <wlr/backend.h>
#include <wlr/types/wlr_alpha_modifier_v1.h>
#include <wlr/types/wlr_content_type_v1.h>
#include <wlr/types/wlr_fractional_scale_v1.h>
#include <wlr/types/wlr_presentation_time.h>
#include <wlr/types/wlr_single_pixel_buffer_v1.h>
#include <wlr/types/wlr_tearing_control_v1.h>
#include <wlr/types/wlr_xdg_decoration_v1.h>
#include <wlr/types/wlr_xdg_foreign_registry.h>
#include <wlr/types/wlr_xdg_foreign_v1.h>
#include <wlr/types/wlr_xdg_foreign_v2.h>

struct hatwm_decoration {
    struct wlr_xdg_toplevel_decoration_v1 *decoration;
    struct wl_listener request_mode;
    struct wl_listener destroy;
};

struct hatwm_desktop_protocols {
    struct wlr_xdg_decoration_manager_v1 *decorations;
    struct wlr_fractional_scale_manager_v1 *fractional_scale;
    struct wlr_presentation *presentation;
    struct wlr_xdg_foreign_registry *xdg_foreign_registry;
    struct wlr_xdg_foreign_v1 *xdg_foreign_v1;
    struct wlr_xdg_foreign_v2 *xdg_foreign_v2;
    struct wl_listener new_decoration;
};

static void handle_decoration_request_mode(
        struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_decoration *item =
        wl_container_of(listener, item, request_mode);
    wlr_xdg_toplevel_decoration_v1_set_mode(
        item->decoration, WLR_XDG_TOPLEVEL_DECORATION_V1_MODE_SERVER_SIDE);
}

static void handle_decoration_destroy(
        struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_decoration *item = wl_container_of(listener, item, destroy);
    wl_list_remove(&item->request_mode.link);
    wl_list_remove(&item->destroy.link);
    free(item);
}

static void handle_new_decoration(struct wl_listener *listener, void *data) {
    (void)listener;
    struct wlr_xdg_toplevel_decoration_v1 *decoration = data;
    struct hatwm_decoration *item = calloc(1, sizeof(*item));
    if (item == NULL) {
        return;
    }
    item->decoration = decoration;
    item->request_mode.notify = handle_decoration_request_mode;
    wl_signal_add(&decoration->events.request_mode, &item->request_mode);
    item->destroy.notify = handle_decoration_destroy;
    wl_signal_add(&decoration->events.destroy, &item->destroy);
    wlr_xdg_toplevel_decoration_v1_set_mode(
        decoration, WLR_XDG_TOPLEVEL_DECORATION_V1_MODE_SERVER_SIDE);
}

struct hatwm_desktop_protocols *hatwm_desktop_protocols_create(
        struct wl_display *display, struct wlr_backend *backend) {
    if (display == NULL || backend == NULL) {
        return NULL;
    }
    struct hatwm_desktop_protocols *protocols = calloc(1, sizeof(*protocols));
    if (protocols == NULL) {
        return NULL;
    }
    protocols->decorations = wlr_xdg_decoration_manager_v1_create(display);
    protocols->fractional_scale =
        wlr_fractional_scale_manager_v1_create(display, 1);
    protocols->presentation = wlr_presentation_create(display, backend);
    protocols->xdg_foreign_registry = wlr_xdg_foreign_registry_create(display);
    if (protocols->xdg_foreign_registry != NULL) {
        protocols->xdg_foreign_v1 = wlr_xdg_foreign_v1_create(
            display, protocols->xdg_foreign_registry);
        protocols->xdg_foreign_v2 = wlr_xdg_foreign_v2_create(
            display, protocols->xdg_foreign_registry);
    }
    bool failed = protocols->decorations == NULL ||
        protocols->fractional_scale == NULL || protocols->presentation == NULL ||
        protocols->xdg_foreign_registry == NULL ||
        protocols->xdg_foreign_v1 == NULL || protocols->xdg_foreign_v2 == NULL ||
        wlr_single_pixel_buffer_manager_v1_create(display) == NULL ||
        wlr_content_type_manager_v1_create(display, 1) == NULL ||
        wlr_tearing_control_manager_v1_create(display, 1) == NULL ||
        wlr_alpha_modifier_v1_create(display) == NULL;
    if (failed) {
        free(protocols);
        return NULL;
    }
    protocols->new_decoration.notify = handle_new_decoration;
    wl_signal_add(&protocols->decorations->events.new_toplevel_decoration,
        &protocols->new_decoration);
    return protocols;
}

void hatwm_desktop_protocols_destroy(struct hatwm_desktop_protocols *protocols) {
    if (protocols == NULL) {
        return;
    }
    wl_list_remove(&protocols->new_decoration.link);
    free(protocols);
}

void hatwm_desktop_protocols_notify_scale(
        struct hatwm_desktop_protocols *protocols,
        struct wlr_surface *surface,
        double scale) {
    if (protocols != NULL && surface != NULL && scale > 0) {
        wlr_fractional_scale_v1_notify_scale(surface, scale);
    }
}
