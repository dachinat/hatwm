package compositor

import "testing"

func TestOutputStateAtUsesLayoutCoordinates(t *testing.T) {
	left := &OutputState{Full: usableBox{x: -1920, y: 0, width: 1920, height: 1080}}
	right := &OutputState{Full: usableBox{x: 0, y: 0, width: 2560, height: 1440}}
	server := &Server{outputs: []*OutputState{left, right}}
	if got := server.outputStateAt(-100, 500); got != left {
		t.Fatalf("negative layout coordinate selected %p, want left output", got)
	}
	if got := server.outputStateAt(2000, 500); got != right {
		t.Fatalf("right coordinate selected %p, want right output", got)
	}
	if got := server.outputStateAt(3000, 500); got != nil {
		t.Fatalf("off-layout coordinate selected %p", got)
	}
}

func TestMappedViewsAreScopedToOutputWorkspace(t *testing.T) {
	left := &OutputState{CurrentWorkspace: 1}
	right := &OutputState{CurrentWorkspace: 2}
	leftVisible := &View{Managed: true, Mapped: true, Output: left, Workspace: 1}
	rightVisible := &View{Managed: true, Mapped: true, Output: right, Workspace: 2}
	rightHidden := &View{Managed: true, Mapped: true, Output: right, Workspace: 1}
	server := &Server{outputs: []*OutputState{left, right}, activeOutput: right,
		views: []*View{leftVisible, rightVisible, rightHidden}}
	got := server.mappedViews()
	if len(got) != 1 || got[0] != rightVisible {
		t.Fatalf("active output views = %v, want only right visible view", got)
	}
	if !server.viewVisible(leftVisible) || server.viewVisible(rightHidden) {
		t.Fatal("per-output workspace visibility is incorrect")
	}
}

func TestOutputFocusHistoryDropsStaleEntries(t *testing.T) {
	output := &OutputState{CurrentWorkspace: 1}
	old := &View{Mapped: false, Output: output, Workspace: 1}
	current := &View{Mapped: true, Output: output, Workspace: 1}
	server := &Server{}
	output.FocusHistory = []*View{old}
	server.rememberOutputFocus(output, current)
	if len(output.FocusHistory) != 1 || output.FocusHistory[0] != current {
		t.Fatalf("focus history = %v, want only current view", output.FocusHistory)
	}
}
