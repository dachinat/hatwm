package compositor

/*
#cgo pkg-config: wlroots-0.18 wayland-server
#cgo CFLAGS: -D_GNU_SOURCE -DWLR_USE_UNSTABLE -I${SRCDIR}/protocols
#include "desktop_protocols.h"
#include <wlr/backend.h>
#include <wlr/types/wlr_compositor.h>
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/swaywm/go-wlroots/wlroots"
)

func backendPointer(backend wlroots.Backend) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&backend))
}

func (s *Server) initDesktopProtocols() error {
	s.desktopProtocols = C.hatwm_desktop_protocols_create(
		(*C.struct_wl_display)(displayPointer(s.display)),
		(*C.struct_wlr_backend)(backendPointer(s.backend)))
	if s.desktopProtocols == nil {
		return fmt.Errorf("failed to initialize desktop Wayland protocols")
	}
	return nil
}

func (s *Server) notifyFractionalScale(surface wlroots.Surface) {
	if s.desktopProtocols == nil {
		return
	}
	scale := float64(1)
	root := surface.RootSurface()
	for _, view := range s.views {
		if view.Associated && view.clientSurface() == root && view.Output != nil {
			scale = float64(view.Output.Output.Scale())
			break
		}
	}
	if layer := s.layerSurfaceForSurface(root); layer != nil && layer.output != nil {
		scale = float64(layer.output.Output.Scale())
	}
	if scale <= 0 {
		scale = 1
	}
	if scale == 1 && len(s.outputs) == 1 {
		if candidate := float64(s.outputs[0].Output.Scale()); candidate > 0 {
			scale = candidate
		}
	}
	C.hatwm_desktop_protocols_notify_scale(s.desktopProtocols,
		(*C.struct_wlr_surface)(surfacePointer(surface)), C.double(scale))
}
