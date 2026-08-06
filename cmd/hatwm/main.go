package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"

	"github.com/swaywm/go-wlroots/wlroots"

	"hatwm/internal/compositor"
)

var (
	startupCmd  = flag.String("s", "", "command to run after HatWM starts")
	checkConfig = flag.Bool("check-config", false, "validate configuration and exit")
	showVersion = flag.Bool("version", false, "show HatWM and wlroots versions")
	debugLogs   = flag.Bool("debug", false, "enable debug logging")
)

func init() {
	// wlroots renderers and graphics drivers rely on thread-local state.
	runtime.LockOSThread()
}

func main() {
	defer reportPanic()
	flag.Parse()
	level := slog.LevelInfo
	if *debugLogs {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr,
		&slog.HandlerOptions{Level: level})))
	if *showVersion {
		fmt.Println(compositor.VersionString())
		return
	}
	if *checkConfig {
		if err := compositor.CheckConfig(); err != nil {
			fatal("configuration is invalid", err)
		}
		fmt.Println("hatwm: configuration is valid")
		return
	}
	wlroots.OnLog(wlroots.LogImportanceInfo, nil)

	server, err := compositor.NewServer()
	if err != nil {
		fatal("initializing compositor", err)
	}
	if err := server.Start(*startupCmd); err != nil {
		fatal("starting compositor", err)
	}
	installSignalHandlers(server)
	if err := server.Run(); err != nil {
		fatal("running compositor", err)
	}
}

func installSignalHandlers(server *compositor.Server) {
	signals := make(chan os.Signal, 4)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		for sig := range signals {
			if sig == syscall.SIGHUP {
				server.RequestReload()
				continue
			}
			server.RequestShutdown()
			return
		}
	}()
}

func reportPanic() {
	if recovered := recover(); recovered != nil {
		slog.Error("HatWM crashed",
			"panic", recovered,
			"stack", string(debug.Stack()))
		os.Exit(2)
	}
}

func fatal(context string, err error) {
	fmt.Fprintf(os.Stderr, "hatwm: %s: %v\n", context, err)
	os.Exit(1)
}
