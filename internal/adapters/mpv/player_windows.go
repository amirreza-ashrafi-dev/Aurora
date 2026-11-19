//go:build windows

package mpv

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/windows"
)

// Player is the libmpv-backed implementation of application.Player.
type Player struct {
	dll windows.Handle
	ctx unsafe.Pointer

	create            func() unsafe.Pointer
	setOptionString   func(unsafe.Pointer, string, string) int32
	initialize        func(unsafe.Pointer) int32
	command           func(unsafe.Pointer, *uintptr) int32
	setPropertyString func(unsafe.Pointer, string, string) int32
	getProperty       func(unsafe.Pointer, string, int32, unsafe.Pointer) int32
	terminateDestroy  func(unsafe.Pointer)
	errorString       func(int32) uintptr
}

func mpvErrorText(ptr uintptr) string {
	if ptr == 0 {
		return "unknown mpv error"
	}
	return windows.BytePtrToString((*byte)(unsafe.Pointer(ptr)))
}

// New loads the bundled libmpv DLL and initializes an embedded D3D11 output.
// resourcesDir is supplied by the composition root so this adapter does not
// depend on MyGo's application/resource APIs.
func New(hwnd uintptr, resourcesDir string) (*Player, error) {
	dllDir := resourcesDir
	dllPath := filepath.Join(dllDir, "libmpv-2.dll")
	if _, err := os.Stat(dllPath); err != nil {
		workDir, workErr := os.Getwd()
		if workErr != nil {
			return nil, workErr
		}
		dllDir = filepath.Join(workDir, "third_party", "mpv")
		dllPath = filepath.Join(dllDir, "libmpv-2.dll")
	}
	if _, err := os.Stat(dllPath); err != nil {
		return nil, fmt.Errorf("MPV DLL not found at %q: %w", dllPath, err)
	}
	if err := windows.SetDllDirectory(dllDir); err != nil {
		return nil, fmt.Errorf("set MPV DLL directory: %w", err)
	}
	dll, err := windows.LoadLibrary(dllPath)
	if err != nil {
		return nil, fmt.Errorf("load MPV DLL: %w", err)
	}

	p := &Player{dll: dll}
	h := uintptr(dll)
	purego.RegisterLibFunc(&p.create, h, "mpv_create")
	purego.RegisterLibFunc(&p.setOptionString, h, "mpv_set_option_string")
	purego.RegisterLibFunc(&p.initialize, h, "mpv_initialize")
	purego.RegisterLibFunc(&p.command, h, "mpv_command")
	purego.RegisterLibFunc(&p.setPropertyString, h, "mpv_set_property_string")
	purego.RegisterLibFunc(&p.getProperty, h, "mpv_get_property")
	purego.RegisterLibFunc(&p.terminateDestroy, h, "mpv_terminate_destroy")
	purego.RegisterLibFunc(&p.errorString, h, "mpv_error_string")

	p.ctx = p.create()
	if p.ctx == nil {
		p.Close()
		return nil, fmt.Errorf("mpv_create returned nil")
	}
	options := map[string]string{
		"wid":                    strconv.FormatUint(uint64(uint32(hwnd)), 10),
		"vo":                     "gpu",
		"gpu-api":                "d3d11",
		"hwdec":                  "auto-safe",
		"keep-open":              "yes",
		"input-default-bindings": "yes",
		"input-vo-keyboard":      "yes",
	}
	for name, value := range options {
		code := p.setOptionString(p.ctx, name, value)
		if code < 0 {
			message := mpvErrorText(p.errorString(code))
			p.Close()
			return nil, fmt.Errorf("mpv option %q failed: %s", name, message)
		}
	}
	if code := p.initialize(p.ctx); code < 0 {
		message := mpvErrorText(p.errorString(code))
		p.Close()
		return nil, fmt.Errorf("mpv_initialize failed: %s", message)
	}
	return p, nil
}

func (p *Player) commandArgs(args ...string) error {
	if p == nil || p.ctx == nil {
		return fmt.Errorf("mpv is not initialized")
	}
	cstrings := make([]*byte, len(args))
	ptrs := make([]uintptr, len(args)+1)
	for i, arg := range args {
		s, err := windows.BytePtrFromString(arg)
		if err != nil {
			return err
		}
		cstrings[i] = s
		ptrs[i] = uintptr(unsafe.Pointer(s))
	}
	ptrs[len(args)] = 0
	code := p.command(p.ctx, &ptrs[0])
	runtime.KeepAlive(cstrings)
	runtime.KeepAlive(ptrs)
	if code < 0 {
		return fmt.Errorf("mpv command failed: %s (code %d)", mpvErrorText(p.errorString(code)), code)
	}
	return nil
}

func (p *Player) Load(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("no video file selected")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, err := os.Stat(abs); err != nil {
		return err
	}
	return p.commandArgs("loadfile", abs, "replace")
}

func (p *Player) Append(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	return p.commandArgs("loadfile", abs, "append")
}

func (p *Player) PlayIndex(index int) error {
	return p.commandArgs("playlist-play-index", strconv.Itoa(index))
}

func (p *Player) RemovePlaylistItem(index int) error {
	if index < 0 {
		return fmt.Errorf("playlist index cannot be negative")
	}
	return p.commandArgs("playlist-remove", strconv.Itoa(index))
}

func (p *Player) GetDouble(name string) (float64, error) {
	if p == nil || p.ctx == nil {
		return 0, fmt.Errorf("mpv is not initialized")
	}
	var value float64
	code := p.getProperty(p.ctx, name, 5, unsafe.Pointer(&value))
	runtime.KeepAlive(&value)
	if code < 0 {
		return 0, fmt.Errorf("get property %q failed: %s", name, mpvErrorText(p.errorString(code)))
	}
	return value, nil
}

func (p *Player) SetPaused(paused bool) error {
	if p == nil || p.ctx == nil {
		return fmt.Errorf("mpv is not initialized")
	}
	value := "no"
	if paused {
		value = "yes"
	}
	code := p.setPropertyString(p.ctx, "pause", value)
	if code < 0 {
		return fmt.Errorf("pause failed: %s", mpvErrorText(p.errorString(code)))
	}
	return nil
}

func (p *Player) Seek(seconds float64) error {
	if seconds < 0 {
		seconds = 0
	}
	return p.commandArgs("seek", strconv.FormatFloat(seconds, 'f', 3, 64), "absolute+exact")
}

func (p *Player) SetVolume(volume float64) error {
	if p == nil || p.ctx == nil {
		return fmt.Errorf("mpv is not initialized")
	}
	if volume < 0 {
		volume = 0
	}
	if volume > 100 {
		volume = 100
	}
	code := p.setPropertyString(p.ctx, "volume", strconv.FormatFloat(volume, 'f', 1, 64))
	if code < 0 {
		return fmt.Errorf("set volume failed: %s", mpvErrorText(p.errorString(code)))
	}
	return nil
}

func (p *Player) Stop() error {
	if p == nil || p.ctx == nil {
		return fmt.Errorf("mpv is not initialized")
	}
	if err := p.SetPaused(true); err != nil {
		return err
	}
	return p.Seek(0)
}

func (p *Player) Close() {
	if p == nil {
		return
	}
	if p.ctx != nil {
		p.terminateDestroy(p.ctx)
		p.ctx = nil
	}
	if p.dll != 0 {
		windows.FreeLibrary(p.dll)
		p.dll = 0
	}
}
