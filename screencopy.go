package main

/*
#cgo pkg-config: wlroots-0.18 wayland-server
#cgo CFLAGS: -D_GNU_SOURCE -DWLR_USE_UNSTABLE
#include <wlr/types/wlr_screencopy_v1.h>
#include <wlr/types/wlr_viewporter.h>
#include <wlr/types/wlr_xdg_output_v1.h>
#include <wayland-server-core.h>

static void init_screencopy(void *display) {
    if (display) {
        wlr_screencopy_manager_v1_create((struct wl_display *)display);
    }
}

static int init_viewporter(void *display) {
    if (!display) {
        return 0;
    }
    return wlr_viewporter_create((struct wl_display *)display) != NULL;
}

static void init_xdg_output(void *display, void *layout) {
    // xdg_output_manager is initialized in layer_shell.go
}
*/
import "C"
import (
	"unsafe"

	"github.com/swaywm/go-wlroots/wlroots"
)

func initScreencopy(display wlroots.Display, layout wlroots.OutputLayout) {
	dPtr := *(*unsafe.Pointer)(unsafe.Pointer(&display))
	lPtr := *(*unsafe.Pointer)(unsafe.Pointer(&layout))

	C.init_screencopy(dPtr)
	C.init_xdg_output(dPtr, lPtr)
}

func initViewporter(display wlroots.Display) bool {
	dPtr := *(*unsafe.Pointer)(unsafe.Pointer(&display))
	return C.init_viewporter(dPtr) != 0
}
