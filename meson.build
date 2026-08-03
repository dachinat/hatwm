project(
  'hatwm',
  version: '0.3.0',
  meson_version: '>=1.1.0',
  default_options: ['prefix=/usr'],
)

go = find_program('go', required: true)
wayland_scanner = find_program('wayland-scanner', required: true)

wlroots = dependency('wlroots-0.18', required: true)
wayland_server = dependency('wayland-server', required: true)
wayland_protocols = dependency('wayland-protocols', required: true)
xkbcommon = dependency('xkbcommon', required: true)
pipewire = dependency('libpipewire-0.3', required: true)
systemd = dependency('libsystemd', required: true)
xcb = dependency('xcb', required: true)
xcb_shape = dependency('xcb-shape', required: true)

source_root = meson.project_source_root()
build_root = meson.project_build_root()

layer_shell_xml = files('protocols/wlr-layer-shell-unstable-v1.xml')
wayland_protocols_dir = wayland_protocols.get_variable(pkgconfig: 'pkgdatadir')
xdg_shell_xml = join_paths(
  wayland_protocols_dir,
  'stable/xdg-shell/xdg-shell.xml',
)
xdg_dialog_xml = join_paths(
  wayland_protocols_dir,
  'staging/xdg-dialog/xdg-dialog-v1.xml',
)

layer_shell_header = custom_target(
  'wlr-layer-shell-server-header',
  input: layer_shell_xml,
  output: 'wlr-layer-shell-unstable-v1-protocol.h',
  command: [wayland_scanner, 'server-header', '@INPUT@', '@OUTPUT@'],
)

layer_shell_code = custom_target(
  'wlr-layer-shell-private-code',
  input: layer_shell_xml,
  output: 'wlr-layer-shell-unstable-v1-protocol.c',
  command: [wayland_scanner, 'private-code', '@INPUT@', '@OUTPUT@'],
)

xdg_shell_header = custom_target(
  'xdg-shell-server-header',
  input: xdg_shell_xml,
  output: 'xdg-shell-protocol.h',
  command: [wayland_scanner, 'server-header', '@INPUT@', '@OUTPUT@'],
)

xdg_dialog_header = custom_target(
  'xdg-dialog-server-header',
  input: xdg_dialog_xml,
  output: 'xdg-dialog-v1-protocol.h',
  command: [wayland_scanner, 'server-header', '@INPUT@', '@OUTPUT@'],
)

xdg_dialog_code = custom_target(
  'xdg-dialog-private-code',
  input: xdg_dialog_xml,
  output: 'xdg-dialog-v1-protocol-generated.c',
  command: [wayland_scanner, 'private-code', '@INPUT@', '@OUTPUT@'],
)

# Go owns the cgo compile and final compositor link. Listing the native and Go
# inputs here lets Ninja rebuild HatWM when either side of the boundary changes.
hatwm_sources = files(
  'go.mod',
  'go.sum',
  'animations.go',
  'appearance.go',
  'config.go',
  'cursor_image.go',
  'decorations.go',
  'desktop_protocols.go',
  'foreign_toplevel.go',
  'input.go',
  'input_protocols.go',
  'ipc.go',
  'layer_shell.go',
  'layout.go',
  'main.go',
  'maximize.go',
  'model.go',
  'output_protocols.go',
  'screencast_portal.go',
  'screencopy.go',
  'server.go',
  'session_lock.go',
  'view_ops.go',
  'views.go',
  'wallpaper.go',
  'window_opacity.go',
  'workspace.go',
  'xdg_activation.go',
  'xdg_dialog.go',
  'xwayland.go',
  'clipboard.c',
  'clipboard.h',
  'cursor_image.c',
  'cursor_image.h',
  'desktop_protocols.c',
  'desktop_protocols.h',
  'foreign_toplevel.c',
  'foreign_toplevel.h',
  'input_protocols.c',
  'input_protocols.h',
  'keyboard_layout.c',
  'keyboard_layout.h',
  'layer_shell.c',
  'layer_shell.h',
  'maximize.c',
  'maximize.h',
  'output_protocols.c',
  'output_protocols.h',
  'screencast_portal.c',
  'screencast_portal.h',
  'session_lock.c',
  'session_lock.h',
  'workspace.c',
  'workspace.h',
  'xdg-dialog-v1-protocol.c',
  'xdg_activation.c',
  'xdg_activation.h',
  'xdg_dialog.c',
  'xdg_dialog.h',
  'xwayland.c',
  'xwayland.h',
  'protocols/content-type-v1-protocol.h',
  'protocols/cursor-shape-v1-protocol.h',
  'protocols/pointer-constraints-unstable-v1-protocol.h',
  'protocols/tearing-control-v1-protocol.h',
  'protocols/wlr-output-power-management-unstable-v1-protocol.h',
)

generated_protocols = [
  layer_shell_header,
  layer_shell_code,
  xdg_shell_header,
  xdg_dialog_header,
  xdg_dialog_code,
]

go_env = environment()
go_env.set('CGO_CFLAGS', '-I' + build_root)

hatwm = custom_target(
  'hatwm',
  input: hatwm_sources + generated_protocols,
  output: 'hatwm',
  command: [
    go,
    '-C', source_root,
    'build',
    '-a',
    '-buildvcs=false',
    '-o', join_paths(build_root, 'hatwm'),
    '.',
  ],
  env: go_env,
  build_by_default: true,
  install: true,
  install_dir: get_option('bindir'),
  console: true,
)

test(
  'go test',
  go,
  args: ['-C', source_root, 'test', './...'],
  env: go_env,
  workdir: source_root,
  depends: generated_protocols,
  timeout: 180,
)

test(
  'go vet',
  go,
  args: ['-C', source_root, 'vet', './...'],
  env: go_env,
  workdir: source_root,
  depends: generated_protocols,
  timeout: 180,
)

run_target(
  'fmt',
  command: [go, '-C', source_root, 'fmt', './...'],
)

run_target(
  'vet',
  command: [go, '-C', source_root, 'vet', './...'],
  env: go_env,
  depends: generated_protocols,
)

nested_env = environment()
nested_env.set('WLR_BACKENDS', 'wayland')
nested_env.set('WLR_WL_OUTPUTS', '1')
run_target('run', command: hatwm)
run_target('nested', command: hatwm, env: nested_env)

install_data(
  'data/hatwm-portals.conf',
  install_dir: join_paths(get_option('datadir'), 'xdg-desktop-portal'),
)
install_data(
  'data/hatwm.portal',
  install_dir: join_paths(
    get_option('datadir'),
    'xdg-desktop-portal/portals',
  ),
)
install_data(
  'data/hatwm-session',
  install_dir: get_option('bindir'),
  install_mode: 'rwxr-xr-x',
)
install_data(
  'data/hatwm.desktop',
  install_dir: join_paths(get_option('datadir'), 'wayland-sessions'),
)

summary(
  {
    'Go': go.full_path(),
    'Wayland scanner': wayland_scanner.full_path(),
    'wlroots': wlroots.version(),
    'Wayland server': wayland_server.version(),
    'Wayland protocols': wayland_protocols.version(),
    'xkbcommon': xkbcommon.version(),
    'PipeWire': pipewire.version(),
    'systemd': systemd.version(),
    'XCB': xcb.version(),
    'Install prefix': get_option('prefix'),
  },
  section: 'hatwm',
)
