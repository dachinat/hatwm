package compositor

import (
	"math"
	"testing"
)

func TestAnimationEaseEndpointsAndRange(t *testing.T) {
	for _, name := range []string{"linear", "ease_out_quad", "ease_out_cubic", "ease_in_out_cubic", "unknown"} {
		t.Run(name, func(t *testing.T) {
			if got := animationEase(name, 0); got != 0 {
				t.Errorf("start = %v, want 0", got)
			}
			if got := animationEase(name, 1); got != 1 {
				t.Errorf("end = %v, want 1", got)
			}
			previous := 0.0
			for step := 1; step <= 20; step++ {
				got := animationEase(name, float64(step)/20)
				if got < previous || got < 0 || got > 1 {
					t.Fatalf("easing is not monotonic and bounded at step %d: previous=%v got=%v", step, previous, got)
				}
				previous = got
			}
		})
	}
}

func TestEaseInOutCubicIsSymmetric(t *testing.T) {
	for _, point := range []float64{0.1, 0.25, 0.4} {
		left := animationEase("ease_in_out_cubic", point)
		right := animationEase("ease_in_out_cubic", 1-point)
		if math.Abs((left+right)-1) > 1e-12 {
			t.Fatalf("curve is not symmetric at %v: %v + %v", point, left, right)
		}
	}
}

func TestAlmostEqualThreshold(t *testing.T) {
	if !almostEqual(100, 100.49) {
		t.Fatal("values inside the threshold are not equal")
	}
	if almostEqual(100, 100.5) {
		t.Fatal("threshold boundary unexpectedly considered equal")
	}
}
