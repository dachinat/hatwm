package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime"

	"github.com/swaywm/go-wlroots/wlroots"

	"hatwm/internal/compositor"
)

var startupCmd = flag.String("s", "", "command to run after HatWM starts")

func init() {
	// wlroots renderers and graphics drivers rely on thread-local state.
	runtime.LockOSThread()
}

func main() {
	flag.Parse()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	wlroots.OnLog(wlroots.LogImportanceInfo, nil)

	server, err := compositor.NewServer()
	if err != nil {
		fatal("initializing compositor", err)
	}
	if err := server.Start(*startupCmd); err != nil {
		fatal("starting compositor", err)
	}
	if err := server.Run(); err != nil {
		fatal("running compositor", err)
	}
}

func fatal(context string, err error) {
	fmt.Fprintf(os.Stderr, "hatwm: %s: %v\n", context, err)
	os.Exit(1)
}
