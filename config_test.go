package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/swaywm/go-wlroots/wlroots"
)

func TestWindowOpacitySetting(t *testing.T) {
	cfg := defaultConfig()
	if cfg.WindowOpacity != 1 {
		t.Fatalf("default opacity = %v, want 1", cfg.WindowOpacity)
	}

	parseSetting(&cfg, "window_opacity", "0.82")
	if cfg.WindowOpacity != 0.82 {
		t.Fatalf("parsed opacity = %v, want 0.82", cfg.WindowOpacity)
	}
}

func TestWindowOpacityRejectsOutOfRangeValue(t *testing.T) {
	cfg := defaultConfig()
	parseSetting(&cfg, "window_opacity", "-0.1")
	if cfg.WindowOpacity != 1 {
		t.Fatalf("negative opacity changed value to %v", cfg.WindowOpacity)
	}
	parseSetting(&cfg, "window_opacity", "1.1")
	if cfg.WindowOpacity != 1 {
		t.Fatalf("oversized opacity changed value to %v", cfg.WindowOpacity)
	}
}

func TestCursorThemeSettings(t *testing.T) {
	cfg := defaultConfig()
	if cfg.CursorTheme != "default" || cfg.CursorSize != 24 {
		t.Fatalf("unexpected cursor defaults: theme=%q size=%d", cfg.CursorTheme, cfg.CursorSize)
	}
	parseSetting(&cfg, "cursor_theme", "Bibata-Modern-Ice")
	parseSetting(&cfg, "cursor_size", "32")
	if cfg.CursorTheme != "Bibata-Modern-Ice" || cfg.CursorSize != 32 {
		t.Fatalf("cursor settings not parsed: theme=%q size=%d", cfg.CursorTheme, cfg.CursorSize)
	}
	parseSetting(&cfg, "cursor_size", "2")
	if cfg.CursorSize != 32 {
		t.Fatalf("invalid cursor size changed value to %d", cfg.CursorSize)
	}
}

func TestAppearanceSettings(t *testing.T) {
	cfg := defaultConfig()
	parseAppearanceSetting(&cfg, "color_scheme", "dark")
	parseAppearanceSetting(&cfg, "gtk_theme", "Adwaita-dark")
	parseAppearanceSetting(&cfg, "icon_theme", "Papirus-Dark")
	parseAppearanceSetting(&cfg, "font_name", "Noto Sans 11")
	parseAppearanceSetting(&cfg, "qt_style", "Fusion")
	parseAppearanceSetting(&cfg, "qt_platform_theme", "qt6ct")
	parseAppearanceSetting(&cfg, "window_button_layout", "menu:maximize,close")
	parseAppearanceSetting(&cfg, "cursor_theme", "Bibata-Modern-Ice")
	parseAppearanceSetting(&cfg, "cursor_size", "32")
	if cfg.ColorScheme != "dark" || cfg.GTKTheme != "Adwaita-dark" ||
		cfg.IconTheme != "Papirus-Dark" || cfg.FontName != "Noto Sans 11" ||
		cfg.QTStyle != "Fusion" || cfg.QTPlatformTheme != "qt6ct" ||
		cfg.CursorTheme != "Bibata-Modern-Ice" || cfg.CursorSize != 32 ||
		cfg.WindowButtonLayout != "menu:maximize,close" {
		t.Fatalf("appearance settings not parsed: %+v", cfg)
	}
	parseAppearanceSetting(&cfg, "color_scheme", "blue")
	if cfg.ColorScheme != "dark" {
		t.Fatalf("invalid color scheme changed value to %q", cfg.ColorScheme)
	}
}

func TestParseColorFormats(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  [4]float32
	}{
		{name: "hash RGB", value: "#336699", want: [4]float32{0x33 / 255.0, 0x66 / 255.0, 0x99 / 255.0, 1}},
		{name: "prefixed RGBA", value: "0xff804020", want: [4]float32{1, 0x80 / 255.0, 0x40 / 255.0, 0x20 / 255.0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseColor(tt.value)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("parseColor(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestParseColorRejectsMalformedValues(t *testing.T) {
	for _, value := range []string{"", "12345", "1234567", "not-a-color", "#gg0000"} {
		t.Run(value, func(t *testing.T) {
			if _, err := parseColor(value); err == nil {
				t.Fatalf("parseColor(%q) unexpectedly succeeded", value)
			}
		})
	}
}

func TestParseSettingBoundaries(t *testing.T) {
	cfg := defaultConfig()
	parseSetting(&cfg, "gaps", "0")
	parseSetting(&cfg, "border_size", "32")
	parseSetting(&cfg, "border_rounding", "128")
	parseSetting(&cfg, "cursor_size", "256")
	parseSetting(&cfg, "workspaces", "1")
	parseSetting(&cfg, "volume_step", "100")
	if cfg.Gaps != 0 || cfg.BorderSize != 32 || cfg.BorderRounding != 128 ||
		cfg.CursorSize != 256 || cfg.WorkspaceCount != 1 || cfg.VolumeStep != 100 {
		t.Fatalf("valid boundary settings were not accepted: %+v", cfg)
	}

	parseSetting(&cfg, "gaps", "-1")
	parseSetting(&cfg, "border_size", "33")
	parseSetting(&cfg, "border_rounding", "129")
	parseSetting(&cfg, "cursor_size", "257")
	parseSetting(&cfg, "workspaces", "0")
	parseSetting(&cfg, "volume_step", "101")
	if cfg.Gaps != 0 || cfg.BorderSize != 32 || cfg.BorderRounding != 128 ||
		cfg.CursorSize != 256 || cfg.WorkspaceCount != 1 || cfg.VolumeStep != 100 {
		t.Fatalf("invalid boundary settings changed the configuration: %+v", cfg)
	}
}

func TestParseKeyComboAliasesAndErrors(t *testing.T) {
	modifier, sym, err := parseKeyCombo("Super+Control+Shift+Return")
	if err != nil {
		t.Fatal(err)
	}
	wantModifier := wlroots.KeyboardModifierLogo |
		wlroots.KeyboardModifierCtrl | wlroots.KeyboardModifierShift
	if modifier != wantModifier || sym == 0 {
		t.Fatalf("unexpected key combination: modifier=%v sym=%v", modifier, sym)
	}

	for _, combo := range []string{"Hyper+Return", "Mod4+DefinitelyNotAKey"} {
		if _, _, err := parseKeyCombo(combo); err == nil {
			t.Fatalf("parseKeyCombo(%q) unexpectedly succeeded", combo)
		}
	}
}

func TestLoadConfigParsesSectionsAndAddsDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDir := filepath.Join(home, ".config", "hatwm")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	contents := `[settings]
gaps = 17
workspaces = 3
keyboard_layouts = us, ge

[appearance]
color_scheme = dark
icon_theme = Papirus-Dark

[keybindings]
Mod4+x = exec example-command
BrokenModifier+x = close

[autostart]
panel = hatwmpanel
`
	if err := os.WriteFile(filepath.Join(configDir, "config"), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Gaps != 17 || cfg.WorkspaceCount != 3 || cfg.ColorScheme != "dark" ||
		cfg.IconTheme != "Papirus-Dark" {
		t.Fatalf("configuration sections were not loaded: %+v", cfg)
	}
	if len(cfg.KeyboardLayouts) != 2 || cfg.KeyboardLayouts[0] != "us" || cfg.KeyboardLayouts[1] != "ge" {
		t.Fatalf("keyboard layouts = %v, want [us ge]", cfg.KeyboardLayouts)
	}
	if len(cfg.Autostart) != 1 || cfg.Autostart[0] != "hatwmpanel" {
		t.Fatalf("autostart = %v, want [hatwmpanel]", cfg.Autostart)
	}

	actions := make(map[string]int)
	for _, binding := range cfg.KeyBindings {
		actions[binding.Action]++
	}
	if actions["exec"] == 0 || actions["workspace"] != 3 ||
		actions["move_to_workspace"] != 3 || actions["volume_up"] != 1 ||
		actions["volume_down"] != 1 || actions["volume_mute"] != 1 {
		t.Fatalf("default bindings were not injected exactly once: %v", actions)
	}
}

func TestLoadConfigCreatesDefaultFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WorkspaceCount != 9 || len(cfg.KeyBindings) == 0 {
		t.Fatalf("unexpected default configuration: %+v", cfg)
	}
	path := filepath.Join(home, ".config", "hatwm", "config")
	if info, err := os.Stat(path); err != nil {
		t.Fatalf("default config was not created: %v", err)
	} else if info.Mode().Perm() != 0o644 {
		t.Fatalf("default config permissions = %o, want 644", info.Mode().Perm())
	}
}

func TestParseCSVTrimsAndDropsEmptyValues(t *testing.T) {
	want := []string{"us", "ge", "ru"}
	if got := parseCSV(" us, ,ge,, ru "); !reflect.DeepEqual(got, want) {
		t.Fatalf("parseCSV() = %v, want %v", got, want)
	}
	if got := parseCSV(" , , "); len(got) != 0 {
		t.Fatalf("empty CSV = %v, want no values", got)
	}
}

func TestExpandHomeOnlyExpandsHomePrefix(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got, want := expandHome("~/Pictures/wall.png"), filepath.Join(home, "Pictures", "wall.png"); got != want {
		t.Fatalf("expanded path = %q, want %q", got, want)
	}
	for _, path := range []string{"~someone/file", "/tmp/file", "relative/file"} {
		if got := expandHome(path); got != path {
			t.Errorf("expandHome(%q) = %q", path, got)
		}
	}
}

func TestParseKeyBindingPreservesCommandArgument(t *testing.T) {
	binding, err := parseKeyBinding("Mod4+Return", "exec kitty --class floating")
	if err != nil {
		t.Fatal(err)
	}
	if binding.Action != "exec" || binding.Arg != "kitty --class floating" {
		t.Fatalf("binding = %+v", binding)
	}
	if _, err := parseKeyBinding("Mod4+Return", "  "); err == nil {
		t.Fatal("empty action unexpectedly succeeded")
	}
}

func TestDefaultBindingInjectionDoesNotOverrideUserBindings(t *testing.T) {
	workspace, err := parseKeyBinding("Mod4+1", "exec custom-workspace-command")
	if err != nil {
		t.Fatal(err)
	}
	volume, err := parseKeyBinding("XF86AudioRaiseVolume", "exec custom-volume-command")
	if err != nil {
		t.Fatal(err)
	}
	cfg := defaultConfig()
	cfg.WorkspaceCount = 2
	cfg.KeyBindings = []KeyBinding{workspace, volume}
	ensureWorkspaceKeyBindings(&cfg)
	ensureMediaKeyBindings(&cfg)

	workspaceKeyCount, volumeKeyCount := 0, 0
	for _, binding := range cfg.KeyBindings {
		if binding.Modifier == workspace.Modifier && binding.Sym == workspace.Sym {
			workspaceKeyCount++
		}
		if binding.Modifier == volume.Modifier && binding.Sym == volume.Sym {
			volumeKeyCount++
		}
	}
	if workspaceKeyCount != 1 || volumeKeyCount != 1 {
		t.Fatalf("user bindings were duplicated: workspace=%d volume=%d", workspaceKeyCount, volumeKeyCount)
	}
}

func TestParseSettingsAcceptsBooleanAndAnimationValues(t *testing.T) {
	cfg := defaultConfig()
	parseSetting(&cfg, "layout", "floating")
	parseSetting(&cfg, "focus_follows_mouse", "true")
	parseSetting(&cfg, "notifications", "false")
	parseSetting(&cfg, "animations", "false")
	parseSetting(&cfg, "animation_duration_ms", "0")
	parseSetting(&cfg, "animation_easing", "linear")
	parseSetting(&cfg, "animation_open_offset", "300")
	if cfg.Tiling || !cfg.FocusFollowsMouse || cfg.Notifications || cfg.Animations ||
		cfg.AnimationDurationMS != 0 || cfg.AnimationEasing != "linear" ||
		cfg.AnimationOpenOffset != 300 {
		t.Fatalf("settings were not parsed: %+v", cfg)
	}
}
