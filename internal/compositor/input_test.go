package compositor

import (
	"testing"

	"github.com/swaywm/go-wlroots/wlroots"
)

func TestEveryDenseGridTileHasResizeBoundary(t *testing.T) {
	area := usableBox{width: 1920, height: 1048}
	for count := 4; count <= 20; count++ {
		rowCounts := tileGridRowCounts(balancedGridTiles(area, count, 20))
		for index := 0; index < count; index++ {
			if !tileGridCellHasResizeBoundary(rowCounts, index) {
				t.Fatalf("count %d tile %d has no resizable boundary; rows=%v", count, index, rowCounts)
			}
		}
	}
}

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

func TestTilingResizeEdgesUsesVerticalSplitForThreeWindowStack(t *testing.T) {
	master, upper, lower := &View{}, &View{}, &View{}
	server := &Server{config: Config{Tiling: true},
		fallbackOutput: OutputState{CurrentWorkspace: 1}}
	server.activeOutput = &server.fallbackOutput
	for _, view := range []*View{master, upper, lower} {
		view.Managed, view.Mapped, view.Workspace = true, true, 1
		view.Output = &server.fallbackOutput
	}
	server.views = []*View{master, upper, lower}

	if got := server.tilingResizeEdges(master); got != wlroots.EdgeRight {
		t.Fatalf("master resize edges = %v, want horizontal", got)
	}
	if got := nearestStackResizeEdges(800, 490, 500, 500, true); got != wlroots.EdgeBottom {
		t.Fatalf("upper stack boundary edges = %v, want bottom", got)
	}
	if got := nearestStackResizeEdges(800, 510, 500, 500, false); got != wlroots.EdgeTop {
		t.Fatalf("lower stack boundary edges = %v, want top", got)
	}
	if got := nearestStackResizeEdges(510, 250, 500, 500, true); got != wlroots.EdgeLeft {
		t.Fatalf("stack left boundary edges = %v, want left", got)
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

func TestResizeFloatingGeometryRoundsPointerDelta(t *testing.T) {
	original := Geometry{X: 100, Y: 100, Width: 400, Height: 300}
	area := usableBox{width: 1200, height: 800}
	got := resizeFloatingGeometry(original, 10.6, 20.4,
		wlroots.EdgeRight|wlroots.EdgeBottom, area, 0, 0, 0, 0, 0)
	if got.Width != 411 || got.Height != 320 {
		t.Fatalf("rounded resize = %+v, want 411x320", got)
	}
}

func TestResizeFloatingGeometryNegativeBorderBehavesAsZero(t *testing.T) {
	original := Geometry{X: 100, Y: 100, Width: 400, Height: 300}
	area := usableBox{width: 500, height: 400}
	negative := resizeFloatingGeometry(original, 100, 100,
		wlroots.EdgeRight|wlroots.EdgeBottom, area, -5, 0, 0, 0, 0)
	zero := resizeFloatingGeometry(original, 100, 100,
		wlroots.EdgeRight|wlroots.EdgeBottom, area, 0, 0, 0, 0, 0)
	if negative != zero {
		t.Fatalf("negative border = %+v, zero border = %+v", negative, zero)
	}
}

func TestResizeFloatingGeometryKeepsLeftTopAnchorAtMinimum(t *testing.T) {
	original := Geometry{X: 300, Y: 250, Width: 500, Height: 400}
	got := resizeFloatingGeometry(original, 1000, 1000,
		wlroots.EdgeLeft|wlroots.EdgeTop,
		usableBox{width: 1600, height: 1000}, 2, 200, 150, 0, 0)
	if got.Width != 200 || got.Height != 150 || got.X != 600 || got.Y != 500 {
		t.Fatalf("unexpected minimum resize geometry: %+v", got)
	}
	if got.X+float64(got.Width) != 800 || got.Y+float64(got.Height) != 650 {
		t.Fatalf("opposite corner moved: %+v", got)
	}
}

func TestResizeCursorNameDefaultsWithoutEdges(t *testing.T) {
	if got := resizeCursorName(0); got != "default" {
		t.Fatalf("resizeCursorName(0) = %q, want default", got)
	}
}
