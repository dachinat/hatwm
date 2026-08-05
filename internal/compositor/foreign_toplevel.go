package compositor

/*
#cgo pkg-config: wlroots-0.18 wayland-server
#cgo CFLAGS: -D_GNU_SOURCE -DWLR_USE_UNSTABLE
#include <stdlib.h>
#include "foreign_toplevel.h"
#include <wlr/types/wlr_ext_foreign_toplevel_list_v1.h>
*/
import "C"

import "unsafe"

func (s *Server) initForeignToplevels() error {
	s.foreignToplevels = C.hatwm_foreign_toplevels_create(
		(*C.struct_wl_display)(displayPointer(s.display)))
	if s.foreignToplevels == nil {
		return &foreignToplevelError{}
	}
	return nil
}

type foreignToplevelError struct{}

func (*foreignToplevelError) Error() string {
	return "failed to initialize foreign-toplevel protocol"
}

func (s *Server) createForeignToplevel(v *View) {
	if s.foreignToplevels == nil || v == nil || v.IsXWayland || v.ForeignToplevel != nil {
		return
	}
	title := C.CString(v.TopLevel.Title())
	appID := C.CString(v.TopLevel.AppId())
	defer C.free(unsafe.Pointer(title))
	defer C.free(unsafe.Pointer(appID))
	v.ForeignToplevel = unsafe.Pointer(C.hatwm_foreign_toplevel_create(
		s.foreignToplevels, title, appID))
}

func (s *Server) updateForeignToplevel(v *View) {
	if v == nil || v.ForeignToplevel == nil || v.IsXWayland {
		return
	}
	title := C.CString(v.TopLevel.Title())
	appID := C.CString(v.TopLevel.AppId())
	defer C.free(unsafe.Pointer(title))
	defer C.free(unsafe.Pointer(appID))
	C.hatwm_foreign_toplevel_update(
		(*C.struct_wlr_ext_foreign_toplevel_handle_v1)(v.ForeignToplevel),
		title, appID)
}

func (s *Server) destroyForeignToplevel(v *View) {
	if v == nil || v.ForeignToplevel == nil {
		return
	}
	C.hatwm_foreign_toplevel_destroy(
		(*C.struct_wlr_ext_foreign_toplevel_handle_v1)(v.ForeignToplevel))
	v.ForeignToplevel = nil
}
