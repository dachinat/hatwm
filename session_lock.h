#ifndef HATWM_SESSION_LOCK_H
#define HATWM_SESSION_LOCK_H

#include <stdbool.h>
#include <wayland-server-core.h>
#include <wlr/types/wlr_output_layout.h>
#include <wlr/types/wlr_scene.h>
#include <wlr/types/wlr_seat.h>
#include <wlr/types/wlr_session_lock_v1.h>

struct hatwm_session_lock_manager;

struct hatwm_session_lock_manager *hatwm_session_lock_manager_create(
    struct wl_display *display);
void hatwm_session_lock_manager_destroy(
    struct hatwm_session_lock_manager *manager);

void hatwm_session_lock_send_locked(struct wlr_session_lock_v1 *lock);
struct wlr_surface *hatwm_session_lock_surface_surface(
    struct wlr_session_lock_surface_v1 *surface);
struct wlr_scene_tree *hatwm_session_lock_surface_create_scene(
    struct wlr_session_lock_surface_v1 *surface,
    struct wlr_scene_tree *parent,
    struct wlr_output_layout *layout);
void hatwm_session_lock_surface_scene_destroy(
    struct wlr_scene_tree *scene);

struct wlr_scene_rect *hatwm_session_lock_background_create(
    struct wlr_scene_tree *parent,
    struct wlr_output_layout *layout);
void hatwm_session_lock_background_update(
    struct wlr_scene_rect *background,
    struct wlr_output_layout *layout);
void hatwm_session_lock_background_destroy(
    struct wlr_scene_rect *background);

void hatwm_session_lock_clear_seat_focus(struct wlr_seat *seat);

#endif
