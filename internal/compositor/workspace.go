package compositor

/*
#cgo pkg-config: wlroots-0.18
#cgo CFLAGS: -D_GNU_SOURCE -DWLR_USE_UNSTABLE
#include "workspace.h"
*/
import "C"

import (
	"log/slog"
	"strconv"
)

func (s *Server) validWorkspace(number int) bool {
	count := s.config.WorkspaceCount
	if count < 1 {
		count = 9
	}
	return number >= 1 && number <= count
}

func (s *Server) switchWorkspaceArg(arg string) bool {
	number, err := strconv.Atoi(arg)
	if err != nil || !s.validWorkspace(number) {
		slog.Warn("invalid workspace", "workspace", arg)
		return false
	}
	s.switchWorkspace(number)
	return true
}

func (s *Server) moveFocusedToWorkspaceArg(arg string) bool {
	number, err := strconv.Atoi(arg)
	if err != nil || !s.validWorkspace(number) {
		slog.Warn("invalid workspace", "workspace", arg)
		return false
	}
	return s.moveFocusedToWorkspace(number)
}

func (s *Server) switchWorkspace(number int) {
	output := s.currentOutputState()
	if number == output.CurrentWorkspace || !s.validWorkspace(number) {
		return
	}

	previous := s.focusedView()
	if previous != nil {
		previous.setActivated(false)
		s.updateDecoration(previous)
	}
	if output.Fullscreen != nil {
		setClientPresentationState(output.Fullscreen, presentationNone)
		output.Fullscreen = nil
		output.FullscreenMode = presentationNone
	}

	C.hatwm_clear_keyboard_focus((*C.struct_wlr_seat)(seatPointer(s.seat)))
	output.CurrentWorkspace = number
	s.applyRuleFullscreenForCurrentWorkspace()
	s.arrange()

	if next := s.focusedViewForOutput(output); next != nil {
		if output.Fullscreen != nil && output.Fullscreen.Workspace == output.CurrentWorkspace {
			next = output.Fullscreen
		}
		surface := next.clientSurface()
		s.focusView(next, &surface)
	}
	s.updateAllDecorations()
	s.emitIPCEvent("workspace_changed", s.ipcState())
}

func (s *Server) moveFocusedToWorkspace(number int) bool {
	view := s.focusedView()
	if view == nil || !s.validWorkspace(number) {
		return false
	}
	if view.Workspace == number {
		return true
	}

	output := s.ensureViewOutput(view)
	if output.Fullscreen == view {
		setClientPresentationState(view, presentationNone)
		output.Fullscreen = nil
		output.FullscreenMode = presentationNone
	}
	view.setActivated(false)
	view.Workspace = number
	view.RootTree.Node().SetEnabled(false)
	C.hatwm_clear_keyboard_focus((*C.struct_wlr_seat)(seatPointer(s.seat)))

	s.arrange()
	if next := s.focusedViewForOutput(output); next != nil {
		surface := next.clientSurface()
		s.focusView(next, &surface)
	}
	s.updateAllDecorations()
	s.emitIPCEvent("window_moved", s.ipcWindow(view))
	s.emitIPCEvent("workspace_updated", s.ipcWorkspaces())
	return true
}
