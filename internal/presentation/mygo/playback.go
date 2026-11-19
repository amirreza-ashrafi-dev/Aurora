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
		a.scheduleCurrentProgressSave()
	} else {
		a.status = "Playing"
		a.cancelPendingProgressSave()
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
	a.scheduleCurrentProgressSave()
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

func (a *App) toggleCinematicMode() {
	if a == nil {
		return
	}
	a.setCinematicMode(!a.cinematicMode)
}

func (a *App) setCinematicMode(enabled bool) {
	if a == nil || a.win == nil || a.win.IsDestroyed() || a.cinematicMode == enabled {
		return
	}
	a.cinematicMode = enabled
	a.videoLayoutFramePending = false
	a.win.Invalidate()
	if a.controller != nil && !a.controller.IsDestroyed() {
		a.controller.Invalidate()
	}
	if enabled {
		a.showController()
		return
	}
	if !a.win.IsFullScreen() {
		a.hideController()
		a.setFullscreenCursorHidden(false)
	}
}

func (a *App) registerShortcuts(c *ui.Context) {
	c.OnWindowShortcut(0, ui.KeyF, a.toggleFullscreen)
	c.OnWindowShortcut(0, ui.KeyF11, a.toggleFullscreen)
	c.OnWindowShortcut(ui.Alt, ui.KeyEnter, a.toggleFullscreen)
	c.OnWindowShortcut(ui.Alt, ui.KeyLeft, func() { a.adjustSubtitleDelay(-0.1) })
	c.OnWindowShortcut(ui.Alt, ui.KeyRight, func() { a.adjustSubtitleDelay(0.1) })
	c.OnWindowShortcut(ui.Alt, ui.KeyBackspace, func() { a.resetSubtitleDelay() })
	c.OnWindowShortcut(0, ui.KeyC, a.toggleCinematicMode)
	c.OnWindowShortcut(0, ui.KeyEscape, func() {
		if a.cinematicMode {
			a.setCinematicMode(false)
			return
		}
		a.exitFullscreen()
	})
	c.OnWindowShortcut(0, ui.KeySpace, a.togglePlayPause)
	c.OnWindowShortcut(0, ui.KeyLeft, func() { a.seekBy(-5) })
	c.OnWindowShortcut(0, ui.KeyRight, func() { a.seekBy(5) })
	c.OnWindowShortcut(0, ui.KeyUp, func() { a.changeVolume(5) })
	c.OnWindowShortcut(0, ui.KeyDown, func() { a.changeVolume(-5) })
}
