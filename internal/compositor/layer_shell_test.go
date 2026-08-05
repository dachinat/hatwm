package compositor

import "testing"

func TestExclusiveAnchorEdge(t *testing.T) {
	tests := []struct {
		name   string
		anchor uint32
		want   uint32
	}{
		{name: "top", anchor: anchorTop, want: anchorTop},
		{name: "top stretched horizontally", anchor: anchorTop | anchorLeft | anchorRight, want: anchorTop},
		{name: "bottom", anchor: anchorBottom, want: anchorBottom},
		{name: "bottom stretched horizontally", anchor: anchorBottom | anchorLeft | anchorRight, want: anchorBottom},
		{name: "left", anchor: anchorLeft, want: anchorLeft},
		{name: "left stretched vertically", anchor: anchorLeft | anchorTop | anchorBottom, want: anchorLeft},
		{name: "right", anchor: anchorRight, want: anchorRight},
		{name: "right stretched vertically", anchor: anchorRight | anchorTop | anchorBottom, want: anchorRight},
		{name: "unanchored", anchor: 0},
		{name: "corner", anchor: anchorTop | anchorRight},
		{name: "parallel edges", anchor: anchorTop | anchorBottom},
		{name: "all edges", anchor: anchorTop | anchorBottom | anchorLeft | anchorRight},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exclusiveAnchorEdge(tt.anchor); got != tt.want {
				t.Fatalf("exclusiveAnchorEdge(%d) = %d, want %d", tt.anchor, got, tt.want)
			}
		})
	}
}

func TestEffectiveExclusiveZone(t *testing.T) {
	if got := effectiveExclusiveZone(anchorTop, 32); got != 32 {
		t.Fatalf("valid positive zone = %d, want 32", got)
	}
	if got := effectiveExclusiveZone(anchorTop|anchorRight, 32); got != 0 {
		t.Fatalf("invalid positive zone = %d, want 0", got)
	}
	if got := effectiveExclusiveZone(anchorTop|anchorRight, 0); got != 0 {
		t.Fatalf("zero zone = %d, want 0", got)
	}
	if got := effectiveExclusiveZone(anchorTop|anchorRight, -1); got != -1 {
		t.Fatalf("negative zone = %d, want -1", got)
	}
}

func TestLayerPlacementBoundsFollowsExclusiveZoneSemantics(t *testing.T) {
	full := usableBox{x: 0, y: 0, width: 1920, height: 1080}
	usable := usableBox{x: 0, y: 32, width: 1920, height: 1048}

	if got := layerPlacementBounds(full, usable, 0); got != usable {
		t.Fatalf("zero-zone bounds = %+v, want usable bounds %+v", got, usable)
	}
	if got := layerPlacementBounds(full, usable, -1); got != full {
		t.Fatalf("negative-zone bounds = %+v, want full bounds %+v", got, full)
	}
	if got := layerPlacementBounds(full, usable, 32); got != full {
		t.Fatalf("positive-zone bounds = %+v, want full bounds %+v", got, full)
	}
}
