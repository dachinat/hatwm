package compositor

/*
#cgo pkg-config: wlroots-0.18
#cgo CFLAGS: -DWLR_USE_UNSTABLE
#include <wlr/version.h>
static const char *hatwm_wlroots_version(void) { return WLR_VERSION_STR; }
*/
import "C"

import "fmt"

const Version = "0.3.0"

func VersionString() string {
	return fmt.Sprintf("HatWM %s (wlroots %s)", Version,
		C.GoString(C.hatwm_wlroots_version()))
}

func CheckConfig() error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	if err := validateKeyboardLayouts(cfg); err != nil {
		return err
	}
	manager, err := createCursorManager(cfg.CursorTheme, cfg.CursorSize)
	if err != nil {
		return err
	}
	manager.Destroy()
	return nil
}
