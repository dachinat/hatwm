/* Native wlroots bridge for the HatWM compositor package. */
#ifndef HATWM_KEYBOARD_LAYOUT_H
#define HATWM_KEYBOARD_LAYOUT_H

#include <stdbool.h>
#include <stdint.h>
#include <wlr/types/wlr_keyboard.h>

bool hatwm_keyboard_set_layouts(struct wlr_keyboard *keyboard,
                                const char *layouts,
                                const char *variants,
                                const char *options);
bool hatwm_keyboard_set_group(struct wlr_keyboard *keyboard, uint32_t group);
uint32_t hatwm_keyboard_group_count(struct wlr_keyboard *keyboard);
uint32_t hatwm_keyboard_base_keysym(struct wlr_keyboard *keyboard, uint32_t keycode);

#endif
