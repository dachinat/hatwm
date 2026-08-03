package main

/*
#cgo pkg-config: wlroots-0.18 wayland-server
#cgo CFLAGS: -D_GNU_SOURCE -DWLR_USE_UNSTABLE -I${SRCDIR}/protocols
#include "output_protocols.h"
#include <wlr/backend.h>
#include <wlr/types/wlr_output.h>
*/
import "C"

import (
	"fmt"

	"github.com/swaywm/go-wlroots/wlroots"
)

func (s *Server) initOutputProtocols() error {
	s.outputProtocols = C.hatwm_output_protocols_create(
		(*C.struct_wl_display)(displayPointer(s.display)),
		(*C.struct_wlr_backend)(backendPointer(s.backend)),
		(*C.struct_wlr_output_layout)(outputLayoutPointer(s.outputLayout)))
	if s.outputProtocols == nil {
		return fmt.Errorf("failed to initialize output Wayland protocols")
	}
	return nil
}

func (s *Server) outputProtocolsAdd(output wlroots.Output) {
	if s.outputProtocols != nil {
		C.hatwm_output_protocols_add(s.outputProtocols,
			(*C.struct_wlr_output)(outputPointer(output)))
	}
}

func (s *Server) outputProtocolsRemove(output wlroots.Output) {
	if s.outputProtocols != nil {
		C.hatwm_output_protocols_remove(s.outputProtocols,
			(*C.struct_wlr_output)(outputPointer(output)))
	}
}
