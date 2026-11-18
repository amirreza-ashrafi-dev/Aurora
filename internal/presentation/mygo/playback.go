package presentation

import (
	"github.com/egoist/mygo/ui"
	"my-go/internal/application"
)

func (a *App) togglePlayPause() {
	if a.player == nil {
		return
	}
	next := !a.state.Paused
	if err := a.player.SetPaused(next); err != nil {
		a.status = err.Error()
		return
	}
	a.state.Paused = next
	if next {
		a.status = "Paused"
	} else {
		a.status = "Playing"
	}
}

func (a *App) seekBy(seconds float64) {
	if a.player == nil {
		return
	}
	target := application.SeekTarget(a.state.Position, seconds, a.state.Duration)
	if err := a.player.Seek(target); err != nil {
		a.status = err.Error()
		return
	}
	a.state.Position = target
}

func (a *App) changeVolume(amount float64) {
	if a.player == nil {
		return
	}
	next := application.ClampVolume(a.state.Volume + amount)
	if err := a.player.SetVolume(next); err != nil {
		a.status = err.Error()
		return
	}
	a.state.Volume = next
}

func (a *App) toggleFullscreen() {
	if a.win != nil {
		a.win.ToggleFullScreen()
	}
}
func (a *App) exitFullscreen() {
	if a.win != nil && a.win.IsFullScreen() {
		a.win.SetFullScreen(false)
	}
}

func (a *App) registerShortcuts(c *ui.Context) {
	c.OnWindowShortcut(0, ui.KeyF, a.toggleFullscreen)
	c.OnWindowShortcut(0, ui.KeyF11, a.toggleFullscreen)
	c.OnWindowShortcut(ui.Alt, ui.KeyEnter, a.toggleFullscreen)
	c.OnWindowShortcut(0, ui.KeyEscape, a.exitFullscreen)
	c.OnWindowShortcut(0, ui.KeySpace, a.togglePlayPause)
	c.OnWindowShortcut(0, ui.KeyLeft, func() { a.seekBy(-5) })
	c.OnWindowShortcut(0, ui.KeyRight, func() { a.seekBy(5) })
	c.OnWindowShortcut(0, ui.KeyUp, func() { a.changeVolume(5) })
	c.OnWindowShortcut(0, ui.KeyDown, func() { a.changeVolume(-5) })
}
