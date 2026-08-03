#define _POSIX_C_SOURCE 200809L

#include "output_protocols.h"

#include <stdlib.h>
#include <wayland-server-core.h>
#include <wlr/backend.h>
#include <wlr/types/wlr_gamma_control_v1.h>
#include <wlr/types/wlr_output.h>
#include <wlr/types/wlr_output_layout.h>
#include <wlr/types/wlr_output_management_v1.h>
#include <wlr/types/wlr_output_power_management_v1.h>

struct hatwm_output_entry {
    struct wl_list link;
    struct wlr_output *output;
};

struct hatwm_output_protocols {
    struct wlr_backend *backend;
    struct wlr_output_layout *layout;
    struct wlr_output_manager_v1 *manager;
    struct wlr_output_power_manager_v1 *power;
    struct wlr_gamma_control_manager_v1 *gamma;
    struct wl_list outputs;
    struct wl_listener apply;
    struct wl_listener test;
    struct wl_listener set_power;
    struct wl_listener set_gamma;
};

static void finish_backend_states(
        struct wlr_backend_output_state *states, size_t len) {
    if (states == NULL) {
        return;
    }
    for (size_t i = 0; i < len; ++i) {
        wlr_output_state_finish(&states[i].base);
    }
    free(states);
}

static void publish_configuration(struct hatwm_output_protocols *protocols) {
    struct wlr_output_configuration_v1 *config =
        wlr_output_configuration_v1_create();
    if (config == NULL) {
        return;
    }
    struct hatwm_output_entry *entry;
    wl_list_for_each(entry, &protocols->outputs, link) {
        struct wlr_output_configuration_head_v1 *head =
            wlr_output_configuration_head_v1_create(config, entry->output);
        struct wlr_output_layout_output *layout_output =
            wlr_output_layout_get(protocols->layout, entry->output);
        if (head != NULL && layout_output != NULL) {
            head->state.x = layout_output->x;
            head->state.y = layout_output->y;
        }
    }
    wlr_output_manager_v1_set_configuration(protocols->manager, config);
}

static void handle_configuration(
        struct hatwm_output_protocols *protocols,
        struct wlr_output_configuration_v1 *config,
        bool apply) {
    size_t states_len = 0;
    struct wlr_backend_output_state *states =
        wlr_output_configuration_v1_build_state(config, &states_len);
    bool ok = states != NULL &&
        wlr_backend_test(protocols->backend, states, states_len);
    if (ok && apply) {
        ok = wlr_backend_commit(protocols->backend, states, states_len);
        if (ok) {
            struct wlr_output_configuration_head_v1 *head;
            wl_list_for_each(head, &config->heads, link) {
                wlr_output_layout_add(protocols->layout,
                    head->state.output, head->state.x, head->state.y);
            }
        }
    }
    if (ok) {
        wlr_output_configuration_v1_send_succeeded(config);
    } else {
        wlr_output_configuration_v1_send_failed(config);
    }
    finish_backend_states(states, states_len);
    if (apply && ok) {
        publish_configuration(protocols);
    }
}

static void handle_apply(struct wl_listener *listener, void *data) {
    struct hatwm_output_protocols *protocols =
        wl_container_of(listener, protocols, apply);
    handle_configuration(protocols, data, true);
}

static void handle_test(struct wl_listener *listener, void *data) {
    struct hatwm_output_protocols *protocols =
        wl_container_of(listener, protocols, test);
    handle_configuration(protocols, data, false);
}

static void handle_set_power(struct wl_listener *listener, void *data) {
    struct hatwm_output_protocols *protocols =
        wl_container_of(listener, protocols, set_power);
    struct wlr_output_power_v1_set_mode_event *event = data;
    struct wlr_output_state state;
    wlr_output_state_init(&state);
    wlr_output_state_set_enabled(&state,
        event->mode == ZWLR_OUTPUT_POWER_V1_MODE_ON);
    wlr_output_commit_state(event->output, &state);
    wlr_output_state_finish(&state);
    publish_configuration(protocols);
}

static void handle_set_gamma(struct wl_listener *listener, void *data) {
    (void)listener;
    struct wlr_gamma_control_manager_v1_set_gamma_event *event = data;
    struct wlr_output_state state;
    wlr_output_state_init(&state);
    if (event->control != NULL &&
        wlr_gamma_control_v1_apply(event->control, &state) &&
        !wlr_output_commit_state(event->output, &state)) {
        wlr_gamma_control_v1_send_failed_and_destroy(event->control);
    }
    wlr_output_state_finish(&state);
}

struct hatwm_output_protocols *hatwm_output_protocols_create(
        struct wl_display *display,
        struct wlr_backend *backend,
        struct wlr_output_layout *layout) {
    if (display == NULL || backend == NULL || layout == NULL) {
        return NULL;
    }
    struct hatwm_output_protocols *protocols = calloc(1, sizeof(*protocols));
    if (protocols == NULL) {
        return NULL;
    }
    protocols->backend = backend;
    protocols->layout = layout;
    wl_list_init(&protocols->outputs);
    protocols->manager = wlr_output_manager_v1_create(display);
    protocols->power = wlr_output_power_manager_v1_create(display);
    protocols->gamma = wlr_gamma_control_manager_v1_create(display);
    if (protocols->manager == NULL || protocols->power == NULL ||
        protocols->gamma == NULL) {
        free(protocols);
        return NULL;
    }
    protocols->apply.notify = handle_apply;
    wl_signal_add(&protocols->manager->events.apply, &protocols->apply);
    protocols->test.notify = handle_test;
    wl_signal_add(&protocols->manager->events.test, &protocols->test);
    protocols->set_power.notify = handle_set_power;
    wl_signal_add(&protocols->power->events.set_mode, &protocols->set_power);
    protocols->set_gamma.notify = handle_set_gamma;
    wl_signal_add(&protocols->gamma->events.set_gamma, &protocols->set_gamma);
    publish_configuration(protocols);
    return protocols;
}

void hatwm_output_protocols_destroy(struct hatwm_output_protocols *protocols) {
    if (protocols == NULL) {
        return;
    }
    wl_list_remove(&protocols->apply.link);
    wl_list_remove(&protocols->test.link);
    wl_list_remove(&protocols->set_power.link);
    wl_list_remove(&protocols->set_gamma.link);
    struct hatwm_output_entry *entry, *tmp;
    wl_list_for_each_safe(entry, tmp, &protocols->outputs, link) {
        wl_list_remove(&entry->link);
        free(entry);
    }
    free(protocols);
}

void hatwm_output_protocols_add(
        struct hatwm_output_protocols *protocols, struct wlr_output *output) {
    if (protocols == NULL || output == NULL) {
        return;
    }
    struct hatwm_output_entry *existing;
    wl_list_for_each(existing, &protocols->outputs, link) {
        if (existing->output == output) {
            return;
        }
    }
    struct hatwm_output_entry *entry = calloc(1, sizeof(*entry));
    if (entry == NULL) {
        return;
    }
    entry->output = output;
    wl_list_insert(protocols->outputs.prev, &entry->link);
    publish_configuration(protocols);
}

void hatwm_output_protocols_remove(
        struct hatwm_output_protocols *protocols, struct wlr_output *output) {
    if (protocols == NULL || output == NULL) {
        return;
    }
    struct hatwm_output_entry *entry, *tmp;
    wl_list_for_each_safe(entry, tmp, &protocols->outputs, link) {
        if (entry->output == output) {
            wl_list_remove(&entry->link);
            free(entry);
            break;
        }
    }
    publish_configuration(protocols);
}
