/* Native wlroots bridge for the HatWM compositor package. */
#include "screencast_portal.h"

#include <errno.h>
#include <pthread.h>
#include <stdatomic.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <systemd/sd-bus.h>
#include <time.h>
#include <unistd.h>
#include <drm_fourcc.h>
#include <pipewire/pipewire.h>
#include <spa/buffer/meta.h>
#include <spa/param/buffers.h>
#include <spa/param/video/format-utils.h>
#include <spa/utils/result.h>
#include <wlr/render/wlr_renderer.h>
#include <wlr/render/wlr_texture.h>
#include <wlr/types/wlr_output.h>
#include <wlr/types/wlr_scene.h>

#define PORTAL_BUS_NAME "org.freedesktop.impl.portal.desktop.hatwm"
#define PORTAL_PATH "/org/freedesktop/portal/desktop"
#define SCREENCAST_IFACE "org.freedesktop.impl.portal.ScreenCast"
#define SETTINGS_IFACE "org.freedesktop.impl.portal.Settings"
#define APPEARANCE_NAMESPACE "org.freedesktop.appearance"
#define SESSION_IFACE "org.freedesktop.impl.portal.Session"

enum { SOURCE_MONITOR = 1 };
enum { CURSOR_HIDDEN = 1, CURSOR_EMBEDDED = 2 };
enum { RESPONSE_SUCCESS = 0, RESPONSE_CANCELLED = 1, RESPONSE_OTHER = 2 };

struct portal_output {
    struct portal_output *next;
    struct wlr_output *output;
    char *name;
    int width, height;
    int logical_width, logical_height;
};

struct portal_session {
    struct portal_session *next;
    struct hatwm_screencast_portal *portal;
    char *path;
    char *app_id;
    uint32_t source_types;
    uint32_t cursor_mode;
    bool sources_selected;
    bool started;
    sd_bus_slot *slot;
};

struct portal_stream {
    struct pw_stream *stream;
    struct spa_hook listener;
    struct portal_output *target;
    uint32_t node_id;
    uint32_t sequence;
    atomic_bool active;
    atomic_bool wants_frame;
    bool with_cursor;
};

struct hatwm_screencast_portal {
    pthread_t thread;
    pthread_mutex_t lock;
    atomic_bool stopping;
    atomic_bool running;
    struct portal_output *outputs;
    struct portal_session *sessions;
    struct portal_stream cast;
    struct wlr_output *render_locked_output;
    bool software_cursors_locked;
    struct pw_thread_loop *pw_loop;
    sd_bus *bus;
    sd_bus_slot *screencast_slot;
    sd_bus_slot *settings_slot;
    atomic_uint color_scheme;
    atomic_uint reduced_motion;
    atomic_bool color_scheme_dirty;
    atomic_bool reduced_motion_dirty;
};

static struct portal_session *find_session(
        struct hatwm_screencast_portal *portal, const char *path) {
    for (struct portal_session *s = portal->sessions; s != NULL; s = s->next) {
        if (strcmp(s->path, path) == 0) {
            return s;
        }
    }
    return NULL;
}

static struct portal_output *find_output(
        struct hatwm_screencast_portal *portal, const char *name) {
    for (struct portal_output *o = portal->outputs; o != NULL; o = o->next) {
        if (name == NULL || strcmp(o->name, name) == 0) {
            return o;
        }
    }
    return NULL;
}

static int append_empty_results(sd_bus_message *reply) {
    return sd_bus_message_append(reply, "a{sv}", 0);
}

static int reply_status(sd_bus_message *message, uint32_t response) {
    sd_bus_message *reply = NULL;
    int ret = sd_bus_message_new_method_return(message, &reply);
    if (ret >= 0) ret = sd_bus_message_append(reply, "u", response);
    if (ret >= 0) ret = append_empty_results(reply);
    if (ret >= 0) ret = sd_bus_send(NULL, reply, NULL);
    sd_bus_message_unref(reply);
    return ret;
}

static int read_options(sd_bus_message *message,
        uint32_t *types, uint32_t *cursor_mode) {
    int ret = sd_bus_message_enter_container(message, 'a', "{sv}");
    if (ret < 0) return ret;
    while ((ret = sd_bus_message_enter_container(message, 'e', "sv")) > 0) {
        const char *key = NULL;
        ret = sd_bus_message_read_basic(message, 's', &key);
        if (ret < 0) return ret;
        if (types != NULL && strcmp(key, "types") == 0) {
            ret = sd_bus_message_read(message, "v", "u", types);
        } else if (cursor_mode != NULL && strcmp(key, "cursor_mode") == 0) {
            ret = sd_bus_message_read(message, "v", "u", cursor_mode);
        } else {
            ret = sd_bus_message_skip(message, "v");
        }
        if (ret < 0) return ret;
        ret = sd_bus_message_exit_container(message);
        if (ret < 0) return ret;
    }
    if (ret < 0) return ret;
    return sd_bus_message_exit_container(message);
}

static void stop_stream_locked(struct hatwm_screencast_portal *portal) {
    if (portal->cast.stream != NULL) {
        pw_thread_loop_lock(portal->pw_loop);
        pw_stream_flush(portal->cast.stream, false);
        pw_stream_disconnect(portal->cast.stream);
        pw_stream_destroy(portal->cast.stream);
        portal->cast.stream = NULL;
        spa_zero(portal->cast.listener);
        pw_thread_loop_unlock(portal->pw_loop);
    }
    portal->cast.target = NULL;
    portal->cast.node_id = SPA_ID_INVALID;
    atomic_store(&portal->cast.active, false);
    atomic_store(&portal->cast.wants_frame, false);
}

static void destroy_session_locked(struct portal_session *session) {
    struct hatwm_screencast_portal *portal = session->portal;
    struct portal_session **link = &portal->sessions;
    while (*link != NULL && *link != session) link = &(*link)->next;
    if (*link == session) *link = session->next;
    if (session->started) stop_stream_locked(portal);
    sd_bus_slot_unref(session->slot);
    free(session->path);
    free(session->app_id);
    free(session);
}

static int method_session_close(sd_bus_message *message,
        void *userdata, sd_bus_error *error) {
    (void)error;
    struct portal_session *session = userdata;
    struct hatwm_screencast_portal *portal = session->portal;
    int ret = sd_bus_reply_method_return(message, "");
    pthread_mutex_lock(&portal->lock);
    destroy_session_locked(session);
    pthread_mutex_unlock(&portal->lock);
    return ret;
}

static const sd_bus_vtable session_vtable[] = {
    SD_BUS_VTABLE_START(0),
    SD_BUS_METHOD("Close", "", "", method_session_close,
        SD_BUS_VTABLE_UNPRIVILEGED),
    SD_BUS_VTABLE_END,
};

static int method_create_session(sd_bus_message *message,
        void *userdata, sd_bus_error *error) {
    (void)error;
    struct hatwm_screencast_portal *portal = userdata;
    const char *request_path, *session_path, *app_id;
    int ret = sd_bus_message_read(message, "oos",
        &request_path, &session_path, &app_id);
    (void)request_path;
    if (ret < 0) return ret;
    ret = read_options(message, NULL, NULL);
    if (ret < 0) return ret;

    pthread_mutex_lock(&portal->lock);
    if (find_session(portal, session_path) != NULL) {
        pthread_mutex_unlock(&portal->lock);
        return reply_status(message, RESPONSE_OTHER);
    }
    struct portal_session *session = calloc(1, sizeof(*session));
    if (session == NULL) {
        pthread_mutex_unlock(&portal->lock);
        return -ENOMEM;
    }
    session->portal = portal;
    session->path = strdup(session_path);
    session->app_id = strdup(app_id);
    session->source_types = SOURCE_MONITOR;
    session->cursor_mode = CURSOR_HIDDEN;
    session->next = portal->sessions;
    portal->sessions = session;
    ret = sd_bus_add_object_vtable(portal->bus, &session->slot,
        session->path, SESSION_IFACE, session_vtable, session);
    if (ret < 0) destroy_session_locked(session);
    pthread_mutex_unlock(&portal->lock);
    return ret < 0 ? ret : reply_status(message, RESPONSE_SUCCESS);
}

static int method_select_sources(sd_bus_message *message,
        void *userdata, sd_bus_error *error) {
    (void)error;
    struct hatwm_screencast_portal *portal = userdata;
    const char *request_path, *session_path, *app_id;
    int ret = sd_bus_message_read(message, "oos",
        &request_path, &session_path, &app_id);
    (void)request_path; (void)app_id;
    if (ret < 0) return ret;

    pthread_mutex_lock(&portal->lock);
    struct portal_session *session = find_session(portal, session_path);
    pthread_mutex_unlock(&portal->lock);
    if (session == NULL) return reply_status(message, RESPONSE_OTHER);
    uint32_t types = SOURCE_MONITOR;
    uint32_t cursor = CURSOR_HIDDEN;
    ret = read_options(message, &types, &cursor);
    if (ret < 0) return ret;
    if ((types & SOURCE_MONITOR) == 0 ||
        (cursor != CURSOR_HIDDEN && cursor != CURSOR_EMBEDDED)) {
        return reply_status(message, RESPONSE_OTHER);
    }
    pthread_mutex_lock(&portal->lock);
    session->source_types = SOURCE_MONITOR;
    session->cursor_mode = cursor;
    session->sources_selected = true;
    pthread_mutex_unlock(&portal->lock);
    return reply_status(message, RESPONSE_SUCCESS);
}

static char *choose_output(struct hatwm_screencast_portal *portal) {
    pthread_mutex_lock(&portal->lock);
    struct portal_output *only = portal->outputs;
    char *only_name = only != NULL && only->next == NULL ?
        strdup(only->name) : NULL;
    pthread_mutex_unlock(&portal->lock);
    if (only_name != NULL) return only_name;

    FILE *pipe = popen(
        "slurp -o -r -d -b '#16161699' -c '#78a9ffff' "
        "-s '#2a2a2acc' -B '#161616ee' -w 3 -f '%o'", "r");
    if (pipe == NULL) return NULL;
    char name[256] = {0};
    char *result = fgets(name, sizeof(name), pipe) != NULL ? name : NULL;
    int status = pclose(pipe);
    if (result == NULL || status != 0) return NULL;
    name[strcspn(name, "\r\n")] = '\0';
    return name[0] != '\0' ? strdup(name) : NULL;
}

static void on_stream_state_changed(void *data,
        enum pw_stream_state old, enum pw_stream_state state,
        const char *error) {
    (void)old;
    struct hatwm_screencast_portal *portal = data;
    if (state == PW_STREAM_STATE_ERROR) {
        fprintf(stderr, "hatwm: ScreenCast PipeWire stream failed: %s\n",
            error != NULL ? error : "unknown error");
    }
    if (state == PW_STREAM_STATE_PAUSED || state == PW_STREAM_STATE_STREAMING ||
        state == PW_STREAM_STATE_ERROR) {
        portal->cast.node_id = portal->cast.stream != NULL ?
            pw_stream_get_node_id(portal->cast.stream) : SPA_ID_INVALID;
        atomic_store(&portal->cast.active, state == PW_STREAM_STATE_STREAMING);
        if (state == PW_STREAM_STATE_STREAMING) {
            atomic_store(&portal->cast.wants_frame, true);
        } else {
            atomic_store(&portal->cast.wants_frame, false);
        }
        pw_thread_loop_signal(portal->pw_loop, false);
    }
}

static void on_stream_param_changed(void *data, uint32_t id,
        const struct spa_pod *param) {
    struct hatwm_screencast_portal *portal = data;
    if (param == NULL || id != SPA_PARAM_Format || portal->cast.stream == NULL) {
        return;
    }

    struct spa_video_info_raw format = {0};
    if (spa_format_video_raw_parse(param, &format) < 0 ||
            format.size.width == 0 || format.size.height == 0) {
        return;
    }
    uint32_t stride = format.size.width * 4;
    uint32_t size = stride * format.size.height;
    uint8_t pod_buffer[1024];
    struct spa_pod_builder builder =
        SPA_POD_BUILDER_INIT(pod_buffer, sizeof(pod_buffer));
    const struct spa_pod *params[2];
    params[0] = spa_pod_builder_add_object(&builder,
        SPA_TYPE_OBJECT_ParamBuffers, SPA_PARAM_Buffers,
        SPA_PARAM_BUFFERS_buffers, SPA_POD_CHOICE_RANGE_Int(4, 2, 8),
        SPA_PARAM_BUFFERS_blocks, SPA_POD_Int(1),
        SPA_PARAM_BUFFERS_size, SPA_POD_Int(size),
        SPA_PARAM_BUFFERS_stride, SPA_POD_Int(stride),
        SPA_PARAM_BUFFERS_align, SPA_POD_Int(16),
        SPA_PARAM_BUFFERS_dataType,
            SPA_POD_CHOICE_FLAGS_Int(1u << SPA_DATA_MemFd));
    params[1] = spa_pod_builder_add_object(&builder,
        SPA_TYPE_OBJECT_ParamMeta, SPA_PARAM_Meta,
        SPA_PARAM_META_type, SPA_POD_Id(SPA_META_Header),
        SPA_PARAM_META_size, SPA_POD_Int(sizeof(struct spa_meta_header)));
    pw_stream_update_params(portal->cast.stream, params, 2);
}

static void on_stream_process(void *data) {
    struct hatwm_screencast_portal *portal = data;
    atomic_store(&portal->cast.wants_frame, true);
}

static const struct pw_stream_events stream_events = {
    PW_VERSION_STREAM_EVENTS,
    .state_changed = on_stream_state_changed,
    .param_changed = on_stream_param_changed,
    .process = on_stream_process,
};

static int start_stream_locked(struct hatwm_screencast_portal *portal,
        struct portal_output *output, bool with_cursor) {
    stop_stream_locked(portal);
    uint8_t pod_buffer[1024];
    struct spa_pod_builder builder =
        SPA_POD_BUILDER_INIT(pod_buffer, sizeof(pod_buffer));
    const struct spa_pod *params[1];
    params[0] = spa_pod_builder_add_object(&builder,
        SPA_TYPE_OBJECT_Format, SPA_PARAM_EnumFormat,
        SPA_FORMAT_mediaType, SPA_POD_Id(SPA_MEDIA_TYPE_video),
        SPA_FORMAT_mediaSubtype, SPA_POD_Id(SPA_MEDIA_SUBTYPE_raw),
        SPA_FORMAT_VIDEO_format, SPA_POD_Id(SPA_VIDEO_FORMAT_BGRx),
        SPA_FORMAT_VIDEO_size, SPA_POD_Rectangle(
            &SPA_RECTANGLE(output->width, output->height)),
        SPA_FORMAT_VIDEO_framerate, SPA_POD_Fraction(&SPA_FRACTION(30, 1)));

    pw_thread_loop_lock(portal->pw_loop);
    portal->cast.stream = pw_stream_new_simple(
        pw_thread_loop_get_loop(portal->pw_loop), "HatWM ScreenCast",
        pw_properties_new(
            PW_KEY_MEDIA_CLASS, "Video/Source",
            PW_KEY_MEDIA_TYPE, "Video",
            PW_KEY_MEDIA_CATEGORY, "Capture",
            PW_KEY_MEDIA_ROLE, "Screen",
            PW_KEY_NODE_DESCRIPTION, output->name,
            NULL),
        &stream_events, portal);
    if (portal->cast.stream == NULL) {
        pw_thread_loop_unlock(portal->pw_loop);
        return -ENOMEM;
    }
    portal->cast.target = output;
    portal->cast.with_cursor = with_cursor;
    portal->cast.node_id = SPA_ID_INVALID;
    portal->cast.sequence = 0;
    int ret = pw_stream_connect(portal->cast.stream, PW_DIRECTION_OUTPUT,
        PW_ID_ANY, PW_STREAM_FLAG_DRIVER | PW_STREAM_FLAG_MAP_BUFFERS,
        params, 1);
    while (ret >= 0 && portal->cast.node_id == SPA_ID_INVALID) {
        ret = pw_thread_loop_timed_wait(portal->pw_loop, 5);
        if (ret < 0) break;
        enum pw_stream_state state = pw_stream_get_state(
            portal->cast.stream, NULL);
        if (state == PW_STREAM_STATE_ERROR || state == PW_STREAM_STATE_UNCONNECTED) {
            ret = -EIO;
            break;
        }
    }
    pw_thread_loop_unlock(portal->pw_loop);
    if (ret < 0) stop_stream_locked(portal);
    return ret;
}

static int reply_start(sd_bus_message *message, const char *output_name,
        int logical_width, int logical_height,
        uint32_t node_id) {
    sd_bus_message *reply = NULL;
    int ret = sd_bus_message_new_method_return(message, &reply);
    if (ret >= 0) ret = sd_bus_message_append(reply, "u", RESPONSE_SUCCESS);
    if (ret >= 0) ret = sd_bus_message_open_container(reply, 'a', "{sv}");
    if (ret >= 0) ret = sd_bus_message_open_container(reply, 'e', "sv");
    if (ret >= 0) ret = sd_bus_message_append(reply, "s", "streams");
    if (ret >= 0) ret = sd_bus_message_open_container(reply, 'v', "a(ua{sv})");
    if (ret >= 0) ret = sd_bus_message_open_container(reply, 'a', "(ua{sv})");
    if (ret >= 0) ret = sd_bus_message_open_container(reply, 'r', "ua{sv}");
    if (ret >= 0) ret = sd_bus_message_append(reply, "u", node_id);
    if (ret >= 0) ret = sd_bus_message_open_container(reply, 'a', "{sv}");
    if (ret >= 0) ret = sd_bus_message_append(reply, "{sv}",
        "size", "(ii)", logical_width, logical_height);
    if (ret >= 0) ret = sd_bus_message_append(reply, "{sv}",
        "source_type", "u", SOURCE_MONITOR);
    if (ret >= 0) ret = sd_bus_message_append(reply, "{sv}",
        "mapping_id", "s", output_name);
    for (int i = 0; ret >= 0 && i < 6; ++i) {
        ret = sd_bus_message_close_container(reply);
    }
    if (ret >= 0) ret = sd_bus_send(NULL, reply, NULL);
    sd_bus_message_unref(reply);
    return ret;
}

static int method_start(sd_bus_message *message,
        void *userdata, sd_bus_error *error) {
    (void)error;
    struct hatwm_screencast_portal *portal = userdata;
    const char *request_path, *session_path, *app_id, *parent_window;
    int ret = sd_bus_message_read(message, "ooss",
        &request_path, &session_path, &app_id, &parent_window);
    (void)request_path; (void)app_id; (void)parent_window;
    if (ret < 0) return ret;
    ret = read_options(message, NULL, NULL);
    if (ret < 0) return ret;

    pthread_mutex_lock(&portal->lock);
    struct portal_session *session = find_session(portal, session_path);
    bool valid = session != NULL && session->sources_selected && !session->started;
    pthread_mutex_unlock(&portal->lock);
    if (!valid) return reply_status(message, RESPONSE_OTHER);

    char *selected_name = choose_output(portal);
    if (selected_name == NULL) return reply_status(message, RESPONSE_CANCELLED);
    pthread_mutex_lock(&portal->lock);
    session = find_session(portal, session_path);
    struct portal_output *output = find_output(portal, selected_name);
    free(selected_name);
    if (session == NULL || !session->sources_selected || session->started ||
            output == NULL || start_stream_locked(portal, output,
            session->cursor_mode == CURSOR_EMBEDDED) < 0) {
        pthread_mutex_unlock(&portal->lock);
        return reply_status(message, RESPONSE_OTHER);
    }
    session->started = true;
    uint32_t node_id = portal->cast.node_id;
    char *output_name = strdup(output->name);
    int logical_width = output->logical_width;
    int logical_height = output->logical_height;
    pthread_mutex_unlock(&portal->lock);
    ret = reply_start(message, output_name,
        logical_width, logical_height, node_id);
    free(output_name);
    return ret;
}

static int property_get(sd_bus *bus, const char *path, const char *interface,
        const char *property, sd_bus_message *reply, void *userdata,
        sd_bus_error *error) {
    (void)bus; (void)path; (void)interface; (void)userdata; (void)error;
    uint32_t value = strcmp(property, "AvailableSourceTypes") == 0 ? SOURCE_MONITOR :
        strcmp(property, "AvailableCursorModes") == 0 ?
            CURSOR_HIDDEN | CURSOR_EMBEDDED : 3;
    return sd_bus_message_append(reply, "u", value);
}

static const sd_bus_vtable screencast_vtable_with_properties[] = {
    SD_BUS_VTABLE_START(0),
    SD_BUS_METHOD("CreateSession", "oosa{sv}", "ua{sv}",
        method_create_session, SD_BUS_VTABLE_UNPRIVILEGED),
    SD_BUS_METHOD("SelectSources", "oosa{sv}", "ua{sv}",
        method_select_sources, SD_BUS_VTABLE_UNPRIVILEGED),
    SD_BUS_METHOD("Start", "oossa{sv}", "ua{sv}",
        method_start, SD_BUS_VTABLE_UNPRIVILEGED),
    SD_BUS_PROPERTY("AvailableSourceTypes", "u", property_get, 0,
        SD_BUS_VTABLE_PROPERTY_CONST),
    SD_BUS_PROPERTY("AvailableCursorModes", "u", property_get, 0,
        SD_BUS_VTABLE_PROPERTY_CONST),
    SD_BUS_PROPERTY("version", "u", property_get, 0,
        SD_BUS_VTABLE_PROPERTY_CONST),
    SD_BUS_VTABLE_END,
};

static bool settings_namespace_matches(const char *requested) {
    if (requested == NULL || requested[0] == '\0' || strcmp(requested, "*") == 0 ||
            strcmp(requested, APPEARANCE_NAMESPACE) == 0) {
        return true;
    }
    size_t length = strlen(requested);
    return length > 0 && requested[length - 1] == '*' &&
        strncmp(APPEARANCE_NAMESPACE, requested, length - 1) == 0;
}

static int append_setting(sd_bus_message *reply,
        const char *key, uint32_t value) {
    int ret = sd_bus_message_open_container(reply, 'e', "sv");
    if (ret >= 0) ret = sd_bus_message_append(reply, "s", key);
    if (ret >= 0) ret = sd_bus_message_open_container(reply, 'v', "u");
    if (ret >= 0) ret = sd_bus_message_append(reply, "u", value);
    if (ret >= 0) ret = sd_bus_message_close_container(reply);
    if (ret >= 0) ret = sd_bus_message_close_container(reply);
    return ret;
}

static int method_settings_read(sd_bus_message *message,
        void *userdata, sd_bus_error *error) {
    struct hatwm_screencast_portal *portal = userdata;
    const char *namespace = NULL, *key = NULL;
    int ret = sd_bus_message_read(message, "ss", &namespace, &key);
    if (ret < 0) return ret;
    if (strcmp(namespace, APPEARANCE_NAMESPACE) != 0) {
        return sd_bus_error_setf(error, "org.freedesktop.portal.Error.NotFound",
            "Unknown settings namespace %s", namespace);
    }
    uint32_t value;
    if (strcmp(key, "color-scheme") == 0) {
        value = atomic_load(&portal->color_scheme);
    } else if (strcmp(key, "reduced-motion") == 0) {
        value = atomic_load(&portal->reduced_motion);
    } else {
        return sd_bus_error_setf(error, "org.freedesktop.portal.Error.NotFound",
            "Unknown setting %s", key);
    }
    return sd_bus_reply_method_return(message, "v", "u", value);
}

static int method_settings_read_all(sd_bus_message *message,
        void *userdata, sd_bus_error *error) {
    (void)error;
    struct hatwm_screencast_portal *portal = userdata;
    bool include_appearance = false;
    int ret = sd_bus_message_enter_container(message, 'a', "s");
    if (ret < 0) return ret;
    const char *namespace = NULL;
    while ((ret = sd_bus_message_read_basic(message, 's', &namespace)) > 0) {
        if (settings_namespace_matches(namespace)) include_appearance = true;
    }
    if (ret < 0) return ret;
    ret = sd_bus_message_exit_container(message);
    if (ret < 0) return ret;

    sd_bus_message *reply = NULL;
    ret = sd_bus_message_new_method_return(message, &reply);
    if (ret >= 0) ret = sd_bus_message_open_container(reply, 'a', "{sa{sv}}");
    if (ret >= 0 && include_appearance) {
        ret = sd_bus_message_open_container(reply, 'e', "sa{sv}");
        if (ret >= 0) ret = sd_bus_message_append(reply, "s", APPEARANCE_NAMESPACE);
        if (ret >= 0) ret = sd_bus_message_open_container(reply, 'a', "{sv}");
        if (ret >= 0) ret = append_setting(reply, "color-scheme",
            atomic_load(&portal->color_scheme));
        if (ret >= 0) ret = append_setting(reply, "reduced-motion",
            atomic_load(&portal->reduced_motion));
        if (ret >= 0) ret = sd_bus_message_close_container(reply);
        if (ret >= 0) ret = sd_bus_message_close_container(reply);
    }
    if (ret >= 0) ret = sd_bus_message_close_container(reply);
    if (ret >= 0) ret = sd_bus_send(NULL, reply, NULL);
    sd_bus_message_unref(reply);
    return ret;
}

static int settings_property_get(sd_bus *bus, const char *path,
        const char *interface, const char *property, sd_bus_message *reply,
        void *userdata, sd_bus_error *error) {
    (void)bus; (void)path; (void)interface; (void)property;
    (void)userdata; (void)error;
    return sd_bus_message_append(reply, "u", 1u);
}

static const sd_bus_vtable settings_vtable[] = {
    SD_BUS_VTABLE_START(0),
    SD_BUS_METHOD("ReadAll", "as", "a{sa{sv}}", method_settings_read_all,
        SD_BUS_VTABLE_UNPRIVILEGED),
    SD_BUS_METHOD("Read", "ss", "v", method_settings_read,
        SD_BUS_VTABLE_UNPRIVILEGED),
    SD_BUS_SIGNAL("SettingChanged", "ssv", 0),
    SD_BUS_PROPERTY("version", "u", settings_property_get, 0,
        SD_BUS_VTABLE_PROPERTY_CONST),
    SD_BUS_VTABLE_END,
};

static void emit_changed_settings(struct hatwm_screencast_portal *portal) {
    if (atomic_exchange(&portal->color_scheme_dirty, false)) {
        sd_bus_emit_signal(portal->bus, PORTAL_PATH, SETTINGS_IFACE,
            "SettingChanged", "ssv", APPEARANCE_NAMESPACE, "color-scheme",
            "u", atomic_load(&portal->color_scheme));
    }
    if (atomic_exchange(&portal->reduced_motion_dirty, false)) {
        sd_bus_emit_signal(portal->bus, PORTAL_PATH, SETTINGS_IFACE,
            "SettingChanged", "ssv", APPEARANCE_NAMESPACE, "reduced-motion",
            "u", atomic_load(&portal->reduced_motion));
    }
}

static void *portal_thread(void *data) {
    struct hatwm_screencast_portal *portal = data;
    int ret = sd_bus_open_user(&portal->bus);
    if (ret >= 0) ret = sd_bus_add_object_vtable(portal->bus,
        &portal->screencast_slot, PORTAL_PATH, SCREENCAST_IFACE,
        screencast_vtable_with_properties, portal);
    if (ret >= 0) ret = sd_bus_add_object_vtable(portal->bus,
        &portal->settings_slot, PORTAL_PATH, SETTINGS_IFACE,
        settings_vtable, portal);
    if (ret >= 0) ret = sd_bus_request_name(
        portal->bus, PORTAL_BUS_NAME, SD_BUS_NAME_REPLACE_EXISTING);
    if (ret >= 0) atomic_store(&portal->running, true);
    while (ret >= 0 && !atomic_load(&portal->stopping)) {
        emit_changed_settings(portal);
        while ((ret = sd_bus_process(portal->bus, NULL)) > 0) {}
        if (ret >= 0) ret = sd_bus_wait(portal->bus, 250000);
    }
    atomic_store(&portal->running, false);
    return NULL;
}

struct hatwm_screencast_portal *hatwm_screencast_portal_create(void) {
    struct hatwm_screencast_portal *portal = calloc(1, sizeof(*portal));
    if (portal == NULL) return NULL;
    pthread_mutex_init(&portal->lock, NULL);
    portal->cast.node_id = SPA_ID_INVALID;
    pw_init(NULL, NULL);
    portal->pw_loop = pw_thread_loop_new("hatwm-screencast", NULL);
    if (portal->pw_loop == NULL || pw_thread_loop_start(portal->pw_loop) < 0 ||
        pthread_create(&portal->thread, NULL, portal_thread, portal) != 0) {
        if (portal->pw_loop != NULL) pw_thread_loop_destroy(portal->pw_loop);
        pthread_mutex_destroy(&portal->lock);
        free(portal);
        return NULL;
    }
    return portal;
}

void hatwm_screencast_portal_destroy(struct hatwm_screencast_portal *portal) {
    if (portal == NULL) return;
    atomic_store(&portal->stopping, true);
    pthread_join(portal->thread, NULL);
    pthread_mutex_lock(&portal->lock);
    if (portal->render_locked_output != NULL) {
        if (portal->software_cursors_locked) {
            wlr_output_lock_software_cursors(
                portal->render_locked_output, false);
            portal->software_cursors_locked = false;
        }
        wlr_output_lock_attach_render(portal->render_locked_output, false);
        portal->render_locked_output = NULL;
    }
    stop_stream_locked(portal);
    while (portal->sessions != NULL) destroy_session_locked(portal->sessions);
    struct portal_output *output = portal->outputs;
    while (output != NULL) {
        struct portal_output *next = output->next;
        free(output->name);
        free(output);
        output = next;
    }
    pthread_mutex_unlock(&portal->lock);
    sd_bus_slot_unref(portal->screencast_slot);
    sd_bus_slot_unref(portal->settings_slot);
    sd_bus_flush_close_unref(portal->bus);
    pw_thread_loop_stop(portal->pw_loop);
    pw_thread_loop_destroy(portal->pw_loop);
    pthread_mutex_destroy(&portal->lock);
    free(portal);
}

void hatwm_screencast_portal_set_appearance(
        struct hatwm_screencast_portal *portal,
        uint32_t color_scheme, uint32_t reduced_motion) {
    if (portal == NULL) return;
    if (atomic_exchange(&portal->color_scheme, color_scheme) != color_scheme) {
        atomic_store(&portal->color_scheme_dirty, true);
    }
    if (atomic_exchange(&portal->reduced_motion, reduced_motion) != reduced_motion) {
        atomic_store(&portal->reduced_motion_dirty, true);
    }
}

void hatwm_screencast_portal_add_output(
        struct hatwm_screencast_portal *portal, struct wlr_output *output) {
    if (portal == NULL || output == NULL) return;
    struct portal_output *item = calloc(1, sizeof(*item));
    if (item == NULL) return;
    item->output = output;
    item->name = strdup(output->name != NULL ? output->name : "output");
    item->width = output->width;
    item->height = output->height;
    wlr_output_effective_resolution(output,
        &item->logical_width, &item->logical_height);
    pthread_mutex_lock(&portal->lock);
    item->next = portal->outputs;
    portal->outputs = item;
    pthread_mutex_unlock(&portal->lock);
}

void hatwm_screencast_portal_remove_output(
        struct hatwm_screencast_portal *portal, struct wlr_output *output) {
    if (portal == NULL || output == NULL) return;
    pthread_mutex_lock(&portal->lock);
    struct portal_output **link = &portal->outputs;
    while (*link != NULL && (*link)->output != output) link = &(*link)->next;
    if (*link != NULL) {
        struct portal_output *item = *link;
        if (portal->cast.target == item) stop_stream_locked(portal);
        if (portal->render_locked_output == output) {
            if (portal->software_cursors_locked) {
                wlr_output_lock_software_cursors(output, false);
                portal->software_cursors_locked = false;
            }
            wlr_output_lock_attach_render(output, false);
            portal->render_locked_output = NULL;
        }
        *link = item->next;
        free(item->name);
        free(item);
    }
    pthread_mutex_unlock(&portal->lock);
}

void hatwm_screencast_portal_tick(struct hatwm_screencast_portal *portal) {
    if (portal == NULL) return;
    pthread_mutex_lock(&portal->lock);
    struct wlr_output *desired = portal->cast.target != NULL &&
        portal->cast.stream != NULL ? portal->cast.target->output : NULL;
    bool want_software_cursors = desired != NULL && portal->cast.with_cursor;
    if (desired != portal->render_locked_output ||
            want_software_cursors != portal->software_cursors_locked) {
        if (portal->render_locked_output != NULL) {
            if (portal->software_cursors_locked) {
                wlr_output_lock_software_cursors(
                    portal->render_locked_output, false);
            }
            wlr_output_lock_attach_render(portal->render_locked_output, false);
        }
        portal->render_locked_output = desired;
        portal->software_cursors_locked = false;
        if (desired != NULL) {
            wlr_output_lock_attach_render(desired, true);
            if (want_software_cursors) {
                wlr_output_lock_software_cursors(desired, true);
                portal->software_cursors_locked = true;
            }
        }
    }
    if (desired != NULL && atomic_load(&portal->cast.active)) {
        pw_thread_loop_lock(portal->pw_loop);
        if (portal->cast.stream != NULL) {
            pw_stream_trigger_process(portal->cast.stream);
        }
        pw_thread_loop_unlock(portal->pw_loop);
        wlr_output_schedule_frame(desired);
    }
    pthread_mutex_unlock(&portal->lock);
}

static void submit_frame(struct hatwm_screencast_portal *portal,
        struct wlr_renderer *renderer, struct wlr_buffer *buffer,
        struct wlr_output *output) {
    if (!atomic_exchange(&portal->cast.wants_frame, false)) return;
    pw_thread_loop_lock(portal->pw_loop);
    struct pw_buffer *pw_buffer = pw_stream_dequeue_buffer(portal->cast.stream);
    if (pw_buffer == NULL || pw_buffer->buffer == NULL ||
        pw_buffer->buffer->n_datas == 0) {
        atomic_store(&portal->cast.wants_frame, true);
        pw_stream_trigger_process(portal->cast.stream);
        pw_thread_loop_unlock(portal->pw_loop);
        return;
    }
    struct spa_buffer *spa_buffer = pw_buffer->buffer;
    struct spa_data *data = &spa_buffer->datas[0];
    uint32_t stride = output->width * 4;
    size_t size = (size_t)stride * output->height;
    bool complete = data->data != NULL && data->chunk != NULL &&
        data->maxsize >= size;
    struct wlr_texture *texture = wlr_texture_from_buffer(renderer, buffer);
    if (complete && texture != NULL) {
        const struct wlr_texture_read_pixels_options options = {
            .data = data->data,
            .format = DRM_FORMAT_XRGB8888,
            .stride = stride,
        };
        complete = wlr_texture_read_pixels(texture, &options);
    } else {
        complete = false;
    }
    if (texture != NULL) {
        wlr_texture_destroy(texture);
    }
    if (data->chunk != NULL) {
        data->chunk->offset = 0;
        data->chunk->stride = stride;
        data->chunk->size = complete ? size : 0;
    }
    struct spa_meta_header *header = spa_buffer_find_meta_data(
        spa_buffer, SPA_META_Header, sizeof(*header));
    if (header != NULL) {
        struct timespec now;
        clock_gettime(CLOCK_MONOTONIC, &now);
        header->pts = SPA_TIMESPEC_TO_NSEC(&now);
        header->flags = complete ? 0 : SPA_META_HEADER_FLAG_CORRUPTED;
        header->seq = portal->cast.sequence++;
        header->dts_offset = 0;
    }
    pw_stream_queue_buffer(portal->cast.stream, pw_buffer);
    pw_stream_trigger_process(portal->cast.stream);
    pw_thread_loop_unlock(portal->pw_loop);
}

bool hatwm_screencast_portal_render(
        struct hatwm_screencast_portal *portal,
        struct wlr_scene_output *scene_output,
        struct wlr_renderer *renderer,
        struct wlr_output *output) {
    struct wlr_output_state state;
    wlr_output_state_init(&state);
    bool ok = wlr_scene_output_build_state(scene_output, &state, NULL);
    if (ok && portal != NULL) {
        pthread_mutex_lock(&portal->lock);
        if (portal->cast.target != NULL &&
            portal->cast.target->output == output && portal->cast.stream != NULL) {
            submit_frame(portal, renderer, state.buffer, output);
        }
        pthread_mutex_unlock(&portal->lock);
    }
    if (ok) ok = wlr_output_commit_state(output, &state);
    wlr_output_state_finish(&state);
    return ok;
}
