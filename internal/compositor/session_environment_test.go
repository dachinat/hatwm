package compositor

import (
	"reflect"
	"testing"
)

func TestSessionEnvironmentCommandsReplacePreviousDesktop(t *testing.T) {
	environment := map[string]string{
		"WAYLAND_DISPLAY":     "wayland-2",
		"DISPLAY":             ":1",
		"XDG_CURRENT_DESKTOP": "HatWM",
		"XDG_SESSION_DESKTOP": "HatWM",
		"XDG_SESSION_TYPE":    "wayland",
	}
	commands := sessionEnvironmentCommands(func(name string) (string, bool) {
		value, ok := environment[name]
		return value, ok
	})
	want := []sessionEnvironmentCommand{
		{name: "systemctl", args: []string{
			"--user", "import-environment", "WAYLAND_DISPLAY", "DISPLAY",
			"XDG_CURRENT_DESKTOP", "XDG_SESSION_DESKTOP", "XDG_SESSION_TYPE",
		}},
		{name: "dbus-update-activation-environment", args: []string{
			"WAYLAND_DISPLAY=wayland-2", "DISPLAY=:1",
			"XDG_CURRENT_DESKTOP=HatWM", "XDG_SESSION_DESKTOP=HatWM",
			"XDG_SESSION_TYPE=wayland",
		}},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("session environment commands = %#v, want %#v", commands, want)
	}
}

func TestSessionEnvironmentCommandsClearMissingDisplay(t *testing.T) {
	environment := map[string]string{
		"WAYLAND_DISPLAY":     "wayland-0",
		"XDG_CURRENT_DESKTOP": "HatWM",
		"XDG_SESSION_DESKTOP": "HatWM",
		"XDG_SESSION_TYPE":    "wayland",
	}
	commands := sessionEnvironmentCommands(func(name string) (string, bool) {
		value, ok := environment[name]
		return value, ok
	})
	if len(commands) != 3 {
		t.Fatalf("got %d commands, want 3: %#v", len(commands), commands)
	}
	if got, want := commands[1], (sessionEnvironmentCommand{
		name: "systemctl", args: []string{"--user", "unset-environment", "DISPLAY"},
	}); !reflect.DeepEqual(got, want) {
		t.Fatalf("unset command = %#v, want %#v", got, want)
	}
	if got := commands[2].args[1]; got != "DISPLAY=" {
		t.Fatalf("D-Bus DISPLAY assignment = %q, want empty assignment", got)
	}
}
