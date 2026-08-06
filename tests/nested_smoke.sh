#!/bin/sh
set -eu

hatwm_binary=$1
kitty_binary=$2
runtime_dir=$(mktemp -d)
test_home="$runtime_dir/home"
hatwm_pid=

cleanup() {
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

HOME="$test_home" \
XDG_RUNTIME_DIR="$runtime_dir" \
WAYLAND_DISPLAY=wayland-0 \
"$kitty_binary" --class hatwm-smoke sh -c 'sleep 0.25'

if ! kill -0 "$hatwm_pid" 2>/dev/null; then
    cat "$runtime_dir/hatwm.log" >&2
    echo 'nested HatWM exited while the client was active' >&2
    exit 1
fi

HATWM_SOCKET="$runtime_dir/hatwm/ipc.sock" hatwmctl state >/dev/null
kill -TERM "$hatwm_pid"
if ! wait "$hatwm_pid"; then
    cat "$runtime_dir/hatwm.log" >&2
    echo 'nested HatWM did not shut down cleanly' >&2
    exit 1
fi
hatwm_pid=
