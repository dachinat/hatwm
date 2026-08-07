package compositor

import "github.com/swaywm/go-wlroots/wlroots"

const floatingVisibleStrip = 48

func (s *Server) arrange() {
	s.arrangeLayers()
}

func (s *Server) arrangeViewsIn(output *OutputState, area usableBox) {
	if output == nil {
		return
	}
	for _, v := range s.views {
		s.syncViewSceneLayer(v)
		v.RootTree.Node().SetEnabled(s.viewVisible(v))
	}
	// Reparenting appends a node to its new layer. Restore the focused view to
	// the top of that layer so a layout-mode change preserves focus ordering.
	if focused := s.focusedView(); focused != nil {
		focused.RootTree.Node().RaiseToTop()
	}
	views := s.mappedViewsForOutput(output)
	if len(views) == 0 {
		return
	}
	outW, outH := area.width, area.height
	ux, uy := area.x, area.y
	gap := s.config.Gaps
	if gap < 0 {
		gap = 0
	}

	for _, v := range views {
		v.RootTree.Node().SetEnabled(true)
	}
	if output.Fullscreen != nil && output.Fullscreen.Mapped {
		for _, v := range views {
			v.RootTree.Node().SetEnabled(v == output.Fullscreen)
		}
		fullscreenArea := s.viewArea(output.Fullscreen)
		s.setViewPosition(output.Fullscreen,
			float64(fullscreenArea.x), float64(fullscreenArea.y))
		output.Fullscreen.setTiledContentSize(
			uint32(fullscreenArea.width), uint32(fullscreenArea.height))
		output.Fullscreen.setSize(
			uint32(fullscreenArea.width), uint32(fullscreenArea.height))
		s.updateDecoration(output.Fullscreen)
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
	layout := s.tileLayoutForOutput(output)
	if len(views) < 4 {
		inheritLegacyTileRatios(layout)
		layout.Grid = tileGridState{Count: len(views)}
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
		preferred := minimumAwareGridTiles(area, minimums, gap, s.config.BorderSize)
		rowCounts := tileGridRowCounts(preferred)
		ensureTileGridState(&layout.Grid, len(views), rowCounts,
			layout.MasterRatio, layout.StackRatio)
		for i, tile := range weightedGridTiles(area, rowCounts, gap,
			s.config.BorderSize, minimums, layout.Grid.RowWeights,
			layout.Grid.ColumnWeights) {
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
	ratio := layout.MasterRatio
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
	stackAvailableH := outH - 2*gap - (count-1)*gap
	stackHeights := []int{stackAvailableH}
	if count == 2 {
		_, firstMinHeight := views[1].minimumSize()
		_, secondMinHeight := views[2].minimumSize()
		stackHeights = splitStackHeights(stackAvailableH, layout.StackRatio,
			firstMinHeightWithBorder(firstMinHeight, s.viewBorderSize(views[1])),
			firstMinHeightWithBorder(secondMinHeight, s.viewBorderSize(views[2])))
	}
	stackY := uy + gap
	for i := 1; i < len(views); i++ {
		v := views[i]
		stackH := stackHeights[i-1]
		w, h := inner(v, stackW, stackH)
		x := ux + masterW + 2*gap
		s.setViewPosition(v, float64(x), float64(stackY))
		v.setTiledContentSize(w, h)
		v.setSize(w, h)
		stackY += stackH + gap
	}
}

func tileGridRowCounts(tiles []usableBox) []int {
	var counts []int
	lastY := 0
	for i, tile := range tiles {
		if i == 0 || tile.y != lastY {
			counts = append(counts, 0)
			lastY = tile.y
		}
		counts[len(counts)-1]++
	}
	return counts
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func ensureTileGridState(state *tileGridState, count int, rowCounts []int,
	masterRatio, stackRatio float64,
) {
	if state == nil {
		return
	}
	if tileGridStateMatches(*state, count, rowCounts) {
		return
	}

	previous := *state
	next := tileGridState{
		Count:     count,
		RowCounts: append([]int(nil), rowCounts...),
	}
	if previous.Count >= 4 && tileGridStateShapeValid(previous) {
		next.RowWeights = migrateTileWeights(previous.RowWeights, len(rowCounts))
		next.ColumnWeights = make([][]float64, len(rowCounts))
		for row, columns := range rowCounts {
			var source []float64
			if row < len(previous.ColumnWeights) {
				source = previous.ColumnWeights[row]
			} else if len(previous.ColumnWeights) > 0 {
				source = previous.ColumnWeights[len(previous.ColumnWeights)-1]
			}
			next.ColumnWeights[row] = migrateTileWeights(source, columns)
		}
	} else {
		next.RowWeights = seededTileWeights(len(rowCounts), stackRatio)
		next.ColumnWeights = make([][]float64, len(rowCounts))
		for row, columns := range rowCounts {
			next.ColumnWeights[row] = seededTileWeights(columns, masterRatio)
		}
	}
	*state = next
}

func tileGridStateMatches(state tileGridState, count int, rowCounts []int) bool {
	return state.Count == count && sameInts(state.RowCounts, rowCounts) &&
		tileGridStateShapeValid(state)
}

func tileGridStateShapeValid(state tileGridState) bool {
	if state.Count < 0 || len(state.RowCounts) != len(state.RowWeights) ||
		len(state.RowCounts) != len(state.ColumnWeights) {
		return false
	}
	total := 0
	for row, columns := range state.RowCounts {
		if columns <= 0 || len(state.ColumnWeights[row]) != columns ||
			state.RowWeights[row] <= 0 {
			return false
		}
		for _, weight := range state.ColumnWeights[row] {
			if weight <= 0 {
				return false
			}
		}
		total += columns
	}
	return total == state.Count
}

// seededTileWeights translates the legacy two-way master/stack ratios into
// grid weights. A 50/50 split becomes [1, 1], while e.g. 70/30 becomes
// [1.4, 0.6]. Extra rows/columns start at weight 1, so adding a tile keeps the
// existing split ratio instead of resetting every boundary to 50/50.
func seededTileWeights(count int, ratio float64) []float64 {
	if count <= 0 {
		return nil
	}
	weights := make([]float64, count)
	for i := range weights {
		weights[i] = 1
	}
	if count < 2 {
		return weights
	}
	ratio = clampTileRatio(ratio, 0.1, 0.9, 0.5)
	weights[0] = 2 * ratio
	weights[1] = 2 * (1 - ratio)
	return weights
}

// migrateTileWeights retains all existing boundary proportions and only gives
// newly introduced rows/columns the old average weight. This is what prevents
// a 4->5, 5->6, ... layout change from flattening user-resized tiles.
func migrateTileWeights(source []float64, count int) []float64 {
	if count <= 0 {
		return nil
	}
	average, total, valid := 1.0, 0.0, 0
	for _, weight := range source {
		if weight > 0 {
			total += weight
			valid++
		}
	}
	if valid > 0 {
		average = total / float64(valid)
	}
	weights := make([]float64, count)
	for i := range weights {
		weights[i] = average
		if i < len(source) && source[i] > 0 {
			weights[i] = source[i]
		}
	}
	return weights
}

func inheritLegacyTileRatios(layout *tileLayoutState) {
	if layout == nil || layout.Grid.Count < 4 || !tileGridStateShapeValid(layout.Grid) {
		return
	}
	if ratio, ok := firstWeightShare(layout.Grid.RowWeights); ok {
		layout.StackRatio = clampTileRatio(ratio, 0.1, 0.9, layout.StackRatio)
	}
	for _, weights := range layout.Grid.ColumnWeights {
		if ratio, ok := firstWeightShare(weights); ok {
			layout.MasterRatio = clampTileRatio(ratio, 0.2, 0.8, layout.MasterRatio)
			break
		}
	}
}

func firstWeightShare(weights []float64) (float64, bool) {
	if len(weights) < 2 || weights[0] <= 0 {
		return 0, false
	}
	total := 0.0
	for _, weight := range weights {
		if weight > 0 {
			total += weight
		}
	}
	if total <= 0 {
		return 0, false
	}
	return weights[0] / total, true
}

func clampTileRatio(value, minimum, maximum, fallback float64) float64 {
	if value <= 0 {
		value = fallback
	}
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func weightedGridTiles(area usableBox, rowCounts []int, gap, border int,
	minimums []tileMinimum, rowWeights []float64, columnWeights [][]float64,
) []usableBox {
	if len(rowCounts) == 0 || area.width <= 0 || area.height <= 0 {
		return nil
	}
	if gap < 0 {
		gap = 0
	}
	if border < 0 {
		border = 0
	}
	availableH := maxInt(len(rowCounts), area.height-(len(rowCounts)+1)*gap)
	rowMinimums := make([]int, len(rowCounts))
	viewIndex := 0
	for row, columns := range rowCounts {
		for column := 0; column < columns && viewIndex < len(minimums); column++ {
			rowMinimums[row] = maxInt(rowMinimums[row], minimums[viewIndex].height+2*border)
			viewIndex++
		}
	}
	rowHeights := weightedTileSizes(availableH, rowWeights, rowMinimums)
	tiles := make([]usableBox, 0, len(minimums))
	y := area.y + gap
	viewIndex = 0
	for row, columns := range rowCounts {
		availableW := maxInt(columns, area.width-(columns+1)*gap)
		columnMinimums := make([]int, columns)
		for column := 0; column < columns && viewIndex+column < len(minimums); column++ {
			columnMinimums[column] = minimums[viewIndex+column].width + 2*border
		}
		weights := []float64(nil)
		if row < len(columnWeights) {
			weights = columnWeights[row]
		}
		widths := weightedTileSizes(availableW, weights, columnMinimums)
		x := area.x + gap
		for column := 0; column < columns; column++ {
			tiles = append(tiles, usableBox{x: x, y: y,
				width: widths[column], height: rowHeights[row]})
			x += widths[column] + gap
		}
		viewIndex += columns
		y += rowHeights[row] + gap
	}
	return tiles
}

func weightedTileSizes(total int, weights []float64, minimums []int) []int {
	count := len(minimums)
	if count == 0 {
		return nil
	}
	if total < count {
		total = count
	}
	minimumTotal := 0
	for i := range minimums {
		minimums[i] = maxInt(1, minimums[i])
		minimumTotal += minimums[i]
	}
	if minimumTotal > total {
		minimumTotal = count
		for i := range minimums {
			minimums[i] = 1
		}
	}
	result := append([]int(nil), minimums...)
	remaining := total - minimumTotal
	weightTotal := 0.0
	for i := 0; i < count; i++ {
		weight := 1.0
		if i < len(weights) && weights[i] > 0 {
			weight = weights[i]
		}
		weightTotal += weight
	}
	allocated := 0
	for i := 0; i < count; i++ {
		share := remaining - allocated
		if i < count-1 {
			weight := 1.0
			if i < len(weights) && weights[i] > 0 {
				weight = weights[i]
			}
			share = int(float64(remaining) * weight / weightTotal)
			allocated += share
		}
		result[i] += share
	}
	return result
}

func firstMinHeightWithBorder(height, border int) int {
	if border < 0 {
		border = 0
	}
	return maxInt(1, height+2*border)
}

func splitStackHeights(available int, ratio float64, firstMin, secondMin int) []int {
	if available < 2 {
		return []int{maxInt(1, available), 1}
	}
	firstMin = maxInt(1, minInt(firstMin, available-1))
	secondMin = maxInt(1, minInt(secondMin, available-1))
	if firstMin+secondMin > available {
		firstMin, secondMin = 1, 1
	}
	first := int(float64(available) * ratio)
	first = maxInt(firstMin, minInt(first, available-secondMin))
	return []int{first, available - first}
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
	for _, output := range s.outputs {
		for i, v := range s.mappedViewsForOutput(output) {
			if output.Fullscreen == v {
				continue
			}
			s.placeFloatingView(v, i)
		}
	}
}

func (s *Server) placeFloatingView(v *View, index int) {
	if v == nil || !v.Mapped || s.viewFullscreen(v) {
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
			if s.viewFullscreen(parent) {
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
	if v == nil || !s.isFloatingView(v) || s.viewFullscreen(v) {
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
	output := s.ensureViewOutput(v)
	if output.Fullscreen == v {
		s.setViewPresentation(v, false, output.FullscreenMode)
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
	output := s.ensureViewOutput(v)
	if output.Fullscreen == v && output.FullscreenMode == presentationMaximizedFullscreen {
		s.setViewPresentation(v, false, presentationMaximizedFullscreen)
		return
	}
	if requested {
		s.setViewPresentation(v, true, presentationMaximizedFullscreen)
		return
	}
	if output.Fullscreen == v {
		setClientPresentationState(v, output.FullscreenMode)
	} else {
		setClientPresentationState(v, presentationNone)
	}
}

func (s *Server) setViewPresentation(
	v *View, enabled bool, mode presentationMode) {
	if v == nil || !v.Mapped {
		return
	}
	output := s.ensureViewOutput(v)

	if !enabled {
		if output.Fullscreen != v || output.FullscreenMode != mode {
			// A maximize and a fullscreen request are independent. Refusing one
			// must not clear the other state if it still owns the presentation.
			if output.Fullscreen == v {
				setClientPresentationState(v, output.FullscreenMode)
			} else {
				setClientPresentationState(v, presentationNone)
			}
			return
		}
		output.Fullscreen = nil
		output.FullscreenMode = presentationNone
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

	if output.Fullscreen == v {
		output.FullscreenMode = mode
		setClientPresentationState(v, mode)
		return
	}

	if output.Fullscreen != nil {
		old := output.Fullscreen
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
	output.Fullscreen = v
	output.FullscreenMode = mode
	setClientPresentationState(v, mode)
	s.arrange()
	s.updateAllDecorations()
	s.emitIPCEvent("fullscreen_changed", s.ipcWindow(v))
}

var _ wlroots.Edges
