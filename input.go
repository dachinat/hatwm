package main

/*
#cgo pkg-config: wlroots-0.18 xkbcommon
#cgo CFLAGS: -D_GNU_SOURCE -DWLR_USE_UNSTABLE
#include <stdlib.h>
#include "keyboard_layout.h"
*/
import "C"

import (
	"fmt"
	"log/slog"
	"math"
	"os/exec"
	"strings"
	"unsafe"

	"github.com/swaywm/go-wlroots/wlroots"
	"github.com/swaywm/go-wlroots/xkb"
)

func (s *Server) handleNewInput(dev wlroots.InputDevice) {
	switch dev.Type() {
	case wlroots.InputDeviceTypePointer:
		s.cursor.AttachInputDevice(dev)
	case wlroots.InputDeviceTypeKeyboard:
		s.handleNewKeyboard(dev)
	}
	caps := wlroots.SeatCapabilityPointer
	if len(s.keyboards) > 0 {
		caps |= wlroots.SeatCapabilityKeyboard
	}
	s.seat.SetCapabilities(caps)
}

func (s *Server) handleNewKeyboard(dev wlroots.InputDevice) {
	kb := dev.Keyboard()
	if !s.configureKeyboardLayouts(kb) {
		// Keep the binding's default keymap as a safe fallback if a configured
		// XKB layout name is unavailable on the system.
		ctx := xkb.NewContext(xkb.KeySymFlagNoFlags)
		keymap := ctx.KeyMap()
		kb.SetKeymap(keymap)
		keymap.Destroy()
		ctx.Destroy()
	}
	kb.SetRepeatInfo(25, 600)
	kb.OnModifiers(func(k wlroots.Keyboard) {
		s.keyboardModifiers = k.Modifiers()
		s.seat.SetKeyboard(dev)
		s.seat.NotifyKeyboardModifiers(k)
	})
	kb.OnKey(s.handleKey)
	s.keyboards = append(s.keyboards, &Keyboard{Device: dev})
	s.seat.SetKeyboard(dev)
}

func (s *Server) handleKey(kb wlroots.Keyboard, time uint32, keyCode uint32, _ bool, state wlroots.KeyState) {
	s.notifyInputActivity()
	// While locked, every key belongs to the lock client. In particular,
	// compositor shortcuts such as exit, exec and workspace switching must not
	// remain available behind swaylock.
	if s.sessionLocked {
		s.seat.SetKeyboard(kb.Base())
		s.seat.NotifyKeyboardKey(time, keyCode, state)
		return
	}

	handled := false
	if s.shortcutsInhibited() {
		s.seat.SetKeyboard(kb.Base())
		s.seat.NotifyKeyboardKey(time, keyCode, state)
		return
	}
	if state == wlroots.KeyStatePressed {
		syms := kb.XKBState().Syms(xkb.KeyCode(keyCode + 8))
		baseSym := xkb.KeySym(C.hatwm_keyboard_base_keysym(
			(*C.struct_wlr_keyboard)(keyboardPointer(kb)),
			C.uint32_t(keyCode+8),
		))
		mods := kb.Modifiers()
		s.keyboardModifiers = mods
		for _, b := range s.config.KeyBindings {
			if b.Modifier != mods {
				continue
			}

			matched := b.Sym == baseSym
			if !matched {
				for _, sym := range syms {
					if b.Sym == sym {
						matched = true
						break
					}
				}
			}
			if matched {
				handled = s.executeAction(b.Action, b.Arg)
				if handled {
					break
				}
			}
		}
	}
	if !handled {
		s.seat.SetKeyboard(kb.Base())
		s.seat.NotifyKeyboardKey(time, keyCode, state)
	}
}

func (s *Server) executeAction(action, arg string) bool {
	switch action {
	case "exit":
		s.running = false
		s.display.Terminate()
	case "exec":
		if arg != "" {
			cmd := exec.Command("/bin/sh", "-c", arg)
			_ = cmd.Start()
			if cmd.Process != nil {
				go func() { _ = cmd.Wait() }()
			}
		}
	case "close":
		if v := s.focusedView(); v != nil {
			v.close()
		}
	case "toggle_keyboard_layout":
		return s.toggleKeyboardLayout()
	case "workspace":
		return s.switchWorkspaceArg(arg)
	case "move_to_workspace":
		return s.moveFocusedToWorkspaceArg(arg)
	case "toggle_tiling":
		if s.config.Tiling {
			s.config.Tiling = false
			s.cascadeFloating()
		} else {
			for _, v := range s.mappedViews() {
				s.rememberFloatingGeometry(v)
			}
			s.config.Tiling = true
		}
		s.arrange()
		s.emitIPCEvent("layout_changed", s.ipcState())
	case "cycle_focus":
		s.cycleFocus()
	case "toggle_fullscreen":
		s.toggleFullscreen()
	case "move_left":
		return s.moveFocused("left")
	case "move_right":
		return s.moveFocused("right")
	case "move_up":
		return s.moveFocused("up")
	case "move_down":
		return s.moveFocused("down")
	case "volume_up":
		s.changeVolume(1)
	case "volume_down":
		s.changeVolume(-1)
	case "volume_mute":
		s.toggleVolumeMute()
	default:
		return false
	}
	return true
}

func (s *Server) changeVolume(direction int) {
	step := s.config.VolumeStep
	if step <= 0 {
		step = 5
	}

	var command string
	switch {
	case commandExists("wpctl"):
		suffix := "%+"
		if direction < 0 {
			suffix = "%-"
		}
		command = fmt.Sprintf("wpctl set-volume -l 1.0 @DEFAULT_AUDIO_SINK@ %d%s", step, suffix)
	case commandExists("pactl"):
		sign := "+"
		if direction < 0 {
			sign = "-"
		}
		command = fmt.Sprintf("pactl set-sink-volume @DEFAULT_SINK@ %s%d%%", sign, step)
	case commandExists("amixer"):
		suffix := "%+"
		if direction < 0 {
			suffix = "%-"
		}
		command = fmt.Sprintf("amixer -q sset Master %d%s", step, suffix)
	default:
		slog.Warn("cannot change volume: install wpctl, pactl, or amixer")
		return
	}
	s.runMediaCommand(command)
}

func (s *Server) toggleVolumeMute() {
	var command string
	switch {
	case commandExists("wpctl"):
		command = "wpctl set-mute @DEFAULT_AUDIO_SINK@ toggle"
	case commandExists("pactl"):
		command = "pactl set-sink-mute @DEFAULT_SINK@ toggle"
	case commandExists("amixer"):
		command = "amixer -q sset Master toggle"
	default:
		slog.Warn("cannot mute volume: install wpctl, pactl, or amixer")
		return
	}
	s.runMediaCommand(command)
}

func (s *Server) runMediaCommand(command string) {
	cmd := exec.Command("/bin/sh", "-c", command)
	if err := cmd.Start(); err != nil {
		slog.Warn("failed to run media command", "command", command, "error", err)
		return
	}
	go func() {
		if err := cmd.Wait(); err != nil {
			slog.Warn("media command failed", "command", command, "error", err)
		}
	}()
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func (s *Server) handleCursorMotion(dev wlroots.InputDevice, time uint32, dx, dy float64) {
	s.notifyInputActivity()
	if !s.handleProtocolRelativeMotion(dev, time, dx, dy) {
		s.cursor.Move(dev, dx, dy)
	}
	s.processCursorMotion(time)
}
func (s *Server) handleCursorMotionAbsolute(dev wlroots.InputDevice, time uint32, x, y float64) {
	s.notifyInputActivity()
	if s.pointerLocked() {
		return
	}
	s.cursor.WarpAbsolute(dev, x, y)
	s.processCursorMotion(time)
}

func (s *Server) processCursorMotion(time uint32) {
	if s.cursorMode == CursorMove {
		s.processCursorMove()
		return
	}
	if s.cursorMode == CursorResize {
		s.processCursorResize()
		return
	}
	if s.cursorMode == CursorTilingMove {
		s.processTilingDrag()
		return
	}
	if s.cursorMode == CursorTilingResize {
		s.processTilingResize()
		return
	}
	view, surface, sx, sy := s.viewAt(s.cursor.X(), s.cursor.Y())
	if surface != nil {
		// XWayland override-redirect surfaces are transient UI such as menus
		// and tooltips. They still need pointer input, but moving onto one must
		// not transfer keyboard focus away from the owning application: many
		// clients (including Electron) dismiss the popup when that happens.
		if s.config.FocusFollowsMouse && view != nil && view.Managed &&
			s.focusedView() != view {
			s.focusView(view, surface)
		}

		s.seat.NotifyPointerEnter(*surface, sx, sy)
		s.updateProtocolPointerFocus(surface, sx, sy)
		s.seat.NotifyPointerMotion(time, sx, sy)
	} else {
		s.seat.ClearPointerFocus()
		s.updateProtocolPointerFocus(nil, 0, 0)
		s.setCursorImage("default")
	}
}
func (s *Server) processCursorMove() {
	if s.grabbedView != nil {
		s.setFloatingPosition(
			s.grabbedView, s.cursor.X()-s.grabX, s.cursor.Y()-s.grabY)
	}
}
func (s *Server) processCursorResize() {
	v := s.grabbedView
	if v == nil {
		return
	}
	minWidth, minHeight, maxWidth, maxHeight := v.sizeConstraints()
	target := resizeFloatingGeometry(Geometry{
		X: float64(s.grabBox.X), Y: float64(s.grabBox.Y),
		Width: uint32(s.grabBox.Width), Height: uint32(s.grabBox.Height),
	}, s.cursor.X()-s.grabX, s.cursor.Y()-s.grabY,
		s.resizeEdges, s.usable, s.config.BorderSize,
		minWidth, minHeight, maxWidth, maxHeight)
	v.setSize(target.Width, target.Height)
	s.setViewPositionImmediate(v, target.X, target.Y)
	v.Floating = target
	v.FloatingValid = true
}

func resizeFloatingGeometry(
	original Geometry, dx, dy float64, edges wlroots.Edges,
	area usableBox, border, minWidth, minHeight, maxWidth, maxHeight int,
) Geometry {
	if border < 0 {
		border = 0
	}
	originalX, originalY := int(math.Round(original.X)), int(math.Round(original.Y))
	originalWidth, originalHeight := int(original.Width), int(original.Height)
	width, height := originalWidth, originalHeight
	if edges&wlroots.EdgeLeft != 0 {
		width -= int(math.Round(dx))
	} else if edges&wlroots.EdgeRight != 0 {
		width += int(math.Round(dx))
	}
	if edges&wlroots.EdgeTop != 0 {
		height -= int(math.Round(dy))
	} else if edges&wlroots.EdgeBottom != 0 {
		height += int(math.Round(dy))
	}

	widthLimit := maxInt(1, area.width-2*border)
	heightLimit := maxInt(1, area.height-2*border)
	if edges&wlroots.EdgeLeft != 0 {
		widthLimit = minInt(widthLimit, originalX+originalWidth-area.x)
	} else if edges&wlroots.EdgeRight != 0 {
		widthLimit = minInt(widthLimit, area.x+area.width-2*border-originalX)
	}
	if edges&wlroots.EdgeTop != 0 {
		heightLimit = minInt(heightLimit, originalY+originalHeight-area.y)
	} else if edges&wlroots.EdgeBottom != 0 {
		heightLimit = minInt(heightLimit, area.y+area.height-2*border-originalY)
	}
	if maxWidth > 0 {
		widthLimit = minInt(widthLimit, maxWidth)
	}
	if maxHeight > 0 {
		heightLimit = minInt(heightLimit, maxHeight)
	}
	widthLimit = maxInt(1, widthLimit)
	heightLimit = maxInt(1, heightLimit)
	width = maxInt(1, minInt(width, widthLimit))
	height = maxInt(1, minInt(height, heightLimit))
	width = maxInt(minInt(maxInt(1, minWidth), widthLimit), width)
	height = maxInt(minInt(maxInt(1, minHeight), heightLimit), height)

	x, y := originalX, originalY
	if edges&wlroots.EdgeLeft != 0 {
		x = originalX + originalWidth - width
	}
	if edges&wlroots.EdgeTop != 0 {
		y = originalY + originalHeight - height
	}
	return Geometry{X: float64(x), Y: float64(y),
		Width: uint32(width), Height: uint32(height)}
}

func resizeCursorName(edges wlroots.Edges) string {
	switch {
	case edges&(wlroots.EdgeLeft|wlroots.EdgeTop) == wlroots.EdgeLeft|wlroots.EdgeTop,
		edges&(wlroots.EdgeRight|wlroots.EdgeBottom) == wlroots.EdgeRight|wlroots.EdgeBottom:
		return "nwse-resize"
	case edges&(wlroots.EdgeRight|wlroots.EdgeTop) == wlroots.EdgeRight|wlroots.EdgeTop,
		edges&(wlroots.EdgeLeft|wlroots.EdgeBottom) == wlroots.EdgeLeft|wlroots.EdgeBottom:
		return "nesw-resize"
	case edges&(wlroots.EdgeLeft|wlroots.EdgeRight) != 0:
		return "ew-resize"
	case edges&(wlroots.EdgeTop|wlroots.EdgeBottom) != 0:
		return "ns-resize"
	default:
		return "default"
	}
}
func (s *Server) handleSetCursorRequest(client wlroots.SeatClient, surface wlroots.Surface, _ uint32, hx, hy int32) {
	if s.cursorMode == CursorPassthrough && s.seat.PointerState().FocusedClient() == client {
		s.cursor.SetSurface(surface, hx, hy)
	}
}
func (s *Server) handleCursorButton(_ wlroots.InputDevice, time uint32, button uint32, state wlroots.ButtonState) {
	s.notifyInputActivity()
	const (
		buttonLeft  = 0x110 // BTN_LEFT from linux/input-event-codes.h
		buttonRight = 0x111 // BTN_RIGHT from linux/input-event-codes.h
	)

	if state == wlroots.ButtonStatePressed {
		s.cursorButtonCount++
		v, surface, _, _ := s.viewAt(s.cursor.X(), s.cursor.Y())
		if v != nil {
			// Keep keyboard focus on the owner of an unmanaged XWayland
			// popup. The button event is still delivered to the popup below.
			if v.Managed {
				s.focusView(v, surface)
			}

			// Mod4 + left drag is compositor-owned movement. In tiling mode,
			// crossing another tile swaps their layout positions. In floating
			// mode, the view follows the pointer directly.
			if button == buttonLeft && s.keyboardModifiers&wlroots.KeyboardModifierLogo != 0 && s.fullscreen != v {
				s.grabbedView = v
				s.grabOwnsButton = true
				s.grabX = s.cursor.X() - float64(v.RootTree.Node().X())
				s.grabY = s.cursor.Y() - float64(v.RootTree.Node().Y())
				if !s.isFloatingView(v) {
					s.cursorMode = CursorTilingMove
				} else {
					s.cursorMode = CursorMove
				}
				s.beginCursorOverride("move")
				return // do not send the compositor-owned press to the client
			}

			// Mod4 + right drag is compositor-owned resizing. Tiled layouts
			// adjust the master/stack split; floating views resize from the
			// corner nearest the initial pointer position.
			if button == buttonRight && s.keyboardModifiers&wlroots.KeyboardModifierLogo != 0 && s.fullscreen != v {
				s.grabbedView = v
				s.grabOwnsButton = true
				if !s.isFloatingView(v) {
					s.cursorMode = CursorTilingResize
					s.grabX = s.cursor.X()
					s.grabMasterRatio = s.tileMasterRatio
					s.beginCursorOverride("ew-resize")
				} else {
					s.beginPointerResize(v)
				}
				return // do not send the compositor-owned press to the client
			}
		} else if surface != nil {
			if ls := s.layerSurfaceForSurface(*surface); ls != nil {
				s.focusLayerSurface(ls)
			}
		}
	}

	// A compositor-owned Mod4 drag must not leak its release to the client
	// without the matching press.
	if state == wlroots.ButtonStateReleased &&
		(button == buttonLeft || button == buttonRight) &&
		s.cursorMode != CursorPassthrough &&
		s.grabbedView != nil && s.grabOwnsButton {
		if s.cursorButtonCount > 0 {
			s.cursorButtonCount--
		}
		if s.cursorButtonCount == 0 {
			s.cursorMode = CursorPassthrough
			s.grabbedView = nil
			s.grabOwnsButton = false
			s.resizeEdges = 0
			s.endCursorOverride()
			s.arrange()
			s.emitIPCEvent("layout_changed", s.ipcState())
		}
		return
	}

	s.seat.NotifyPointerButton(time, button, state)
	if state == wlroots.ButtonStateReleased {
		finishedGrab := s.cursorMode != CursorPassthrough && s.grabbedView != nil
		if s.cursorButtonCount > 0 {
			s.cursorButtonCount--
		}
		if s.cursorButtonCount == 0 {
			s.cursorMode = CursorPassthrough
			s.grabbedView = nil
			s.grabOwnsButton = false
			s.resizeEdges = 0
			s.endCursorOverride()
			if finishedGrab {
				s.arrange()
				s.emitIPCEvent("layout_changed", s.ipcState())
			}
		}
	}
}

func (s *Server) handleCursorAxis(_ wlroots.InputDevice, time uint32, source wlroots.AxisSource, orientation wlroots.AxisOrientation, delta float64, discrete int32) {
	s.notifyInputActivity()
	s.seat.NotifyPointerAxis(time, orientation, delta, discrete, source, wlroots.RelativeDirectionIdentical)
}
func (s *Server) handleCursorFrame() { s.seat.NotifyPointerFrame() }

func (s *Server) moveFocused(direction string) bool {
	v := s.focusedView()
	if v == nil || s.fullscreen == v {
		return true
	}

	switch direction {
	case "left", "right", "up", "down":
	default:
		return false
	}

	if !s.isFloatingView(v) {
		target := s.directionalNeighbor(v, direction)
		if target == nil {
			return true
		}
		s.swapViewOrder(v, target)
		s.arrange()
		s.emitIPCEvent("layout_changed", s.ipcState())
		return true
	}

	step := float64(s.config.MoveStep)
	x := float64(v.RootTree.Node().X())
	y := float64(v.RootTree.Node().Y())
	switch direction {
	case "left":
		x -= step
	case "right":
		x += step
	case "up":
		y -= step
	case "down":
		y += step
	}
	s.setFloatingPosition(v, x, y)
	s.emitIPCEvent("layout_changed", s.ipcState())
	return true
}

func (s *Server) directionalNeighbor(v *View, direction string) *View {
	if v == nil {
		return nil
	}
	vx, vy := viewCenter(v)
	var best *View
	bestScore := 0.0

	for _, candidate := range s.mappedTiledViews() {
		if candidate == v {
			continue
		}
		cx, cy := viewCenter(candidate)
		dx, dy := cx-vx, cy-vy

		valid := false
		primary, secondary := 0.0, 0.0
		switch direction {
		case "left":
			valid, primary, secondary = dx < 0, -dx, absFloat(dy)
		case "right":
			valid, primary, secondary = dx > 0, dx, absFloat(dy)
		case "up":
			valid, primary, secondary = dy < 0, -dy, absFloat(dx)
		case "down":
			valid, primary, secondary = dy > 0, dy, absFloat(dx)
		}
		if !valid {
			continue
		}

		// Prefer windows primarily in the requested direction, then the
		// nearest one. This behaves naturally for master/stack layouts.
		score := primary + secondary*2
		if best == nil || score < bestScore {
			best, bestScore = candidate, score
		}
	}
	return best
}

func viewCenter(v *View) (float64, float64) {
	g := v.geometry()
	x := float64(v.RootTree.Node().X()+g.X) + float64(g.Width)/2
	y := float64(v.RootTree.Node().Y()+g.Y) + float64(g.Height)/2
	return x, y
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func (s *Server) swapViewOrder(a, b *View) {
	ai, bi := -1, -1
	for i, v := range s.views {
		if v == a {
			ai = i
		}
		if v == b {
			bi = i
		}
	}
	if ai >= 0 && bi >= 0 {
		s.views[ai], s.views[bi] = s.views[bi], s.views[ai]
	}
}

func (s *Server) processTilingDrag() {
	v := s.grabbedView
	if v == nil || !s.config.Tiling {
		return
	}

	// Determine the tile under the pointer from layout geometry rather than
	// scene hit-testing: the grabbed view may be raised above its neighbors.
	var target *View
	x, y := s.cursor.X(), s.cursor.Y()
	for _, candidate := range s.mappedViews() {
		if candidate == v {
			continue
		}
		g := candidate.geometry()
		left := float64(candidate.RootTree.Node().X())
		top := float64(candidate.RootTree.Node().Y())
		right := left + float64(g.Width+2*s.config.BorderSize)
		bottom := top + float64(g.Height+2*s.config.BorderSize)
		if x >= left && x < right && y >= top && y < bottom {
			target = candidate
			break
		}
	}
	if target == nil {
		return
	}

	s.swapViewOrder(v, target)
	s.arrange()
}

func (s *Server) beginPointerResize(v *View) {
	if v == nil {
		return
	}
	g := v.geometry()
	left := v.RootTree.Node().X()
	top := v.RootTree.Node().Y()
	right := left + g.Width + 2*s.config.BorderSize
	bottom := top + g.Height + 2*s.config.BorderSize

	edges := wlroots.Edges(0)
	if int(s.cursor.X()) < left+(right-left)/2 {
		edges |= wlroots.EdgeLeft
	} else {
		edges |= wlroots.EdgeRight
	}
	if int(s.cursor.Y()) < top+(bottom-top)/2 {
		edges |= wlroots.EdgeTop
	} else {
		edges |= wlroots.EdgeBottom
	}

	s.cursorMode = CursorResize
	s.resizeEdges = edges
	s.grabX, s.grabY = s.cursor.X(), s.cursor.Y()
	s.grabBox = wlroots.GeoBox{X: left, Y: top, Width: g.Width, Height: g.Height}
	s.beginCursorOverride(resizeCursorName(edges))
}

func (s *Server) processTilingResize() {
	if !s.config.Tiling || len(s.mappedTiledViews()) < 2 || s.fullscreen != nil {
		return
	}
	area := s.usable
	if area.width <= 0 || area.height <= 0 {
		return
	}
	gap := s.config.Gaps
	availableW := area.width - 3*gap
	if availableW <= 1 {
		return
	}
	ratio := s.grabMasterRatio + (s.cursor.X()-s.grabX)/float64(availableW)
	if ratio < 0.2 {
		ratio = 0.2
	}
	if ratio > 0.8 {
		ratio = 0.8
	}
	if absFloat(ratio-s.tileMasterRatio) < 0.002 {
		return
	}
	s.tileMasterRatio = ratio
	s.arrange()
}

func keyboardPointer(keyboard wlroots.Keyboard) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&keyboard))
}

func (s *Server) configureKeyboardLayouts(keyboard wlroots.Keyboard) bool {
	layouts := strings.Join(s.config.KeyboardLayouts, ",")
	if layouts == "" {
		layouts = "us"
	}
	cLayouts := C.CString(layouts)
	cVariants := C.CString(s.config.KeyboardVariants)
	cOptions := C.CString(s.config.KeyboardOptions)
	defer C.free(unsafe.Pointer(cLayouts))
	defer C.free(unsafe.Pointer(cVariants))
	defer C.free(unsafe.Pointer(cOptions))

	ok := bool(C.hatwm_keyboard_set_layouts(
		(*C.struct_wlr_keyboard)(keyboardPointer(keyboard)),
		cLayouts,
		cVariants,
		cOptions,
	))
	if !ok {
		slog.Error("failed to compile keyboard layouts", "layouts", layouts,
			"variants", s.config.KeyboardVariants, "options", s.config.KeyboardOptions)
		return false
	}

	count := int(C.hatwm_keyboard_group_count(
		(*C.struct_wlr_keyboard)(keyboardPointer(keyboard)),
	))
	if count > 0 && s.keyboardLayoutIndex >= count {
		s.keyboardLayoutIndex = 0
	}
	if s.keyboardLayoutIndex > 0 {
		C.hatwm_keyboard_set_group(
			(*C.struct_wlr_keyboard)(keyboardPointer(keyboard)),
			C.uint32_t(s.keyboardLayoutIndex),
		)
	}
	return true
}

func (s *Server) toggleKeyboardLayout() bool {
	if len(s.config.KeyboardLayouts) < 2 || len(s.keyboards) == 0 {
		return true
	}

	groupCount := len(s.config.KeyboardLayouts)
	for _, keyboard := range s.keyboards {
		count := int(C.hatwm_keyboard_group_count(
			(*C.struct_wlr_keyboard)(keyboardPointer(keyboard.Device.Keyboard())),
		))
		if count > 0 && count < groupCount {
			groupCount = count
		}
	}
	if groupCount < 2 {
		return true
	}

	s.keyboardLayoutIndex = (s.keyboardLayoutIndex + 1) % groupCount
	changed := false
	for _, keyboard := range s.keyboards {
		kb := keyboard.Device.Keyboard()
		if bool(C.hatwm_keyboard_set_group(
			(*C.struct_wlr_keyboard)(keyboardPointer(kb)),
			C.uint32_t(s.keyboardLayoutIndex),
		)) {
			changed = true
			s.seat.SetKeyboard(keyboard.Device)
			s.seat.NotifyKeyboardModifiers(kb)
		}
	}
	if !changed {
		return false
	}

	layout := fmt.Sprintf("group %d", s.keyboardLayoutIndex+1)
	if s.keyboardLayoutIndex < len(s.config.KeyboardLayouts) {
		layout = s.config.KeyboardLayouts[s.keyboardLayoutIndex]
	}
	slog.Info("keyboard layout changed", "layout", layout,
		"index", s.keyboardLayoutIndex)
	//s.showKeyboardLayoutNotification(layout)
	s.emitIPCEvent("keyboard_layout_changed", map[string]any{"layout": layout, "index": s.keyboardLayoutIndex})
	return true
}

// func (s *Server) showKeyboardLayoutNotification(layout string) {
// 	if !s.config.Notifications {
// 		return
// 	}
// 	if _, err := exec.LookPath("notify-send"); err != nil {
// 		return
// 	}
// 	cmd := exec.Command("notify-send", "-a", "HatWM", "Keyboard layout", strings.ToUpper(layout))
// 	if err := cmd.Start(); err != nil {
// 		return
// 	}
// 	go func() { _ = cmd.Wait() }()
// }
