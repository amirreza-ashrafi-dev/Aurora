package presentation

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

func (a *App) fullscreenControls(c *ui.Context) {
	a.registerShortcuts(c)
	applyAuroraTheme(c)
	c.Root().Background(ui.Transparent)
	ui.Column(c).Fill().Padding(9).Children(func() {
		a.resumePrompt(c)
		ui.Column(c).Fill().Padding(13).Gap(9).Background(ui.Hex(auroraPanel)).Border(1, ui.Hex(auroraBorder)).Radius(15).Children(func() {
			ui.Row(c).Height(30).Gap(10).Children(func() {
				ui.Column(c).Gap(1).Children(func() {
					ui.Text(c, a.currentTitle()).FontSize(14).Bold().TextColor(ui.Hex(auroraText))
					ui.Text(c, "AURORA  /  NOW PLAYING").FontSize(8).LetterSpacing(1).TextColor(ui.Hex(auroraMuted))
				})
				ui.Spacer(c)
				cinemaLabel := "Cinematic"
				if a.cinematicMode {
					cinemaLabel = "Exit cinema"
				}
				if auroraQuietButton(c, cinemaLabel).Clicked() {
					a.toggleCinematicMode()
				}
				a.fullscreenSubtitleStudioFlyoutButton(c)
				if a.win != nil && a.win.IsFullScreen() {
					if auroraQuietButton(c, "Exit fullscreen").Clicked() {
						a.exitFullscreen()
					}
				}
			})

			maxDuration := a.state.Duration
			if maxDuration <= 0 {
				maxDuration = 1
			}
			ui.Row(c).Gap(10).Children(func() {
				ui.Text(c, formatTime(a.state.Position)).Width(48).FontSize(10).TextColor(ui.Hex(auroraMuted))
				timeline := ui.Slider(c, &a.state.Position, 0, maxDuration).Fill().Height(22)
				a.updateTimeline(timeline.Dragging(), timeline.Changed())
				ui.Text(c, formatTime(a.state.Duration)).Width(48).FontSize(10).TextColor(ui.Hex(auroraMuted))
			})

			ui.Row(c).Gap(8).Children(func() {
				if auroraQuietButton(c, "⏮ Previous").Clicked() {
					a.previous()
				}
				playLabel := "Ⅱ  Pause"
				if a.state.Paused {
					playLabel = "▶  Play"
				}
				if auroraPrimaryButton(c, playLabel).Clicked() {
					a.togglePlayPause()
				}
				if auroraQuietButton(c, "Next ⏭").Clicked() {
					a.next()
				}
				if auroraQuietButton(c, "■ Stop").Clicked() && a.player != nil {
					if err := a.player.Stop(); err != nil {
						a.status = err.Error()
					} else {
						a.state.Position = 0
						a.state.Paused = true
						a.clearCurrentProgress()
						a.status = "Stopped"
					}
				}
				ui.Spacer(c)
				ui.Text(c, "VOLUME").FontSize(8).LetterSpacing(1).TextColor(ui.Hex(auroraMuted))
				volumeSlider := ui.Slider(c, &a.state.Volume, 0, 100).Width(105).Height(22)
				if volumeSlider.Changed() && a.player != nil {
					if err := a.player.SetVolume(a.state.Volume); err != nil {
						a.status = err.Error()
					}
				}
				ui.Text(c, fmt.Sprintf("%.0f%%", a.state.Volume)).Width(38).FontSize(10).TextColor(ui.Hex(auroraText))
			})
		})
	})
}
