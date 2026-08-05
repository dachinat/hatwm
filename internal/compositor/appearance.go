package compositor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"time"
)

func portalColorScheme(scheme string) uint32 {
	switch scheme {
	case "dark":
		return 1
	case "light":
		return 2
	default:
		return 0
	}
}

func (s *Server) applyAppearanceEnvironment() {
	// GTK_THEME is a debugging override, not a desktop preference. Exporting it
	// globally can make GTK4/libadwaita applications load an incompatible GTK3
	// theme. GTK follows the GSettings values synchronized below instead.
	_ = os.Unsetenv("GTK_THEME")
	setOptionalEnvironment("QT_STYLE_OVERRIDE", s.config.QTStyle)
	setOptionalEnvironment("QT_QPA_PLATFORMTHEME", s.config.QTPlatformTheme)
	_ = os.Setenv("XCURSOR_THEME", s.config.CursorTheme)
	_ = os.Setenv("XCURSOR_SIZE", strconv.Itoa(s.config.CursorSize))
}

func setOptionalEnvironment(name, value string) {
	if value == "" {
		return
	}
	_ = os.Setenv(name, value)
}

func (s *Server) applyAppearanceProfile() {
	s.applyAppearanceEnvironment()
	config := s.config
	go synchronizeGSettings(config)
	s.updatePortalAppearance()
}

func synchronizeGSettings(config Config) {
	if _, err := exec.LookPath("gsettings"); err == nil {
		type gsetting struct {
			schema string
			key    string
			value  string
			number bool
		}
		settings := []gsetting{
			{"org.gnome.desktop.interface", "color-scheme", gsettingsColorScheme(config.ColorScheme), false},
			{"org.gnome.desktop.interface", "cursor-theme", config.CursorTheme, false},
			{"org.gnome.desktop.interface", "cursor-size", strconv.Itoa(config.CursorSize), true},
			{"org.gnome.desktop.interface", "gtk-theme", config.GTKTheme, false},
			{"org.gnome.desktop.interface", "icon-theme", config.IconTheme, false},
			{"org.gnome.desktop.interface", "font-name", config.FontName, false},
			{"org.gnome.desktop.wm.preferences", "button-layout", config.WindowButtonLayout, false},
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		for _, setting := range settings {
			if setting.value == "" {
				continue
			}
			value := setting.value
			if !setting.number {
				value = strconv.Quote(value)
			}
			if output, err := exec.CommandContext(ctx, "gsettings", "set",
				setting.schema, setting.key, value).CombinedOutput(); err != nil {
				slog.Warn("could not synchronize appearance setting", "key", setting.key,
					"error", err, "output", string(output))
				if ctx.Err() != nil {
					return
				}
			}
		}
	} else {
		slog.Debug("gsettings unavailable; GTK appearance synchronization skipped")
	}
}

func gsettingsColorScheme(scheme string) string {
	switch scheme {
	case "dark":
		return "prefer-dark"
	case "light":
		return "prefer-light"
	case "default":
		return "default"
	default:
		return ""
	}
}

func appearanceChanged(a, b Config) bool {
	return a.ColorScheme != b.ColorScheme || a.GTKTheme != b.GTKTheme ||
		a.IconTheme != b.IconTheme || a.CursorTheme != b.CursorTheme ||
		a.CursorSize != b.CursorSize || a.FontName != b.FontName ||
		a.QTStyle != b.QTStyle || a.QTPlatformTheme != b.QTPlatformTheme ||
		a.WindowButtonLayout != b.WindowButtonLayout ||
		a.Animations != b.Animations
}

func (s *Server) appearanceDescription() string {
	return fmt.Sprintf("scheme=%s gtk=%q icons=%q cursor=%q/%d qt=%q",
		s.config.ColorScheme, s.config.GTKTheme, s.config.IconTheme,
		s.config.CursorTheme, s.config.CursorSize, s.config.QTStyle)
}
