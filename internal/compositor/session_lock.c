/* Native wlroots bridge for the HatWM compositor package. */
#include "session_lock.h"

#include <stddef.h>
#include <stdlib.h>

extern void hatwmGoSessionLockNew(void *lock);
extern void hatwmGoSessionLockSurfaceNew(void *surface);
extern void hatwmGoSessionLockSurfaceMap(void *surface);
extern void hatwmGoSessionLockSurfaceUnmap(void *surface);
extern void hatwmGoSessionLockSurfaceDestroy(void *surface);
extern void hatwmGoSessionLockUnlock(void *lock);
extern void hatwmGoSessionLockDestroy(void *lock, bool unlocked);

struct hatwm_session_lock {
    struct wlr_session_lock_v1 *lock;
    struct hatwm_session_lock_manager *manager;
    struct wl_listener new_surface;
    struct wl_listener unlock;
    struct wl_listener destroy;
    bool unlocked;
};

struct hatwm_session_lock_surface {
    struct wlr_session_lock_surface_v1 *surface;
    struct wl_listener map;
    struct wl_listener unmap;
    struct wl_listener destroy;
    struct wlr_scene_tree *scene_tree;
};

struct hatwm_session_lock_manager {
    struct wlr_session_lock_manager_v1 *manager;
    struct hatwm_session_lock *current;
    struct wl_listener new_lock;
    struct wl_listener display_destroy;
};

static void handle_surface_map(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_session_lock_surface *surface =
        wl_container_of(listener, surface, map);
    hatwmGoSessionLockSurfaceMap(surface->surface);
}

static void handle_surface_unmap(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_session_lock_surface *surface =
        wl_container_of(listener, surface, unmap);
    hatwmGoSessionLockSurfaceUnmap(surface->surface);
}

static void handle_surface_destroy(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_session_lock_surface *surface =
        wl_container_of(listener, surface, destroy);

    hatwmGoSessionLockSurfaceDestroy(surface->surface);
    wl_list_remove(&surface->map.link);
    wl_list_remove(&surface->unmap.link);
    wl_list_remove(&surface->destroy.link);
    free(surface);
}

static void handle_new_surface(struct wl_listener *listener, void *data) {
    (void)listener;
    struct wlr_session_lock_surface_v1 *wlr_surface = data;
    struct hatwm_session_lock_surface *surface =
        calloc(1, sizeof(*surface));
    if (surface == NULL) {
        return;
    }

    surface->surface = wlr_surface;
    surface->map.notify = handle_surface_map;
    surface->unmap.notify = handle_surface_unmap;
    surface->destroy.notify = handle_surface_destroy;
    wl_signal_add(&wlr_surface->surface->events.map, &surface->map);
    wl_signal_add(&wlr_surface->surface->events.unmap, &surface->unmap);
    wl_signal_add(&wlr_surface->events.destroy, &surface->destroy);
    hatwmGoSessionLockSurfaceNew(wlr_surface);
}

static void handle_unlock(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_session_lock *lock =
        wl_container_of(listener, lock, unlock);
    lock->unlocked = true;
    hatwmGoSessionLockUnlock(lock->lock);
}

static void handle_lock_destroy(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_session_lock *lock =
        wl_container_of(listener, lock, destroy);

    if (lock->manager != NULL && lock->manager->current == lock) {
        lock->manager->current = NULL;
    }
    hatwmGoSessionLockDestroy(lock->lock, lock->unlocked);
    wl_list_remove(&lock->new_surface.link);
    wl_list_remove(&lock->unlock.link);
    wl_list_remove(&lock->destroy.link);
    free(lock);
}

static void handle_new_lock(struct wl_listener *listener, void *data) {
    struct hatwm_session_lock_manager *manager =
        wl_container_of(listener, manager, new_lock);
    struct wlr_session_lock_v1 *wlr_lock = data;

    /* Only one live protocol lock is allowed at a time. A replacement is
     * accepted after a lock client crashes so the session can be recovered,
     * while the compositor remains visually and interactively locked. */
    if (manager->current != NULL) {
        wlr_session_lock_v1_destroy(wlr_lock);
        return;
    }

    struct hatwm_session_lock *lock = calloc(1, sizeof(*lock));
    if (lock == NULL) {
        wlr_session_lock_v1_destroy(wlr_lock);
        return;
    }

    lock->lock = wlr_lock;
    lock->manager = manager;
    lock->new_surface.notify = handle_new_surface;
    lock->unlock.notify = handle_unlock;
    lock->destroy.notify = handle_lock_destroy;
    wl_signal_add(&wlr_lock->events.new_surface, &lock->new_surface);
    wl_signal_add(&wlr_lock->events.unlock, &lock->unlock);
    wl_signal_add(&wlr_lock->events.destroy, &lock->destroy);
    manager->current = lock;
    hatwmGoSessionLockNew(wlr_lock);
}

static void handle_display_destroy(struct wl_listener *listener, void *data) {
    (void)data;
    struct hatwm_session_lock_manager *manager =
        wl_container_of(listener, manager, display_destroy);
    if (manager->current != NULL) {
        manager->current->manager = NULL;
    }
    wl_list_remove(&manager->new_lock.link);
    wl_list_remove(&manager->display_destroy.link);
    free(manager);
}

struct hatwm_session_lock_manager *hatwm_session_lock_manager_create(
        struct wl_display *display) {
    struct hatwm_session_lock_manager *manager =
        calloc(1, sizeof(*manager));
    if (manager == NULL) {
        return NULL;
    }

    manager->manager = wlr_session_lock_manager_v1_create(display);
    if (manager->manager == NULL) {
        free(manager);
        return NULL;
    }

    manager->new_lock.notify = handle_new_lock;
    wl_signal_add(&manager->manager->events.new_lock, &manager->new_lock);
    manager->display_destroy.notify = handle_display_destroy;
    wl_display_add_destroy_listener(display, &manager->display_destroy);
    return manager;
}

void hatwm_session_lock_manager_destroy(
        struct hatwm_session_lock_manager *manager) {
    /* The wlroots global is display-owned. Only detach HatWM's listeners. */
    if (manager == NULL) {
        return;
    }
    if (manager->current != NULL) {
        manager->current->manager = NULL;
    }
    wl_list_remove(&manager->new_lock.link);
    wl_list_remove(&manager->display_destroy.link);
    free(manager);
}

void hatwm_session_lock_send_locked(struct wlr_session_lock_v1 *lock) {
    if (lock != NULL) {
        wlr_session_lock_v1_send_locked(lock);
    }
}

struct wlr_surface *hatwm_session_lock_surface_surface(
        struct wlr_session_lock_surface_v1 *surface) {
    return surface != NULL ? surface->surface : NULL;
}

struct wlr_scene_tree *hatwm_session_lock_surface_create_scene(
        struct wlr_session_lock_surface_v1 *surface,
        struct wlr_scene_tree *parent,
        struct wlr_output_layout *layout) {
    if (surface == NULL || parent == NULL || layout == NULL ||
            surface->surface == NULL || surface->output == NULL) {
        return NULL;
    }

    struct wlr_scene_tree *tree =
        wlr_scene_subsurface_tree_create(parent, surface->surface);
    if (tree == NULL) {
        return NULL;
    }

    struct wlr_box box = {0};
    wlr_output_layout_get_box(layout, surface->output, &box);
    int width = box.width, height = box.height;
    int x = box.x, y = box.y;
    if (width <= 0 || height <= 0) {
        wlr_output_effective_resolution(surface->output, &width, &height);
    }
    if (width <= 0) {
        width = 1;
    }
    if (height <= 0) {
        height = 1;
    }

    wlr_scene_node_set_position(&tree->node, x, y);
    wlr_session_lock_surface_v1_configure(surface, width, height);
    return tree;
}

void hatwm_session_lock_surface_scene_destroy(
        struct wlr_scene_tree *scene) {
    if (scene != NULL) {
        wlr_scene_node_destroy(&scene->node);
    }
}

static void get_layout_box(
        struct wlr_output_layout *layout, struct wlr_box *box) {
    wlr_output_layout_get_box(layout, NULL, box);
    if (box->width <= 0 || box->height <= 0) {
        *box = (struct wlr_box){.x = 0, .y = 0, .width = 1, .height = 1};
    }
}

struct wlr_scene_rect *hatwm_session_lock_background_create(
        struct wlr_scene_tree *parent,
        struct wlr_output_layout *layout) {
    if (parent == NULL || layout == NULL) {
        return NULL;
    }
    struct wlr_box box;
    get_layout_box(layout, &box);
    const float black[4] = {0.0f, 0.0f, 0.0f, 1.0f};
    struct wlr_scene_rect *background =
        wlr_scene_rect_create(parent, box.width, box.height, black);
    if (background != NULL) {
        wlr_scene_node_set_position(&background->node, box.x, box.y);
        wlr_scene_node_lower_to_bottom(&background->node);
    }
    return background;
}

void hatwm_session_lock_background_update(
        struct wlr_scene_rect *background,
        struct wlr_output_layout *layout) {
    if (background == NULL || layout == NULL) {
        return;
    }
    struct wlr_box box;
    get_layout_box(layout, &box);
    wlr_scene_rect_set_size(background, box.width, box.height);
    wlr_scene_node_set_position(&background->node, box.x, box.y);
}

void hatwm_session_lock_background_destroy(
        struct wlr_scene_rect *background) {
    if (background != NULL) {
        wlr_scene_node_destroy(&background->node);
    }
}

void hatwm_session_lock_clear_seat_focus(struct wlr_seat *seat) {
    if (seat == NULL) {
        return;
    }
    wlr_seat_keyboard_clear_focus(seat);
    wlr_seat_pointer_clear_focus(seat);
}
