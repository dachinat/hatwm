package main

/*
#cgo pkg-config: wlroots-0.18 wayland-server
#cgo CFLAGS: -D_GNU_SOURCE -DWLR_USE_UNSTABLE
#include <stdlib.h>
#include "maximize.h"
*/
import "C"

import (
	"sync"
	"unsafe"

	"github.com/swaywm/go-wlroots/wlroots"
)

var maximizeRegistry = struct {
	sync.RWMutex
	next  uintptr
	views map[uintptr]*View
}{
	next:  1,
	views: make(map[uintptr]*View),
}

func xdgTopLevelPointer(top wlroots.XDGTopLevel) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&top))
}

func setSupportedToplevelCapabilities(top wlroots.XDGTopLevel) {
	C.hatwm_xdg_toplevel_set_supported_capabilities(
		(*C.struct_wlr_xdg_toplevel)(xdgTopLevelPointer(top)))
}

func (s *Server) listenForMaximize(v *View) {
	maximizeRegistry.Lock()
	token := maximizeRegistry.next
	maximizeRegistry.next++
	maximizeRegistry.views[token] = v
	maximizeRegistry.Unlock()

	listener := C.hatwm_xdg_toplevel_listen_maximize(
		(*C.struct_wlr_xdg_toplevel)(xdgTopLevelPointer(v.TopLevel)),
		C.uintptr_t(token),
	)
	if listener == nil {
		maximizeRegistry.Lock()
		delete(maximizeRegistry.views, token)
		maximizeRegistry.Unlock()
		return
	}

	v.MaximizeToken = token
	v.MaximizeListener = unsafe.Pointer(listener)
}

func (s *Server) stopListeningForMaximize(v *View) {
	if v.MaximizeListener != nil {
		C.hatwm_xdg_toplevel_unlisten_maximize(
			(*C.struct_hatwm_maximize_listener)(v.MaximizeListener),
		)
		v.MaximizeListener = nil
	}
	if v.MaximizeToken != 0 {
		maximizeRegistry.Lock()
		delete(maximizeRegistry.views, v.MaximizeToken)
		maximizeRegistry.Unlock()
		v.MaximizeToken = 0
	}
}

func setClientFullscreenState(v *View, enabled bool) {
	if v == nil {
		return
	}
	if v.IsXWayland {
		v.setXWaylandWindowState(enabled)
		return
	}
	C.hatwm_xdg_toplevel_set_window_state(
		(*C.struct_wlr_xdg_toplevel)(xdgTopLevelPointer(v.TopLevel)),
		C.bool(enabled),
	)
}

//export hatwmGoRequestMaximize
func hatwmGoRequestMaximize(token C.uintptr_t, maximized C.bool) {
	maximizeRegistry.RLock()
	v := maximizeRegistry.views[uintptr(token)]
	maximizeRegistry.RUnlock()
	if v == nil || v.Server == nil || !v.Mapped {
		return
	}
	v.Server.setViewFullscreen(v, bool(maximized))
}

//export hatwmGoMaximizeListenerDestroy
func hatwmGoMaximizeListenerDestroy(token C.uintptr_t) {
	maximizeRegistry.Lock()
	v := maximizeRegistry.views[uintptr(token)]
	delete(maximizeRegistry.views, uintptr(token))
	if v != nil {
		v.MaximizeListener = nil
		v.MaximizeToken = 0
	}
	maximizeRegistry.Unlock()
}
