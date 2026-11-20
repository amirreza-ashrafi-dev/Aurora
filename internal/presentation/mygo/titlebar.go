package presentation

import "github.com/egoist/mygo/ui"

func (a *App) windowTitleBar(c *ui.Context) {
	maximizeGlyph := "□"
	if a.win != nil && a.win.IsMaximized() {
		maximizeGlyph = "▣"
	}

	ui.Row(c).
		Height(40).
		FillWidth().
		Padding(0, 16).
		Gap(10).
		Background(ui.Hex(auroraHeader)).
		Children(func() {
			// Until the final logo bitmap is embedded in the repository, retain
			// the wordmark treatment and avoid inventing a substitute logo mark.
			ui.Column(c).Gap(0).Children(func() {
				ui.Text(c, "AURORA").FontSize(12).Bold().LetterSpacing(1.8).TextColor(ui.Hex(auroraAccent))
			})
			ui.Text(c, "PLAYER  /  VERSION 0.1.0").
				FontSize(8).
				LetterSpacing(0.7).
				TextColor(ui.Hex(auroraMuted))
			ui.Box(c).Fill().Height(40).DragWindow()
			if a.win != nil {
				ui.Row(c).Height(40).Gap(0).Children(func() {
					if auroraWindowButton(c, "−", "Minimize", false).Clicked() {
						a.win.Minimize()
					}
					maximizeButton := auroraWindowButton(c, maximizeGlyph, "Maximize / restore", false).MaximizeButton()
					if maximizeButton.Clicked() {
						a.win.ToggleMaximize()
					}
					if auroraWindowButton(c, "×", "Close Aurora Player", true).Clicked() {
						a.win.Close()
					}
				})
			}
		})
}
