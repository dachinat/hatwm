package compositor

import "testing"

func TestTransientDescendant(t *testing.T) {
	root := &View{ID: 1}
	child := &View{ID: 2}
	grandchild := &View{ID: 3}
	unrelated := &View{ID: 4}
	parents := map[*View]*View{child: root, grandchild: child}
	parentOf := func(v *View) *View { return parents[v] }

	if !transientDescendant(child, root, parentOf) {
		t.Fatal("direct transient was not recognized")
	}
	if !transientDescendant(grandchild, root, parentOf) {
		t.Fatal("nested transient was not recognized")
	}
	if transientDescendant(unrelated, root, parentOf) {
		t.Fatal("unrelated window was recognized as a transient")
	}
	if transientDescendant(root, root, parentOf) {
		t.Fatal("a window must not be its own transient descendant")
	}
}

func TestTransientDescendantHandlesParentCycle(t *testing.T) {
	a := &View{ID: 1}
	b := &View{ID: 2}
	parents := map[*View]*View{a: b, b: a}
	if transientDescendant(a, &View{ID: 3}, func(v *View) *View { return parents[v] }) {
		t.Fatal("cyclic parent chain matched an unrelated ancestor")
	}
}

func TestMappedViewsFiltersManagementMappingAndWorkspace(t *testing.T) {
	visible := &View{ID: 1, Managed: true, Mapped: true, Workspace: 2}
	server := &Server{views: []*View{
		visible,
		{ID: 2, Managed: false, Mapped: true, Workspace: 2},
		{ID: 3, Managed: true, Mapped: false, Workspace: 2},
		{ID: 4, Managed: true, Mapped: true, Workspace: 1},
	},
	}
	server.fallbackOutput.CurrentWorkspace = 2
	got := server.mappedViews()
	if len(got) != 1 || got[0] != visible {
		t.Fatalf("mapped views = %v, want only visible view", got)
	}
}

func TestMappedTiledViewsExcludesAutoFloatingViews(t *testing.T) {
	tiled := &View{ID: 1, Managed: true, Mapped: true, Workspace: 1}
	server := &Server{views: []*View{
		tiled,
		{ID: 2, Managed: true, Mapped: true, Workspace: 1, AutoFloating: true},
	}}
	server.fallbackOutput.CurrentWorkspace = 1
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

func TestDesiredViewSceneLayer(t *testing.T) {
	server := &Server{config: Config{Tiling: true}}
	tests := []struct {
		name string
		view *View
		want viewSceneLayer
	}{
		{name: "tiled window", view: &View{Managed: true}, want: viewSceneLayerTiled},
		{name: "floating window", view: &View{Managed: true, AutoFloating: true}, want: viewSceneLayerFloating},
		{name: "unmanaged window", view: &View{}, want: viewSceneLayerFloating},
		{name: "keep above rule", view: &View{Managed: true, RuleActions: WindowRuleActions{HasKeepAbove: true, KeepAbove: true}}, want: viewSceneLayerFloating},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := server.desiredViewSceneLayer(tc.view); got != tc.want {
				t.Fatalf("desired scene layer = %v, want %v", got, tc.want)
			}
		})
	}

	server.config.Tiling = false
	if got := server.desiredViewSceneLayer(&View{Managed: true}); got != viewSceneLayerFloating {
		t.Fatalf("floating-layout scene layer = %v, want floating", got)
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
