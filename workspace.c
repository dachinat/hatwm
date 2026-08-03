#include <wlr/types/wlr_seat.h>
#include "workspace.h"
void hatwm_clear_keyboard_focus(struct wlr_seat *seat) {
    if (seat != NULL) {
        wlr_seat_keyboard_clear_focus(seat);
    }
}
