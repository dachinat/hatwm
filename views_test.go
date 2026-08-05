package main

import "testing"

func TestMappedViewsFiltersManagementMappingAndWorkspace(t *testing.T) {
	visible := &View{ID: 1, Managed: true, Mapped: true, Workspace: 2}
	server := &Server{
		currentWorkspace: 2,
		views: []*View{
			visible,
			{ID: 2, Managed: false, Mapped: true, Workspace: 2},
			{ID: 3, Managed: true, Mapped: false, Workspace: 2},
			{ID: 4, Managed: true, Mapped: true, Workspace: 1},
		},
	}
	got := server.mappedViews()
	if len(got) != 1 || got[0] != visible {
		t.Fatalf("mapped views = %v, want only visible view", got)
	}
}

func TestMappedTiledViewsExcludesAutoFloatingViews(t *testing.T) {
	tiled := &View{ID: 1, Managed: true, Mapped: true, Workspace: 1}
	server := &Server{currentWorkspace: 1, views: []*View{
		tiled,
		{ID: 2, Managed: true, Mapped: true, Workspace: 1, AutoFloating: true},
	}}
	got := server.mappedTiledViews()
	if len(got) != 1 || got[0] != tiled {
		t.Fatalf("mapped tiled views = %v, want only tiled view", got)
	}
}

func TestIsFloatingViewReflectsLayoutAndViewPolicy(t *testing.T) {
	server := &Server{config: Config{Tiling: true}}
	regular := &View{}
	dialog := &View{AutoFloating: true}
	if server.isFloatingView(nil) || server.isFloatingView(regular) || !server.isFloatingView(dialog) {
		t.Fatal("tiling-mode floating classification is incorrect")
	}
	server.config.Tiling = false
	if !server.isFloatingView(regular) {
		t.Fatal("regular view is not floating in floating layout")
	}
}

func TestRemoveViewOnlyRemovesTarget(t *testing.T) {
	a, b, c := &View{ID: 1}, &View{ID: 2}, &View{ID: 3}
	server := &Server{views: []*View{a, b, c}}
	server.removeView(b)
	if len(server.views) != 2 || server.views[0] != a || server.views[1] != c {
		t.Fatalf("views after removal = %v", server.views)
	}
	server.removeView(&View{ID: 99})
	if len(server.views) != 2 {
		t.Fatal("removing an unknown view changed the list")
	}
}

func TestMoveViewFrontPreservesRelativeOrder(t *testing.T) {
	a, b, c, d := &View{ID: 1}, &View{ID: 2}, &View{ID: 3}, &View{ID: 4}
	server := &Server{views: []*View{a, b, c, d}}
	server.moveViewFront(c)
	want := []*View{c, a, b, d}
	for i := range want {
		if server.views[i] != want[i] {
			t.Fatalf("views after move = %v", server.views)
		}
	}
	server.moveViewFront(c)
	server.moveViewFront(&View{ID: 99})
	for i := range want {
		if server.views[i] != want[i] {
			t.Fatal("no-op front move changed the list")
		}
	}
}

func TestSwapViewOrder(t *testing.T) {
	a, b, c := &View{ID: 1}, &View{ID: 2}, &View{ID: 3}
	server := &Server{views: []*View{a, b, c}}
	server.swapViewOrder(a, c)
	if server.views[0] != c || server.views[1] != b || server.views[2] != a {
		t.Fatalf("views after swap = %v", server.views)
	}
	server.swapViewOrder(a, &View{ID: 99})
	if server.views[0] != c || server.views[1] != b || server.views[2] != a {
		t.Fatal("swap with unknown view changed the list")
	}
}

func TestAbsFloat(t *testing.T) {
	for input, want := range map[float64]float64{-3.5: 3.5, 0: 0, 8.25: 8.25} {
		if got := absFloat(input); got != want {
			t.Errorf("absFloat(%v) = %v, want %v", input, got, want)
		}
	}
}
