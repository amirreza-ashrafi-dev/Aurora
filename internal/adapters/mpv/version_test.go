package mpv

import "testing"

func TestSupportsRuntimeVideoOutputReinit(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"mpv 0.38.0", false},
		{"mpv 0.39.0", true},
		{"mpv 0.40.0-12-gabcdef", true},
		{"mpv 0.41.0", true},
		{"mpv 1.0.0", true},
		{"unknown", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			if got := supportsRuntimeVideoOutputReinit(tt.version); got != tt.want {
				t.Fatalf("supportsRuntimeVideoOutputReinit(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}
