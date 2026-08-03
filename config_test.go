package main

import "testing"

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
