//go:build !windows

package windows

import (
	"errors"
	"my-go/internal/domain"
)

// Platform is a build-time placeholder. Aurora's native video host is Windows-only.
type Platform struct{}

func New() (*Platform, error) {
	return nil, errors.New("Aurora Player's native video host is supported only on Windows")
}
func (*Platform) CreateVideoHost(uintptr) (uintptr, error)      { return 0, errors.New("Windows only") }
func (*Platform) InstallVideoDoubleClick(uintptr, func()) error { return errors.New("Windows only") }
func (*Platform) LayoutVideo(uintptr, uintptr, bool, domain.Rect) error {
	return errors.New("Windows only")
}
func (*Platform) CursorInsideWindow(uintptr) (bool, domain.Point) { return false, domain.Point{} }
func (*Platform) WindowDPI(uintptr) uint32                        { return 96 }
func (*Platform) ShowCursor(int32) int32                          { return 0 }
func (*Platform) ShowWindow(uintptr, int32) int32                 { return 0 }
func (*Platform) UpdateWindow(uintptr) int32                      { return 0 }
func (*Platform) RevealChild(uintptr) error                       { return errors.New("Windows only") }
func (*Platform) DestroyWindow(uintptr) int32                     { return 0 }
func (*Platform) RemoveVideoSubclass(uintptr)                     {}
func (*Platform) LogHWND(string, uintptr)                         {}
func (*Platform) Close()                                          {}
