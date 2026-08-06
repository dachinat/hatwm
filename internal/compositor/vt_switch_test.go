package compositor

import (
	"testing"

	"github.com/swaywm/go-wlroots/wlroots"
	"github.com/swaywm/go-wlroots/xkb"
)

func TestVirtualTerminalForKey(t *testing.T) {
	required := wlroots.KeyboardModifierCtrl | wlroots.KeyboardModifierAlt
	tests := []struct {
		name      string
		sym       xkb.KeySym
		modifiers wlroots.KeyboardModifier
		wantVT    uint
		wantOK    bool
	}{
		{name: "first VT", sym: xkb.KeySymF1, modifiers: required, wantVT: 1, wantOK: true},
		{name: "third VT", sym: xkb.KeySymF3, modifiers: required, wantVT: 3, wantOK: true},
		{name: "twelfth VT", sym: xkb.KeySymF12, modifiers: required, wantVT: 12, wantOK: true},
		{name: "lock modifier tolerated", sym: xkb.KeySymF2, modifiers: required | wlroots.KeyboardModifierCaps, wantVT: 2, wantOK: true},
		{name: "control missing", sym: xkb.KeySymF2, modifiers: wlroots.KeyboardModifierAlt},
		{name: "alt missing", sym: xkb.KeySymF2, modifiers: wlroots.KeyboardModifierCtrl},
		{name: "not a function key", sym: xkb.KeySymEscape, modifiers: required},
		{name: "function key beyond VT range", sym: xkb.KeySymF13, modifiers: required},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotVT, gotOK := virtualTerminalForKey(tc.sym, tc.modifiers)
			if gotVT != tc.wantVT || gotOK != tc.wantOK {
				t.Fatalf("virtualTerminalForKey(%v, %v) = (%d, %v), want (%d, %v)",
					tc.sym, tc.modifiers, gotVT, gotOK, tc.wantVT, tc.wantOK)
			}
		})
	}
}
