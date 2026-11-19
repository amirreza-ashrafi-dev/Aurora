package presentation

import (
	"log"
	"time"

	mygo "github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"my-go/internal/domain"
)

const (
	controllerWidth  = 820
	controllerHeight = 240
	controllerBottom = 24
)

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
		return
	}
	log.Printf("[diag] updateControllerAnchor: old=%+v new=%+v", a.controllerAnchor, anchor)
	a.controller.SetAnchor(anchor)
	a.controllerAnchor, a.hasControllerAnchor = anchor, true
}

func (a *App) showController() {
	if a.controller == nil || a.controller.IsDestroyed() {
		a.createFullscreenController()
	}
	if a.controller == nil || a.controller.IsDestroyed() {
		return
	}
	a.updateControllerBounds()
	if !a.controller.IsVisible() {
		a.controller.ShowInactive()
	}
}

func (a *App) hideController() {
	if a.controller == nil || a.controller.IsDestroyed() {
		return
	}
	if a.controller.IsVisible() {
		a.controller.Hide()
	}
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
						if visible {
							a.hideController()
						}
						visible = false
						a.setFullscreenCursorHidden(false)
						wasOverlayActive = false
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
						visible = true
						now := time.Now()
						lastActivity, lastMouseMovement = now, now
						haveLastPoint = false
						wasOverlayActive = true
					}

					// Fullscreen callbacks can show the flyout while Cinematic Mode
					// is already active. Reconcile local monitor state with the
					// actual window visibility before applying the hide timeout.
					visible = a.controller.IsVisible()

					insideMain, point := a.platform.CursorInsideWindow(a.win.NativeHandle())
					insideController, _ := a.platform.CursorInsideWindow(a.controller.NativeHandle())
					if !insideMain && !insideController {
						if visible && a.pendingResume == nil {
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
					if visible && a.pendingResume == nil && !insideController && time.Since(lastActivity) >= hideDelay {
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
