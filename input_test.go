package main

import (
	"testing"

	"github.com/swaywm/go-wlroots/wlroots"
)

func TestResizeFloatingGeometryUsesPointerDeltaWithoutInitialJump(t *testing.T) {
	original := Geometry{X: 300, Y: 200, Width: 800, Height: 600}
	got := resizeFloatingGeometry(original, 0, 0,
		wlroots.EdgeRight|wlroots.EdgeBottom,
		usableBox{x: 0, y: 30, width: 1920, height: 1050}, 2,
		100, 100, 0, 0)
	if got != original {
		t.Fatalf("zero-delta resize changed geometry: got %+v, want %+v", got, original)
	}
}

func TestResizeFloatingGeometryKeepsOppositeCornerFixed(t *testing.T) {
	original := Geometry{X: 300, Y: 200, Width: 800, Height: 600}
	got := resizeFloatingGeometry(original, 120, 80,
		wlroots.EdgeLeft|wlroots.EdgeTop,
		usableBox{x: 0, y: 30, width: 1920, height: 1050}, 2,
		100, 100, 0, 0)
	if got.X != 420 || got.Y != 280 || got.Width != 680 || got.Height != 520 {
		t.Fatalf("unexpected geometry: %+v", got)
	}
	if got.X+float64(got.Width) != original.X+float64(original.Width) ||
		got.Y+float64(got.Height) != original.Y+float64(original.Height) {
		t.Fatalf("opposite corner moved: got %+v, original %+v", got, original)
	}
}

func TestResizeFloatingGeometryHonorsClientAndOutputLimits(t *testing.T) {
	original := Geometry{X: 300, Y: 200, Width: 800, Height: 600}
	area := usableBox{x: 0, y: 30, width: 1200, height: 800}
	got := resizeFloatingGeometry(original, 1000, 1000,
		wlroots.EdgeRight|wlroots.EdgeBottom, area, 2,
		400, 300, 850, 650)
	if got.Width != 850 || got.Height != 626 {
		t.Fatalf("maximum/output limits not honored: %+v", got)
	}

	got = resizeFloatingGeometry(original, -1000, -1000,
		wlroots.EdgeRight|wlroots.EdgeBottom, area, 2,
		400, 300, 850, 650)
	if got.Width != 400 || got.Height != 300 {
		t.Fatalf("minimum limits not honored: %+v", got)
	}
}

func TestResizeCursorNameMatchesEdges(t *testing.T) {
	tests := []struct {
		edges wlroots.Edges
		want  string
	}{
		{wlroots.EdgeLeft | wlroots.EdgeTop, "nwse-resize"},
		{wlroots.EdgeRight | wlroots.EdgeBottom, "nwse-resize"},
		{wlroots.EdgeRight | wlroots.EdgeTop, "nesw-resize"},
		{wlroots.EdgeLeft | wlroots.EdgeBottom, "nesw-resize"},
		{wlroots.EdgeLeft, "ew-resize"},
		{wlroots.EdgeBottom, "ns-resize"},
	}
	for _, tt := range tests {
		if got := resizeCursorName(tt.edges); got != tt.want {
			t.Errorf("resizeCursorName(%v) = %q, want %q", tt.edges, got, tt.want)
		}
	}
}
