package compositor

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func (s *Server) startWallpaper() {
	if strings.TrimSpace(s.config.Wallpaper) == "" {
		s.stopWallpaper()
		return
	}
	if err := s.setWallpaper(s.config.Wallpaper); err != nil {
		slog.Warn("failed to start hatwmbg",
			"wallpaper", s.config.Wallpaper, "error", err)
	}
}

func (s *Server) setWallpaper(path string) error {
	path, err := prepareWallpaper(path)
	if err != nil {
		return err
	}
	binary, err := exec.LookPath("hatwmbg")
	if err != nil {
		return fmt.Errorf("find hatwmbg in PATH: %w", err)
	}
	cmd := exec.Command(binary, "--mode", "fill", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start hatwmbg: %w", err)
	}

	previous := s.wallpaperCmd
	s.wallpaperCmd = cmd
	s.wallpaperPath = path
	go waitForWallpaper(cmd, path)

	// hatwmbg handles SIGTERM and removes its layer surfaces cleanly. Start the
	// replacement first to minimize any gap while changing the wallpaper.
	stopWallpaperProcess(previous)
	slog.Info("wallpaper started", "path", path, "command", binary)
	return nil
}

func prepareWallpaper(path string) (string, error) {
	path = expandHome(strings.TrimSpace(path))
	if path == "" {
		return "", fmt.Errorf("wallpaper path is required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve wallpaper path: %w", err)
	}
	file, err := os.Open(absolute)
	if err != nil {
		return "", fmt.Errorf("open wallpaper: %w", err)
	}
	defer file.Close()
	if _, format, err := image.Decode(file); err != nil {
		return "", fmt.Errorf("decode wallpaper: %w", err)
	} else if format != "jpeg" && format != "png" && format != "gif" {
		return "", fmt.Errorf("unsupported wallpaper format %q", format)
	}
	return absolute, nil
}

func waitForWallpaper(cmd *exec.Cmd, path string) {
	if err := cmd.Wait(); err != nil {
		slog.Warn("hatwmbg exited", "wallpaper", path, "error", err)
	}
}

func stopWallpaperProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		slog.Warn("failed to stop hatwmbg cleanly", "error", err)
	}
}

func (s *Server) stopWallpaper() {
	cmd := s.wallpaperCmd
	s.wallpaperCmd = nil
	s.wallpaperPath = ""
	stopWallpaperProcess(cmd)
}
