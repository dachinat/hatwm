package compositor

/*
#cgo pkg-config: wlroots-0.18 wayland-server
#cgo CFLAGS: -D_GNU_SOURCE -DWLR_USE_UNSTABLE
#include <stdlib.h>
#include "cursor_image.h"
*/
import "C"

import (
	"fmt"
	"log/slog"
	"unsafe"

	"github.com/swaywm/go-wlroots/wlroots"
)

func createCursorManager(theme string, size int) (wlroots.XCursorManager, error) {
	if theme == "" {
		theme = "default"
	}
	manager := wlroots.NewXCursorManager(theme, size)
	ptr := (*C.struct_wlr_xcursor_manager)(xcursorManagerPointer(manager))
	if ptr == nil {
		return wlroots.XCursorManager{}, fmt.Errorf("could not create cursor theme %q", theme)
	}
	if !bool(C.hatwm_cursor_manager_load(ptr, C.float(1))) {
		manager.Destroy()
		return wlroots.XCursorManager{}, fmt.Errorf("could not load cursor theme %q at size %d", theme, size)
	}
	return manager, nil
}

func (s *Server) configureCursorTheme(theme string, size int) error {
	manager, err := createCursorManager(theme, size)
	if err != nil {
		return err
	}
	s.installCursorManager(manager)
	return nil
}

func (s *Server) installCursorManager(manager wlroots.XCursorManager) {
	old := s.cursorMgr
	s.cursorMgr = manager
	s.setInputProtocolCursorManager(manager)
	cursorName := "default"
	switch s.cursorMode {
	case CursorMove, CursorTilingMove:
		cursorName = "move"
	case CursorResize:
		cursorName = resizeCursorName(s.resizeEdges)
	case CursorTilingResize:
		cursorName = "ew-resize"
	}
	s.setCursorImage(cursorName)
	s.updateXWaylandCursor()
	if xcursorManagerPointer(old) != nil {
		old.Destroy()
	}
}

func (s *Server) setCursorImage(name string) bool {
	if name == "" {
		name = "default"
	}
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	ok := bool(C.hatwm_cursor_set_named(
		(*C.struct_wlr_cursor)(cursorPointer(s.cursor)),
		(*C.struct_wlr_xcursor_manager)(xcursorManagerPointer(s.cursorMgr)),
		cName, C.float(1)))
	if !ok {
		slog.Warn("cursor theme does not provide requested cursor", "name", name)
	}
	return ok
}

func (s *Server) beginCursorOverride(name string) {
	s.setProtocolCursorOverride(true)
	s.setCursorImage(name)
}

func (s *Server) endCursorOverride() {
	s.setCursorImage("default")
	s.setProtocolCursorOverride(false)
}
