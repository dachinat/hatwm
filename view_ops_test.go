package main

import "testing"

func TestInvalidateXWaylandRoundedClipForcesReapply(t *testing.T) {
	view := &View{
		IsXWayland:  true,
		XClipRadius: 8,
		XClipWidth:  1280,
		XClipHeight: 720,
	}

	view.invalidateXWaylandRoundedClip()

	if view.XClipRadius != -1 ||
		view.XClipWidth != -1 || view.XClipHeight != -1 {
		t.Fatalf(
			"clip cache was not invalidated: radius=%d width=%d height=%d",
			view.XClipRadius, view.XClipWidth, view.XClipHeight)
	}
}

func TestViewTargetRootPositionUsesAnimationDestination(t *testing.T) {
	view := &View{Animation: ViewAnimation{Initialized: true, ToX: 123.5, ToY: -45.25}}
	x, y := view.targetRootPosition()
	if x != 123.5 || y != -45.25 {
		t.Fatalf("target position = (%v, %v)", x, y)
	}
}

func TestViewSurfaceOffset(t *testing.T) {
	server := &Server{config: Config{BorderSize: 4}}
	view := &View{Managed: true, Server: server}
	if got := view.surfaceOffset(); got != 4 {
		t.Fatalf("managed surface offset = %d, want 4", got)
	}
	view.Managed = false
	if got := view.surfaceOffset(); got != 0 {
		t.Fatalf("unmanaged surface offset = %d, want 0", got)
	}
	view.Managed = true
	server.fullscreen = view
	if got := view.surfaceOffset(); got != 0 {
		t.Fatalf("fullscreen surface offset = %d, want 0", got)
	}
}

func TestNilViewHelpersAreSafe(t *testing.T) {
	var view *View
	if !view.clientSurface().Nil() {
		t.Fatal("nil view returned a client surface")
	}
	if geometry := view.geometry(); geometry.Width != 0 || geometry.Height != 0 {
		t.Fatalf("nil view geometry = %+v", geometry)
	}
	if minWidth, minHeight := view.minimumSize(); minWidth != 0 || minHeight != 0 {
		t.Fatalf("nil view minimum = %dx%d", minWidth, minHeight)
	}
	if minWidth, minHeight, maxWidth, maxHeight := view.sizeConstraints(); minWidth != 0 || minHeight != 0 || maxWidth != 0 || maxHeight != 0 {
		t.Fatalf("nil view constraints = %d,%d,%d,%d", minWidth, minHeight, maxWidth, maxHeight)
	}
	view.invalidateXWaylandRoundedClip()
}
