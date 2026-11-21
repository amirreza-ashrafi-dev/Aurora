//go:build !windows

package ffmpeg

import "os/exec"

// configureFFmpegCommand leaves process creation unchanged on non-Windows
// platforms, where the Windows console flags are not available.
func configureFFmpegCommand(_ *exec.Cmd) {}
