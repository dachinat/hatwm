package compositor

/*
#cgo pkg-config: wlroots-0.18 libinput
#cgo CFLAGS: -D_GNU_SOURCE -DWLR_USE_UNSTABLE
#include <stdlib.h>
#include "input_config.h"
*/
import "C"

import (
	"unsafe"

	"github.com/swaywm/go-wlroots/wlroots"
)

func (s *Server) configureInputDevice(device wlroots.InputDevice) {
	profile := C.CString(s.config.PointerAccelProfile)
	scrollMethod := C.CString(s.config.TouchpadScrollMethod)
	defer C.free(unsafe.Pointer(profile))
	defer C.free(unsafe.Pointer(scrollMethod))
	C.hatwm_configure_libinput_device(
		(*C.struct_wlr_input_device)(inputDevicePointer(device)),
		C.bool(s.config.TouchpadTapToClick),
		C.bool(s.config.PointerNaturalScroll),
		C.double(s.config.PointerAccelSpeed),
		profile,
		C.bool(s.config.PointerLeftHanded),
		scrollMethod,
		C.bool(s.config.TouchpadDisableWhileTyping),
	)
}

func inputConfigChanged(a, b Config) bool {
	return a.KeyboardRepeatRate != b.KeyboardRepeatRate ||
		a.KeyboardRepeatDelay != b.KeyboardRepeatDelay ||
		a.TouchpadTapToClick != b.TouchpadTapToClick ||
		a.PointerNaturalScroll != b.PointerNaturalScroll ||
		a.PointerAccelSpeed != b.PointerAccelSpeed ||
		a.PointerAccelProfile != b.PointerAccelProfile ||
		a.PointerLeftHanded != b.PointerLeftHanded ||
		a.TouchpadScrollMethod != b.TouchpadScrollMethod ||
		a.TouchpadDisableWhileTyping != b.TouchpadDisableWhileTyping
}
