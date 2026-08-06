package compositor

import "github.com/swaywm/go-wlroots/wlroots"

const floatingVisibleStrip = 48

func (s *Server) arrange() {
	s.arrangeLayers()
}

func (s *Server) arrangeViewsIn(area usableBox) {
	if len(s.outputs) == 0 {
		return
	}
	for _, v := range s.views {
		v.RootTree.Node().SetEnabled(v.Mapped && v.Workspace == s.currentWorkspace)
	}
	views := s.mappedViews()
	if len(views) == 0 {
		return
	}
	defer s.keepAutoFloatingViewsAboveTiles()
	outW, outH := area.width, area.height
	ux, uy := area.x, area.y
	gap := s.config.Gaps
	if gap < 0 {
		gap = 0
	}

	for _, v := range views {
		v.RootTree.Node().SetEnabled(true)
	}
	if s.fullscreen != nil && s.fullscreen.Mapped {
		for _, v := range views {
			v.RootTree.Node().SetEnabled(v == s.fullscreen)
		}
		fullscreenArea := s.viewArea(s.fullscreen)
		s.setViewPosition(s.fullscreen,
			float64(fullscreenArea.x), float64(fullscreenArea.y))
		s.fullscreen.setTiledContentSize(
			uint32(fullscreenArea.width), uint32(fullscreenArea.height))
		s.fullscreen.setSize(
			uint32(fullscreenArea.width), uint32(fullscreenArea.height))
		s.updateDecoration(s.fullscreen)
		return
	}

	if !s.config.Tiling {
		for i, v := range views {
			s.placeFloatingView(v, i)
		}
		return
	}
	// Transient and fixed-size dialogs remain floating while normal toplevels
	// participate in the tiled layout.
	tiled := make([]*View, 0, len(views))
	floatingIndex := 0
	for _, v := range views {
		if v.AutoFloating {
			s.placeFloatingView(v, floatingIndex)
			floatingIndex++
			continue
		}
		tiled = append(tiled, v)
	}
	views = tiled
	if len(views) == 0 {
		return
	}
	inner := func(v *View, w, h int) (uint32, uint32) {
		b := 2 * s.viewBorderSize(v)
		if w <= b {
			w = b + 1
		}
		if h <= b {
			h = b + 1
		}
		return uint32(w - b), uint32(h - b)
	}
	if len(views) == 1 {
		v := views[0]
		w, h := inner(v, outW-2*gap, outH-2*gap)
		s.setViewPosition(v, float64(ux+gap), float64(uy+gap))
		v.setTiledContentSize(w, h)
		v.setSize(w, h)
		return
	}
	if len(views) >= 4 {
		minimums := make([]tileMinimum, len(views))
		for i, v := range views {
			minimums[i].width, minimums[i].height = v.minimumSize()
		}
		for i, tile := range minimumAwareGridTiles(
			area, minimums, gap, s.config.BorderSize) {
			v := views[i]
			w, h := inner(v, tile.width, tile.height)
			s.setViewPosition(v, float64(tile.x), float64(tile.y))
			v.setTiledContentSize(w, h)
			v.setSize(w, h)
		}
		return
	}

	availableW := outW - 3*gap
	if availableW < 2 {
		availableW = 2
	}
	ratio := s.tileMasterRatio
	if ratio < 0.2 {
		ratio = 0.2
	}
	if ratio > 0.8 {
		ratio = 0.8
	}
	masterW := int(float64(availableW) * ratio)
	if masterW < 1 {
		masterW = 1
	}
	if masterW >= availableW {
		masterW = availableW - 1
	}
	stackW := outW - masterW - 3*gap
	master := views[0]
	mw, mh := inner(master, masterW, outH-2*gap)
	s.setViewPosition(master, float64(ux+gap), float64(uy+gap))
	master.setTiledContentSize(mw, mh)
	master.setSize(mw, mh)
	count := len(views) - 1
	stackH := (outH - 2*gap - (count-1)*gap) / count
	for i := 1; i < len(views); i++ {
		v := views[i]
		w, h := inner(v, stackW, stackH)
		x := ux + masterW + 2*gap
		y := uy + gap + (i-1)*(stackH+gap)
		s.setViewPosition(v, float64(x), float64(y))
		v.setTiledContentSize(w, h)
		v.setSize(w, h)
	}
}

// balancedGridTiles keeps dense workspaces usable when a vertical master
// stack would make each client shorter than common application minimums. Each
// row spans the full usable width and row populations differ by at most one.
func balancedGridTiles(area usableBox, count, gap int) []usableBox {
	if count <= 0 || area.width <= 0 || area.height <= 0 {
		return nil
	}
	if gap < 0 {
		gap = 0
	}
	columns := 1
	for columns*columns < count {
		columns++
	}
	rows := (count + columns - 1) / columns
	return balancedGridTilesWithRows(area, count, gap, rows)
}

type tileMinimum struct {
	width, height int
}

func minimumAwareGridTiles(
	area usableBox, minimums []tileMinimum, gap, border int,
) []usableBox {
	count := len(minimums)
	preferred := balancedGridTiles(area, count, gap)
	if count == 0 {
		return preferred
	}
	preferredRows := distinctTileRows(preferred)
	bestDistance := count + 1
	var best []usableBox
	for rows := 1; rows <= count; rows++ {
		tiles := balancedGridTilesWithRows(area, count, gap, rows)
		fits := len(tiles) == count
		for i, tile := range tiles {
			contentW := tile.width - 2*border
			contentH := tile.height - 2*border
			if contentW < minimums[i].width || contentH < minimums[i].height {
				fits = false
				break
			}
		}
		if !fits {
			continue
		}
		distance := rows - preferredRows
		if distance < 0 {
			distance = -distance
		}
		if distance < bestDistance {
			bestDistance = distance
			best = tiles
		}
	}
	if best != nil {
		return best
	}
	return preferred
}

func distinctTileRows(tiles []usableBox) int {
	rows := 0
	lastY := 0
	for i, tile := range tiles {
		if i == 0 || tile.y != lastY {
			rows++
			lastY = tile.y
		}
	}
	return rows
}

func balancedGridTilesWithRows(area usableBox, count, gap, rows int) []usableBox {
	if count <= 0 || rows <= 0 || rows > count ||
		area.width <= 0 || area.height <= 0 {
		return nil
	}
	rowBase := count / rows
	rowExtra := count % rows

	availableH := area.height - (rows+1)*gap
	if availableH < rows {
		availableH = rows
	}
	baseH := availableH / rows
	extraH := availableH % rows

	tiles := make([]usableBox, 0, count)
	y := area.y + gap
	for row := 0; row < rows; row++ {
		rowCount := rowBase
		if row < rowExtra {
			rowCount++
		}
		rowH := baseH
		if row < extraH {
			rowH++
		}
		availableW := area.width - (rowCount+1)*gap
		if availableW < rowCount {
			availableW = rowCount
		}
		baseW := availableW / rowCount
		extraW := availableW % rowCount
		x := area.x + gap
		for column := 0; column < rowCount; column++ {
			width := baseW
			if column < extraW {
				width++
			}
			tiles = append(tiles, usableBox{x: x, y: y, width: width, height: rowH})
			x += width + gap
		}
		y += rowH + gap
	}
	return tiles
}

func (s *Server) cascadeFloating() {
	for i, v := range s.mappedViews() {
		if s.fullscreen == v {
			continue
		}
		s.placeFloatingView(v, i)
	}
}

func (s *Server) placeFloatingView(v *View, index int) {
	if v == nil || !v.Mapped || s.fullscreen == v {
		return
	}
	wasTiled := v.TileWidth > 0 && v.TileHeight > 0
	v.clearTiledContentSize()
	area := s.viewArea(v)
	if area.width <= 0 || area.height <= 0 {
		return
	}
	geometry := v.Floating
	newPlacement := !v.FloatingValid
	if newPlacement {
		current := v.geometry()
		width, height := current.Width, current.Height
		border := s.viewBorderSize(v)
		maxW := area.width - 2*border
		maxH := area.height - 2*border
		if wasTiled || width <= 1 || width > maxW {
			width = minInt(800, maxW)
		}
		if wasTiled || height <= 1 || height > maxH {
			height = minInt(600, maxH)
		}
		offset := (index % 6) * 32
		centerX := area.x + area.width/2
		centerY := area.y + area.height/2
		if parent := s.parentView(v); parent != nil {
			parentGeometry := parent.geometry()
			parentBorder := s.viewBorderSize(parent)
			if s.fullscreen == parent {
				parentBorder = 0
			}
			centerX = parent.RootTree.Node().X() + (parentGeometry.Width+2*parentBorder)/2
			centerY = parent.RootTree.Node().Y() + (parentGeometry.Height+2*parentBorder)/2
		}
		if v.AutoFloating {
			offset = 0
		}
		geometry = Geometry{
			X:     float64(centerX - (width+2*border)/2 + offset),
			Y:     float64(centerY - (height+2*border)/2 + offset),
			Width: uint32(width), Height: uint32(height),
		}
	}
	if newPlacement || v.AutoFloating || v.Dialog || v.Modal {
		minWidth, minHeight := v.minimumSize()
		geometry = clampFloatingGeometry(
			geometry, area, s.viewBorderSize(v), minWidth, minHeight)
	} else {
		geometry = clampFloatingMoveGeometry(
			geometry, area, s.viewBorderSize(v), floatingVisibleStrip)
	}
	s.setViewPositionImmediate(v, geometry.X, geometry.Y)
	v.setSize(geometry.Width, geometry.Height)
	v.Floating = geometry
	v.FloatingValid = true
	s.updateDecoration(v)
}

func (s *Server) rememberFloatingGeometry(v *View) {
	if v == nil || !s.isFloatingView(v) || s.fullscreen == v {
		return
	}
	geometry := v.geometry()
	if geometry.Width <= 0 || geometry.Height <= 0 {
		return
	}
	v.Floating = Geometry{
		X:     float64(v.RootTree.Node().X()),
		Y:     float64(v.RootTree.Node().Y()),
		Width: uint32(geometry.Width), Height: uint32(geometry.Height),
	}
	v.FloatingValid = true
}

func (s *Server) setFloatingPosition(v *View, x, y float64) {
	if v == nil {
		return
	}
	geometry := v.geometry()
	target := Geometry{X: x, Y: y,
		Width:  uint32(maxInt(1, geometry.Width)),
		Height: uint32(maxInt(1, geometry.Height))}
	if v.AutoFloating || v.Dialog || v.Modal {
		minWidth, minHeight := v.minimumSize()
		target = clampFloatingGeometry(
			target, s.viewArea(v), s.viewBorderSize(v), minWidth, minHeight)
	} else {
		target = clampFloatingMoveGeometry(
			target, s.viewArea(v), s.viewBorderSize(v), floatingVisibleStrip)
	}
	s.setViewPositionImmediate(v, target.X, target.Y)
	v.Floating = target
	v.FloatingValid = true
}

// clampFloatingMoveGeometry allows regular floating windows to cross the
// left, right, and bottom output edges while preserving a reachable strip.
// The top edge remains inside the usable area so a client-side title bar (or
// the upper resize edge) cannot be lost behind a panel or outside the output.
// Unlike initial placement, moving never changes the client's size.
func clampFloatingMoveGeometry(
	geometry Geometry, area usableBox, border, visibleStrip int,
) Geometry {
	if area.width <= 0 || area.height <= 0 {
		return geometry
	}
	if border < 0 {
		border = 0
	}
	if visibleStrip < 1 {
		visibleStrip = 1
	}
	outerWidth := maxInt(1, int(geometry.Width)+2*border)
	outerHeight := maxInt(1, int(geometry.Height)+2*border)
	visibleX := minInt(visibleStrip, outerWidth)
	visibleY := minInt(visibleStrip, outerHeight)
	minX := float64(area.x - outerWidth + visibleX)
	maxX := float64(area.x + area.width - visibleX)
	minY := float64(area.y)
	maxY := float64(area.y + area.height - visibleY)
	geometry.X = clampFloat(geometry.X, minX, maxX)
	geometry.Y = clampFloat(geometry.Y, minY, maxY)
	return geometry
}

func clampFloatingGeometry(
	geometry Geometry, area usableBox, border int, minWidth, minHeight int,
) Geometry {
	if border < 0 {
		border = 0
	}
	maxW := maxInt(1, area.width-2*border)
	maxH := maxInt(1, area.height-2*border)
	width := int(geometry.Width)
	height := int(geometry.Height)
	if width < maxInt(1, minWidth) {
		width = maxInt(1, minWidth)
	}
	if height < maxInt(1, minHeight) {
		height = maxInt(1, minHeight)
	}
	width = minInt(width, maxW)
	height = minInt(height, maxH)
	minX := float64(area.x)
	minY := float64(area.y)
	maxX := float64(area.x + area.width - width - 2*border)
	maxY := float64(area.y + area.height - height - 2*border)
	geometry.X = clampFloat(geometry.X, minX, maxX)
	geometry.Y = clampFloat(geometry.Y, minY, maxY)
	geometry.Width = uint32(width)
	geometry.Height = uint32(height)
	return geometry
}

func clampFloat(value, low, high float64) float64 {
	if high < low {
		high = low
	}
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (s *Server) toggleFullscreen() {
	v := s.focusedView()
	if v == nil {
		return
	}
	if s.fullscreen == v {
		s.setViewPresentation(v, false, s.fullscreenMode)
		return
	}
	s.setViewFullscreen(v, true)
}

func (s *Server) setViewFullscreen(v *View, enabled bool) {
	s.setViewPresentation(v, enabled, presentationFullscreen)
}

func (s *Server) handleViewMaximizeRequest(v *View, requested bool) {
	if v == nil || !v.Mapped {
		return
	}
	if s.fullscreen == v && s.fullscreenMode == presentationMaximizedFullscreen {
		s.setViewPresentation(v, false, presentationMaximizedFullscreen)
		return
	}
	if requested {
		s.setViewPresentation(v, true, presentationMaximizedFullscreen)
		return
	}
	if s.fullscreen == v {
		setClientPresentationState(v, s.fullscreenMode)
	} else {
		setClientPresentationState(v, presentationNone)
	}
}

func (s *Server) setViewPresentation(
	v *View, enabled bool, mode presentationMode) {
	if v == nil || !v.Mapped {
		return
	}

	if !enabled {
		if s.fullscreen != v || s.fullscreenMode != mode {
			// A maximize and a fullscreen request are independent. Refusing one
			// must not clear the other state if it still owns the presentation.
			if s.fullscreen == v {
				setClientPresentationState(v, s.fullscreenMode)
			} else {
				setClientPresentationState(v, presentationNone)
			}
			return
		}
		s.fullscreen = nil
		s.fullscreenMode = presentationNone
		setClientPresentationState(v, presentationNone)
		if s.isFloatingView(v) {
			s.setViewPosition(v, v.Saved.X, v.Saved.Y)
			v.setSize(v.Saved.Width, v.Saved.Height)
		}
		s.arrange()
		s.updateAllDecorations()
		s.emitIPCEvent("fullscreen_changed", s.ipcWindow(v))
		return
	}

	if s.fullscreen == v {
		s.fullscreenMode = mode
		setClientPresentationState(v, mode)
		return
	}

	if s.fullscreen != nil {
		old := s.fullscreen
		setClientPresentationState(old, presentationNone)
		if s.isFloatingView(old) {
			s.setViewPosition(old, old.Saved.X, old.Saved.Y)
			old.setSize(old.Saved.Width, old.Saved.Height)
		}
	}

	g := v.geometry()
	v.Saved = Geometry{
		X:      float64(v.RootTree.Node().X()),
		Y:      float64(v.RootTree.Node().Y()),
		Width:  uint32(g.Width),
		Height: uint32(g.Height),
	}
	s.fullscreen = v
	s.fullscreenMode = mode
	setClientPresentationState(v, mode)
	s.arrange()
	s.updateAllDecorations()
	s.emitIPCEvent("fullscreen_changed", s.ipcWindow(v))
}

var _ wlroots.Edges
