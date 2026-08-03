#ifndef HATWM_CLIPBOARD_H
#define HATWM_CLIPBOARD_H

#include <wayland-server-core.h>
#include <wlr/types/wlr_seat.h>

struct hatwm_clipboard;

struct hatwm_clipboard *hatwm_clipboard_create(struct wlr_seat *seat);
void hatwm_clipboard_destroy(struct hatwm_clipboard *clipboard);

#endif
