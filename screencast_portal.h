#ifndef HATWM_SCREENCAST_PORTAL_H
#define HATWM_SCREENCAST_PORTAL_H

#include <stdbool.h>
#include <stdint.h>

struct wlr_output;
struct wlr_renderer;
struct wlr_scene_output;

struct hatwm_screencast_portal;

struct hatwm_screencast_portal *hatwm_screencast_portal_create(void);
void hatwm_screencast_portal_destroy(struct hatwm_screencast_portal *portal);
void hatwm_screencast_portal_add_output(
    struct hatwm_screencast_portal *portal, struct wlr_output *output);
void hatwm_screencast_portal_remove_output(
    struct hatwm_screencast_portal *portal, struct wlr_output *output);
void hatwm_screencast_portal_tick(struct hatwm_screencast_portal *portal);
void hatwm_screencast_portal_set_appearance(
    struct hatwm_screencast_portal *portal,
    uint32_t color_scheme, uint32_t reduced_motion);
bool hatwm_screencast_portal_render(
    struct hatwm_screencast_portal *portal,
    struct wlr_scene_output *scene_output,
    struct wlr_renderer *renderer,
    struct wlr_output *output);

#endif
