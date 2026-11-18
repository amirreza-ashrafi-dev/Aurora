package application

import "my-go/internal/domain"

// Player is the application-facing port implemented by the libmpv adapter.
type Player interface {
	Load(path string) error
	Append(path string) error
	PlayIndex(index int) error
	GetDouble(name string) (float64, error)
	SetPaused(paused bool) error
	Seek(seconds float64) error
	SetVolume(volume float64) error
	Stop() error
	Close()
}

// NativePlatform is the narrow Win32 port required by the player presentation.
// It deliberately uses domain geometry types and has no MyGo dependency.
type NativePlatform interface {
	CreateVideoHost(parent uintptr) (uintptr, error)
	InstallVideoDoubleClick(hwnd uintptr, onDoubleClick func()) error
	LayoutVideo(parent, child uintptr, fullscreen bool, stage domain.Rect) error
	CursorInsideWindow(hwnd uintptr) (bool, domain.Point)
	WindowDPI(hwnd uintptr) uint32
	ShowCursor(show int32) int32
	ShowWindow(hwnd uintptr, command int32) int32
	UpdateWindow(hwnd uintptr) int32
	RevealChild(hwnd uintptr) error
	DestroyWindow(hwnd uintptr) int32
	RemoveVideoSubclass(hwnd uintptr)
	LogHWND(label string, hwnd uintptr)
	Close()
}

// PlayerFactory delays MPV initialization until the native video child HWND exists.
type PlayerFactory func(videoHWND uintptr) (Player, error)
