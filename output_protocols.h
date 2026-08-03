#ifndef HATWM_OUTPUT_PROTOCOLS_H
#define HATWM_OUTPUT_PROTOCOLS_H

struct wl_display;
struct wlr_backend;
struct wlr_output;
struct wlr_output_layout;

struct hatwm_output_protocols;

struct hatwm_output_protocols *hatwm_output_protocols_create(
    struct wl_display *display,
    struct wlr_backend *backend,
    struct wlr_output_layout *layout);
void hatwm_output_protocols_destroy(struct hatwm_output_protocols *protocols);
void hatwm_output_protocols_add(
    struct hatwm_output_protocols *protocols, struct wlr_output *output);
void hatwm_output_protocols_remove(
    struct hatwm_output_protocols *protocols, struct wlr_output *output);

#endif
