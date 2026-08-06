#ifndef HATWM_INPUT_CONFIG_H
#define HATWM_INPUT_CONFIG_H

#include <stdbool.h>
#include <wlr/types/wlr_input_device.h>

bool hatwm_configure_libinput_device(
    struct wlr_input_device *device,
    bool tap_to_click,
    bool natural_scroll,
    double accel_speed,
    const char *accel_profile,
    bool left_handed,
    const char *scroll_method,
    bool disable_while_typing);

#endif
