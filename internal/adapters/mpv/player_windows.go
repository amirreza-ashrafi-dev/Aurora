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

	"my-go/internal/domain"
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
	getPropertyString func(unsafe.Pointer, string) uintptr
	free              func(uintptr)
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
	purego.RegisterLibFunc(&p.getPropertyString, h, "mpv_get_property_string")
	purego.RegisterLibFunc(&p.free, h, "mpv_free")

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

func (p *Player) getInt64(name string) (int64, error) {
	if p == nil || p.ctx == nil { return 0, fmt.Errorf("mpv is not initialized") }
	var value int64
	code := p.getProperty(p.ctx, name, 4, unsafe.Pointer(&value))
	runtime.KeepAlive(&value)
	if code < 0 { return 0, fmt.Errorf("get property %q failed: %s", name, mpvErrorText(p.errorString(code))) }
	return value, nil
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


// LoadExternalSubtitle loads and selects an external subtitle for the current
// media item. MPV owns the track after this command succeeds.
func (p *Player) LoadExternalSubtitle(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("no subtitle file selected")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("subtitle file is unavailable: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("subtitle path is a directory")
	}
	return p.commandArgs("sub-add", abs, "select")
}

// DisableSubtitles disables the active subtitle track without removing tracks.
func (p *Player) CycleSubtitleTrack() error { return p.commandArgs("cycle", "sid") }

func (p *Player) RemoveSelectedExternalSubtitle() error {
	id, err := p.getInt64("sid")
	if err != nil { return err }
	if id < 0 { return fmt.Errorf("no subtitle track is selected") }
	external, err := p.getPropertyStringValue(fmt.Sprintf("track-list/%d/external", id))
	if err != nil || external != "yes" { return fmt.Errorf("the selected subtitle is not an external track") }
	return p.commandArgs("sub-remove", strconv.FormatInt(int64(id), 10))
}

func (p *Player) getPropertyStringValue(name string) (string, error) {
	if p == nil || p.ctx == nil { return "", fmt.Errorf("mpv is not initialized") }
	ptr := p.getPropertyString(p.ctx, name)
	if ptr == 0 { return "", fmt.Errorf("mpv property %q is unavailable", name) }
	defer p.free(ptr)
	return windows.BytePtrToString((*byte)(unsafe.Pointer(ptr))), nil
}

// SetSubtitleVisibility hides or shows subtitles without changing the
// selected track, so enabling subtitles restores the same track.
func (p *Player) SetSubtitleVisibility(visible bool) error {
	value := "no"
	if visible {
		value = "yes"
	}
	return p.setSubtitleProperty("sub-visibility", value)
}

// DisableSubtitles is retained as a convenience for callers that only hide
// subtitles; it does not clear the selected track.
func (p *Player) DisableSubtitles() error {
	return p.SetSubtitleVisibility(false)
}

// SetSubtitleDelay applies a signed delay in seconds.
func (p *Player) SetSubtitleDelay(seconds float64) error {
	if seconds < -3600 || seconds > 3600 {
		return fmt.Errorf("subtitle delay must be between -3600 and 3600 seconds")
	}
	return p.setSubtitleProperty("sub-delay", strconv.FormatFloat(seconds, 'f', 3, 64))
}

// SetSubtitleStyle applies text-subtitle appearance properties. MPV and the
// subtitle format decide whether individual properties are applicable.
func (p *Player) SetSubtitleStyle(style domain.SubtitleStyle) error {
	properties := []struct{ name, value string }{
		{"sub-font", style.FontFamily},
		{"sub-font-size", strconv.FormatFloat(style.FontSize, 'f', 1, 64)},
		{"sub-color", style.TextColor},
		{"sub-outline-color", style.OutlineColor},
		{"sub-outline-size", strconv.FormatFloat(style.OutlineSize, 'f', 1, 64)},
		{"sub-pos", strconv.Itoa(style.VerticalPosition)},
	}
	for _, property := range properties {
		if err := p.setSubtitleProperty(property.name, property.value); err != nil {
			return err
		}
	}
	return nil
}

// ActiveSubtitleLanguage returns the language metadata of the selected
// subtitle track when MPV exposes it. Empty metadata is not an error.
func (p *Player) ActiveSubtitleLanguage() (string, error) {
	if p == nil || p.ctx == nil {
		return "", fmt.Errorf("mpv is not initialized")
	}
	ptr := p.getPropertyString(p.ctx, "current-tracks/sub/lang")
	if ptr == 0 {
		return "", nil
	}
	defer p.free(ptr)
	return windows.BytePtrToString((*byte)(unsafe.Pointer(ptr))), nil
}

func (p *Player) setSubtitleProperty(name, value string) error {
	if p == nil || p.ctx == nil {
		return fmt.Errorf("mpv is not initialized")
	}
	code := p.setPropertyString(p.ctx, name, value)
	if code < 0 {
		return fmt.Errorf("set subtitle property %q failed: %s", name, mpvErrorText(p.errorString(code)))
	}
	return nil
}
