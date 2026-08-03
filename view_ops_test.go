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
