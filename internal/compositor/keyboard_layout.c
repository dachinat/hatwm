/* Native wlroots bridge for the HatWM compositor package. */
#include "keyboard_layout.h"

#include <xkbcommon/xkbcommon.h>

bool hatwm_keyboard_layouts_valid(const char *layouts,
                                  const char *variants,
                                  const char *options) {
    if (layouts == NULL || layouts[0] == '\0') {
        return false;
    }
    struct xkb_context *context = xkb_context_new(XKB_CONTEXT_NO_FLAGS);
    if (context == NULL) {
        return false;
    }
    struct xkb_rule_names names = {
        .rules = NULL,
        .model = NULL,
        .layout = layouts,
        .variant = variants != NULL && variants[0] != '\0' ? variants : NULL,
        .options = options != NULL && options[0] != '\0' ? options : NULL,
    };
    struct xkb_keymap *keymap = xkb_keymap_new_from_names(
        context, &names, XKB_KEYMAP_COMPILE_NO_FLAGS);
    if (keymap != NULL) {
        xkb_keymap_unref(keymap);
    }
    xkb_context_unref(context);
    return keymap != NULL;
}

bool hatwm_keyboard_set_layouts(struct wlr_keyboard *keyboard,
                                const char *layouts,
                                const char *variants,
                                const char *options) {
    if (keyboard == NULL || layouts == NULL || layouts[0] == '\0') {
        return false;
    }

    struct xkb_context *context = xkb_context_new(XKB_CONTEXT_NO_FLAGS);
    if (context == NULL) {
        return false;
    }

    struct xkb_rule_names names = {
        .rules = NULL,
        .model = NULL,
        .layout = layouts,
        .variant = variants != NULL && variants[0] != '\0' ? variants : NULL,
        .options = options != NULL && options[0] != '\0' ? options : NULL,
    };
    struct xkb_keymap *keymap = xkb_keymap_new_from_names(
        context, &names, XKB_KEYMAP_COMPILE_NO_FLAGS);
    if (keymap == NULL) {
        xkb_context_unref(context);
        return false;
    }

    wlr_keyboard_set_keymap(keyboard, keymap);
    xkb_keymap_unref(keymap);
    xkb_context_unref(context);
    return keyboard->xkb_state != NULL;
}

bool hatwm_keyboard_set_group(struct wlr_keyboard *keyboard, uint32_t group) {
    if (keyboard == NULL || keyboard->xkb_state == NULL || keyboard->keymap == NULL) {
        return false;
    }

    xkb_layout_index_t count = xkb_keymap_num_layouts(keyboard->keymap);
    if (count == 0 || group >= count) {
        return false;
    }

    xkb_mod_mask_t depressed = xkb_state_serialize_mods(
        keyboard->xkb_state, XKB_STATE_MODS_DEPRESSED);
    xkb_mod_mask_t latched = xkb_state_serialize_mods(
        keyboard->xkb_state, XKB_STATE_MODS_LATCHED);
    xkb_mod_mask_t locked = xkb_state_serialize_mods(
        keyboard->xkb_state, XKB_STATE_MODS_LOCKED);

    xkb_state_update_mask(keyboard->xkb_state,
                          depressed,
                          latched,
                          locked,
                          0,
                          0,
                          group);
    return true;
}

uint32_t hatwm_keyboard_group_count(struct wlr_keyboard *keyboard) {
    if (keyboard == NULL || keyboard->keymap == NULL) {
        return 0;
    }
    return (uint32_t)xkb_keymap_num_layouts(keyboard->keymap);
}

uint32_t hatwm_keyboard_base_keysym(struct wlr_keyboard *keyboard, uint32_t keycode) {
    if (keyboard == NULL || keyboard->keymap == NULL) {
        return XKB_KEY_NoSymbol;
    }

    const xkb_keysym_t *syms = NULL;
    int count = xkb_keymap_key_get_syms_by_level(
        keyboard->keymap, (xkb_keycode_t)keycode, 0, 0, &syms);
    if (count <= 0 || syms == NULL) {
        return XKB_KEY_NoSymbol;
    }
    return (uint32_t)syms[0];
}
