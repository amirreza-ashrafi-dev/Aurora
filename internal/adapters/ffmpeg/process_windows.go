//go:build windows

package ffmpeg

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// configureFFmpegCommand prevents each FFmpeg extraction process from
// creating a visible Windows console window when Aurora is launched as a GUI.
func configureFFmpegCommand(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
