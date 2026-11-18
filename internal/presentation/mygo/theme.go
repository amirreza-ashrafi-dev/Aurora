package presentation

import "github.com/egoist/mygo/ui"

const (
	auroraBackground = "#0D1017"
	auroraPanel      = "#151A24"
	auroraPanelAlt   = "#1B2230"
	auroraBorder     = "#2A3242"
	auroraText       = "#F3F1F8"
	auroraMuted      = "#9AA3B5"
	auroraAccent     = "#A78BFA"
	auroraAccentSoft = "#322849"
)

const (
	auroraMainPadding    = 20
	auroraStageMinHeight = 320
	auroraQueueWidth     = 280
	auroraStageGap       = 14
)

func applyAuroraTheme(c *ui.Context) {
	if c == nil {
		return
	}
	theme := *ui.DarkTheme()
	theme.Background = ui.Hex(auroraBackground)
	theme.Surface = ui.Hex(auroraPanel)
	theme.SurfaceHover = ui.Hex("#222B3A")
	theme.SurfacePressed = ui.Hex("#2A3446")
	theme.Border = ui.Hex(auroraBorder)
	theme.Text = ui.Hex(auroraText)
	theme.TextMuted = ui.Hex(auroraMuted)
	theme.Accent = ui.Hex(auroraAccent)
	theme.AccentHover = ui.Hex("#BBA7FF")
	theme.AccentPressed = ui.Hex("#8E6FE8")
	theme.AccentText = ui.Hex("#FFFFFF")
	theme.InactiveAccent = ui.Hex(auroraAccent)
	theme.Focus = ui.Hex(auroraAccent)
	theme.Radius = 11
	theme.Spacing = 4
	theme.FontSize = 13
	c.SetTheme(theme.WithPreferences(c.Preferences()))
}

func auroraQuietButton(c *ui.Context, label string) ui.Element {
	return ui.Button(c, label).Padding(8, 12).Radius(9).Background(ui.Hex(auroraPanelAlt)).Border(1, ui.Hex(auroraBorder)).TextColor(ui.Hex(auroraText))
}

func auroraPrimaryButton(c *ui.Context, label string) ui.Element {
	return ui.PrimaryButton(c, label).Padding(8, 15).Radius(9).Background(ui.Hex(auroraAccent)).Border(1, ui.Hex("#BBA7FF")).TextColor(ui.Hex("#FFFFFF"))
}

func auroraAddVideosButton(c *ui.Context) ui.Element {
	return ui.Button(c, "+  Add videos").FillWidth().Height(40).Padding(8, 12).Radius(10).Background(ui.Hex(auroraAccentSoft)).Border(1, ui.Hex("#59477D")).TextColor(ui.Hex("#E8DEFF"))
}

func auroraWindowButton(c *ui.Context, glyph, tooltip string, closeButton bool) ui.Element {
	button := ui.Button(c, glyph).Label(tooltip).Width(44).Height(54).Padding(0).Radius(0).Background(ui.Transparent).Border(0, ui.Transparent).Tooltip(tooltip)
	if button.Hovered() {
		if closeButton {
			button.TextColor(ui.Hex("#F07883"))
		} else {
			button.TextColor(ui.Hex("#FFFFFF"))
		}
	} else {
		button.TextColor(ui.Hex("#AEB7C7"))
	}
	return button.FontSize(18)
}
