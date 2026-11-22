//go:build !windows && !darwin

package mpv

import (
	"errors"
	"my-go/internal/domain"
)

type Player struct{}

func New(uintptr, string) (*Player, error) { return nil, errors.New("the bundled libmpv adapter is supported only on Windows and macOS") }
func (*Player) Load(string) error { return errors.New("unsupported platform") }
func (*Player) Append(string) error { return errors.New("unsupported platform") }
func (*Player) PlayIndex(int) error { return errors.New("unsupported platform") }
func (*Player) RemovePlaylistItem(int) error { return errors.New("unsupported platform") }
func (*Player) GetDouble(string) (float64, error) { return 0, errors.New("unsupported platform") }
func (*Player) GetVideoHeight() (int, error) { return 0, errors.New("unsupported platform") }
func (*Player) SetPaused(bool) error { return errors.New("unsupported platform") }
func (*Player) Seek(float64) error { return errors.New("unsupported platform") }
func (*Player) SetVolume(float64) error { return errors.New("unsupported platform") }
func (*Player) Stop() error { return errors.New("unsupported platform") }
func (*Player) Unload() error { return errors.New("unsupported platform") }
func (*Player) SupportsRuntimeVideoOutputRelease() bool { return false }
func (*Player) CaptureThumbnail(string) error { return errors.New("unsupported platform") }
func (*Player) Close() {}
func (*Player) LoadExternalSubtitle(string) error { return errors.New("unsupported platform") }
func (*Player) CycleSubtitleTrack() error { return errors.New("unsupported platform") }
func (*Player) RemoveSelectedExternalSubtitle() error { return errors.New("unsupported platform") }
func (*Player) SetSubtitleVisibility(bool) error { return errors.New("unsupported platform") }
func (*Player) DisableSubtitles() error { return errors.New("unsupported platform") }
func (*Player) SetSubtitleDelay(float64) error { return errors.New("unsupported platform") }
func (*Player) SetSubtitleStyle(domain.SubtitleStyle) error { return errors.New("unsupported platform") }
func (*Player) ActiveSubtitleLanguage() (string, error) { return "", errors.New("unsupported platform") }
