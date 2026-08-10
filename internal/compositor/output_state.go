package compositor

import (
	"strings"

	"github.com/swaywm/go-wlroots/wlroots"
)

// OutputState owns all compositor state scoped to one physical or nested
// output. Geometry is expressed in output-layout coordinates.
type OutputState struct {
	Output           wlroots.Output
	LayoutOutput     wlroots.OutputLayoutOutput
	SceneOutput      wlroots.SceneOutput
	Full             usableBox
	Usable           usableBox
	CurrentWorkspace int
	Fullscreen       *View
	FullscreenMode   presentationMode
	Focused          *View
	FocusHistory     []*View
	tileLayouts      map[int]*tileLayoutState
}

func (s *Server) tileLayoutForOutput(output *OutputState) *tileLayoutState {
	if output == nil {
		output = s.currentOutputState()
	}
	return s.tileLayoutForWorkspace(output, output.CurrentWorkspace)
}

func (s *Server) tileLayoutForView(v *View) *tileLayoutState {
	if v == nil {
		return s.tileLayoutForOutput(s.currentOutputState())
	}
	output := s.ensureViewOutput(v)
	workspace := v.Workspace
	if workspace <= 0 {
		workspace = output.CurrentWorkspace
	}
	return s.tileLayoutForWorkspace(output, workspace)
}

func (s *Server) tileLayoutForWorkspace(output *OutputState, workspace int) *tileLayoutState {
	if output == nil {
		output = &s.fallbackOutput
	}
	if workspace <= 0 {
		workspace = 1
	}
	if output.tileLayouts == nil {
		output.tileLayouts = make(map[int]*tileLayoutState)
	}
	layout := output.tileLayouts[workspace]
	if layout == nil {
		layout = &tileLayoutState{MasterRatio: 0.5, StackRatio: 0.5}
		output.tileLayouts[workspace] = layout
	}
	return layout
}

func (s *Server) currentOutputState() *OutputState {
	if s.activeOutput != nil {
		return s.activeOutput
	}
	if len(s.outputs) > 0 {
		return s.outputs[0]
	}
	return &s.fallbackOutput
}

func (s *Server) outputStateFor(output wlroots.Output) *OutputState {
	for _, state := range s.outputs {
		if state.Output == output {
			return state
		}
	}
	return nil
}

func (s *Server) outputStateByName(name string) *OutputState {
	for _, state := range s.outputs {
		if strings.EqualFold(strings.TrimSpace(state.Output.Name()), strings.TrimSpace(name)) {
			return state
		}
	}
	return nil
}

func (s *Server) outputStateAt(x, y float64) *OutputState {
	for _, output := range s.outputs {
		box := output.Full
		if x >= float64(box.x) && x < float64(box.x+box.width) &&
			y >= float64(box.y) && y < float64(box.y+box.height) {
			return output
		}
	}
	return nil
}

func (s *Server) ensureViewOutput(v *View) *OutputState {
	if v == nil {
		return s.currentOutputState()
	}
	if v.Output == nil {
		v.Output = s.currentOutputState()
	}
	return v.Output
}

func (s *Server) viewVisible(v *View) bool {
	if v == nil || !v.Mapped || v.InHat {
		return false
	}
	output := s.ensureViewOutput(v)
	return v.Workspace == output.CurrentWorkspace
}

func (s *Server) viewFullscreen(v *View) bool {
	return v != nil && s.ensureViewOutput(v).Fullscreen == v
}

func (s *Server) viewOutputName(v *View) string {
	if v == nil || v.Output == nil || v.Output == &s.fallbackOutput {
		return ""
	}
	return v.Output.Output.Name()
}

func (s *Server) rememberOutputFocus(output *OutputState, v *View) {
	if output == nil || v == nil {
		return
	}
	history := output.FocusHistory[:0]
	for _, candidate := range output.FocusHistory {
		if candidate != v && candidate.Mapped && !candidate.InHat &&
			candidate.Output == output {
			history = append(history, candidate)
		}
	}
	output.FocusHistory = append([]*View{v}, history...)
}

func (s *Server) moveViewToOutput(v *View, target *OutputState) bool {
	if v == nil || target == nil || target == &s.fallbackOutput || v.Output == target {
		return false
	}
	source := s.ensureViewOutput(v)
	if source.Fullscreen == v {
		setClientPresentationState(v, presentationNone)
		source.Fullscreen = nil
		source.FullscreenMode = presentationNone
	}
	if source.Focused == v {
		source.Focused = nil
	}
	v.Output = target
	v.Workspace = target.CurrentWorkspace
	target.Focused = v
	s.activeOutput = target
	s.rememberOutputFocus(target, v)
	s.arrange()
	s.emitIPCEvent("window_output_changed", s.ipcWindow(v))
	return true
}

func (s *Server) focusedViewForOutput(output *OutputState) *View {
	if output == nil {
		return nil
	}
	for _, v := range output.FocusHistory {
		if v.Mapped && !v.InHat && v.Output == output &&
			v.Workspace == output.CurrentWorkspace {
			return v
		}
	}
	for _, v := range s.views {
		if v.Managed && v.Mapped && !v.InHat &&
			s.ensureViewOutput(v) == output &&
			v.Workspace == output.CurrentWorkspace {
			return v
		}
	}
	return nil
}
