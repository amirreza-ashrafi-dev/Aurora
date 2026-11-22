package presentation

import "github.com/egoist/mygo/ui"

// Aurora's premium palette is intentionally warm: near-black surfaces,
// champagne-gold actions, and warm neutral text.
const (
	auroraBackground = "#0D0C0A"
	auroraHeader     = "#090807"
	auroraPanel      = "#12110E"
	auroraPanelAlt   = "#171511"
	auroraFooter     = "#0B0A09"
	auroraBorder     = "#332E22"
	auroraBorderSoft = "#29251C"
	auroraText       = "#EEEAE0"
	auroraMuted      = "#999181"
	auroraAccent     = "#DDBB53"
	auroraAccentHover = "#E9CB70"
	auroraAccentPressed = "#BE9836"
	auroraAccentText = "#17130A"
	auroraAccentSoft = "#2A2312"
)

const (
	auroraMainPadding    = 16
	auroraStageMinHeight = 320
	auroraQueueWidth     = 320
	auroraStageGap       = 16
)

func applyAuroraTheme(c *ui.Context) {
	if c == nil {
		return
	}
	theme := *ui.DarkTheme()
	// Keep desktop accessibility preferences (high contrast and text scale),
	// but Aurora owns its accent color in both active and inactive windows.
	theme = *theme.WithPreferences(c.Preferences())
	theme.Background = ui.Hex(auroraBackground)
	theme.Surface = ui.Hex(auroraPanel)
	theme.SurfaceHover = ui.Hex("#201C14")
	theme.SurfacePressed = ui.Hex("#292315")
	theme.Border = ui.Hex(auroraBorder)
	theme.Text = ui.Hex(auroraText)
	theme.TextMuted = ui.Hex(auroraMuted)
	theme.Accent = ui.Hex(auroraAccent)
	theme.AccentHover = ui.Hex(auroraAccentHover)
	theme.AccentPressed = ui.Hex(auroraAccentPressed)
	theme.AccentText = ui.Hex(auroraAccentText)
	theme.InactiveAccent = ui.Hex(auroraAccent)
	theme.Selection = ui.RGBA(221, 187, 83, 0.28)
	theme.Focus = ui.RGBA(221, 187, 83, 0.72)
	if c.Preferences().HighContrast {
		theme.Focus = ui.Hex(auroraAccent)
	}
	theme.Radius = 8
	theme.Spacing = 4
	theme.FontSize = 13
	// TeX Gyre Heros is the closest redistributable companion to Nimbus Sans.
	// Keep a system fallback for machines where the font package is not installed.
	theme.Font = "Segoe UI"
	c.SetTheme(&theme)
}

func auroraQuietButton(c *ui.Context, label string) ui.Element {
	return ui.Button(c, label).
		Padding(8, 12).
		Radius(8).
		Background(ui.Hex(auroraPanelAlt)).
		Border(1, ui.Hex(auroraBorder)).
		TextColor(ui.Hex(auroraText))
}

func auroraPrimaryButton(c *ui.Context, label string) ui.Element {
	return ui.PrimaryButton(c, label).
		Padding(8, 15).
		Radius(9).
		Background(ui.Hex(auroraAccent)).
		Border(1, ui.Hex(auroraAccentHover)).
		TextColor(ui.Hex(auroraAccentText))
}

func auroraAddVideosButton(c *ui.Context) ui.Element {
	return ui.Button(c, "+  Add videos").
		FillWidth().
		Height(38).
		Padding(8, 12).
		Radius(10).
		Background(ui.Hex(auroraAccentSoft)).
		Border(1, ui.Hex("#725C28")).
		TextColor(ui.Hex("#EBD79D"))
}

func auroraWindowButton(c *ui.Context, glyph, tooltip string, closeButton bool) ui.Element {
	button := ui.Button(c, glyph).
		Label(tooltip).
		Width(38).
		Height(40).
		Padding(0).
		Radius(0).
		Background(ui.Transparent).
		Border(0, ui.Transparent)
	if button.Hovered() {
		if closeButton {
			button.Background(ui.Hex("#5B211B")).TextColor(ui.Hex("#FFFFFF"))
		} else {
			button.Background(ui.Hex("#242016")).TextColor(ui.Hex("#FFFFFF"))
		}
	} else {
		button.TextColor(ui.Hex("#A9A292"))
	}
	return button.FontSize(15)
}
