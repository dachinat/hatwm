package compositor

/*
#cgo pkg-config: wlroots-0.18 wayland-server
#cgo CFLAGS: -D_GNU_SOURCE -DWLR_USE_UNSTABLE
#include "xdg_activation.h"
*/
import "C"

import "unsafe"

func (s *Server) handleXDGRequestActivate(surface unsafe.Pointer) {
	if s == nil || surface == nil {
		return
	}
	for _, view := range s.views {
		if view.Mapped && surfacePointer(view.clientSurface()) == surface {
			s.requestViewActivation(view)
			return
		}
	}
}

//export hatwmGoXDGRequestActivate
func hatwmGoXDGRequestActivate(surface unsafe.Pointer) {
	if activeServer != nil {
		activeServer.handleXDGRequestActivate(surface)
	}
}
