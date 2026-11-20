package mpv

import (
	"fmt"
	"strings"
)

// supportsRuntimeVideoOutputReinit reports whether an MPV version supports
// changing the "vo" option at runtime and rebuilding the video output.
func supportsRuntimeVideoOutputReinit(version string) bool {
	start := strings.IndexFunc(version, func(r rune) bool {
		return r >= '0' && r <= '9'
	})
	if start < 0 {
		return false
	}

	var major, minor int
	if _, err := fmt.Sscanf(version[start:], "%d.%d", &major, &minor); err != nil {
		return false
	}
	return major > 0 || (major == 0 && minor >= 39)
}
