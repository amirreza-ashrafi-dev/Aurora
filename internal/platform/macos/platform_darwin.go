//go:build darwin

package macos

import (
	"errors"
	"my-go/internal/domain"
)

// Platform leaves window ownership and video layout to MyGo. MPV's patched
// macvk output embeds its child Metal view in the MyGo-hosted NSView.
type Platform struct{}

func New() (*Platform, error) { return &Platform{}, nil }

func (*Platform) CreateVideoHost(uintptr) (uintptr, error) {
	return 0, errors.New("macOS video hosts are created through the MyGo native-view module")
}
func (*Platform) InstallVideoDoubleClick(uintptr, func()) error { return errors.ErrUnsupported }
func (*Platform) InstallVideoMouseWheel(uintptr, func(int32)) error { return errors.ErrUnsupported }
func (*Platform) InstallVideoFileDrop(uintptr, func([]string)) error { return nil }
func (*Platform) LayoutVideo(uintptr, uintptr, bool, domain.Rect) error { return nil }
func (*Platform) CursorInsideWindow(uintptr) (bool, domain.Point) { return false, domain.Point{} }
func (*Platform) WindowDPI(uintptr) uint32 { return 96 }
func (*Platform) ShowCursor(int32) int32 { return 0 }
func (*Platform) ShowWindow(uintptr, int32) int32 { return 0 }
func (*Platform) UpdateWindow(uintptr) int32 { return 0 }
func (*Platform) RevealChild(uintptr) error { return nil }
func (*Platform) DestroyWindow(uintptr) int32 { return 0 }
func (*Platform) RemoveVideoSubclass(uintptr) {}
func (*Platform) LogHWND(string, uintptr) {}
func (*Platform) Close() {}
