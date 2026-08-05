/* Native wlroots bridge for the HatWM compositor package. */
#include "xwayland.h"

#include <limits.h>
#include <math.h>
#include <stddef.h>
#include <stdlib.h>
#include <xcb/shape.h>
#include <xcb/xcb.h>

extern void hatwmGoXWaylandNew(void *surface);
extern void hatwmGoXWaylandAssociate(void *surface);
extern void hatwmGoXWaylandDissociate(void *surface);
extern void hatwmGoXWaylandMap(void *surface);
extern void hatwmGoXWaylandUnmap(void *surface);
extern void hatwmGoXWaylandCommit(void *surface);
extern void hatwmGoXWaylandDestroy(void *surface);
extern void hatwmGoXWaylandRequestConfigure(
    void *surface, int16_t x, int16_t y, uint16_t width, uint16_t height);
extern void hatwmGoXWaylandRequestMove(void *surface);
extern void hatwmGoXWaylandRequestResize(void *surface, uint32_t edges);
extern void hatwmGoXWaylandRequestFullscreen(void *surface, bool fullscreen);
extern void hatwmGoXWaylandRequestActivate(void *surface);
extern void hatwmGoXWaylandOverrideRedirect(void *surface, bool value);

struct hatwm_xwayland_surface {
    struct wlr_xwayland_surface *surface;

    struct wl_listener associate;
    struct wl_listener dissociate;
    struct wl_listener destroy;
    struct wl_listener request_configure;
    struct wl_listener request_move;
    struct wl_listener request_resize;
    struct wl_listener request_fullscreen;
    struct wl_listener request_maximize;
    struct wl_listener request_activate;
    struct wl_listener set_geometry;
    struct wl_listener set_override_redirect;

    struct wl_listener map;
    struct wl_listener unmap;
    struct wl_listener commit;
    bool surface_listeners_attached;
};

struct hatwm_xwayland {
    struct wlr_xwayland *xwayland;
    struct wl_listener new_surface;
};

static void handle_surface_map(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_xwayland_surface *state =
        wl_container_of(listener, state, map);
    hatwmGoXWaylandMap(state->surface);
}

static void handle_surface_unmap(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_xwayland_surface *state =
        wl_container_of(listener, state, unmap);
    hatwmGoXWaylandUnmap(state->surface);
}

static void handle_surface_commit(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_xwayland_surface *state =
        wl_container_of(listener, state, commit);
    hatwmGoXWaylandCommit(state->surface);
}

static void attach_surface_listeners(struct hatwm_xwayland_surface *state) {
    if (state->surface_listeners_attached || state->surface->surface == NULL) {
        return;
    }
    state->map.notify = handle_surface_map;
    state->unmap.notify = handle_surface_unmap;
    state->commit.notify = handle_surface_commit;
    wl_signal_add(&state->surface->surface->events.map, &state->map);
    wl_signal_add(&state->surface->surface->events.unmap, &state->unmap);
    wl_signal_add(&state->surface->surface->events.commit, &state->commit);
    state->surface_listeners_attached = true;
}

static void detach_surface_listeners(struct hatwm_xwayland_surface *state) {
    if (!state->surface_listeners_attached) {
        return;
    }
    wl_list_remove(&state->map.link);
    wl_list_remove(&state->unmap.link);
    wl_list_remove(&state->commit.link);
    state->surface_listeners_attached = false;
}

static void handle_associate(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_xwayland_surface *state =
        wl_container_of(listener, state, associate);
    attach_surface_listeners(state);
    hatwmGoXWaylandAssociate(state->surface);
}

static void handle_dissociate(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_xwayland_surface *state =
        wl_container_of(listener, state, dissociate);
    hatwmGoXWaylandDissociate(state->surface);
    detach_surface_listeners(state);
}

static void handle_request_configure(
        struct wl_listener *listener, void *data) {
    struct hatwm_xwayland_surface *state =
        wl_container_of(listener, state, request_configure);
    struct wlr_xwayland_surface_configure_event *event = data;
    if (event == NULL) {
        return;
    }
    hatwmGoXWaylandRequestConfigure(
        state->surface, event->x, event->y, event->width, event->height);
}

static void handle_request_move(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_xwayland_surface *state =
        wl_container_of(listener, state, request_move);
    hatwmGoXWaylandRequestMove(state->surface);
}

static void handle_request_resize(struct wl_listener *listener, void *data) {
    struct hatwm_xwayland_surface *state =
        wl_container_of(listener, state, request_resize);
    struct wlr_xwayland_resize_event *event = data;
    if (event != NULL) {
        hatwmGoXWaylandRequestResize(state->surface, event->edges);
    }
}

static void handle_request_fullscreen(
        struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_xwayland_surface *state =
        wl_container_of(listener, state, request_fullscreen);
    hatwmGoXWaylandRequestFullscreen(
        state->surface, state->surface->fullscreen);
}

static void handle_request_maximize(
        struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_xwayland_surface *state =
        wl_container_of(listener, state, request_maximize);
    bool maximized =
        state->surface->maximized_horz && state->surface->maximized_vert;
    hatwmGoXWaylandRequestFullscreen(state->surface, maximized);
}

static void handle_request_activate(
        struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_xwayland_surface *state =
        wl_container_of(listener, state, request_activate);
    hatwmGoXWaylandRequestActivate(state->surface);
}

static void handle_set_geometry(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_xwayland_surface *state =
        wl_container_of(listener, state, set_geometry);
    hatwmGoXWaylandCommit(state->surface);
}

static void handle_set_override_redirect(
        struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_xwayland_surface *state =
        wl_container_of(listener, state, set_override_redirect);
    hatwmGoXWaylandOverrideRedirect(
        state->surface, state->surface->override_redirect);
}

static void handle_surface_destroy(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_xwayland_surface *state =
        wl_container_of(listener, state, destroy);

    hatwmGoXWaylandDestroy(state->surface);
    detach_surface_listeners(state);
    wl_list_remove(&state->associate.link);
    wl_list_remove(&state->dissociate.link);
    wl_list_remove(&state->destroy.link);
    wl_list_remove(&state->request_configure.link);
    wl_list_remove(&state->request_move.link);
    wl_list_remove(&state->request_resize.link);
    wl_list_remove(&state->request_fullscreen.link);
    wl_list_remove(&state->request_maximize.link);
    wl_list_remove(&state->request_activate.link);
    wl_list_remove(&state->set_geometry.link);
    wl_list_remove(&state->set_override_redirect.link);
    free(state);
}

static void handle_new_surface(struct wl_listener *listener, void *data) {
    (void)listener;
    struct wlr_xwayland_surface *surface = data;
    struct hatwm_xwayland_surface *state = calloc(1, sizeof(*state));
    if (state == NULL) {
        return;
    }

    state->surface = surface;
    state->associate.notify = handle_associate;
    state->dissociate.notify = handle_dissociate;
    state->destroy.notify = handle_surface_destroy;
    state->request_configure.notify = handle_request_configure;
    state->request_move.notify = handle_request_move;
    state->request_resize.notify = handle_request_resize;
    state->request_fullscreen.notify = handle_request_fullscreen;
    state->request_maximize.notify = handle_request_maximize;
    state->request_activate.notify = handle_request_activate;
    state->set_geometry.notify = handle_set_geometry;
    state->set_override_redirect.notify = handle_set_override_redirect;

    wl_signal_add(&surface->events.associate, &state->associate);
    wl_signal_add(&surface->events.dissociate, &state->dissociate);
    wl_signal_add(&surface->events.destroy, &state->destroy);
    wl_signal_add(&surface->events.request_configure, &state->request_configure);
    wl_signal_add(&surface->events.request_move, &state->request_move);
    wl_signal_add(&surface->events.request_resize, &state->request_resize);
    wl_signal_add(&surface->events.request_fullscreen, &state->request_fullscreen);
    wl_signal_add(&surface->events.request_maximize, &state->request_maximize);
    wl_signal_add(&surface->events.request_activate, &state->request_activate);
    wl_signal_add(&surface->events.set_geometry, &state->set_geometry);
    wl_signal_add(
        &surface->events.set_override_redirect,
        &state->set_override_redirect);

    hatwmGoXWaylandNew(surface);
    if (surface->surface != NULL) {
        attach_surface_listeners(state);
        hatwmGoXWaylandAssociate(surface);
    }
}

struct hatwm_xwayland *hatwm_xwayland_create(
        struct wl_display *display,
        struct wlr_compositor *compositor,
        bool lazy) {
    if (display == NULL || compositor == NULL) {
        return NULL;
    }
    struct hatwm_xwayland *state = calloc(1, sizeof(*state));
    if (state == NULL) {
        return NULL;
    }
    state->xwayland = wlr_xwayland_create(display, compositor, lazy);
    if (state->xwayland == NULL) {
        free(state);
        return NULL;
    }
    state->new_surface.notify = handle_new_surface;
    wl_signal_add(
        &state->xwayland->events.new_surface,
        &state->new_surface);
    return state;
}

void hatwm_xwayland_destroy(struct hatwm_xwayland *state) {
    if (state == NULL) {
        return;
    }
    wl_list_remove(&state->new_surface.link);
    wlr_xwayland_destroy(state->xwayland);
    free(state);
}

void hatwm_xwayland_set_seat(
        struct hatwm_xwayland *state, struct wlr_seat *seat) {
    if (state != NULL) {
        wlr_xwayland_set_seat(state->xwayland, seat);
    }
}

void hatwm_xwayland_set_cursor(
        struct hatwm_xwayland *state,
        struct wlr_xcursor_manager *manager,
        const char *name,
        float scale) {
    if (state == NULL || state->xwayland == NULL || manager == NULL) {
        return;
    }
    struct wlr_xcursor *cursor =
        wlr_xcursor_manager_get_xcursor(manager, name, scale);
    if (cursor == NULL || cursor->image_count == 0) {
        return;
    }
    struct wlr_xcursor_image *image = cursor->images[0];
    wlr_xwayland_set_cursor(
        state->xwayland,
        image->buffer,
        image->width * 4,
        image->width,
        image->height,
        image->hotspot_x,
        image->hotspot_y);
}

const char *hatwm_xwayland_display_name(struct hatwm_xwayland *state) {
    if (state == NULL || state->xwayland == NULL) {
        return NULL;
    }
    return state->xwayland->display_name;
}

struct wlr_surface *hatwm_xwayland_surface_surface(
        struct wlr_xwayland_surface *surface) {
    return surface != NULL ? surface->surface : NULL;
}

bool hatwm_xwayland_surface_override_redirect(
        struct wlr_xwayland_surface *surface) {
    return surface != NULL && surface->override_redirect;
}

bool hatwm_xwayland_surface_wants_focus(
        struct wlr_xwayland_surface *surface) {
    return surface != NULL && wlr_xwayland_or_surface_wants_focus(surface);
}

int hatwm_xwayland_surface_x(struct wlr_xwayland_surface *surface) {
    return surface != NULL ? surface->x : 0;
}

int hatwm_xwayland_surface_y(struct wlr_xwayland_surface *surface) {
    return surface != NULL ? surface->y : 0;
}

int hatwm_xwayland_surface_width(struct wlr_xwayland_surface *surface) {
    return surface != NULL ? surface->width : 0;
}

int hatwm_xwayland_surface_height(struct wlr_xwayland_surface *surface) {
    return surface != NULL ? surface->height : 0;
}

static int16_t clamp_int16(int value) {
    if (value < INT16_MIN) {
        return INT16_MIN;
    }
    if (value > INT16_MAX) {
        return INT16_MAX;
    }
    return (int16_t)value;
}

static uint16_t clamp_uint16(int value) {
    if (value < 1) {
        return 1;
    }
    if (value > UINT16_MAX) {
        return UINT16_MAX;
    }
    return (uint16_t)value;
}

void hatwm_xwayland_surface_configure(
        struct wlr_xwayland_surface *surface,
        int x, int y, int width, int height) {
    if (surface == NULL) {
        return;
    }
    wlr_xwayland_surface_configure(
        surface,
        clamp_int16(x),
        clamp_int16(y),
        clamp_uint16(width),
        clamp_uint16(height));
}

void hatwm_xwayland_surface_activate(
        struct wlr_xwayland_surface *surface, bool activated) {
    if (surface == NULL) {
        return;
    }
    wlr_xwayland_surface_activate(surface, activated);
    // Override-redirect windows (for example Electron context menus) manage
    // their own stacking. wlroots rejects attempts to restack them.
    if (activated && !surface->override_redirect) {
        wlr_xwayland_surface_restack(surface, NULL, XCB_STACK_MODE_ABOVE);
    }
}

void hatwm_xwayland_surface_close(
        struct wlr_xwayland_surface *surface) {
    if (surface != NULL) {
        wlr_xwayland_surface_close(surface);
    }
}

void hatwm_xwayland_surface_set_window_state(
        struct wlr_xwayland_surface *surface, bool fullscreen) {
    if (surface == NULL) {
        return;
    }
    wlr_xwayland_surface_set_maximized(surface, fullscreen);
    wlr_xwayland_surface_set_fullscreen(surface, fullscreen);
}

void hatwm_xwayland_surface_set_rounded_clip(
        struct hatwm_xwayland *state,
        struct wlr_xwayland_surface *surface,
        int radius) {
    if (state == NULL || state->xwayland == NULL || surface == NULL ||
            surface->window_id == XCB_WINDOW_NONE) {
        return;
    }

    xcb_connection_t *connection =
        wlr_xwayland_get_xwm_connection(state->xwayland);
    if (connection == NULL) {
        return;
    }
    if (radius <= 0 || surface->width <= 0 || surface->height <= 0) {
        xcb_shape_mask(
            connection, XCB_SHAPE_SO_SET, XCB_SHAPE_SK_BOUNDING,
            surface->window_id, 0, 0, XCB_PIXMAP_NONE);
        xcb_flush(connection);
        return;
    }

    int max_radius = surface->width / 2;
    if (surface->height / 2 < max_radius) {
        max_radius = surface->height / 2;
    }
    if (radius > max_radius) {
        radius = max_radius;
    }

    xcb_rectangle_t *rects =
        calloc((size_t)surface->height, sizeof(*rects));
    if (rects == NULL) {
        return;
    }
    for (int y = 0; y < surface->height; y++) {
        int edge_y = y < radius ? y : surface->height - 1 - y;
        int inset = 0;
        if (edge_y < radius) {
            double from_center = (double)radius - ((double)edge_y + 0.5);
            double inside = (double)radius * radius -
                from_center * from_center;
            inset = (int)ceil(
                (double)radius - sqrt(fmax(inside, 0.0)));
        }
        rects[y] = (xcb_rectangle_t){
            .x = (int16_t)inset,
            .y = (int16_t)y,
            .width = (uint16_t)(surface->width - 2 * inset),
            .height = 1,
        };
    }
    xcb_shape_rectangles(
        connection, XCB_SHAPE_SO_SET, XCB_SHAPE_SK_BOUNDING,
        XCB_CLIP_ORDERING_YX_BANDED, surface->window_id, 0, 0,
        (uint32_t)surface->height, rects);
    xcb_flush(connection);
    free(rects);
}
