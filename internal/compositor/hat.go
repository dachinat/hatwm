package compositor

/*
#cgo pkg-config: wlroots-0.18
#cgo CFLAGS: -D_GNU_SOURCE -DWLR_USE_UNSTABLE
#include "workspace.h"
*/
import "C"

import (
	"strconv"
	"strings"
)

// addViewToHat makes v the first window restored from The Hat. Keeping the
// ordering separate from s.views preserves tiling order and provides a stable
// MRU list for panels and launchers.
func (s *Server) addViewToHat(v *View) {
	if v == nil {
		return
	}
	s.removeViewFromHat(v)
	v.InHat = true
	s.hat = append([]*View{v}, s.hat...)
}

func (s *Server) removeViewFromHat(v *View) bool {
	if v == nil {
		return false
	}
	removed := false
	original := s.hat
	kept := original[:0]
	for _, candidate := range original {
		if candidate == v {
			removed = true
			continue
		}
		kept = append(kept, candidate)
	}
	for i := len(kept); i < len(original); i++ {
		original[i] = nil
	}
	s.hat = kept
	if removed || v.InHat {
		v.InHat = false
		return true
	}
	return false
}

func (s *Server) stashFocusedInHat() bool {
	v := s.focusedView()
	if v == nil || !v.Managed || !v.Mapped || v.InHat {
		return false
	}
	output := s.ensureViewOutput(v)
	if output.Fullscreen == v {
		setClientPresentationState(v, presentationNone)
		output.Fullscreen = nil
		output.FullscreenMode = presentationNone
	}
	if s.isFloatingView(v) {
		s.rememberFloatingGeometry(v)
	}
	v.setActivated(false)
	v.Urgent = false
	if output.Focused == v {
		output.Focused = nil
	}
	s.addViewToHat(v)
	v.Animation.Running = false
	v.RootTree.Node().SetEnabled(false)
	C.hatwm_clear_keyboard_focus((*C.struct_wlr_seat)(seatPointer(s.seat)))
	s.arrange()
	if next := s.focusedViewForOutput(output); next != nil {
		surface := next.clientSurface()
		s.focusView(next, &surface)
	}
	s.updateAllDecorations()
	s.emitIPCEvent("window_stashed", s.ipcWindow(v))
	s.emitIPCEvent("hat_changed", s.ipcHat())
	s.emitIPCEvent("workspace_updated", s.ipcWorkspaces())
	return true
}

func (s *Server) restoreHatWindowArg(arg string) bool {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		if len(s.hat) == 0 {
			return false
		}
		return s.restoreHatWindow(s.hat[0])
	}
	id, err := strconv.ParseUint(arg, 10, 64)
	if err != nil || id == 0 {
		return false
	}
	return s.restoreHatWindowByID(id)
}

func (s *Server) restoreHatWindowByID(id uint64) bool {
	for _, v := range s.hat {
		if v != nil && v.ID == id {
			return s.restoreHatWindow(v)
		}
	}
	return false
}

func (s *Server) restoreHatWindow(v *View) bool {
	if v == nil || !v.Mapped || !v.InHat || !s.removeViewFromHat(v) {
		return false
	}
	output := s.currentOutputState()
	if output.Fullscreen != nil {
		setClientPresentationState(output.Fullscreen, presentationNone)
		output.Fullscreen = nil
		output.FullscreenMode = presentationNone
	}
	v.Output = output
	v.Workspace = output.CurrentWorkspace
	v.Urgent = false
	s.activeOutput = output
	s.moveViewFront(v)
	s.arrange()
	surface := v.clientSurface()
	s.focusView(v, &surface)
	s.updateAllDecorations()
	s.emitIPCEvent("window_restored", s.ipcWindow(v))
	s.emitIPCEvent("hat_changed", s.ipcHat())
	s.emitIPCEvent("workspace_updated", s.ipcWorkspaces())
	return true
}

func (s *Server) cycleHat() bool {
	if len(s.hat) == 0 {
		return false
	}
	if len(s.hat) > 1 {
		first := s.hat[0]
		copy(s.hat, s.hat[1:])
		s.hat[len(s.hat)-1] = first
	}
	s.emitIPCEvent("hat_changed", s.ipcHat())
	return true
}
