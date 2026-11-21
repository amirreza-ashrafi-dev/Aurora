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
		ui.Column(c).Fill().Padding(15, 18).Gap(11).
			Background(ui.Hex("#080807")).
			Border(1, ui.Hex("#665323")).
			Radius(16).
			Children(func() {
			ui.Row(c).FillWidth().MinWidth(0).Height(34).Gap(10).Children(func() {
				title := a.currentTitle()
				ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(1).Children(func() {
					ui.Text(c, title).FillWidth().MinWidth(0).SingleLine().
						FontSize(14).Bold().
						TextColor(ui.Hex(auroraText)).
						Tooltip(title)
					ui.Text(c, "AURORA  /  NOW PLAYING").FontSize(8).
						LetterSpacing(1.4).TextColor(ui.Hex(auroraAccent))
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
				a.filmstripToggleButton(c)
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
			ui.Row(c).Gap(12).Children(func() {
				displayPosition := a.state.Position
				if a.scrubbing {
					displayPosition = a.timelinePreviewPosition
				}
				ui.Text(c, formatTime(displayPosition)).Width(48).FontSize(10).TextColor(ui.Hex(auroraMuted))
				if !a.scrubbing {
					a.timelinePreviewPosition = a.state.Position
				}
				timeline := ui.Slider(c, &a.timelinePreviewPosition, 0, maxDuration).Fill().Height(24)
				a.updateTimeline(timeline.Dragging(), timeline.Changed(), timeline.PressCanceled())
				ui.Text(c, formatTime(a.state.Duration)).Width(48).FontSize(10).TextColor(ui.Hex(auroraMuted))
			})
			a.filmstripView(c)

			ui.Row(c).Gap(9).Children(func() {
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
				volumeSlider := ui.Slider(c, &a.state.Volume, 0, 100).Width(125).Height(22)
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
