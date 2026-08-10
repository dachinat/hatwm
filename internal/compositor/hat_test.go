package compositor

import "testing"

func TestHatMaintainsMRUOrderWithoutChangingViewOrder(t *testing.T) {
	a, b, c := &View{ID: 1}, &View{ID: 2}, &View{ID: 3}
	server := &Server{views: []*View{a, b, c}}
	server.addViewToHat(a)
	server.addViewToHat(c)
	server.addViewToHat(a)
	if len(server.hat) != 2 || server.hat[0] != a || server.hat[1] != c {
		t.Fatalf("unexpected Hat order: %v", server.hat)
	}
	if server.views[0] != a || server.views[1] != b || server.views[2] != c {
		t.Fatalf("Hat operations changed layout order: %v", server.views)
	}
}

func TestHatCycleAndRemoval(t *testing.T) {
	a, b, c := &View{ID: 1}, &View{ID: 2}, &View{ID: 3}
	server := &Server{}
	server.addViewToHat(c)
	server.addViewToHat(b)
	server.addViewToHat(a)
	if !server.cycleHat() || server.hat[0] != b || server.hat[2] != a {
		t.Fatalf("Hat did not cycle: %v", server.hat)
	}
	if !server.removeViewFromHat(b) || b.InHat || len(server.hat) != 2 {
		t.Fatalf("Hat removal failed: %v", server.hat)
	}
	if server.removeViewFromHat(&View{ID: 99}) {
		t.Fatal("removing an unknown window unexpectedly succeeded")
	}
}

func TestHatWindowsAreNotVisibleOrCountedAsMapped(t *testing.T) {
	output := &OutputState{CurrentWorkspace: 1}
	visible := &View{ID: 1, Managed: true, Mapped: true, Workspace: 1, Output: output}
	stashed := &View{ID: 2, Managed: true, Mapped: true, InHat: true, Workspace: 1, Output: output}
	server := &Server{views: []*View{visible, stashed}, activeOutput: output}
	if server.viewVisible(stashed) {
		t.Fatal("stashed window is visible")
	}
	views := server.mappedViewsForOutput(output)
	if len(views) != 1 || views[0] != visible {
		t.Fatalf("mapped views include The Hat: %v", views)
	}
}

func TestRestoreHatWindowArgumentSelectsByID(t *testing.T) {
	server := &Server{}
	if server.restoreHatWindowArg("") || server.restoreHatWindowArg("bad") ||
		server.restoreHatWindowByID(42) {
		t.Fatal("empty Hat accepted a restore request")
	}
}
