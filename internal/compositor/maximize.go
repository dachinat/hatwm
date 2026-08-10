package compositor

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

type presentationMode uint8

const (
	presentationNone presentationMode = iota
	presentationFullscreen
	presentationMaximizedFullscreen
)

func presentationClientState(mode presentationMode) (maximized, fullscreen bool) {
	return mode == presentationMaximizedFullscreen,
		mode == presentationFullscreen || mode == presentationMaximizedFullscreen
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

func setClientPresentationState(v *View, mode presentationMode) {
	if v == nil {
		return
	}
	maximized, fullscreen := presentationClientState(mode)
	if v.IsXWayland {
		v.setXWaylandWindowState(maximized, fullscreen)
		return
	}
	C.hatwm_xdg_toplevel_set_window_state(
		(*C.struct_wlr_xdg_toplevel)(xdgTopLevelPointer(v.TopLevel)),
		C.bool(maximized), C.bool(fullscreen),
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
	v.Server.handleViewMaximizeRequest(v, bool(maximized))
}

//export hatwmGoRequestFullscreen
func hatwmGoRequestFullscreen(token C.uintptr_t, fullscreen C.bool) {
	maximizeRegistry.RLock()
	v := maximizeRegistry.views[uintptr(token)]
	maximizeRegistry.RUnlock()
	if v == nil || v.Server == nil || !v.Mapped {
		return
	}
	v.Server.setViewFullscreen(v, bool(fullscreen))
}

//export hatwmGoWindowMetadataChanged
func hatwmGoWindowMetadataChanged(token C.uintptr_t) {
	maximizeRegistry.RLock()
	v := maximizeRegistry.views[uintptr(token)]
	maximizeRegistry.RUnlock()
	if v == nil || v.Server == nil {
		return
	}
	wasFloating, oldWorkspace := v.AutoFloating, v.Workspace
	v.Server.applyWindowMetadata(v)
	if v.Mapped && (wasFloating != v.AutoFloating || oldWorkspace != v.Workspace) {
		v.Server.arrange()
		v.Server.emitIPCEvent("window_updated", v.Server.ipcWindow(v))
	}
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
