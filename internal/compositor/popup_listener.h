/* Native wlroots bridge for xdg-popup events missing from go-wlroots. */
#ifndef HATWM_POPUP_LISTENER_H
#define HATWM_POPUP_LISTENER_H

#include <stdint.h>
#include <wlr/types/wlr_xdg_shell.h>

struct hatwm_popup_listener;

struct hatwm_popup_listener *hatwm_xdg_popup_listen(
    struct wlr_xdg_popup *popup,
    uintptr_t token);

#endif
