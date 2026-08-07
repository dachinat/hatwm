package compositor

import "testing"

func TestPopupConstraintBoxUsesToplevelSurfaceCoordinates(t *testing.T) {
	output := usableBox{x: 0, y: 0, width: 1920, height: 1080}
	got := popupConstraintBox(output, 1200, 100, 0, 0)
	want := usableBox{x: -1200, y: -100, width: 1920, height: 1080}
	if got != want {
		t.Fatalf("unexpected popup constraint box: got %+v, want %+v", got, want)
	}
}

func TestPopupConstraintBoxIncludesWindowGeometryOffset(t *testing.T) {
	output := usableBox{x: -1920, y: 0, width: 1920, height: 1080}
	got := popupConstraintBox(output, -1000, 50, 8, 10)
	want := usableBox{x: -912, y: -40, width: 1920, height: 1080}
	if got != want {
		t.Fatalf("unexpected popup constraint box: got %+v, want %+v", got, want)
	}
}
