//go:build windows

package ffmpeg

import (
	"os/exec"
	"testing"
)

func TestConfigureFFmpegCommandHidesConsoleWindow(t *testing.T) {
	cmd := exec.Command("ffmpeg")
	configureFFmpegCommand(cmd)
	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr is nil")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Fatal("HideWindow is false")
	}
	if cmd.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Fatalf("CreationFlags = %#x, want CREATE_NO_WINDOW (%#x)", cmd.SysProcAttr.CreationFlags, createNoWindow)
	}
}
