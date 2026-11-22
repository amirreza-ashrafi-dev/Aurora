//go:build darwin

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
)

const (
	formatInt64 int32 = 4
	formatDouble int32 = 5
)

type Player struct {
	dll uintptr
	ctx unsafe.Pointer

	create            func() unsafe.Pointer
	setOptionString   func(unsafe.Pointer, string, string) int32
	initialize        func(unsafe.Pointer) int32
	command           func(unsafe.Pointer, unsafe.Pointer) int32
	setPropertyString func(unsafe.Pointer, string, string) int32
	getProperty       func(unsafe.Pointer, string, int32, unsafe.Pointer) int32
	getPropertyString func(unsafe.Pointer, string) unsafe.Pointer
	terminateDestroy  func(unsafe.Pointer)
	errorString       func(int32) unsafe.Pointer
	free              func(unsafe.Pointer)

	runtimeVideoOutputReinit bool
	videoOutputReleased      bool
}

func cString(ptr unsafe.Pointer) string {
	if ptr == nil {
		return ""
	}
	const limit = 64 * 1024
	out := make([]byte, 0, 128)
	for i := uintptr(0); i < limit; i++ {
		b := *(*byte)(unsafe.Add(ptr, i))
		if b == 0 {
			return string(out)
		}
		out = append(out, b)
	}
	return string(out)
}

func (p *Player) errorText(code int32) string {
	if p == nil || p.errorString == nil {
		return fmt.Sprintf("MPV error %d", code)
	}
	return cString(p.errorString(code))
}

func (p *Player) version() string {
	if p == nil || p.ctx == nil {
		return ""
	}
	ptr := p.getPropertyString(p.ctx, "mpv-version")
	if ptr == nil {
		return ""
	}
	defer p.free(ptr)
	return cString(ptr)
}

func (p *Player) SupportsRuntimeVideoOutputRelease() bool {
	return p != nil && p.runtimeVideoOutputReinit
}

func findLibrary(resourcesDir string) (string, error) {
	candidates := []string{}
	if explicit := strings.TrimSpace(os.Getenv("AURORA_LIBMPV_PATH")); explicit != "" {
		candidates = append(candidates, explicit)
	}
	for _, name := range []string{"libmpv.2.dylib", "libmpv.dylib"} {
		candidates = append(candidates,
			filepath.Join(resourcesDir, name),
			filepath.Join(resourcesDir, "..", "Frameworks", name),
			filepath.Join("third_party", "mpv", "macos", "lib", name),
		)
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			abs, err := filepath.Abs(candidate)
			if err != nil {
				return "", err
			}
			return abs, nil
		}
	}
	return "", fmt.Errorf("macOS libmpv not found; set AURORA_LIBMPV_PATH or run scripts/build-mpv-macos.sh")
}

// New creates libmpv with the Aurora-patched macOS macvk host-view contract.
// videoNSView is an NSView* returned by MyGo's native.View implementation.
func New(videoNSView uintptr, resourcesDir string) (*Player, error) {
	if videoNSView == 0 {
		return nil, fmt.Errorf("macOS MPV host NSView is nil")
	}
	library, err := findLibrary(resourcesDir)
	if err != nil {
		return nil, err
	}
	handle, err := purego.Dlopen(library, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, fmt.Errorf("load libmpv %q: %w", library, err)
	}
	p := &Player{dll: handle}
	registerErr := func() (err error) {
		defer func() {
			if v := recover(); v != nil {
				err = fmt.Errorf("resolve libmpv symbols: %v", v)
			}
		}()
		purego.RegisterLibFunc(&p.create, handle, "mpv_create")
		purego.RegisterLibFunc(&p.setOptionString, handle, "mpv_set_option_string")
		purego.RegisterLibFunc(&p.initialize, handle, "mpv_initialize")
		purego.RegisterLibFunc(&p.command, handle, "mpv_command")
		purego.RegisterLibFunc(&p.setPropertyString, handle, "mpv_set_property_string")
		purego.RegisterLibFunc(&p.getProperty, handle, "mpv_get_property")
		purego.RegisterLibFunc(&p.getPropertyString, handle, "mpv_get_property_string")
		purego.RegisterLibFunc(&p.terminateDestroy, handle, "mpv_terminate_destroy")
		purego.RegisterLibFunc(&p.errorString, handle, "mpv_error_string")
		purego.RegisterLibFunc(&p.free, handle, "mpv_free")
		return nil
	}()
	if registerErr != nil {
		p.Close()
		return nil, registerErr
	}
	p.ctx = p.create()
	if p.ctx == nil {
		p.Close()
		return nil, fmt.Errorf("mpv_create returned nil")
	}

	// The patched macOS macvk output interprets wid as the NSView pointer.
	// Windows continues to use its existing HWND/D3D11 implementation.
	options := [][2]string{
		{"wid", strconv.FormatUint(uint64(videoNSView), 10)},
		{"vo", "gpu-next"},
		{"gpu-api", "vulkan"},
		{"gpu-context", "macvk"},
		{"hwdec", "videotoolbox"},
		{"keep-open", "yes"},
		{"input-default-bindings", "yes"},
		{"input-vo-keyboard", "yes"},
	}
	for _, option := range options {
		if code := p.setOptionString(p.ctx, option[0], option[1]); code < 0 {
			message := p.errorText(code)
			p.Close()
			return nil, fmt.Errorf("set MPV option %q: %s", option[0], message)
		}
	}
	if code := p.initialize(p.ctx); code < 0 {
		message := p.errorText(code)
		p.Close()
		return nil, fmt.Errorf("mpv_initialize: %s", message)
	}
	p.runtimeVideoOutputReinit = supportsRuntimeVideoOutputReinit(p.version())
	return p, nil
}

func (p *Player) commandArgs(args ...string) error {
	if p == nil || p.ctx == nil {
		return fmt.Errorf("MPV is not initialized")
	}
	if len(args) == 0 {
		return fmt.Errorf("MPV command has no arguments")
	}
	cstrings := make([][]byte, len(args))
	pointers := make([]uintptr, len(args)+1)
	for i, arg := range args {
		if strings.IndexByte(arg, 0) >= 0 {
			return fmt.Errorf("MPV command argument contains a NUL byte")
		}
		cstrings[i] = append([]byte(arg), 0)
		pointers[i] = uintptr(unsafe.Pointer(&cstrings[i][0]))
	}
	code := p.command(p.ctx, unsafe.Pointer(&pointers[0]))
	runtime.KeepAlive(cstrings)
	runtime.KeepAlive(pointers)
	if code < 0 {
		return fmt.Errorf("MPV command %q: %s (code %d)", args[0], p.errorText(code), code)
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
	if p.videoOutputReleased {
		if err := p.setProperty("vo", "gpu-next"); err != nil {
			return fmt.Errorf("restore MPV macvk video output: %w", err)
		}
		p.videoOutputReleased = false
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
	if p == nil || p.ctx == nil {
		return 0, fmt.Errorf("MPV is not initialized")
	}
	var value int64
	code := p.getProperty(p.ctx, name, formatInt64, unsafe.Pointer(&value))
	runtime.KeepAlive(&value)
	if code < 0 {
		return 0, fmt.Errorf("get MPV property %q: %s", name, p.errorText(code))
	}
	return value, nil
}

func (p *Player) GetDouble(name string) (float64, error) {
	if p == nil || p.ctx == nil {
		return 0, fmt.Errorf("MPV is not initialized")
	}
	var value float64
	code := p.getProperty(p.ctx, name, formatDouble, unsafe.Pointer(&value))
	runtime.KeepAlive(&value)
	if code < 0 {
		return 0, fmt.Errorf("get MPV property %q: %s", name, p.errorText(code))
	}
	return value, nil
}

func (p *Player) GetVideoHeight() (int, error) {
	height, err := p.getInt64("height")
	if err != nil {
		height, err = p.getInt64("video-params/h")
	}
	if err != nil {
		return 0, err
	}
	if height < 0 {
		return 0, fmt.Errorf("MPV returned a negative video height")
	}
	return int(height), nil
}

func (p *Player) setProperty(name, value string) error {
	if p == nil || p.ctx == nil {
		return fmt.Errorf("MPV is not initialized")
	}
	code := p.setPropertyString(p.ctx, name, value)
	if code < 0 {
		return fmt.Errorf("set MPV property %q: %s", name, p.errorText(code))
	}
	return nil
}

func (p *Player) SetPaused(paused bool) error {
	value := "no"
	if paused { value = "yes" }
	return p.setProperty("pause", value)
}

func (p *Player) Seek(seconds float64) error {
	if seconds < 0 { seconds = 0 }
	return p.commandArgs("seek", strconv.FormatFloat(seconds, 'f', 3, 64), "absolute+exact")
}

func (p *Player) SetVolume(volume float64) error {
	volume = min(100, max(0, volume))
	return p.setProperty("volume", strconv.FormatFloat(volume, 'f', 1, 64))
}

func (p *Player) Stop() error {
	if err := p.SetPaused(true); err != nil { return err }
	return p.Seek(0)
}

func (p *Player) Unload() error {
	if err := p.commandArgs("stop"); err != nil { return err }
	if !p.runtimeVideoOutputReinit {
		return fmt.Errorf("this MPV build does not support runtime video-output release")
	}
	if err := p.setProperty("vo", "null"); err != nil { return err }
	p.videoOutputReleased = true
	return nil
}

func (p *Player) CaptureThumbnail(path string) error {
	if strings.TrimSpace(path) == "" { return fmt.Errorf("thumbnail path is empty") }
	abs, err := filepath.Abs(path)
	if err != nil { return err }
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Errorf("create thumbnail cache: %w", err)
	}
	return p.commandArgs("screenshot-to-file", abs, "video")
}

func (p *Player) LoadExternalSubtitle(path string) error {
	path = strings.TrimSpace(path)
	if path == "" { return fmt.Errorf("no subtitle file selected") }
	abs, err := filepath.Abs(path)
	if err != nil { return err }
	info, err := os.Stat(abs)
	if err != nil { return fmt.Errorf("subtitle file is unavailable: %w", err) }
	if info.IsDir() { return fmt.Errorf("subtitle path is a directory") }
	return p.commandArgs("sub-add", abs, "select")
}

func (p *Player) CycleSubtitleTrack() error {
	return p.commandArgs("cycle", "sid")
}

func (p *Player) RemoveSelectedExternalSubtitle() error {
	id, err := p.getInt64("sid")
	if err != nil { return err }
	if id < 0 { return fmt.Errorf("no subtitle track is selected") }
	external, err := p.getPropertyStringValue(fmt.Sprintf("track-list/%d/external", id))
	if err != nil || external != "yes" {
		return fmt.Errorf("the selected subtitle is not an external track")
	}
	return p.commandArgs("sub-remove", strconv.FormatInt(id, 10))
}

func (p *Player) SetSubtitleVisibility(visible bool) error {
	value := "no"
	if visible { value = "yes" }
	return p.setProperty("sub-visibility", value)
}

func (p *Player) DisableSubtitles() error {
	return p.SetSubtitleVisibility(false)
}

func (p *Player) SetSubtitleDelay(seconds float64) error {
	if seconds < -3600 || seconds > 3600 {
		return fmt.Errorf("subtitle delay must be between -3600 and 3600 seconds")
	}
	return p.setProperty("sub-delay", strconv.FormatFloat(seconds, 'f', 3, 64))
}

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
		if err := p.setProperty(property.name, property.value); err != nil { return err }
	}
	return nil
}

func (p *Player) getPropertyStringValue(name string) (string, error) {
	if p == nil || p.ctx == nil { return "", fmt.Errorf("MPV is not initialized") }
	ptr := p.getPropertyString(p.ctx, name)
	if ptr == nil { return "", fmt.Errorf("MPV property %q is unavailable", name) }
	defer p.free(ptr)
	return cString(ptr), nil
}

func (p *Player) ActiveSubtitleLanguage() (string, error) {
	if p == nil || p.ctx == nil { return "", fmt.Errorf("MPV is not initialized") }
	ptr := p.getPropertyString(p.ctx, "current-tracks/sub/lang")
	if ptr == nil { return "", nil }
	defer p.free(ptr)
	return cString(ptr), nil
}

func (p *Player) Close() {
	if p == nil { return }
	if p.ctx != nil {
		p.terminateDestroy(p.ctx)
		p.ctx = nil
	}
	if p.dll != 0 {
		_ = purego.Dlclose(p.dll)
		p.dll = 0
	}
}
