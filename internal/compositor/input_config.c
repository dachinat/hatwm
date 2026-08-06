#include "input_config.h"

#include <string.h>
#include <libinput.h>
#include <wlr/backend/libinput.h>

bool hatwm_configure_libinput_device(
        struct wlr_input_device *device,
        bool tap_to_click,
        bool natural_scroll,
        double accel_speed,
        const char *accel_profile,
        bool left_handed,
        const char *scroll_method,
        bool disable_while_typing) {
    if (device == NULL || !wlr_input_device_is_libinput(device)) {
        return false;
    }
    struct libinput_device *libinput = wlr_libinput_get_device_handle(device);
    if (libinput == NULL) {
        return false;
    }

    if (libinput_device_config_tap_get_finger_count(libinput) > 0) {
        libinput_device_config_tap_set_enabled(libinput,
            tap_to_click ? LIBINPUT_CONFIG_TAP_ENABLED : LIBINPUT_CONFIG_TAP_DISABLED);
    }
    if (libinput_device_config_scroll_has_natural_scroll(libinput)) {
        libinput_device_config_scroll_set_natural_scroll_enabled(
            libinput, natural_scroll);
    }
    if (libinput_device_config_accel_is_available(libinput)) {
        libinput_device_config_accel_set_speed(libinput, accel_speed);
        enum libinput_config_accel_profile profile = LIBINPUT_CONFIG_ACCEL_PROFILE_ADAPTIVE;
        if (accel_profile != NULL && strcmp(accel_profile, "flat") == 0) {
            profile = LIBINPUT_CONFIG_ACCEL_PROFILE_FLAT;
        }
        if (libinput_device_config_accel_get_profiles(libinput) & profile) {
            libinput_device_config_accel_set_profile(libinput, profile);
        }
    }
    if (libinput_device_config_left_handed_is_available(libinput)) {
        libinput_device_config_left_handed_set(libinput, left_handed);
    }

    uint32_t supported = libinput_device_config_scroll_get_methods(libinput);
    enum libinput_config_scroll_method method = LIBINPUT_CONFIG_SCROLL_NO_SCROLL;
    if (scroll_method != NULL && strcmp(scroll_method, "two_finger") == 0) {
        method = LIBINPUT_CONFIG_SCROLL_2FG;
    } else if (scroll_method != NULL && strcmp(scroll_method, "edge") == 0) {
        method = LIBINPUT_CONFIG_SCROLL_EDGE;
    } else if (scroll_method != NULL && strcmp(scroll_method, "button") == 0) {
        method = LIBINPUT_CONFIG_SCROLL_ON_BUTTON_DOWN;
    }
    if (method == LIBINPUT_CONFIG_SCROLL_NO_SCROLL || (supported & method)) {
        libinput_device_config_scroll_set_method(libinput, method);
    }
    if (libinput_device_config_dwt_is_available(libinput)) {
        libinput_device_config_dwt_set_enabled(libinput,
            disable_while_typing ? LIBINPUT_CONFIG_DWT_ENABLED : LIBINPUT_CONFIG_DWT_DISABLED);
    }
    return true;
}
