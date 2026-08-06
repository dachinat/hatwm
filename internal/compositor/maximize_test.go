package compositor

import "testing"

func TestPresentationClientStateSeparatesMaximizeAndFullscreen(t *testing.T) {
	tests := []struct {
		name       string
		mode       presentationMode
		maximized  bool
		fullscreen bool
	}{
		{name: "none", mode: presentationNone},
		{name: "maximized", mode: presentationMaximized, maximized: true},
		{name: "fullscreen", mode: presentationFullscreen, fullscreen: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maximized, fullscreen := presentationClientState(tt.mode)
			if maximized != tt.maximized || fullscreen != tt.fullscreen {
				t.Fatalf(
					"presentationClientState(%d) = (%t, %t), want (%t, %t)",
					tt.mode, maximized, fullscreen, tt.maximized, tt.fullscreen)
			}
		})
	}
}
