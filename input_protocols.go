package main

/*
#cgo pkg-config: wlroots-0.18 wayland-server
#cgo CFLAGS: -D_GNU_SOURCE -DWLR_USE_UNSTABLE -I${SRCDIR}/protocols
#include "input_protocols.h"
#include <wlr/types/wlr_cursor.h>
#include <wlr/types/wlr_input_device.h>
#include <wlr/types/wlr_seat.h>
#include <wlr/types/wlr_xcursor_manager.h>
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/swaywm/go-wlroots/wlroots"
)

func cursorPointer(cursor wlroots.Cursor) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&cursor))
}

func inputDevicePointer(device wlroots.InputDevice) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&device))
}

func (s *Server) initInputProtocols() error {
	s.inputProtocols = C.hatwm_input_protocols_create(
		(*C.struct_wl_display)(displayPointer(s.display)),
		(*C.struct_wlr_cursor)(cursorPointer(s.cursor)),
		(*C.struct_wlr_xcursor_manager)(xcursorManagerPointer(s.cursorMgr)),
		(*C.struct_wlr_seat)(seatPointer(s.seat)))
	if s.inputProtocols == nil {
		return fmt.Errorf("failed to initialize input Wayland protocols")
	}
	return nil
}

func (s *Server) notifyInputActivity() {
	if s.inputProtocols != nil {
		C.hatwm_input_protocols_notify_activity(s.inputProtocols)
	}
}

func (s *Server) setProtocolCursorOverride(enabled bool) {
	if s.inputProtocols != nil {
		C.hatwm_input_protocols_set_cursor_override(
			s.inputProtocols, C.bool(enabled))
	}
}

func (s *Server) setInputProtocolCursorManager(manager wlroots.XCursorManager) {
	if s.inputProtocols != nil {
		C.hatwm_input_protocols_set_cursor_manager(s.inputProtocols,
			(*C.struct_wlr_xcursor_manager)(xcursorManagerPointer(manager)))
	}
}

func (s *Server) shortcutsInhibited() bool {
	return s.inputProtocols != nil &&
		bool(C.hatwm_input_protocols_shortcuts_inhibited(s.inputProtocols))
}

func (s *Server) pointerLocked() bool {
	return s.inputProtocols != nil &&
		bool(C.hatwm_input_protocols_pointer_locked(s.inputProtocols))
}

func (s *Server) handleProtocolRelativeMotion(
	device wlroots.InputDevice, time uint32, dx, dy float64,
) bool {
	return s.inputProtocols != nil && bool(C.hatwm_input_protocols_handle_relative_motion(
		s.inputProtocols,
		(*C.struct_wlr_input_device)(inputDevicePointer(device)),
		C.uint64_t(time)*1000,
		C.double(dx), C.double(dy), C.double(dx), C.double(dy)))
}

func (s *Server) updateProtocolPointerFocus(surface *wlroots.Surface, sx, sy float64) {
	if s.inputProtocols == nil {
		return
	}
	var ptr *C.struct_wlr_surface
	if surface != nil {
		ptr = (*C.struct_wlr_surface)(surfacePointer(*surface))
	}
	C.hatwm_input_protocols_pointer_focus(
		s.inputProtocols, ptr, C.double(sx), C.double(sy))
}
