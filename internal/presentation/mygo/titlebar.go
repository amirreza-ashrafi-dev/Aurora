package presentation

import "github.com/egoist/mygo/ui"

func (a *App) windowTitleBar(c *ui.Context) {
	maximizeGlyph := "□"
	if a.win != nil && a.win.IsMaximized() {
		maximizeGlyph = "▣"
	}

	var hoveredTooltip string
	var hoveredTooltipBounds ui.Rect

	ui.Row(c).
		Height(40).
		FillWidth().
		Padding(0, 16).
		Gap(10).
		Background(ui.Hex(auroraHeader)).
		Children(func() {
			if logo := auroraLogo(); logo != nil {
				ui.Image(c, logo).Width(27).Height(27).Fit(ui.Contain)
			}
			ui.Column(c).Gap(0).Children(func() {
				ui.Text(c, "AURORA").FontSize(11).Bold().LetterSpacing(1.8).TextColor(ui.Hex(auroraAccent))
				ui.Text(c, "CINEMA PLAYER").FontSize(6).LetterSpacing(1.4).TextColor(ui.Hex(auroraMuted))
			})
			ui.Text(c, "PLAYER  /  VERSION 0.1.0").
				FontSize(8).
				LetterSpacing(0.7).
				TextColor(ui.Hex(auroraMuted))
			ui.Box(c).Fill().Height(40).DragWindow()
			if a.win != nil {
				ui.Row(c).Height(40).Gap(0).Children(func() {
					minimizeButton := auroraWindowButton(c, "−", "Minimize", false)
					if minimizeButton.Hovered() {
						hoveredTooltip = "Minimize"
						hoveredTooltipBounds = minimizeButton.Bounds()
					}
					if minimizeButton.Clicked() {
						a.win.Minimize()
					}

					maximizeButton := auroraWindowButton(c, maximizeGlyph, "Maximize / restore", false).MaximizeButton()
					if maximizeButton.Hovered() {
						hoveredTooltip = "Maximize / restore"
						hoveredTooltipBounds = maximizeButton.Bounds()
					}
					if maximizeButton.Clicked() {
						a.win.ToggleMaximize()
					}

					closeButton := auroraWindowButton(c, "×", "Close Aurora Player", true)
					if closeButton.Hovered() {
						hoveredTooltip = "Close Aurora Player"
						hoveredTooltipBounds = closeButton.Bounds()
					}
					if closeButton.Clicked() {
						a.win.Close()
					}
				})
			}
		})

	// These controls sit above a native video child HWND. Their tooltip must
	// therefore use an owned flyout window, not MyGo's in-canvas tooltip layer,
	// which the video HWND can cover.
	a.syncWindowButtonTooltip(c, hoveredTooltip, hoveredTooltipBounds)
}
