#!/bin/sh
set -eu

hatwm_binary=$1
kitty_binary=$2
runtime_dir=$(mktemp -d)
test_home="$runtime_dir/home"
hatwm_pid=
client_pid=

cleanup() {
    if [ -n "$client_pid" ] && kill -0 "$client_pid" 2>/dev/null; then
        kill -TERM "$client_pid" 2>/dev/null || true
        wait "$client_pid" 2>/dev/null || true
    fi
    if [ -n "$hatwm_pid" ] && kill -0 "$hatwm_pid" 2>/dev/null; then
        kill -TERM "$hatwm_pid" 2>/dev/null || true
        wait "$hatwm_pid" 2>/dev/null || true
    fi
    rm -rf "$runtime_dir"
}
trap cleanup EXIT INT TERM

mkdir -p "$test_home/.config/hatwm"
printf '%s\n' \
    '[settings]' \
    'notifications = false' \
    'keyboard_layouts = us' \
    'animations = false' \
    '' \
    '[window-rule title-urgency]' \
    'app_id = hatwm-smoke' \
    'urgent_on_title_change = true' \
    >"$test_home/.config/hatwm/config"

HOME="$test_home" \
XDG_RUNTIME_DIR="$runtime_dir" \
WLR_BACKENDS=headless \
WLR_HEADLESS_OUTPUTS=2 \
WLR_RENDERER=pixman \
HATWM_DISABLE_XWAYLAND=1 \
"$hatwm_binary" >"$runtime_dir/hatwm.log" 2>&1 &
hatwm_pid=$!

attempt=0
while [ ! -S "$runtime_dir/hatwm/ipc.sock" ] || [ ! -S "$runtime_dir/wayland-0" ]; do
    if ! kill -0 "$hatwm_pid" 2>/dev/null; then
        cat "$runtime_dir/hatwm.log" >&2
        exit 1
    fi
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 100 ]; then
        cat "$runtime_dir/hatwm.log" >&2
        echo 'nested HatWM did not become ready' >&2
        exit 1
    fi
    sleep 0.05
done

HATWM_SOCKET="$runtime_dir/hatwm/ipc.sock" hatwmctl workspace 2 >/dev/null
HOME="$test_home" \
XDG_RUNTIME_DIR="$runtime_dir" \
WAYLAND_DISPLAY=wayland-0 \
"$kitty_binary" --class hatwm-smoke sh -c \
    'sleep 0.5; printf "\033]2;hidden title changed\007"; sleep 3' &
client_pid=$!

sleep 0.2
HATWM_SOCKET="$runtime_dir/hatwm/ipc.sock" hatwmctl workspace 1 >/dev/null
sleep 0.7

if ! kill -0 "$hatwm_pid" 2>/dev/null; then
    cat "$runtime_dir/hatwm.log" >&2
    echo 'nested HatWM exited while the client was active' >&2
    exit 1
fi

workspace_json=$(HATWM_SOCKET="$runtime_dir/hatwm/ipc.sock" \
    hatwmctl -json workspaces)
workspace_compact=$(printf '%s' "$workspace_json" | tr -d '[:space:]')
case "$workspace_compact" in
    *'"number":2'*'"urgent":true'*) ;;
    *)
        printf '%s\n' "$workspace_json" >&2
        echo 'hidden title change did not mark workspace 2 urgent' >&2
        exit 1
        ;;
esac

# Exercise The Hat through raw IPC so the test does not depend on the installed
# hatwmctl version supporting commands added in the same source checkout.
HATWM_SOCKET="$runtime_dir/hatwm/ipc.sock" hatwmctl workspace 2 >/dev/null
printf '%s\n' '{"type":"command","id":20,"command":"hat_stash"}' |
    socat -T 1 -,ignoreeof UNIX-CONNECT:"$runtime_dir/hatwm/ipc.sock" >/dev/null
hat_json=$(printf '%s\n' '{"type":"get_hat","id":21}' |
    socat -T 1 -,ignoreeof UNIX-CONNECT:"$runtime_dir/hatwm/ipc.sock")
hat_compact=$(printf '%s' "$hat_json" | tr -d '[:space:]')
case "$hat_compact" in
    *'"result":[{'*'"in_hat":true'*) ;;
    *)
        printf '%s\n' "$hat_json" >&2
        echo 'stashed window was not returned by get_hat' >&2
        exit 1
        ;;
esac

HATWM_SOCKET="$runtime_dir/hatwm/ipc.sock" hatwmctl workspace 1 >/dev/null
restore_json=$(printf '%s\n' \
    '{"type":"command","id":22,"command":"hat_restore"}' |
    socat -T 1 -,ignoreeof UNIX-CONNECT:"$runtime_dir/hatwm/ipc.sock")
restore_compact=$(printf '%s' "$restore_json" | tr -d '[:space:]')
case "$restore_compact" in
    *'"success":true'*'"workspace":1'*'"hat_count":0'*) ;;
    *)
        printf '%s\n' "$restore_json" >&2
        echo 'Hat restore did not summon the window onto workspace 1' >&2
        exit 1
        ;;
esac

kill -TERM "$client_pid" 2>/dev/null || true
wait "$client_pid" 2>/dev/null || true
client_pid=
kill -TERM "$hatwm_pid"
if ! wait "$hatwm_pid"; then
    cat "$runtime_dir/hatwm.log" >&2
    echo 'nested HatWM did not shut down cleanly' >&2
    exit 1
fi
hatwm_pid=
