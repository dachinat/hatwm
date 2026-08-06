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
libinput = dependency('libinput', required: true)
pipewire = dependency('libpipewire-0.3', required: true)
systemd = dependency('libsystemd', required: true)
xcb = dependency('xcb', required: true)
xcb_shape = dependency('xcb-shape', required: true)

source_root = meson.project_source_root()
build_root = meson.project_build_root()

layer_shell_xml = files(
  'internal/compositor/protocols/wlr-layer-shell-unstable-v1.xml',
)
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
  'cmd/hatwm/main.go',
  'internal/compositor/animations.go',
  'internal/compositor/appearance.go',
  'internal/compositor/build_info.go',
  'internal/compositor/config.go',
  'internal/compositor/cursor_image.go',
  'internal/compositor/decorations.go',
  'internal/compositor/desktop_protocols.go',
  'internal/compositor/foreign_toplevel.go',
  'internal/compositor/input.go',
  'internal/compositor/input_config.go',
  'internal/compositor/input_protocols.go',
  'internal/compositor/ipc.go',
  'internal/compositor/layer_shell.go',
  'internal/compositor/layout.go',
  'internal/compositor/maximize.go',
  'internal/compositor/model.go',
  'internal/compositor/output_protocols.go',
  'internal/compositor/output_state.go',
  'internal/compositor/screencast_portal.go',
  'internal/compositor/screencopy.go',
  'internal/compositor/server.go',
  'internal/compositor/session_lock.go',
  'internal/compositor/view_ops.go',
  'internal/compositor/views.go',
  'internal/compositor/wallpaper.go',
  'internal/compositor/window_opacity.go',
  'internal/compositor/window_rule_config.go',
  'internal/compositor/window_rules.go',
  'internal/compositor/workspace.go',
  'internal/compositor/xdg_activation.go',
  'internal/compositor/xdg_dialog.go',
  'internal/compositor/xwayland.go',
  'internal/compositor/clipboard.c',
  'internal/compositor/clipboard.h',
  'internal/compositor/cursor_image.c',
  'internal/compositor/cursor_image.h',
  'internal/compositor/desktop_protocols.c',
  'internal/compositor/desktop_protocols.h',
  'internal/compositor/foreign_toplevel.c',
  'internal/compositor/foreign_toplevel.h',
  'internal/compositor/input_protocols.c',
  'internal/compositor/input_protocols.h',
  'internal/compositor/input_config.c',
  'internal/compositor/input_config.h',
  'internal/compositor/keyboard_layout.c',
  'internal/compositor/keyboard_layout.h',
  'internal/compositor/layer_shell.c',
  'internal/compositor/layer_shell.h',
  'internal/compositor/maximize.c',
  'internal/compositor/maximize.h',
  'internal/compositor/output_protocols.c',
  'internal/compositor/output_protocols.h',
  'internal/compositor/screencast_portal.c',
  'internal/compositor/screencast_portal.h',
  'internal/compositor/session_lock.c',
  'internal/compositor/session_lock.h',
  'internal/compositor/workspace.c',
  'internal/compositor/workspace.h',
  'internal/compositor/xdg-dialog-v1-protocol.c',
  'internal/compositor/xdg_activation.c',
  'internal/compositor/xdg_activation.h',
  'internal/compositor/xdg_dialog.c',
  'internal/compositor/xdg_dialog.h',
  'internal/compositor/xwayland.c',
  'internal/compositor/xwayland.h',
  'internal/compositor/protocols/content-type-v1-protocol.h',
  'internal/compositor/protocols/cursor-shape-v1-protocol.h',
  'internal/compositor/protocols/pointer-constraints-unstable-v1-protocol.h',
  'internal/compositor/protocols/tearing-control-v1-protocol.h',
  'internal/compositor/protocols/wlr-output-power-management-unstable-v1-protocol.h',
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
    './cmd/hatwm',
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

coverage_file = join_paths(build_root, 'coverage.out')
run_target(
  'test-coverage',
  command: [
    go, '-C', source_root, 'test',
    '-coverprofile=' + coverage_file,
    './...',
  ],
  env: go_env,
  depends: generated_protocols,
)

sanitizer_env = environment()
sanitizer_env.set('CGO_CFLAGS', '-I' + build_root + ' -fsanitize=address,undefined -fno-omit-frame-pointer')
sanitizer_env.set('CGO_LDFLAGS', '-fsanitize=address,undefined')
sanitizer_env.set('ASAN_OPTIONS', 'detect_leaks=0:halt_on_error=1')
sanitizer_env.set('UBSAN_OPTIONS', 'halt_on_error=1:print_stacktrace=1')
run_target(
  'test-sanitizers',
  command: [go, '-C', source_root, 'test', './...'],
  env: sanitizer_env,
  depends: generated_protocols,
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

smoke_test = find_program('tests/nested_smoke.sh')
kitty = find_program('kitty', required: false)
if kitty.found()
  run_target(
    'nested-smoke',
    command: [smoke_test, join_paths(build_root, 'hatwm'), kitty],
    depends: hatwm,
  )
endif

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
    'libinput': libinput.version(),
    'PipeWire': pipewire.version(),
    'systemd': systemd.version(),
    'XCB': xcb.version(),
    'Install prefix': get_option('prefix'),
  },
  section: 'hatwm',
)
