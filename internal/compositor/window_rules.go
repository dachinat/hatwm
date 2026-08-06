package compositor

import (
	"log/slog"
	"path"
	"strings"
)

func rulePatternMatches(pattern, value string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	value = strings.ToLower(strings.TrimSpace(value))
	if pattern == "" {
		return true
	}
	matched, err := path.Match(pattern, value)
	return err == nil && matched
}

func (r WindowRule) matches(v *View) bool {
	if v == nil ||
		!rulePatternMatches(r.AppID, v.AppID) ||
		!rulePatternMatches(r.Class, v.XWaylandClass) ||
		!rulePatternMatches(r.Instance, v.XWaylandInstance) ||
		!rulePatternMatches(r.Title, v.Title) {
		return false
	}
	if r.HasDialog && r.Dialog != v.Dialog {
		return false
	}
	if r.HasModal && r.Modal != v.Modal {
		return false
	}
	return true
}

func mergeWindowRuleActions(target *WindowRuleActions, source WindowRuleActions) {
	if source.HasFloating {
		target.Floating, target.HasFloating = source.Floating, true
	}
	if source.HasCentered {
		target.Centered, target.HasCentered = source.Centered, true
	}
	if source.HasKeepAbove {
		target.KeepAbove, target.HasKeepAbove = source.KeepAbove, true
	}
	if source.HasOpacity {
		target.Opacity, target.HasOpacity = source.Opacity, true
	}
	if source.HasWorkspace {
		target.Workspace, target.HasWorkspace = source.Workspace, true
	}
	if source.HasOutput {
		target.Output, target.HasOutput = source.Output, true
	}
	if source.HasWidth {
		target.Width, target.HasWidth = source.Width, true
	}
	if source.HasHeight {
		target.Height, target.HasHeight = source.Height, true
	}
	if source.HasX {
		target.X, target.HasX = source.X, true
	}
	if source.HasY {
		target.Y, target.HasY = source.Y, true
	}
	if source.HasFullscreen {
		target.Fullscreen, target.HasFullscreen = source.Fullscreen, true
	}
	if source.HasFocus {
		target.Focus, target.HasFocus = source.Focus, true
	}
	if source.HasBorder {
		target.Border, target.HasBorder = source.Border, true
	}
	if source.HasBorderSize {
		target.BorderSize, target.HasBorderSize = source.BorderSize, true
	}
	if source.HasBorderRounding {
		target.BorderRounding, target.HasBorderRounding = source.BorderRounding, true
	}
}

func (s *Server) resolveWindowRules(v *View) (WindowRuleActions, string) {
	var actions WindowRuleActions
	names := make([]string, 0, len(s.config.WindowRules))
	for _, rule := range s.config.WindowRules {
		if !rule.matches(v) {
			continue
		}
		mergeWindowRuleActions(&actions, rule.Actions)
		names = append(names, rule.Name)
	}
	return actions, strings.Join(names, ",")
}

func (v *View) ruleGeometryForcesFloating() bool {
	a := v.RuleActions
	return a.HasWidth || a.HasHeight || a.HasX || a.HasY ||
		a.HasOutput || (a.HasCentered && a.Centered)
}

func (v *View) ruleAllowsFocus() bool {
	return v == nil || !v.RuleActions.HasFocus || v.RuleActions.Focus
}

func (v *View) shouldKeepAbove() bool {
	if v == nil {
		return false
	}
	if v.RuleActions.HasKeepAbove {
		return v.RuleActions.KeepAbove
	}
	return v.AutoFloating
}

func (s *Server) viewBorderSize(v *View) int {
	if v != nil {
		if v.RuleActions.HasBorder && !v.RuleActions.Border {
			return 0
		}
		if v.RuleActions.HasBorderSize {
			return v.RuleActions.BorderSize
		}
	}
	return s.config.BorderSize
}

func (s *Server) viewBorderRounding(v *View) int {
	if v != nil && v.RuleActions.HasBorderRounding {
		return v.RuleActions.BorderRounding
	}
	return s.config.BorderRounding
}

func (s *Server) viewArea(v *View) usableBox {
	if v == nil || !v.RuleActions.HasOutput || v.RuleActions.Output == "" {
		return s.usable
	}
	name := strings.TrimSpace(v.RuleActions.Output)
	for i, output := range s.outputs {
		if !strings.EqualFold(output.Name(), name) {
			continue
		}
		if i == 0 {
			return s.usable
		}
		x, y := s.outputLayout.Coords(output)
		width, height := output.EffectiveResolution()
		return usableBox{x: int(x), y: int(y), width: width, height: height}
	}
	return s.usable
}

func (s *Server) applyRuleGeometry(v *View) {
	if v == nil || !s.isFloatingView(v) {
		return
	}
	a := v.RuleActions
	if !a.HasOutput && !a.HasWidth && !a.HasHeight && !a.HasX && !a.HasY &&
		!(a.HasCentered && a.Centered) {
		return
	}
	area := s.viewArea(v)
	current := v.geometry()
	width, height := int(current.Width), int(current.Height)
	if width < 1 {
		width = minInt(800, area.width)
	}
	if height < 1 {
		height = minInt(600, area.height)
	}
	if a.HasWidth {
		width = a.Width
	}
	if a.HasHeight {
		height = a.Height
	}
	x := v.RootTree.Node().X()
	y := v.RootTree.Node().Y()
	border := s.viewBorderSize(v)
	if a.HasOutput || (a.HasCentered && a.Centered) {
		x = area.x + (area.width-width-2*border)/2
		y = area.y + (area.height-height-2*border)/2
	}
	if a.HasX {
		x = area.x + a.X
	}
	if a.HasY {
		y = area.y + a.Y
	}
	geometry := clampFloatingGeometry(Geometry{
		X: float64(x), Y: float64(y), Width: uint32(width), Height: uint32(height),
	}, area, border, 0, 0)
	v.Floating, v.FloatingValid = geometry, true
	s.setViewPositionImmediate(v, geometry.X, geometry.Y)
	v.setSize(geometry.Width, geometry.Height)
}

func (s *Server) applyWindowRules(v *View, initial bool) {
	if v == nil {
		return
	}
	v.refreshWindowIdentity()
	oldActions, oldMatches := v.RuleActions, v.MatchedRules
	actions, matches := s.resolveWindowRules(v)
	if !initial && actions == oldActions && matches == oldMatches {
		return
	}
	v.RuleActions, v.MatchedRules = actions, matches
	v.AutoFloating = v.shouldAutoFloat()
	if actions.HasWorkspace && s.validWorkspace(actions.Workspace) {
		v.Workspace = actions.Workspace
	}
	if actions.HasOutput {
		found := false
		for _, output := range s.outputs {
			if strings.EqualFold(output.Name(), actions.Output) {
				found = true
				break
			}
		}
		if !found && len(s.outputs) > 0 {
			slog.Warn("window rule refers to unknown output",
				"rules", matches, "output", actions.Output)
		}
	}
	s.applyRuleGeometry(v)
	s.applyWindowOpacity(v)
	if !v.Mapped {
		return
	}
	fullscreenChanged := initial || actions.Fullscreen != oldActions.Fullscreen ||
		actions.HasFullscreen != oldActions.HasFullscreen
	if v.Workspace == s.currentWorkspace && fullscreenChanged {
		if actions.HasFullscreen {
			s.setViewFullscreen(v, actions.Fullscreen)
		} else if oldActions.HasFullscreen && oldActions.Fullscreen &&
			s.fullscreen == v && s.fullscreenMode == presentationFullscreen {
			s.setViewFullscreen(v, false)
		}
	}
	v.RootTree.Node().SetEnabled(v.Workspace == s.currentWorkspace)
	s.updateDecoration(v)
	if v.shouldKeepAbove() {
		v.RootTree.Node().RaiseToTop()
	}
}

func (s *Server) applyRuleFullscreenForCurrentWorkspace() {
	for _, v := range s.views {
		if v.Managed && v.Mapped && v.Workspace == s.currentWorkspace &&
			v.RuleActions.HasFullscreen && v.RuleActions.Fullscreen {
			s.setViewFullscreen(v, true)
			return
		}
	}
}

func (s *Server) reapplyWindowRules() {
	for _, v := range s.views {
		s.applyWindowRules(v, false)
	}
}
