package main

import (
	"math"
	"time"
)

// ViewAnimation only animates scene-tree position. Client size/configure state is
// changed immediately by the layout code, avoiding configure storms while still
// making opens and layout transitions visually smooth.
type ViewAnimation struct {
	Initialized bool
	Running     bool
	Start       time.Time
	FromX       float64
	FromY       float64
	ToX         float64
	ToY         float64
}

func (s *Server) setViewPosition(v *View, x, y float64) {
	if v == nil {
		return
	}

	if !s.config.Animations || s.config.AnimationDurationMS <= 0 ||
		s.cursorMode == CursorTilingResize {
		v.Animation.Running = false
		v.Animation.Initialized = true
		v.Animation.ToX, v.Animation.ToY = x, y
		v.RootTree.Node().SetPosition(x, y)
		v.configureXWaylandPosition(x, y)
		return
	}

	now := time.Now()
	if v.Animation.Running && almostEqual(v.Animation.ToX, x) && almostEqual(v.Animation.ToY, y) {
		return
	}
	currentX, currentY := float64(v.RootTree.Node().X()), float64(v.RootTree.Node().Y())

	if !v.Animation.Initialized {
		currentX = x
		currentY = y + float64(s.config.AnimationOpenOffset)
		v.RootTree.Node().SetPosition(currentX, currentY)
		v.Animation.Initialized = true
	}

	if almostEqual(currentX, x) && almostEqual(currentY, y) {
		v.Animation.Running = false
		v.Animation.ToX, v.Animation.ToY = x, y
		v.RootTree.Node().SetPosition(x, y)
		v.configureXWaylandPosition(x, y)
		return
	}

	v.Animation = ViewAnimation{
		Initialized: true,
		Running:     true,
		Start:       now,
		FromX:       currentX,
		FromY:       currentY,
		ToX:         x,
		ToY:         y,
	}
	v.configureXWaylandPosition(x, y)
}

func (s *Server) setViewPositionImmediate(v *View, x, y float64) {
	if v == nil {
		return
	}
	v.Animation.Running = false
	v.Animation.Initialized = true
	v.Animation.ToX, v.Animation.ToY = x, y
	v.RootTree.Node().SetPosition(x, y)
	v.configureXWaylandPosition(x, y)
}

func (s *Server) tickAnimations(now time.Time) {
	duration := time.Duration(s.config.AnimationDurationMS) * time.Millisecond
	if !s.config.Animations || duration <= 0 {
		for _, v := range s.views {
			if v != nil && v.Animation.Running {
				v.RootTree.Node().SetPosition(v.Animation.ToX, v.Animation.ToY)
				v.Animation.Running = false
			}
		}
		return
	}

	for _, v := range s.views {
		if v == nil || !v.Mapped || !v.Animation.Running {
			continue
		}

		progress := float64(now.Sub(v.Animation.Start)) / float64(duration)
		if progress >= 1 {
			v.RootTree.Node().SetPosition(v.Animation.ToX, v.Animation.ToY)
			v.Animation.Running = false
			continue
		}
		if progress < 0 {
			progress = 0
		}

		t := animationEase(s.config.AnimationEasing, progress)
		x := v.Animation.FromX + (v.Animation.ToX-v.Animation.FromX)*t
		y := v.Animation.FromY + (v.Animation.ToY-v.Animation.FromY)*t
		v.RootTree.Node().SetPosition(x, y)
	}
}

func animationEase(name string, t float64) float64 {
	switch name {
	case "linear":
		return t
	case "ease_out_quad":
		return 1 - (1-t)*(1-t)
	case "ease_in_out_cubic":
		if t < 0.5 {
			return 4 * t * t * t
		}
		return 1 - math.Pow(-2*t+2, 3)/2
	case "ease_out_cubic":
		fallthrough
	default:
		return 1 - math.Pow(1-t, 3)
	}
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.5
}
