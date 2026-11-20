//go:build !windows

package mpv

import (
	"errors"
	"my-go/internal/domain"
)

// Player is a build-time placeholder for non-Windows targets.
type Player struct{}

func New(uintptr, string) (*Player, error) {
	return nil, errors.New("the bundled libmpv adapter is currently Windows-only")
}
func (*Player) Load(string) error                 { return errors.New("Windows only") }
func (*Player) Append(string) error               { return errors.New("Windows only") }
func (*Player) PlayIndex(int) error               { return errors.New("Windows only") }
func (*Player) RemovePlaylistItem(int) error        { return errors.New("Windows only") }
func (*Player) GetDouble(string) (float64, error) { return 0, errors.New("Windows only") }
func (*Player) GetVideoHeight() (int, error) { return 0, errors.New("Windows only") }
func (*Player) SetPaused(bool) error              { return errors.New("Windows only") }
func (*Player) Seek(float64) error                { return errors.New("Windows only") }
func (*Player) SetVolume(float64) error           { return errors.New("Windows only") }
func (*Player) Stop() error                       { return errors.New("Windows only") }
func (*Player) Unload() error                     { return errors.New("Windows only") }
func (*Player) SupportsRuntimeVideoOutputRelease() bool { return false }
func (*Player) CaptureThumbnail(string) error { return errors.New("Windows only") }
func (*Player) Close()                            {}

func (*Player) LoadExternalSubtitle(string) error { return errors.New("Windows only") }
func (*Player) CycleSubtitleTrack() error { return errors.New("Windows only") }
func (*Player) RemoveSelectedExternalSubtitle() error { return errors.New("Windows only") }
func (*Player) SetSubtitleVisibility(bool) error { return errors.New("Windows only") }
func (*Player) DisableSubtitles() error { return errors.New("Windows only") }
func (*Player) SetSubtitleDelay(float64) error { return errors.New("Windows only") }
func (*Player) SetSubtitleStyle(domain.SubtitleStyle) error { return errors.New("Windows only") }
func (*Player) ActiveSubtitleLanguage() (string, error) { return "", errors.New("Windows only") }
