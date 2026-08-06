package compositor

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/swaywm/go-wlroots/wlroots"
	"github.com/swaywm/go-wlroots/xkb"
)

type KeyBinding struct {
	Modifier wlroots.KeyboardModifier
	Sym      xkb.KeySym
	Action   string
	Arg      string
}

type Config struct {
	Gaps                int
	Tiling              bool
	Wallpaper           string
	WindowOpacity       float64
	BorderSize          int
	BorderRounding      int
	FocusFollowsMouse   bool
	CursorTheme         string
	CursorSize          int
	ColorScheme         string
	GTKTheme            string
	IconTheme           string
	FontName            string
	QTStyle             string
	QTPlatformTheme     string
	WindowButtonLayout  string
	MoveStep            int
	VolumeStep          int
	WorkspaceCount      int
	KeyboardLayouts     []string
	KeyboardVariants    string
	KeyboardOptions     string
	Notifications       bool
	NotificationDaemon  string
	Animations          bool
	AnimationDurationMS int
	AnimationEasing     string
	AnimationOpenOffset int
	ActiveBorderColor   [4]float32
	InactiveBorderColor [4]float32
	WindowRules         []WindowRule
	KeyBindings         []KeyBinding
	Autostart           []string
}

const defaultConfigFile = `# HatWM configuration

[settings]
gaps = 10
layout = tiling
# wallpaper = ~/Pictures/wallpaper.jpg
window_opacity = 1.0
border_size = 2
border_rounding = 0
focus_follows_mouse = false
move_step = 40
volume_step = 5
workspaces = 9
keyboard_layouts = us,ge,ru
# keyboard_variants =
# keyboard_options =
notifications = true
notification_daemon = auto
animations = true
animation_duration_ms = 180
animation_easing = ease_out_cubic
animation_open_offset = 24
active_border_color = 0x6aa9e9ff
inactive_border_color = 0x34465fff

[appearance]
color_scheme = default
# gtk_theme = Adwaita
# icon_theme = Adwaita
cursor_theme = default
cursor_size = 24
# font_name = Noto Sans 10
# qt_style = Fusion
# qt_platform_theme = qt6ct
window_button_layout = appmenu:maximize,close

[keybindings]
Mod4+Return = exec kitty
Mod4+1 = workspace 1
Mod4+2 = workspace 2
Mod4+3 = workspace 3
Mod4+4 = workspace 4
Mod4+5 = workspace 5
Mod4+6 = workspace 6
Mod4+7 = workspace 7
Mod4+8 = workspace 8
Mod4+9 = workspace 9
Mod4+Shift+1 = move_to_workspace 1
Mod4+Shift+2 = move_to_workspace 2
Mod4+Shift+3 = move_to_workspace 3
Mod4+Shift+4 = move_to_workspace 4
Mod4+Shift+5 = move_to_workspace 5
Mod4+Shift+6 = move_to_workspace 6
Mod4+Shift+7 = move_to_workspace 7
Mod4+Shift+8 = move_to_workspace 8
Mod4+Shift+9 = move_to_workspace 9
Mod4+q = close
Mod4+space = toggle_tiling
Mod4+Shift+space = toggle_keyboard_layout
Mod4+f = toggle_fullscreen
Mod4+Tab = cycle_focus
Mod4+Left = move_left
Mod4+Right = move_right
Mod4+Up = move_up
Mod4+Down = move_down
XF86AudioRaiseVolume = volume_up
XF86AudioLowerVolume = volume_down
XF86AudioMute = volume_mute
Mod4+Shift+Escape = exit

[autostart]
# waybar = waybar

# [window-rule color-picker]
# app_id = com.github.wayland-color-picker-gtk4
# floating = true
# centered = true
# keep_above = true
# opacity = 1.0
`

func defaultConfig() Config {
	active, _ := parseColor("0x6aa9e9ff")
	inactive, _ := parseColor("0x34465fff")
	return Config{
		Gaps:              10,
		Tiling:            true,
		WindowOpacity:     1,
		BorderSize:        2,
		BorderRounding:    0,
		FocusFollowsMouse: false,
		CursorTheme:       "default",
		CursorSize:        24,
		// Empty means that an older config without [appearance] leaves the
		// user's existing toolkit preference untouched. Newly generated configs
		// explicitly select "default" in that section.
		ColorScheme:         "",
		WindowButtonLayout:  "appmenu:maximize,close",
		MoveStep:            40,
		VolumeStep:          5,
		WorkspaceCount:      9,
		KeyboardLayouts:     []string{"us"},
		Notifications:       true,
		NotificationDaemon:  "auto",
		Animations:          true,
		AnimationDurationMS: 180,
		AnimationEasing:     "ease_out_cubic",
		AnimationOpenOffset: 24,
		ActiveBorderColor:   active,
		InactiveBorderColor: inactive,
	}
}

func GetConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "hatwm", "config")
}

func LoadConfig() (Config, error) {
	cfg := defaultConfig()
	path := GetConfigPath()
	if path == "" {
		return cfg, fmt.Errorf("could not determine user home directory")
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return cfg, err
		}
		if err := os.WriteFile(path, []byte(defaultConfigFile), 0o644); err != nil {
			return cfg, err
		}
		slog.Info("created default config", "path", path)
	}

	f, err := os.Open(path)
	if err != nil {
		return cfg, err
	}
	defer f.Close()

	section := ""
	var currentWindowRule *WindowRule
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			currentWindowRule = nil
			if name, ok := windowRuleSectionName(section); ok {
				cfg.WindowRules = append(cfg.WindowRules, WindowRule{Name: name})
				currentWindowRule = &cfg.WindowRules[len(cfg.WindowRules)-1]
				section = "window-rule"
			}
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch section {
		case "settings":
			parseSetting(&cfg, key, value)
		case "appearance":
			parseAppearanceSetting(&cfg, key, value)
		case "keybindings":
			binding, err := parseKeyBinding(key, value)
			if err != nil {
				slog.Warn("ignoring invalid keybinding", "binding", key, "error", err)
				continue
			}
			cfg.KeyBindings = append(cfg.KeyBindings, binding)
		case "autostart":
			if value != "" {
				cfg.Autostart = append(cfg.Autostart, value)
			}
		case "window-rule":
			if currentWindowRule == nil {
				continue
			}
			if err := parseWindowRuleSetting(currentWindowRule, key, value); err != nil {
				slog.Warn("ignoring invalid window rule setting",
					"rule", currentWindowRule.Name, "setting", key, "error", err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return cfg, err
	}
	ensureMediaKeyBindings(&cfg)
	ensureWorkspaceKeyBindings(&cfg)
	return cfg, nil
}

func parseAppearanceSetting(cfg *Config, key, value string) {
	switch strings.ToLower(key) {
	case "color_scheme":
		switch scheme := strings.ToLower(strings.TrimSpace(value)); scheme {
		case "default", "dark", "light":
			cfg.ColorScheme = scheme
		}
	case "gtk_theme":
		cfg.GTKTheme = strings.TrimSpace(value)
	case "icon_theme":
		cfg.IconTheme = strings.TrimSpace(value)
	case "cursor_theme", "cursor_size":
		// Keep these accepted under [settings] too for existing configurations.
		parseSetting(cfg, key, value)
	case "font_name":
		cfg.FontName = strings.TrimSpace(value)
	case "qt_style":
		cfg.QTStyle = strings.TrimSpace(value)
	case "qt_platform_theme":
		cfg.QTPlatformTheme = strings.TrimSpace(value)
	case "window_button_layout":
		if layout := strings.TrimSpace(value); layout != "" {
			cfg.WindowButtonLayout = layout
		}
	}
}

func ensureWorkspaceKeyBindings(cfg *Config) {
	count := cfg.WorkspaceCount
	if count < 1 || count > 9 {
		count = 9
	}
	for number := 1; number <= count; number++ {
		key := strconv.Itoa(number)
		sym := xkb.SymFromName(key, xkb.KeySymFlagNoFlags)
		defaults := []KeyBinding{
			{Modifier: wlroots.KeyboardModifierLogo, Sym: sym, Action: "workspace", Arg: key},
			{Modifier: wlroots.KeyboardModifierLogo | wlroots.KeyboardModifierShift, Sym: sym, Action: "move_to_workspace", Arg: key},
		}
		for _, candidate := range defaults {
			found := false
			for _, binding := range cfg.KeyBindings {
				if binding.Modifier == candidate.Modifier && binding.Sym == candidate.Sym {
					found = true
					break
				}
			}
			if !found {
				cfg.KeyBindings = append(cfg.KeyBindings, candidate)
			}
		}
	}
}

func ensureMediaKeyBindings(cfg *Config) {
	defaults := []struct {
		key    string
		action string
	}{
		{"XF86AudioRaiseVolume", "volume_up"},
		{"XF86AudioLowerVolume", "volume_down"},
		{"XF86AudioMute", "volume_mute"},
	}

	for _, item := range defaults {
		sym := xkb.SymFromName(item.key, xkb.KeySymFlagNoFlags)
		found := false
		for _, binding := range cfg.KeyBindings {
			if binding.Modifier == 0 && binding.Sym == sym {
				found = true
				break
			}
		}
		if !found {
			cfg.KeyBindings = append(cfg.KeyBindings, KeyBinding{Sym: sym, Action: item.action})
		}
	}
}

func parseSetting(cfg *Config, key, value string) {
	switch strings.ToLower(key) {
	case "gaps":
		if n, err := strconv.Atoi(value); err == nil && n >= 0 && n <= 200 {
			cfg.Gaps = n
		}
	case "layout":
		cfg.Tiling = strings.ToLower(value) != "floating"
	case "wallpaper":
		cfg.Wallpaper = expandHome(value)
	case "window_opacity":
		if opacity, err := strconv.ParseFloat(value, 64); err == nil &&
			opacity >= 0 && opacity <= 1 {
			cfg.WindowOpacity = opacity
		}
	case "border_size":
		if n, err := strconv.Atoi(value); err == nil && n >= 0 && n <= 32 {
			cfg.BorderSize = n
		}
	case "border_rounding":
		if n, err := strconv.Atoi(value); err == nil && n >= 0 && n <= 128 {
			cfg.BorderRounding = n
		}
	case "focus_follows_mouse":
		if enabled, err := strconv.ParseBool(value); err == nil {
			cfg.FocusFollowsMouse = enabled
		}
	case "cursor_theme":
		if theme := strings.TrimSpace(value); theme != "" {
			cfg.CursorTheme = theme
		}
	case "cursor_size":
		if n, err := strconv.Atoi(value); err == nil && n >= 8 && n <= 256 {
			cfg.CursorSize = n
		}
	case "keyboard_layouts":
		layouts := parseCSV(value)
		if len(layouts) > 0 {
			cfg.KeyboardLayouts = layouts
		}
	case "keyboard_variants":
		cfg.KeyboardVariants = strings.TrimSpace(value)
	case "keyboard_options":
		cfg.KeyboardOptions = strings.TrimSpace(value)
	case "move_step":
		if n, err := strconv.Atoi(value); err == nil && n >= 1 && n <= 500 {
			cfg.MoveStep = n
		}
	case "workspaces":
		if n, err := strconv.Atoi(value); err == nil && n >= 1 && n <= 9 {
			cfg.WorkspaceCount = n
		}
	case "volume_step":
		if n, err := strconv.Atoi(value); err == nil && n >= 1 && n <= 100 {
			cfg.VolumeStep = n
		}
	case "notifications":
		if enabled, err := strconv.ParseBool(value); err == nil {
			cfg.Notifications = enabled
		}
	case "notification_daemon":
		if daemon := strings.TrimSpace(value); daemon != "" {
			cfg.NotificationDaemon = daemon
		}
	case "animations":
		if enabled, err := strconv.ParseBool(value); err == nil {
			cfg.Animations = enabled
		}
	case "animation_duration_ms":
		if n, err := strconv.Atoi(value); err == nil && n >= 0 && n <= 2000 {
			cfg.AnimationDurationMS = n
		}
	case "animation_easing":
		switch strings.ToLower(value) {
		case "linear", "ease_out_quad", "ease_out_cubic", "ease_in_out_cubic":
			cfg.AnimationEasing = strings.ToLower(value)
		}
	case "animation_open_offset":
		if n, err := strconv.Atoi(value); err == nil && n >= 0 && n <= 300 {
			cfg.AnimationOpenOffset = n
		}
	case "active_border_color":
		if c, err := parseColor(value); err == nil {
			cfg.ActiveBorderColor = c
		}
	case "inactive_border_color":
		if c, err := parseColor(value); err == nil {
			cfg.InactiveBorderColor = c
		}
	}
}

func parseKeyBinding(combo, command string) (KeyBinding, error) {
	mod, sym, err := parseKeyCombo(combo)
	if err != nil {
		return KeyBinding{}, err
	}
	fields := strings.SplitN(strings.TrimSpace(command), " ", 2)
	if len(fields) == 0 || fields[0] == "" {
		return KeyBinding{}, fmt.Errorf("empty action")
	}
	binding := KeyBinding{Modifier: mod, Sym: sym, Action: fields[0]}
	if len(fields) == 2 {
		binding.Arg = strings.TrimSpace(fields[1])
	}
	return binding, nil
}

func parseKeyCombo(combo string) (wlroots.KeyboardModifier, xkb.KeySym, error) {
	parts := strings.Split(combo, "+")
	if len(parts) == 0 {
		return 0, 0, fmt.Errorf("empty key combination")
	}
	var mod wlroots.KeyboardModifier
	for _, raw := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "mod4", "super", "logo":
			mod |= wlroots.KeyboardModifierLogo
		case "mod1", "alt":
			mod |= wlroots.KeyboardModifierAlt
		case "shift":
			mod |= wlroots.KeyboardModifierShift
		case "ctrl", "control":
			mod |= wlroots.KeyboardModifierCtrl
		default:
			return 0, 0, fmt.Errorf("unknown modifier %q", raw)
		}
	}
	keyName := strings.TrimSpace(parts[len(parts)-1])
	sym := xkb.SymFromName(keyName, xkb.KeySymFlagNoFlags)
	if sym == xkb.KeySymNoSymbol {
		return 0, 0, fmt.Errorf("unknown keysym %q", keyName)
	}
	return mod, sym, nil
}

func parseColor(value string) ([4]float32, error) {
	s := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(value), "#"), "0x")
	var rgba uint64
	var err error
	switch len(s) {
	case 6:
		rgba, err = strconv.ParseUint(s, 16, 32)
		rgba = (rgba << 8) | 0xff
	case 8:
		rgba, err = strconv.ParseUint(s, 16, 32)
	default:
		return [4]float32{}, fmt.Errorf("expected RRGGBB or RRGGBBAA")
	}
	if err != nil {
		return [4]float32{}, err
	}
	return [4]float32{
		float32((rgba>>24)&0xff) / 255,
		float32((rgba>>16)&0xff) / 255,
		float32((rgba>>8)&0xff) / 255,
		float32(rgba&0xff) / 255,
	}, nil
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func parseCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
