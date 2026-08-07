package compositor

import "github.com/swaywm/go-wlroots/wlroots"

func (s *Server) handleNewXDGTopLevel(top wlroots.XDGTopLevel) {
	setSupportedToplevelCapabilities(top)
	root := s.tiledTree.NewSceneTree()
	surfaceTree := root.NewXDGSurface(top.Base())
	clientSurface := top.Base().Surface()
	s.nextViewID++
	output := s.currentOutputState()
	v := &View{
		ID:            s.nextViewID,
		TopLevel:      top,
		Managed:       true,
		Associated:    true,
		ClientSurface: clientSurface,
		RootTree:      root,
		SurfaceTree:   surfaceTree,
		Server:        s,
		Workspace:     output.CurrentWorkspace,
		Output:        output,
		SceneLayer:    viewSceneLayerTiled,
	}
	root.Node().SetEnabled(false)
	s.views = append(s.views, v)
	s.createForeignToplevel(v)
	s.applyWindowOpacity(v)
	top.Base().SetData(root)
	s.listenForMaximize(v)

	top.Base().OnMap(func(_ wlroots.XDGSurface) {
		s.notifyFractionalScale(clientSurface)
		v.Mapped = true
		s.updateXDGDialogState(v)
		if parent := s.parentView(v); parent != nil {
			v.Output = parent.Output
			v.Workspace = parent.Workspace
		}
		s.applyWindowRules(v, true)
		s.syncViewSceneLayer(v)
		v.Animation = ViewAnimation{}
		s.applyWindowOpacity(v)
		v.RootTree.Node().SetEnabled(s.viewVisible(v))
		// Tile order is independent from focus order. New windows retain the
		// existing behavior of becoming the master tile, but later focus
		// changes must not silently rearrange the layout.
		s.moveViewFront(v)
		if v.ruleAllowsFocus() {
			s.requestViewActivation(v)
		}
		s.arrange()
		s.updateDecoration(v)
		s.emitIPCEvent("window_opened", s.ipcWindow(v))
		s.emitIPCEvent("workspace_updated", s.ipcWorkspaces())
	})
	top.Base().OnUnmap(func(_ wlroots.XDGSurface) {
		s.unmapView(v)
		s.emitIPCEvent("window_closed", map[string]any{"id": v.ID, "workspace": v.Workspace})
		s.emitIPCEvent("workspace_updated", s.ipcWorkspaces())
	})
	top.Base().OnDestroy(func(_ wlroots.XDGSurface) {
		if s.grabbedView == v {
			s.cancelViewGrab()
		}
		s.destroyDecoration(v)
		s.destroyForeignToplevel(v)
		s.removeView(v)
		v.RootTree.Node().Destroy()
		s.arrange()
	})
	top.Base().OnCommit(func(surface wlroots.XDGSurface) {
		s.notifyFractionalScale(surface.Surface())
		s.updateForeignToplevel(v)
		if surface.InitialCommit() {
			surface.ScheduleConfigure()
		}
		if v.Mapped {
			s.applyWindowOpacity(v)
			wasAutoFloating := v.AutoFloating
			oldWorkspace := v.Workspace
			s.applyWindowRules(v, false)
			if s.isFloatingView(v) && !s.viewFullscreen(v) {
				s.rememberFloatingGeometry(v)
			}
			s.updateDecoration(v)
			if wasAutoFloating != v.AutoFloating || oldWorkspace != v.Workspace {
				s.arrange()
			}
		}
	})
	top.OnRequestMove(func(_ wlroots.SeatClient, _ uint32) { s.beginInteractive(v, CursorMove, 0) })
	top.OnRequestResize(func(_ wlroots.SeatClient, _ uint32, edges wlroots.Edges) { s.beginInteractive(v, CursorResize, edges) })
}

func (s *Server) handleNewXDGPopup(p wlroots.XDGPopup) {
	base := p.Base()
	var owner *View
	base.OnCommit(func(surface wlroots.XDGSurface) {
		s.notifyFractionalScale(surface.Surface())
		if surface.InitialCommit() {
			surface.ScheduleConfigure()
		}
		s.applyWindowOpacity(owner)
	})

	parent := p.Parent()
	// A parentless xdg_popup is not a standalone desktop popup. Layer-shell
	// clients create it with a NULL xdg parent, then associate it through
	// zwlr_layer_surface_v1.get_popup. The layer-shell listener will attach it
	// to the correct scene tree when that request arrives.
	if parent.Nil() {
		return
	}

	// Layer-shell popups are parented and constrained by the layer-shell
	// listener, which has the layer surface's scene position and output bounds.
	// Creating the same popup here would also place a duplicate in normalTree,
	// using output coordinates instead of coordinates relative to the panel.
	if s.layerSurfaceForSurface(parent) != nil {
		return
	}

	tree := s.floatingTree
	if parent.Type() == wlroots.SurfaceTypeXDG {
		tree = parent.XDGSurface().SceneTree()
	}
	st := tree.NewXDGSurface(base)
	base.SetData(st)
	root := parent.RootSurface()
	for _, view := range s.views {
		if view.Associated && view.clientSurface() == root {
			owner = view
			break
		}
	}
	s.applyWindowOpacity(owner)
}

func (s *Server) removeView(target *View) {
	for i, v := range s.views {
		if v == target {
			s.views = append(s.views[:i], s.views[i+1:]...)
			return
		}
	}
}
func (s *Server) mappedViews() []*View {
	return s.mappedViewsForOutput(s.currentOutputState())
}

func (s *Server) mappedViewsForOutput(output *OutputState) []*View {
	out := make([]*View, 0, len(s.views))
	for _, v := range s.views {
		if v.Managed && v.Mapped && s.ensureViewOutput(v) == output &&
			v.Workspace == output.CurrentWorkspace {
			out = append(out, v)
		}
	}
	return out
}

func (s *Server) mappedTiledViews() []*View {
	return s.mappedTiledViewsForOutput(s.currentOutputState())
}

func (s *Server) mappedTiledViewsForOutput(output *OutputState) []*View {
	out := make([]*View, 0, len(s.views))
	for _, v := range s.mappedViewsForOutput(output) {
		if !v.AutoFloating {
			out = append(out, v)
		}
	}
	return out
}

func (s *Server) isFloatingView(v *View) bool {
	return v != nil && (!s.config.Tiling || v.AutoFloating)
}

func (s *Server) desiredViewSceneLayer(v *View) viewSceneLayer {
	if v == nil || !v.Managed || s.isFloatingView(v) || v.shouldKeepAbove() {
		return viewSceneLayerFloating
	}
	return viewSceneLayerTiled
}

func (s *Server) syncViewSceneLayer(v *View) {
	if v == nil || v.RootTree.Nil() {
		return
	}
	desired := s.desiredViewSceneLayer(v)
	if v.SceneLayer == desired {
		return
	}
	parent := s.tiledTree
	if desired == viewSceneLayerFloating {
		parent = s.floatingTree
	}
	v.RootTree.Node().Reparent(parent)
	v.SceneLayer = desired
}

func shouldAutoFloatXDG(
	isDialog, isModal, hasParent bool,
	minWidth, minHeight, maxWidth, maxHeight int,
) bool {
	fixedSize := minWidth > 0 && minHeight > 0 &&
		maxWidth == minWidth && maxHeight == minHeight
	if hasParent {
		return true
	}
	if isDialog {
		return isModal && fixedSize
	}
	// Compatibility fallback for toolkits which do not bind xdg-dialog-v1.
	return fixedSize
}

func (v *View) shouldAutoFloat() bool {
	if v == nil {
		return false
	}
	if v.RuleActions.HasFloating {
		return v.RuleActions.Floating
	}
	if v.ruleGeometryForcesFloating() {
		return true
	}
	if v.IsXWayland || v.TopLevel.Nil() {
		return v.Dialog || v.Modal
	}
	minWidth, minHeight, maxWidth, maxHeight := v.sizeConstraints()
	return shouldAutoFloatXDG(v.Dialog, v.Modal, !v.TopLevel.Parent().Nil(),
		minWidth, minHeight, maxWidth, maxHeight)
}

func (s *Server) parentView(v *View) *View {
	if v == nil || v.IsXWayland || v.TopLevel.Nil() {
		return nil
	}
	parent := v.TopLevel.Parent()
	if parent.Nil() {
		return nil
	}
	parentSurface := parent.Base().Surface()
	for _, candidate := range s.views {
		if candidate != v && candidate.Mapped && candidate.clientSurface() == parentSurface {
			return candidate
		}
	}
	return nil
}
func (s *Server) focusedView() *View {
	if output := s.currentOutputState(); output.Focused != nil &&
		s.viewVisible(output.Focused) {
		return output.Focused
	}
	surf := s.seat.KeyboardState().FocusedSurface()
	if surf.Nil() {
		return nil
	}
	for _, v := range s.views {
		if s.viewVisible(v) &&
			v.clientSurface() == surf {
			return v
		}
	}
	return nil
}

func (s *Server) focusView(v *View, surface *wlroots.Surface) {
	if s.sessionLocked || v == nil || !s.viewVisible(v) {
		return
	}
	if exclusive := s.exclusiveKeyboardLayer(); exclusive != nil {
		s.focusLayerSurface(exclusive)
		return
	}
	s.setViewUrgent(v, false)
	prev := s.focusedView()
	s.activeOutput = s.ensureViewOutput(v)
	s.rememberOutputFocus(s.activeOutput, v)
	s.activeOutput.Focused = v
	clientSurface := v.clientSurface()
	// OutputState.Focused is also the remembered focus used when returning to
	// a workspace. It can still point at v after keyboard focus was cleared,
	// so only take the fast path when the seat really focuses this surface.
	if prev == v && !clientSurface.Nil() &&
		s.seat.KeyboardState().FocusedSurface() == clientSurface {
		return
	}
	if prev != nil && prev != v {
		prev.setActivated(false)
		s.updateDecoration(prev)
	}
	v.RootTree.Node().RaiseToTop()
	v.setActivated(true)
	if clientSurface.Nil() {
		return
	}
	if len(s.keyboards) > 0 {
		s.seat.NotifyKeyboardEnter(clientSurface, s.seat.Keyboard())
	}
	s.updateDecoration(v)
	s.emitIPCEvent("focus_changed", s.ipcWindow(v))
}

func (s *Server) setViewUrgent(v *View, urgent bool) {
	if v == nil || !v.Mapped || v.Urgent == urgent {
		return
	}
	v.Urgent = urgent
	s.emitIPCEvent("window_urgent", s.ipcWindow(v))
	s.emitIPCEvent("workspace_updated", s.ipcWorkspaces())
}

func (s *Server) requestViewActivation(v *View) {
	if v == nil || !v.Mapped {
		return
	}
	if !s.viewVisible(v) {
		s.setViewUrgent(v, true)
		return
	}
	surface := v.clientSurface()
	s.focusView(v, &surface)
}
func (s *Server) moveViewFront(v *View) {
	idx := -1
	for i, x := range s.views {
		if x == v {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return
	}
	copy(s.views[1:idx+1], s.views[0:idx])
	s.views[0] = v
}

func (s *Server) cycleFocus() {
	m := s.mappedViews()
	if len(m) < 2 {
		return
	}
	current := s.focusedView()
	idx := -1
	for i, v := range m {
		if v == current {
			idx = i
			break
		}
	}
	next := m[0]
	if idx >= 0 {
		next = m[(idx+1)%len(m)]
	}
	surf := next.clientSurface()
	s.focusView(next, &surf)
}

func (s *Server) viewAt(x, y float64) (*View, *wlroots.Surface, float64, float64) {
	node, sx, sy := s.scene.Tree().Node().At(x, y)
	if node.Nil() || node.Type() != wlroots.SceneNodeBuffer {
		return nil, nil, 0, 0
	}
	ss := node.SceneBuffer().SceneSurface()
	if ss.Nil() {
		return nil, nil, 0, 0
	}
	surf := ss.Surface()
	if surf.Nil() {
		return nil, nil, 0, 0
	}
	root := surf.RootSurface()
	if root.Nil() {
		return nil, &surf, sx, sy
	}
	for _, v := range s.views {
		if v.Mapped && v.Associated &&
			v.clientSurface() == root {
			return v, &surf, sx, sy
		}
	}
	return nil, &surf, sx, sy
}

func (s *Server) beginInteractive(v *View, mode CursorMode, edges wlroots.Edges) {
	if v == nil || (v.Managed && !s.isFloatingView(v)) ||
		s.viewFullscreen(v) || s.cursorButtonCount == 0 {
		return
	}
	s.grabbedView = v
	s.grabOwnsButton = false
	s.cursorMode = mode
	s.resizeEdges = edges
	if mode == CursorMove {
		s.grabX = s.cursor.X() - float64(v.RootTree.Node().X())
		s.grabY = s.cursor.Y() - float64(v.RootTree.Node().Y())
		s.beginCursorOverride("move")
		return
	}
	box := v.geometry()
	s.grabX, s.grabY = s.cursor.X(), s.cursor.Y()
	s.grabBox = wlroots.GeoBox{X: v.RootTree.Node().X(), Y: v.RootTree.Node().Y(),
		Width: box.Width, Height: box.Height}
	s.beginCursorOverride(resizeCursorName(edges))
}

func (s *Server) cancelViewGrab() {
	s.grabbedView = nil
	s.grabOwnsButton = false
	s.cursorMode = CursorPassthrough
	s.cursorButtonCount = 0
	s.grabGrid = tileGridResizeGrab{}
	s.resizeEdges = 0
	s.endCursorOverride()
}

func (s *Server) unmapView(v *View) {
	if v == nil {
		return
	}
	wasFocused := v.Associated &&
		s.seat.KeyboardState().FocusedSurface() == v.clientSurface()
	v.Mapped = false
	v.Urgent = false
	v.Animation.Running = false
	v.RootTree.Node().SetEnabled(false)
	s.updateDecoration(v)
	output := s.ensureViewOutput(v)
	if output.Focused == v {
		output.Focused = nil
	}
	if output.Fullscreen == v {
		output.Fullscreen = nil
		output.FullscreenMode = presentationNone
	}
	if s.grabbedView == v {
		s.cancelViewGrab()
	}
	v.setActivated(false)
	if wasFocused {
		if mapped := s.mappedViews(); len(mapped) > 0 {
			surface := mapped[0].clientSurface()
			s.focusView(mapped[0], &surface)
		}
	}
	s.arrange()
}
