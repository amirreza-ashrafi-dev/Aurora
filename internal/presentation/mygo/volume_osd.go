package presentation

import (
	"fmt"
	"time"

	mygo "github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	volumeWheelDetent = int32(120)
	volumeStepPercent = 5.0
	volumeOSDDuration = 1200 * time.Millisecond
)

func (a *App) handleVolumeWheel(delta int32) {
	if a == nil || delta == 0 { return }
	a.wheelRemainder += delta
	steps := a.wheelRemainder / volumeWheelDetent
	a.wheelRemainder %= volumeWheelDetent
	if steps != 0 { a.changeVolume(float64(steps) * volumeStepPercent) }
}

func (a *App) showVolumeOSD() {
	if a == nil || a.win == nil || a.win.IsDestroyed() { return }
	bounds := a.win.ContentBounds()
	if bounds.Width < 1 || bounds.Height < 1 { return }
	anchor := mygo.Rectangle{X: bounds.Width/2, Y: bounds.Height/2, Width: 1, Height: 1}
	if a.volumeOSD != nil && !a.volumeOSD.IsDestroyed() {
		a.volumeOSD.SetAnchor(anchor)
		a.volumeOSD.Invalidate()
		if !a.volumeOSD.IsVisible() { a.volumeOSD.ShowInactive() }
		a.volumeOSDUntil = time.Now().Add(volumeOSDDuration)
		return
	}
	a.volumeOSD = mygo.NewFlyout(mygo.FlyoutOptions{
		Parent: a.win, Anchor: anchor, Placement: mygo.PlacementBottom,
		Gap: 0, Width: 190, Height: 62, Focusable: false, Hidden: true,
		Content: ui.View(a.volumeOSDView),
	})
	a.volumeOSDUntil = time.Now().Add(volumeOSDDuration)
	a.volumeOSD.ShowInactive()
}

func (a *App) hideVolumeOSD() {
	if a.volumeOSD != nil && !a.volumeOSD.IsDestroyed() && a.volumeOSD.IsVisible() { a.volumeOSD.Hide() }
	a.volumeOSDUntil = time.Time{}
}

func (a *App) closeVolumeOSD() {
	if a.volumeOSD != nil {
		if !a.volumeOSD.IsDestroyed() { a.volumeOSD.Destroy() }
		a.volumeOSD = nil
	}
	a.volumeOSDUntil = time.Time{}
}

func (a *App) volumeOSDView(c *ui.Context) {
	applyAuroraTheme(c)
	c.Root().Background(ui.Transparent)
	volume := a.state.Volume
	if volume < 0 { volume = 0 }
	if volume > 100 { volume = 100 }
	ui.Column(c).Fill().Padding(10).Children(func() {
		ui.Column(c).Fill().Padding(10).Gap(5).
			Background(ui.Hex("#11100D")).Border(1, ui.Hex("#80672D")).Radius(12).
			Children(func() {
				ui.Text(c, "VOLUME").FontSize(8).Bold().TextColor(ui.Hex(auroraMuted))
				ui.Text(c, fmt.Sprintf("%.0f%%", volume)).FontSize(20).Bold().TextColor(ui.Hex(auroraAccent))
			})
	})
}
