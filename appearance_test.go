package main

import (
	"os"
	"strings"
	"testing"
)

func TestPortalColorScheme(t *testing.T) {
	for _, tt := range []struct {
		name string
		want uint32
	}{
		{name: "default", want: 0},
		{name: "dark", want: 1},
		{name: "light", want: 2},
		{name: "unknown", want: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := portalColorScheme(tt.name); got != tt.want {
				t.Fatalf("portalColorScheme(%q) = %d, want %d", tt.name, got, tt.want)
			}
		})
	}
}

func TestGSettingsColorScheme(t *testing.T) {
	for _, tt := range []struct {
		input, want string
	}{
		{input: "default", want: "default"},
		{input: "dark", want: "prefer-dark"},
		{input: "light", want: "prefer-light"},
		{input: "unknown", want: ""},
	} {
		if got := gsettingsColorScheme(tt.input); got != tt.want {
			t.Errorf("gsettingsColorScheme(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestAppearanceChangedOnlyTracksAppearance(t *testing.T) {
	base := defaultConfig()
	same := base
	same.Gaps++
	same.WorkspaceCount--
	if appearanceChanged(base, same) {
		t.Fatal("non-appearance settings reported an appearance change")
	}

	changes := []func(*Config){
		func(c *Config) { c.ColorScheme = "dark" },
		func(c *Config) { c.GTKTheme = "Adwaita-dark" },
		func(c *Config) { c.IconTheme = "Papirus-Dark" },
		func(c *Config) { c.CursorSize++ },
		func(c *Config) { c.FontName = "Sans 11" },
		func(c *Config) { c.Animations = !c.Animations },
	}
	for i, change := range changes {
		candidate := base
		change(&candidate)
		if !appearanceChanged(base, candidate) {
			t.Errorf("appearance mutation %d was not detected", i)
		}
	}
}

func TestApplyAppearanceEnvironment(t *testing.T) {
	t.Setenv("GTK_THEME", "must-be-removed")
	t.Setenv("QT_STYLE_OVERRIDE", "old-style")
	t.Setenv("QT_QPA_PLATFORMTHEME", "old-platform-theme")
	server := &Server{config: Config{
		QTStyle:         "Fusion",
		QTPlatformTheme: "qt6ct",
		CursorTheme:     "Bibata-Modern-Ice",
		CursorSize:      32,
	}}
	server.applyAppearanceEnvironment()

	if _, exists := os.LookupEnv("GTK_THEME"); exists {
		t.Fatal("GTK_THEME was not removed")
	}
	for name, want := range map[string]string{
		"QT_STYLE_OVERRIDE":    "Fusion",
		"QT_QPA_PLATFORMTHEME": "qt6ct",
		"XCURSOR_THEME":        "Bibata-Modern-Ice",
		"XCURSOR_SIZE":         "32",
	} {
		if got := os.Getenv(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestSetOptionalEnvironmentLeavesExistingValueWhenEmpty(t *testing.T) {
	const name = "HATWM_TEST_OPTIONAL_ENVIRONMENT"
	t.Setenv(name, "existing")
	setOptionalEnvironment(name, "")
	if got := os.Getenv(name); got != "existing" {
		t.Fatalf("empty optional environment changed value to %q", got)
	}
	setOptionalEnvironment(name, "replacement")
	if got := os.Getenv(name); got != "replacement" {
		t.Fatalf("optional environment = %q, want replacement", got)
	}
}

func TestAppearanceDescriptionIncludesSelectedProfile(t *testing.T) {
	server := &Server{config: Config{
		ColorScheme: "dark", GTKTheme: "Adwaita-dark", IconTheme: "Papirus-Dark",
		CursorTheme: "Bibata", CursorSize: 24, QTStyle: "Fusion",
	}}
	got := server.appearanceDescription()
	for _, value := range []string{"scheme=dark", `gtk="Adwaita-dark"`, `icons="Papirus-Dark"`, `cursor="Bibata"/24`, `qt="Fusion"`} {
		if !strings.Contains(got, value) {
			t.Errorf("appearance description %q does not contain %q", got, value)
		}
	}
}
