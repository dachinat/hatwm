package compositor

import "testing"

func TestDecorationRadiiGrowBorderOutward(t *testing.T) {
	outer, inner := decorationRadii(10, 2, 800, 600)
	if outer != 12 || inner != 10 {
		t.Fatalf("decorationRadii() = (%d, %d), want (12, 10)", outer, inner)
	}
}

func TestDecorationRadiiDisableRoundingAtZero(t *testing.T) {
	outer, inner := decorationRadii(0, 2, 800, 600)
	if outer != 0 || inner != 0 {
		t.Fatalf("decorationRadii() = (%d, %d), want square corners", outer, inner)
	}
}

func TestDecorationRadiiClampToSmallClient(t *testing.T) {
	outer, inner := decorationRadii(20, 2, 12, 8)
	if outer != 6 || inner != 4 {
		t.Fatalf("decorationRadii() = (%d, %d), want (6, 4)", outer, inner)
	}
}
