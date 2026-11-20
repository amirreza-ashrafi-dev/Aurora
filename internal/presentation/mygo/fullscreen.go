package presentation

import (
	"log"
	"time"

	mygo "github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"my-go/internal/domain"
)

const (
	controllerWidth  = 896
	controllerHeight = 194
	controllerBottom = 32
)

func (a *App) showFullscreenHeader() {
	if a == nil || a.win == nil || a.win.IsDestroyed() {
		return
	}
	bounds := a.win.ContentBounds()
	if bounds.Width < 1 {
		return
	}
	width := min(bounds.Width, 1280)
	if width < 320 {
		width = bounds.Width
	}
	anchor := mygo.Rectangle{X: bounds.Width / 2, Y: 0, Width: 1, Height: 1}
	if a.fullscreenHeader != nil && !a.fullscreenHeader.IsDestroyed() {
		if anchor != a.fullscreenHeaderAnchor {
			a.fullscreenHeader.SetAnchor(anchor)
			a.fullscreenHeaderAnchor = anchor
		}
		if !a.fullscreenHeader.IsVisible() {
			a.fullscreenHeader.ShowInactive()
		}
		return
	}
	a.fullscreenHeader = mygo.NewFlyout(mygo.FlyoutOptions{
		Parent: a.win, Anchor: anchor, Placement: mygo.PlacementBottom,
		Gap: 0, Width: width, Height: 72, Focusable: false,
		Hidden: true, Content: ui.View(a.fullscreenHeaderView),
	})
	a.fullscreenHeaderAnchor = anchor
	a.fullscreenHeader.ShowInactive()
}

func (a *App) hideFullscreenHeader() {
	if a.fullscreenHeader != nil && !a.fullscreenHeader.IsDestroyed() {
		a.fullscreenHeader.Hide()
	}
}

func (a *App) hideResumeBanner() {
	if a.resumeBanner != nil && !a.resumeBanner.IsDestroyed() {
		a.resumeBanner.Hide()
	}
}

func (a *App) fullscreenHeaderView(c *ui.Context) {
	applyAuroraTheme(c)
	c.Root().Background(ui.Transparent)
	ui.Row(c).Fill().Padding(10, 16).Gap(10).
		Background(ui.Hex("#101211")).
		Border(1, ui.Hex(auroraBorderSoft)).Radius(12).
		AlignItems(ui.Center).Children(func() {
			if logo := auroraLogo(); logo != nil {
				ui.Image(c, logo).Width(28).Height(28).Fit(ui.Contain)
			}
			ui.Column(c).Gap(1).Children(func() {
				ui.Text(c, "AURORA CINEMA").FontSize(10).Bold().
					LetterSpacing(1.6).TextColor(ui.Hex(auroraAccent))
				ui.Text(c, "MASTER PLAYBACK SUITE").FontSize(7).
					LetterSpacing(0.8).TextColor(ui.Hex(auroraMuted))
			})
			ui.Spacer(c)
			if a.win != nil && a.win.IsFullScreen() {
				if auroraQuietButton(c, "⛶  Exit fullscreen").Clicked() {
					a.exitFullscreen()
				}
			} else if a.cinematicMode {
				if auroraQuietButton(c, "Exit cinematic").Clicked() {
					a.toggleCinematicMode()
				}
			}
		})
}

func (a *App) controllerAnchorTarget() (mygo.Rectangle, bool) {
	if a == nil || a.win == nil || a.win.IsDestroyed() {
		return mygo.Rectangle{}, false
	}
	bounds := a.win.ContentBounds()
	if bounds.Width < 1 || bounds.Height < 1 {
		return mygo.Rectangle{}, false
	}
	y := bounds.Height - controllerBottom
	if y < 0 {
		y = 0
	}
	return mygo.Rectangle{X: 0, Y: y, Width: bounds.Width, Height: 1}, true
}

func (a *App) createFullscreenController() {
	if a == nil || a.win == nil || a.win.IsDestroyed() {
		return
	}
	if a.controller != nil && !a.controller.IsDestroyed() {
		return
	}
	anchor, ok := a.controllerAnchorTarget()
	if !ok {
		log.Println("controller: main window has no usable content bounds yet; deferring")
		return
	}
	a.controller = mygo.NewFlyout(mygo.FlyoutOptions{
		Parent:    a.win,
		Anchor:    anchor,
		Placement: mygo.PlacementTop,
		Gap:       0,
		Width:     controllerWidth,
		Height:    controllerHeight,
		Focusable: false,
		Hidden:    true,
		Content:   ui.View(a.fullscreenControls),
	})
	a.controllerAnchor, a.hasControllerAnchor = anchor, true
	a.logDiagnostics("after-create-controller-flyout")
}

func (a *App) updateControllerBounds() {
	if a == nil || a.win == nil || a.win.IsDestroyed() || a.controller == nil || a.controller.IsDestroyed() {
		return
	}
	anchor, ok := a.controllerAnchorTarget()
	if !ok {
		return
	}
	if a.hasControllerAnchor && anchor == a.controllerAnchor {
		a.updateFullscreenSubtitleFlyoutAnchor()
		return
	}
	log.Printf("[diag] updateControllerAnchor: old=%+v new=%+v", a.controllerAnchor, anchor)
	a.controller.SetAnchor(anchor)
	a.controllerAnchor, a.hasControllerAnchor = anchor, true
	a.updateFullscreenSubtitleFlyoutAnchor()
}

func (a *App) updateFullscreenSubtitleFlyoutAnchor() {
	if a == nil || a.win == nil || a.win.IsDestroyed() ||
		a.controller == nil || a.controller.IsDestroyed() ||
		a.fullscreenSubtitleFlyout == nil || a.fullscreenSubtitleFlyout.IsDestroyed() {
		return
	}
	button := a.fullscreenSubtitleButtonBounds
	if button.Width <= 0 || button.Height <= 0 {
		return
	}

	controllerContent := a.controller.ContentBounds()
	mainContent := a.win.ContentBounds()
	anchor := mygo.Rectangle{
		X:      controllerContent.X - mainContent.X + button.X,
		Y:      controllerContent.Y - mainContent.Y + button.Y,
		Width:  button.Width,
		Height: button.Height,
	}
	if anchor != a.fullscreenSubtitleFlyoutAnchor {
		a.fullscreenSubtitleFlyout.SetAnchor(anchor)
		a.fullscreenSubtitleFlyoutAnchor = anchor
	}
}

func (a *App) showController() {
	if a.controller == nil || a.controller.IsDestroyed() {
		a.createFullscreenController()
	}
	if a.controller == nil || a.controller.IsDestroyed() {
		return
	}
	a.updateControllerBounds()
	a.showFullscreenHeader()
	if a.pendingResume != nil {
		a.showResumeBanner()
	} else {
		a.hideResumeBanner()
	}
	if !a.controller.IsVisible() {
		a.controller.ShowInactive()
	}
}

func (a *App) hideController() {
	a.closeFullscreenSubtitleFlyout()
	if a.controller != nil && !a.controller.IsDestroyed() && a.controller.IsVisible() {
		a.controller.Hide()
	}
	a.hideFullscreenHeader()
	a.hideResumeBanner()
}

func (a *App) startControllerMonitor() {
	if a.controllerStop != nil || a.platform == nil || a.win == nil {
		return
	}
	a.controllerStop = make(chan struct{})
	a.controllerDone = make(chan struct{})
	go func() {
		defer close(a.controllerDone)
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		var lastPoint domain.Point
		haveLastPoint := false
		var lastActivity, lastMouseMovement time.Time
		wasOverlayActive, visible := false, false
		wasResumePending := false
		const hideDelay = 3 * time.Second
		for {
			select {
			case <-a.controllerStop:
				return
			case <-ticker.C:
				mygo.RunOnMain(func() {
					if a.win == nil || a.win.IsDestroyed() {
						return
					}

					overlayActive := a.win.IsFullScreen() || a.cinematicMode
					if !overlayActive {
						a.hideController()
						a.closeResumeBanner()
						visible = false
						a.setFullscreenCursorHidden(false)
						wasOverlayActive = false
						wasResumePending = false
						haveLastPoint = false
						lastMouseMovement = time.Time{}
						return
					}

					if a.controller == nil || a.controller.IsDestroyed() {
						a.createFullscreenController()
					}
					if a.controller == nil || a.controller.IsDestroyed() {
						a.setFullscreenCursorHidden(false)
						return
					}
					a.updateControllerBounds()
					if !wasOverlayActive {
						a.showController()
						visible = a.controller.IsVisible()
						now := time.Now()
						lastActivity, lastMouseMovement = now, now
						haveLastPoint = false
						wasOverlayActive = true
					}

					// Fullscreen callbacks or a newly offered resume prompt can show
					// the flyouts outside this monitor. Treat that transition as
					// fresh activity so the entire overlay group gets its full delay.
					visible = a.controller.IsVisible()
					resumePending := a.pendingResume != nil
					if resumePending && !wasResumePending {
						a.showController()
						visible = a.controller.IsVisible()
						now := time.Now()
						lastActivity, lastMouseMovement = now, now
						a.setFullscreenCursorHidden(false)
					}
					wasResumePending = resumePending

					insideMain, point := a.platform.CursorInsideWindow(a.win.NativeHandle())
					insideController, _ := a.platform.CursorInsideWindow(a.controller.NativeHandle())
					// Treat every auxiliary flyout as an interactive surface. This
					// prevents the group from hiding while the pointer is over the
					// top bar, resume banner, or subtitle panel.
					for _, overlay := range []*mygo.Window{
						a.fullscreenHeader,
						a.resumeBanner,
						a.fullscreenSubtitleFlyout,
					} {
						if overlay == nil || overlay.IsDestroyed() || !overlay.IsVisible() {
							continue
						}
						insideOverlay, _ := a.platform.CursorInsideWindow(overlay.NativeHandle())
						insideController = insideController || insideOverlay
					}
					if !insideMain && !insideController {
						if visible {
							a.hideController()
							visible = false
						}
						a.setFullscreenCursorHidden(false)
						lastMouseMovement = time.Time{}
						haveLastPoint = false
						return
					}

					if insideController {
						if !visible {
							a.showController()
							visible = true
						}
						now := time.Now()
						lastActivity, lastMouseMovement = now, now
						a.setFullscreenCursorHidden(false)
					}

					moved := !haveLastPoint || point.X != lastPoint.X || point.Y != lastPoint.Y
					if moved {
						now := time.Now()
						lastActivity, lastMouseMovement = now, now
						a.setFullscreenCursorHidden(false)
						if !visible {
							a.showController()
							visible = true
						}
					}
					if visible && !insideController && time.Since(lastActivity) >= hideDelay {
						a.hideController()
						visible = false
					}
					if !lastMouseMovement.IsZero() && !a.cursorHidden && time.Since(lastMouseMovement) >= hideDelay {
						a.setFullscreenCursorHidden(true)
					}
					lastPoint, haveLastPoint = point, true
				})
			}
		}
	}()
}

func (a *App) stopControllerMonitor() {
	if a.controllerStop == nil {
		return
	}
	close(a.controllerStop)
	if a.controllerDone != nil {
		<-a.controllerDone
	}
	a.controllerStop, a.controllerDone = nil, nil
}
