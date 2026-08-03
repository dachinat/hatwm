package main

import "testing"

func TestBalancedGridTilesFourWindows(t *testing.T) {
	area := usableBox{x: 0, y: 32, width: 1920, height: 1048}
	got := balancedGridTiles(area, 4, 20)
	want := []usableBox{
		{x: 20, y: 52, width: 930, height: 494},
		{x: 970, y: 52, width: 930, height: 494},
		{x: 20, y: 566, width: 930, height: 494},
		{x: 970, y: 566, width: 930, height: 494},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d tiles, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("tile %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestBalancedGridTilesStaySeparatedAndInBounds(t *testing.T) {
	area := usableBox{x: 11, y: 17, width: 1913, height: 1031}
	const gap = 19
	for count := 4; count <= 12; count++ {
		tiles := balancedGridTiles(area, count, gap)
		if len(tiles) != count {
			t.Fatalf("count %d produced %d tiles", count, len(tiles))
		}
		for i, tile := range tiles {
			if tile.x < area.x+gap || tile.y < area.y+gap ||
				tile.x+tile.width > area.x+area.width-gap ||
				tile.y+tile.height > area.y+area.height-gap {
				t.Errorf("count %d tile %d outside area: %+v", count, i, tile)
			}
			for j := 0; j < i; j++ {
				other := tiles[j]
				xSeparated := tile.x >= other.x+other.width+gap ||
					other.x >= tile.x+tile.width+gap
				ySeparated := tile.y >= other.y+other.height+gap ||
					other.y >= tile.y+tile.height+gap
				if !xSeparated && !ySeparated {
					t.Errorf("count %d tiles %d and %d lack gap: %+v %+v",
						count, j, i, other, tile)
				}
			}
		}
	}
}

func TestMinimumAwareGridUsesFewerRowsForTallClients(t *testing.T) {
	area := usableBox{x: 0, y: 32, width: 1920, height: 1048}
	minimums := make([]tileMinimum, 7)
	for i := range minimums {
		minimums[i] = tileMinimum{width: 200, height: 350}
	}
	tiles := minimumAwareGridTiles(area, minimums, 20, 2)
	if rows := distinctTileRows(tiles); rows != 2 {
		t.Fatalf("got %d rows, want 2: %+v", rows, tiles)
	}
	for i, tile := range tiles {
		if tile.height-4 < minimums[i].height {
			t.Errorf("tile %d content height %d is below minimum %d",
				i, tile.height-4, minimums[i].height)
		}
	}
}

func TestMinimumAwareGridKeepsPreferredShapeWhenClientsFit(t *testing.T) {
	area := usableBox{x: 0, y: 32, width: 1920, height: 1048}
	minimums := make([]tileMinimum, 7)
	got := minimumAwareGridTiles(area, minimums, 20, 2)
	want := balancedGridTiles(area, 7, 20)
	if len(got) != len(want) {
		t.Fatalf("got %d tiles, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("tile %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestClampFloatingGeometryKeepsWindowInsideUsableArea(t *testing.T) {
	area := usableBox{x: 0, y: 32, width: 1920, height: 1048}
	got := clampFloatingGeometry(Geometry{
		X: -300, Y: 1600, Width: 800, Height: 600,
	}, area, 2, 200, 200)
	if got.X != 0 || got.Y != 476 || got.Width != 800 || got.Height != 600 {
		t.Fatalf("unexpected clamped geometry: %+v", got)
	}
}

func TestClampFloatingGeometryHonorsMinimumAndOutputMaximum(t *testing.T) {
	area := usableBox{x: 10, y: 20, width: 1000, height: 700}
	got := clampFloatingGeometry(Geometry{
		X: 400, Y: 300, Width: 100, Height: 100,
	}, area, 3, 480, 320)
	if got.Width != 480 || got.Height != 320 {
		t.Fatalf("minimum size not honored: %+v", got)
	}
	if got.X != 400 || got.Y != 300 {
		t.Fatalf("valid position changed: %+v", got)
	}

	got = clampFloatingGeometry(Geometry{
		X: 50, Y: 60, Width: 4000, Height: 3000,
	}, area, 3, 0, 0)
	if got.X != 10 || got.Y != 20 || got.Width != 994 || got.Height != 694 {
		t.Fatalf("oversized window not fitted to output: %+v", got)
	}
}

func TestClampFloatingMoveGeometryAllowsPartialOffscreenPlacement(t *testing.T) {
	area := usableBox{x: 0, y: 32, width: 1920, height: 1048}
	got := clampFloatingMoveGeometry(Geometry{
		X: -1000, Y: 1600, Width: 800, Height: 600,
	}, area, 2, 48)
	if got.X != -756 || got.Y != 1032 {
		t.Fatalf("unexpected recoverable position: %+v", got)
	}
	if got.Width != 800 || got.Height != 600 {
		t.Fatalf("moving changed window size: %+v", got)
	}

	got = clampFloatingMoveGeometry(Geometry{
		X: 3000, Y: -500, Width: 800, Height: 600,
	}, area, 2, 48)
	if got.X != 1872 || got.Y != 32 {
		t.Fatalf("unexpected opposite-edge position: %+v", got)
	}
}

func TestClampFloatingMoveGeometryKeepsSmallWindowReachable(t *testing.T) {
	area := usableBox{x: 10, y: 20, width: 1000, height: 700}
	got := clampFloatingMoveGeometry(Geometry{
		X: -500, Y: 900, Width: 30, Height: 20,
	}, area, 2, 48)
	if got.X != 10 || got.Y != 696 {
		t.Fatalf("small window was allowed outside the output: %+v", got)
	}
}

func TestShouldAutoFloatXDG(t *testing.T) {
	tests := []struct {
		name                                     string
		isDialog, isModal, hasParent             bool
		minWidth, minHeight, maxWidth, maxHeight int
		want                                     bool
	}{
		{name: "transient", hasParent: true, want: true},
		{name: "modal fixed dialog", isDialog: true, isModal: true, minWidth: 400, minHeight: 500, maxWidth: 400, maxHeight: 500, want: true},
		{name: "non-modal fixed dialog", isDialog: true, minWidth: 400, minHeight: 500, maxWidth: 400, maxHeight: 500, want: false},
		{name: "modal resizable dialog", isDialog: true, isModal: true, minWidth: 400, minHeight: 500, want: false},
		{name: "legacy fixed-size fallback", minWidth: 400, minHeight: 500, maxWidth: 400, maxHeight: 500, want: true},
		{name: "resizable", minWidth: 400, minHeight: 500, maxWidth: 0, maxHeight: 0, want: false},
		{name: "only fixed width", minWidth: 400, minHeight: 100, maxWidth: 400, maxHeight: 0, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldAutoFloatXDG(tt.isDialog, tt.isModal, tt.hasParent,
				tt.minWidth, tt.minHeight, tt.maxWidth, tt.maxHeight)
			if got != tt.want {
				t.Fatalf("shouldAutoFloatXDG() = %v, want %v", got, tt.want)
			}
		})
	}
}
