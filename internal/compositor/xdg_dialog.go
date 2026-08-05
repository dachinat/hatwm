package compositor

/*
#cgo pkg-config: wlroots-0.18 wayland-server
#cgo CFLAGS: -D_GNU_SOURCE -DWLR_USE_UNSTABLE -I${SRCDIR}/protocols
#include "xdg_dialog.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func (s *Server) initXDGDialog() error {
	s.xdgDialog = C.hatwm_xdg_dialog_manager_create(
		(*C.struct_wl_display)(displayPointer(s.display)))
	if s.xdgDialog == nil {
		return fmt.Errorf("failed to initialize xdg-dialog-v1")
	}
	return nil
}

func (s *Server) updateXDGDialogState(v *View) {
	if v == nil || v.IsXWayland || v.TopLevel.Nil() || s.xdgDialog == nil {
		return
	}
	var dialog, modal C.bool
	C.hatwm_xdg_dialog_state(s.xdgDialog,
		(*C.struct_wlr_xdg_toplevel)(xdgTopLevelPointer(v.TopLevel)),
		&dialog, &modal)
	v.Dialog = bool(dialog)
	v.Modal = bool(modal)
}

func (s *Server) viewForXDGTopLevelPointer(toplevel unsafe.Pointer) *View {
	if toplevel == nil {
		return nil
	}
	for _, v := range s.views {
		if !v.IsXWayland && !v.TopLevel.Nil() && xdgTopLevelPointer(v.TopLevel) == toplevel {
			return v
		}
	}
	return nil
}

//export hatwmGoXDGDialogChanged
func hatwmGoXDGDialogChanged(
	toplevel *C.struct_wlr_xdg_toplevel, isDialog C.bool, isModal C.bool,
) {
	if activeServer == nil || toplevel == nil {
		return
	}
	v := activeServer.viewForXDGTopLevelPointer(unsafe.Pointer(toplevel))
	if v == nil {
		return
	}
	wasAutoFloating := v.AutoFloating
	becameModal := !v.Modal && bool(isModal)
	modalChanged := v.Modal != bool(isModal)
	v.Dialog = bool(isDialog)
	v.Modal = bool(isModal)
	v.AutoFloating = v.shouldAutoFloat()
	if v.Mapped && (wasAutoFloating != v.AutoFloating || modalChanged) {
		activeServer.arrange()
		if becameModal {
			activeServer.requestViewActivation(v)
		}
		activeServer.emitIPCEvent("window_updated", activeServer.ipcWindow(v))
	}
}
