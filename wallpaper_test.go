package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareWallpaperAcceptsSupportedImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wallpaper.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	source := image.NewRGBA(image.Rect(0, 0, 2, 2))
	source.Set(0, 0, color.RGBA{R: 0x33, G: 0x66, B: 0x99, A: 0xff})
	if err := png.Encode(file, source); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := prepareWallpaper(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("prepareWallpaper() = %q, want %q", got, path)
	}
}

func TestPrepareWallpaperRejectsInvalidImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-an-image.jpg")
	if err := os.WriteFile(path, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareWallpaper(path); err == nil {
		t.Fatal("expected invalid image to be rejected")
	}
}

func TestPrepareWallpaperExpandsHomeAndReturnsAbsolutePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, "wallpaper.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := prepareWallpaper("~/wallpaper.png")
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("prepareWallpaper() = %q, want %q", got, path)
	}
}

func TestPrepareWallpaperRejectsEmptyAndMissingPaths(t *testing.T) {
	if _, err := prepareWallpaper("   "); err == nil {
		t.Fatal("empty wallpaper path unexpectedly succeeded")
	}
	if _, err := prepareWallpaper(filepath.Join(t.TempDir(), "missing.png")); err == nil {
		t.Fatal("missing wallpaper path unexpectedly succeeded")
	}
}
