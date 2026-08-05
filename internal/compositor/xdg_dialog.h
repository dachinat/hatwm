/* Native wlroots bridge for the HatWM compositor package. */
#ifndef HATWM_XDG_DIALOG_H
#define HATWM_XDG_DIALOG_H

#include <stdbool.h>
#include <wayland-server-core.h>
#include <wlr/types/wlr_xdg_shell.h>

struct hatwm_xdg_dialog_manager;

struct hatwm_xdg_dialog_manager *hatwm_xdg_dialog_manager_create(
    struct wl_display *display);
void hatwm_xdg_dialog_manager_destroy(
    struct hatwm_xdg_dialog_manager *manager);
bool hatwm_xdg_dialog_state(
    struct hatwm_xdg_dialog_manager *manager,
    struct wlr_xdg_toplevel *toplevel,
    bool *is_dialog,
    bool *is_modal);

#endif
