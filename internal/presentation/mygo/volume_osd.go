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
	volumeOSDWidth    = 180
	volumeOSDHeight   = 66
)

func (a *App) handleVolumeWheel(delta int32) {
	if a == nil || delta == 0 {
		return
	}
	a.wheelRemainder += delta
	steps := a.wheelRemainder / volumeWheelDetent
	a.wheelRemainder %= volumeWheelDetent
	if steps != 0 {
		a.changeVolume(float64(steps) * volumeStepPercent)
	}
}

func (a *App) volumeOSDTarget() (mygo.Rectangle, bool) {
	if a == nil || a.win == nil || a.win.IsDestroyed() {
		return mygo.Rectangle{}, false
	}
	bounds := a.win.ContentBounds()
	if bounds.Width < 1 || bounds.Height < 1 {
		return mygo.Rectangle{}, false
	}

	// Use the real video viewport in windowed and Cinematic Mode. Fullscreen
	// uses the whole client area because the native MPV child fills that area.
	left, top := 0, 0
	viewportWidth, viewportHeight := bounds.Width, bounds.Height
	if !a.fullscreen && !a.win.IsFullScreen() && a.hasVideoStageBounds {
		left = int(a.videoStageBounds.X)
		top = int(a.videoStageBounds.Y)
		viewportWidth = int(a.videoStageBounds.W)
		viewportHeight = int(a.videoStageBounds.H)
	}
	if viewportWidth <= 0 || viewportHeight <= 0 {
		return mygo.Rectangle{}, false
	}

	// MyGo's bottom flyout placement centers the popup on its anchor. Anchor
	// at the center of the desired card rectangle, not at its left edge; this
	// keeps the entire 180-DIP indicator inside the video/window.
	const inset = 16
	cardLeft := left + inset
	maxCardLeft := left + viewportWidth - inset - volumeOSDWidth
	if maxCardLeft < cardLeft {
		// Narrow viewport fallback: center the indicator in the video region.
		cardLeft = left + (viewportWidth-volumeOSDWidth)/2
	}
	if cardLeft < 0 {
		cardLeft = 0
	}
	if cardLeft+volumeOSDWidth > bounds.Width {
		cardLeft = bounds.Width - volumeOSDWidth
		if cardLeft < 0 {
			cardLeft = 0
		}
	}

	cardTop := top + inset
	if cardTop+volumeOSDHeight > bounds.Height {
		cardTop = bounds.Height - volumeOSDHeight
		if cardTop < 0 {
			cardTop = 0
		}
	}
	return mygo.Rectangle{
		X: cardLeft + volumeOSDWidth/2,
		Y: cardTop,
		Width: 1,
		Height: 1,
	}, true
}

func (a *App) showVolumeOSD() {
	if a == nil {
		return
	}
	anchor, ok := a.volumeOSDTarget()
	if !ok {
		return
	}
	if a.volumeOSD != nil && !a.volumeOSD.IsDestroyed() {
		a.volumeOSD.SetAnchor(anchor)
		a.volumeOSD.Invalidate()
		if !a.volumeOSD.IsVisible() {
			a.volumeOSD.ShowInactive()
		}
		a.volumeOSDUntil = time.Now().Add(volumeOSDDuration)
		return
	}
	a.volumeOSD = mygo.NewFlyout(mygo.FlyoutOptions{
		Parent: a.win,
		Anchor: anchor,
		Placement: mygo.PlacementBottom,
		Gap: 0,
		Width: volumeOSDWidth,
		Height: volumeOSDHeight,
		Focusable: false,
		Hidden: true,
		Content: ui.View(a.volumeOSDView),
	})
	a.volumeOSDUntil = time.Now().Add(volumeOSDDuration)
	a.volumeOSD.ShowInactive()
}

func (a *App) hideVolumeOSD() {
	if a.volumeOSD != nil && !a.volumeOSD.IsDestroyed() && a.volumeOSD.IsVisible() {
		a.volumeOSD.Hide()
	}
	a.volumeOSDUntil = time.Time{}
}

func (a *App) closeVolumeOSD() {
	if a.volumeOSD != nil {
		if !a.volumeOSD.IsDestroyed() {
			a.volumeOSD.Destroy()
		}
		a.volumeOSD = nil
	}
	a.volumeOSDUntil = time.Time{}
}

func (a *App) volumeOSDView(c *ui.Context) {
	applyAuroraTheme(c)
	c.Root().Background(ui.Transparent)

	volume := a.state.Volume
	if volume < 0 {
		volume = 0
	}
	if volume > 100 {
		volume = 100
	}

	// Keep the percentage in a fixed-width, single-line slot. The previous
	// stacked labels relied on intrinsic sizing and could overflow at 100%.
	ui.Column(c).Fill().Padding(4).Children(func() {
		ui.Column(c).Fill().Padding(10, 12).Gap(6).
			Background(ui.Hex("#11100D")).
			Border(1, ui.Hex("#80672D")).
			Radius(12).
			Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Text(c, "VOLUME").
						FontSize(8).Bold().LetterSpacing(1.1).
						TextColor(ui.Hex(auroraMuted))
					ui.Spacer(c)
					ui.Text(c, fmt.Sprintf("%.0f%%", volume)).
						Width(58).SingleLine().
						FontSize(16).Bold().
						TextColor(ui.Hex(auroraAccent))
				})
				ui.Box(c).FillWidth().Height(4).Radius(2).
					Background(ui.Hex("#393326")).Clip().
					Children(func() {
						if volume > 0 {
							ui.Box(c).Width(float32(volume) * 1.52).Height(4).
								Radius(2).Background(ui.Hex(auroraAccent))
						}
					})
			})
	})
}
