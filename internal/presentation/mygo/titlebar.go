package presentation

import "github.com/egoist/mygo/ui"

func (a *App) windowTitleBar(c *ui.Context) {
	maximizeGlyph := "□"
	if a.win != nil && a.win.IsMaximized() {
		maximizeGlyph = "❐"
	}
	ui.Row(c).Height(54).Padding(0, 18).Gap(11).Background(ui.Hex(auroraPanel)).Children(func() {
		ui.Text(c, "A").FontSize(27).Bold().TextColor(ui.Hex(auroraAccent))
		ui.Column(c).Gap(0).Children(func() {
			ui.Text(c, "AURORA").FontSize(15).Bold().LetterSpacing(2).TextColor(ui.Hex(auroraText))
			ui.Text(c, "PLAYER  /  VERSION 0.1.0").FontSize(8).LetterSpacing(1.2).TextColor(ui.Hex(auroraMuted))
		})
		ui.Box(c).Fill().Height(54).DragWindow()
		if a.win != nil {
			ui.Row(c).Height(54).Gap(0).Children(func() {
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
