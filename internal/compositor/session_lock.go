package compositor

/*
#cgo pkg-config: wlroots-0.18 wayland-server
#cgo CFLAGS: -D_GNU_SOURCE -DWLR_USE_UNSTABLE
#include "session_lock.h"
*/
import "C"

import (
	"log/slog"
	"unsafe"

	"github.com/swaywm/go-wlroots/wlroots"
)

type SessionLockSurface struct {
	ptr    *C.struct_wlr_session_lock_surface_v1
	scene  *C.struct_wlr_scene_tree
	mapped bool
	output *OutputState
}

func (s *Server) initSessionLock() error {
	s.sessionLockManager = C.hatwm_session_lock_manager_create(
		(*C.struct_wl_display)(displayPointer(s.display)),
	)
	if s.sessionLockManager == nil {
		return errSessionLockInit
	}
	return nil
}

var errSessionLockInit = &sessionLockError{}

type sessionLockError struct{}

func (*sessionLockError) Error() string {
	return "failed to create ext-session-lock-v1 global"
}

func (s *Server) handleSessionLockNew(lock *C.struct_wlr_session_lock_v1) {
	if lock == nil {
		return
	}

	s.sessionLock = lock
	s.sessionLocked = true
	s.lockTree.Node().SetEnabled(true)
	if s.lockBackground == nil {
		s.lockBackground = C.hatwm_session_lock_background_create(
			(*C.struct_wlr_scene_tree)(sceneTreePointer(s.lockTree)),
			(*C.struct_wlr_output_layout)(outputLayoutPointer(s.outputLayout)),
		)
	} else {
		s.updateSessionLockBackground()
	}

	// Stop compositor grabs and prevent the previously focused client from
	// receiving input in the interval before swaylock maps its surface.
	s.cursorMode = CursorPassthrough
	s.grabbedView = nil
	C.hatwm_session_lock_clear_seat_focus(
		(*C.struct_wlr_seat)(seatPointer(s.seat)),
	)
	C.hatwm_session_lock_send_locked(lock)
	slog.Info("session locked")
}

func (s *Server) handleSessionLockSurfaceNew(
	ptr *C.struct_wlr_session_lock_surface_v1) {
	if ptr == nil || s.sessionLock == nil {
		return
	}

	scene := C.hatwm_session_lock_surface_create_scene(
		ptr,
		(*C.struct_wlr_scene_tree)(sceneTreePointer(s.lockTree)),
		(*C.struct_wlr_output_layout)(outputLayoutPointer(s.outputLayout)),
	)
	if scene == nil {
		slog.Error("failed to create session lock surface scene")
		return
	}
	s.lockSurfaces = append(s.lockSurfaces, &SessionLockSurface{
		ptr:   ptr,
		scene: scene,
		output: s.outputStateForPointer(
			C.hatwm_session_lock_surface_output(ptr)),
	})
}

func (s *Server) handleSessionLockSurfaceMap(
	ptr *C.struct_wlr_session_lock_surface_v1) {
	surface := s.findSessionLockSurface(ptr)
	if surface == nil || !s.sessionLocked {
		return
	}
	surface.mapped = true
	s.focusSessionLockSurface(surface)
}

func (s *Server) handleSessionLockSurfaceUnmap(
	ptr *C.struct_wlr_session_lock_surface_v1) {
	surface := s.findSessionLockSurface(ptr)
	if surface == nil {
		return
	}
	surface.mapped = false
	if s.sessionLocked {
		C.hatwm_session_lock_clear_seat_focus(
			(*C.struct_wlr_seat)(seatPointer(s.seat)),
		)
	}
}

func (s *Server) handleSessionLockSurfaceDestroy(
	ptr *C.struct_wlr_session_lock_surface_v1) {
	for i, surface := range s.lockSurfaces {
		if surface.ptr != ptr {
			continue
		}
		if surface.scene != nil {
			C.hatwm_session_lock_surface_scene_destroy(surface.scene)
			surface.scene = nil
		}
		s.lockSurfaces = append(s.lockSurfaces[:i], s.lockSurfaces[i+1:]...)
		break
	}
}

func (s *Server) handleSessionUnlock(
	lock *C.struct_wlr_session_lock_v1) {
	if lock == nil || lock != s.sessionLock {
		return
	}
	s.sessionLocked = false
	s.lockTree.Node().SetEnabled(false)
	if s.lockBackground != nil {
		C.hatwm_session_lock_background_destroy(s.lockBackground)
		s.lockBackground = nil
	}
	s.restoreFocusAfterSessionUnlock()
	slog.Info("session unlocked")
}

func (s *Server) handleSessionLockDestroy(
	lock *C.struct_wlr_session_lock_v1, unlocked bool) {
	if lock != s.sessionLock {
		return
	}
	s.sessionLock = nil

	if !unlocked {
		// A crashed lock client must never unlock the desktop. Keep the opaque
		// lock background above all clients and accept a replacement lock
		// process so the user can recover.
		s.sessionLocked = true
		s.lockTree.Node().SetEnabled(true)
		C.hatwm_session_lock_clear_seat_focus(
			(*C.struct_wlr_seat)(seatPointer(s.seat)),
		)
		slog.Warn("session lock client disappeared; keeping session locked")
	}
}

func (s *Server) findSessionLockSurface(
	ptr *C.struct_wlr_session_lock_surface_v1) *SessionLockSurface {
	for _, surface := range s.lockSurfaces {
		if surface.ptr == ptr {
			return surface
		}
	}
	return nil
}

func (s *Server) focusSessionLockSurface(surface *SessionLockSurface) {
	if surface == nil || !surface.mapped || surface.ptr == nil ||
		!s.sessionLocked {
		return
	}
	ptr := C.hatwm_session_lock_surface_surface(surface.ptr)
	if ptr == nil {
		return
	}
	if surface.output != nil {
		s.activeOutput = surface.output
	}
	var wlrSurface wlroots.Surface
	*(*unsafe.Pointer)(unsafe.Pointer(&wlrSurface)) = unsafe.Pointer(ptr)
	if len(s.keyboards) > 0 {
		s.seat.NotifyKeyboardEnter(wlrSurface, s.seat.Keyboard())
	}
}

func (s *Server) restoreFocusAfterSessionUnlock() {
	if layer := s.exclusiveKeyboardLayer(); layer != nil {
		s.focusLayerSurface(layer)
		return
	}
	if views := s.mappedViews(); len(views) > 0 {
		surface := views[0].clientSurface()
		s.focusView(views[0], &surface)
	}
}

func (s *Server) updateSessionLockBackground() {
	if s.lockBackground == nil {
		return
	}
	C.hatwm_session_lock_background_update(
		s.lockBackground,
		(*C.struct_wlr_output_layout)(outputLayoutPointer(s.outputLayout)),
	)
}

//export hatwmGoSessionLockNew
func hatwmGoSessionLockNew(lock unsafe.Pointer) {
	if activeServer != nil {
		activeServer.handleSessionLockNew(
			(*C.struct_wlr_session_lock_v1)(lock),
		)
	}
}

//export hatwmGoSessionLockSurfaceNew
func hatwmGoSessionLockSurfaceNew(surface unsafe.Pointer) {
	if activeServer != nil {
		activeServer.handleSessionLockSurfaceNew(
			(*C.struct_wlr_session_lock_surface_v1)(surface),
		)
	}
}

//export hatwmGoSessionLockSurfaceMap
func hatwmGoSessionLockSurfaceMap(surface unsafe.Pointer) {
	if activeServer != nil {
		activeServer.handleSessionLockSurfaceMap(
			(*C.struct_wlr_session_lock_surface_v1)(surface),
		)
	}
}

//export hatwmGoSessionLockSurfaceUnmap
func hatwmGoSessionLockSurfaceUnmap(surface unsafe.Pointer) {
	if activeServer != nil {
		activeServer.handleSessionLockSurfaceUnmap(
			(*C.struct_wlr_session_lock_surface_v1)(surface),
		)
	}
}

//export hatwmGoSessionLockSurfaceDestroy
func hatwmGoSessionLockSurfaceDestroy(surface unsafe.Pointer) {
	if activeServer != nil {
		activeServer.handleSessionLockSurfaceDestroy(
			(*C.struct_wlr_session_lock_surface_v1)(surface),
		)
	}
}

//export hatwmGoSessionLockUnlock
func hatwmGoSessionLockUnlock(lock unsafe.Pointer) {
	if activeServer != nil {
		activeServer.handleSessionUnlock(
			(*C.struct_wlr_session_lock_v1)(lock),
		)
	}
}

//export hatwmGoSessionLockDestroy
func hatwmGoSessionLockDestroy(lock unsafe.Pointer, unlocked C.bool) {
	if activeServer != nil {
		activeServer.handleSessionLockDestroy(
			(*C.struct_wlr_session_lock_v1)(lock),
			bool(unlocked),
		)
	}
}
