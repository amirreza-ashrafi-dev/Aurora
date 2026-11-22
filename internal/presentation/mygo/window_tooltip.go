package presentation

import (
	"time"

	mygo "github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const windowButtonTooltipDelay = 600 * time.Millisecond

func (a *App) syncWindowButtonTooltip(c *ui.Context, label string, bounds ui.Rect) {
	if a == nil || c == nil {
		return
	}
	if label == "" || a.win == nil || a.win.IsDestroyed() {
		a.windowButtonTooltipLabel = ""
		a.windowButtonTooltipSince = time.Time{}
		a.windowButtonTooltipPending = false
		a.hideWindowButtonTooltip()
		return
	}

	anchor := mygo.Rectangle{
		X:      int(bounds.X),
		Y:      int(bounds.Y),
		Width:  int(bounds.W),
		Height: int(bounds.H),
	}
	if anchor.Width < 1 || anchor.Height < 1 {
		return
	}

	if label != a.windowButtonTooltipLabel {
		a.windowButtonTooltipLabel = label
		a.windowButtonTooltipAnchor = anchor
		a.windowButtonTooltipSince = time.Now()
		a.windowButtonTooltipPending = true
		a.hideWindowButtonTooltip()
		c.After(windowButtonTooltipDelay)
		return
	}

	if anchor != a.windowButtonTooltipAnchor {
		a.windowButtonTooltipAnchor = anchor
		if a.windowButtonTooltipFlyout != nil && !a.windowButtonTooltipFlyout.IsDestroyed() {
			a.windowButtonTooltipFlyout.SetAnchor(anchor)
		}
	}

	if a.windowButtonTooltipFlyout != nil &&
		!a.windowButtonTooltipFlyout.IsDestroyed() &&
		a.windowButtonTooltipFlyout.IsVisible() {
		a.windowButtonTooltipFlyout.Invalidate()
		return
	}

	if remaining := windowButtonTooltipDelay - time.Since(a.windowButtonTooltipSince); remaining > 0 {
		if !a.windowButtonTooltipPending {
			a.windowButtonTooltipPending = true
			c.After(remaining)
		}
		return
	}

	a.windowButtonTooltipPending = false
	a.showWindowButtonTooltip()
}

func (a *App) showWindowButtonTooltip() {
	if a == nil || a.win == nil || a.win.IsDestroyed() || a.windowButtonTooltipLabel == "" {
		return
	}

	if a.windowButtonTooltipFlyout == nil || a.windowButtonTooltipFlyout.IsDestroyed() {
		a.windowButtonTooltipFlyout = mygo.NewFlyout(mygo.FlyoutOptions{
			Parent:    a.win,
			Anchor:    a.windowButtonTooltipAnchor,
			Placement: mygo.PlacementBottom,
			Gap:       3,
			Width:     148,
			Height:    32,
			Focusable: false,
			Hidden:    true,
			Shadow:    true,
			Content:   ui.View(a.windowButtonTooltipView),
		})
	} else {
		a.windowButtonTooltipFlyout.SetAnchor(a.windowButtonTooltipAnchor)
		a.windowButtonTooltipFlyout.SetSize(148, 32)
	}

	if !a.windowButtonTooltipFlyout.IsVisible() {
		a.windowButtonTooltipFlyout.ShowInactive()
	}
	a.windowButtonTooltipFlyout.Invalidate()
}

func (a *App) hideWindowButtonTooltip() {
	if a == nil || a.windowButtonTooltipFlyout == nil || a.windowButtonTooltipFlyout.IsDestroyed() {
		return
	}
	if a.windowButtonTooltipFlyout.IsVisible() {
		a.windowButtonTooltipFlyout.Hide()
	}
}

func (a *App) windowButtonTooltipView(c *ui.Context) {
	applyAuroraTheme(c)
	c.Root().Background(ui.Transparent)
	ui.Box(c).Fill().Center().Padding(7, 12).
		Background(ui.Hex("#F3F0E8")).
		Border(1, ui.Hex("#D8D0BE")).
		Radius(7).
		Children(func() {
			ui.Text(c, a.windowButtonTooltipLabel).
				FontSize(10).
				Bold().
				TextColor(ui.Hex("#211F1B"))
		})
}
