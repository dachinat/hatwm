package compositor

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"time"
)

var sessionEnvironmentNames = []string{
	"WAYLAND_DISPLAY",
	"DISPLAY",
	"XDG_CURRENT_DESKTOP",
	"XDG_SESSION_DESKTOP",
	"XDG_SESSION_TYPE",
}

type sessionEnvironmentCommand struct {
	name string
	args []string
}

// sessionEnvironmentCommands builds independent updates for the systemd user
// manager and the D-Bus activation environment. Explicitly clearing missing
// values is important when entering HatWM after another compositor: importing
// only present variables would leave the previous session's DISPLAY behind.
func sessionEnvironmentCommands(lookup func(string) (string, bool)) []sessionEnvironmentCommand {
	setNames := make([]string, 0, len(sessionEnvironmentNames))
	unsetNames := make([]string, 0, len(sessionEnvironmentNames))
	assignments := make([]string, 0, len(sessionEnvironmentNames))
	for _, name := range sessionEnvironmentNames {
		value, present := lookup(name)
		if present {
			setNames = append(setNames, name)
		} else {
			unsetNames = append(unsetNames, name)
			value = ""
		}
		// D-Bus' update tool ignores absent names, so use an explicit empty
		// assignment to replace a value inherited from an earlier session.
		assignments = append(assignments, name+"="+value)
	}

	commands := make([]sessionEnvironmentCommand, 0, 3)
	if len(setNames) > 0 {
		commands = append(commands, sessionEnvironmentCommand{
			name: "systemctl",
			args: append([]string{"--user", "import-environment"}, setNames...),
		})
	}
	if len(unsetNames) > 0 {
		commands = append(commands, sessionEnvironmentCommand{
			name: "systemctl",
			args: append([]string{"--user", "unset-environment"}, unsetNames...),
		})
	}
	commands = append(commands, sessionEnvironmentCommand{
		name: "dbus-update-activation-environment",
		args: assignments,
	})
	return commands
}

func synchronizeSessionEnvironment() {
	for _, command := range sessionEnvironmentCommands(os.LookupEnv) {
		if _, err := exec.LookPath(command.name); err != nil {
			slog.Debug("session environment helper unavailable; skipping",
				"helper", command.name)
			continue
		}
		// A stale or unhealthy user bus must not prevent HatWM from starting,
		// and failure of one manager must not prevent updating the other.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		output, err := exec.CommandContext(ctx, command.name, command.args...).CombinedOutput()
		cancel()
		if err != nil {
			slog.Warn("could not synchronize session environment",
				"helper", command.name, "error", err, "output", string(output))
		}
	}
}
