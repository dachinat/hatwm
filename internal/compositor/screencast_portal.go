package compositor

/*
#cgo pkg-config: wlroots-0.18 wayland-server
#cgo CFLAGS: -D_GNU_SOURCE -DWLR_USE_UNSTABLE -D_REENTRANT -I/usr/include/pipewire-0.3 -I/usr/include/spa-0.2
#cgo LDFLAGS: -lpipewire-0.3 -lsystemd
#include "screencast_portal.h"
#include <wlr/render/wlr_renderer.h>
#include <wlr/types/wlr_output.h>
#include <wlr/types/wlr_scene.h>
*/
import "C"

import (
	"fmt"
	"time"
	"unsafe"

	"github.com/swaywm/go-wlroots/wlroots"
)

func rendererPointer(renderer wlroots.Renderer) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&renderer))
}

func sceneOutputPointer(output wlroots.SceneOutput) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&output))
}

func (s *Server) startScreenCastPortal() error {
	if s.screencastPortal != nil {
		return nil
	}
	s.screencastPortal = C.hatwm_screencast_portal_create()
	if s.screencastPortal == nil {
		return fmt.Errorf("failed to start HatWM ScreenCast portal backend")
	}
	for _, state := range s.outputs {
		C.hatwm_screencast_portal_add_output(s.screencastPortal,
			(*C.struct_wlr_output)(outputPointer(state.Output)))
	}
	s.updatePortalAppearance()
	return nil
}

func (s *Server) updatePortalAppearance() {
	if s.screencastPortal == nil {
		return
	}
	reducedMotion := C.uint32_t(0)
	if !s.config.Animations {
		reducedMotion = 1
	}
	C.hatwm_screencast_portal_set_appearance(s.screencastPortal,
		C.uint32_t(portalColorScheme(s.config.ColorScheme)), reducedMotion)
}

func (s *Server) stopScreenCastPortal() {
	if s.screencastPortal != nil {
		C.hatwm_screencast_portal_destroy(s.screencastPortal)
		s.screencastPortal = nil
	}
}

func (s *Server) tickScreenCastPortal(now time.Time) {
	if s.screencastPortal == nil || now.Sub(s.lastScreencastFrame) < time.Second/30 {
		return
	}
	s.lastScreencastFrame = now
	C.hatwm_screencast_portal_tick(s.screencastPortal)
}

func (s *Server) renderOutput(output wlroots.Output, sceneOutput wlroots.SceneOutput) bool {
	return bool(C.hatwm_screencast_portal_render(
		s.screencastPortal,
		(*C.struct_wlr_scene_output)(sceneOutputPointer(sceneOutput)),
		(*C.struct_wlr_renderer)(rendererPointer(s.renderer)),
		(*C.struct_wlr_output)(outputPointer(output))))
}
