# HatWM IPC protocol v1

HatWM listens on:

```text
$XDG_RUNTIME_DIR/hatwm/ipc.sock
```

It also exports the path as `HATWM_SOCKET` before running autostart commands.
The socket directory is mode `0700` and the socket is mode `0600`.

The protocol is newline-delimited JSON: one JSON object per line.

## Handshake

Request:

```json
{"type":"hello","id":1,"protocol_version":1,"client":"hatwm-panel","client_version":"0.1.0"}
```

## Queries

```json
{"type":"get_state","id":2}
{"type":"get_workspaces","id":3}
{"type":"get_windows","id":4}
```

## Event subscription

```json
{"type":"subscribe","id":5,"events":["workspace_changed","workspace_updated","window_opened","window_closed","window_moved","window_updated","window_urgent","focus_changed","layout_changed","fullscreen_changed","keyboard_layout_changed","wallpaper_changed","config_reloaded","shutdown"]}
```

Use `"*"` to subscribe to every event.

## Commands

```json
{"type":"command","id":6,"command":"workspace","workspace":2}
{"type":"command","id":7,"command":"move_to_workspace","workspace":3}
{"type":"command","id":8,"command":"toggle_tiling"}
{"type":"command","id":9,"command":"toggle_keyboard_layout"}
{"type":"command","id":10,"command":"toggle_fullscreen"}
{"type":"command","id":11,"command":"cycle_focus"}
{"type":"command","id":12,"command":"close"}
{"type":"command","id":13,"command":"reload_config"}
{"type":"command","id":14,"command":"set_wallpaper","wallpaper":"/home/user/Pictures/wallpaper.png"}
```

`set_wallpaper` changes the running session without rewriting HatWM's config.
HatWM validates the image before replacing the active `hatwmbg` process.
Successful changes emit `wallpaper_changed`, and `get_state` reports the
active absolute path in its `wallpaper` field.

## Manual testing

```sh
printf '%s\n' '{"type":"get_state","id":1}' | socat - UNIX-CONNECT:"$XDG_RUNTIME_DIR/hatwm/ipc.sock"
```

For an event stream:

```sh
socat - UNIX-CONNECT:"$XDG_RUNTIME_DIR/hatwm/ipc.sock"
```

Then enter:

```json
{"type":"subscribe","id":1,"events":["*"]}
```

## Geometry data for panels and pagers

`get_state` includes the primary output bounds. `get_windows` includes each
mapped window's outer rectangle. These fields are additive and keep protocol v1
backward compatible.

Example output data in `get_state`:

```json
{
  "output": {
    "x": 0,
    "y": 0,
    "width": 1920,
    "height": 1080,
    "usable_x": 0,
    "usable_y": 32,
    "usable_width": 1920,
    "usable_height": 1048
  }
}
```

Example window returned by `get_windows`:

```json
{
  "id": 12,
  "workspace": 2,
  "mapped": true,
  "focused": false,
  "urgent": true,
  "dialog": false,
  "modal": false,
  "floating": false,
  "fullscreen": false,
  "xwayland": false,
  "x": 20,
  "y": 52,
  "width": 930,
  "height": 1008
}
```

Workspace and window records include an additive `urgent` boolean. HatWM sets
it when a mapped application requests activation from another workspace and
clears it when that window receives focus. Urgency changes emit
`window_urgent` and `workspace_updated`.

Window records also expose additive `dialog`, `modal`, and `floating` fields.
An `xdg-dialog-v1` state change emits `window_updated`.

The rectangle uses compositor/global output coordinates and includes HatWM's
server-side border when one is visible. A workspace pager can normalize these
coordinates against `output.width` and `output.height` to draw miniature window
positions.
