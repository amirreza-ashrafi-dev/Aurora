package presentation

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

func (a *App) view(c *ui.Context) {
	a.registerShortcuts(c)
	applyAuroraTheme(c)
	a.syncVideoHostToStage(c)

	ui.Column(c).Fill().Background(ui.Hex(auroraBackground)).Children(func() {
		a.windowTitleBar(c)
		ui.Column(c).Fill().Padding(auroraMainPadding).Gap(10).Background(ui.Hex(auroraBackground)).Children(func() {
			ui.Row(c).
				Basis(0).Grow(1).Shrink(1).FillWidth().MinHeight(auroraStageMinHeight).
				AlignItems(ui.Stretch).Gap(auroraStageGap).Children(func() {
				videoStage := ui.Box(c).Grow(1).FillHeight().MinHeight(auroraStageMinHeight).MinWidth(0)
				if strings.TrimSpace(a.path) == "" {
					videoStage = videoStage.Center().Padding(30).Gap(11).Radius(14).
						Background(ui.Hex("#111722")).Border(1, ui.Hex(auroraBorder)).Children(func() {
						ui.Box(c).Size(72, 72).Radius(22).Background(ui.Hex(auroraAccentSoft)).Border(1, ui.Hex("#59477D")).Center().Children(func() {
							ui.Text(c, "▶").FontSize(29).Bold().TextColor(ui.Hex("#DCCEFF"))
						})
						ui.Text(c, "YOUR CINEMA, READY").FontSize(9).Bold().LetterSpacing(2).TextColor(ui.Hex(auroraAccent))
						ui.Text(c, "Nothing playing yet").FontSize(22).Bold().TextColor(ui.Hex(auroraText))
						ui.Text(c, "Choose a video from your queue to begin watching.").FontSize(11).TextColor(ui.Hex(auroraMuted))
					})
				}
				videoStage.Bind(&a.videoStageHandle)

				ui.Column(c).Width(auroraQueueWidth).Shrink(0).FillHeight().MinHeight(0).
					Padding(14).Gap(11).Background(ui.Hex(auroraPanel)).Border(1, ui.Hex(auroraBorder)).
					Radius(14).Clip().Children(func() {
					ui.Row(c).Shrink(0).Gap(8).Children(func() {
						ui.Column(c).Gap(1).Children(func() {
							ui.Text(c, "PLAY QUEUE").FontSize(11).Bold().LetterSpacing(1).TextColor(ui.Hex(auroraText))
							ui.Text(c, "UP NEXT").FontSize(8).LetterSpacing(1).TextColor(ui.Hex(auroraMuted))
						})
						ui.Spacer(c)
						ui.Text(c, fmt.Sprintf("%02d", len(a.playlist))).FontSize(11).Bold().Padding(5, 8).
							Radius(8).Background(ui.Hex(auroraAccentSoft)).TextColor(ui.Hex("#DCCEFF"))
					})

					if len(a.playlist) == 0 {
						ui.Column(c).Basis(0).Grow(1).Shrink(1).FillWidth().MinHeight(0).Padding(14).Gap(7).
							Background(ui.Hex("#1A202B")).Border(1, ui.Hex(auroraBorder)).Radius(11).Children(func() {
							ui.Text(c, "Your queue is empty").FontSize(13).Bold().TextColor(ui.Hex(auroraText))
							ui.Text(c, "Add a few videos to start watching.").FontSize(10).TextColor(ui.Hex(auroraMuted))
						})
					} else {
						ui.List(c, nil, len(a.playlist), func(i int) {
							index := i
							label := filepath.Base(a.playlist[i])
							item := ui.Button(c, label).FillWidth().Height(38).Padding(8, 10).Radius(9).NoWrap().Ellipsis("…")
							if i == a.playlistPos {
								item = item.Selected(true).Background(ui.Hex(auroraAccentSoft)).Border(1, ui.Hex("#65518C")).TextColor(ui.Hex("#F5F0FF"))
							} else {
								item = item.Background(ui.Hex("#1B202B")).Border(1, ui.Hex("#252C39")).TextColor(ui.Hex("#D0D4DF"))
							}
							if item.Clicked() {
								a.playPlaylistIndex(index)
							}
						}).Basis(0).Grow(1).Shrink(1).FillWidth().MinHeight(0).Gap(8)
					}

					ui.Column(c).Shrink(0).Gap(6).Children(func() {
						if auroraAddVideosButton(c).Clicked() {
							a.chooseFiles()
						}
						ui.Text(c, "SPACE  Play / Pause").FontSize(8).TextColor(ui.Hex(auroraMuted))
						ui.Text(c, "← →  Seek     ↑ ↓  Volume").FontSize(8).TextColor(ui.Hex(auroraMuted))
					})
				})
			})

			ui.Row(c).Shrink(0).Gap(10).Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					ui.Text(c, a.currentTitle()).FontSize(16).Bold().TextColor(ui.Hex(auroraText))
					ui.Text(c, a.status).FontSize(10).TextColor(ui.Hex(auroraMuted)).Ellipsis("…")
				})
				ui.Spacer(c)
				ui.Text(c, "NOW PLAYING").FontSize(8).LetterSpacing(1.2).Padding(7, 10).Radius(8).
					Background(ui.Hex(auroraAccentSoft)).TextColor(ui.Hex("#DCCEFF"))
			})

			maxDuration := a.state.Duration
			if maxDuration <= 0 {
				maxDuration = 1
			}
			ui.Row(c).Shrink(0).Gap(10).Children(func() {
				ui.Text(c, formatTime(a.state.Position)).Width(48).FontSize(10).TextColor(ui.Hex(auroraMuted))
				timeline := ui.Slider(c, &a.state.Position, 0, maxDuration).Fill().Height(24)
				a.scrubbing = timeline.Dragging()
				if timeline.Changed() && a.player != nil {
					if err := a.player.Seek(a.state.Position); err != nil {
						a.status = err.Error()
					}
				}
				ui.Text(c, formatTime(a.state.Duration)).Width(48).FontSize(10).TextColor(ui.Hex(auroraMuted))
			})

			ui.Row(c).Shrink(0).Gap(9).Padding(10, 13).Background(ui.Hex(auroraPanel)).
				Border(1, ui.Hex(auroraBorder)).Radius(13).Children(func() {
				if auroraQuietButton(c, "⏮").Tooltip("Previous").Clicked() {
					a.previous()
				}
				playLabel := "Ⅱ  Pause"
				if a.state.Paused {
					playLabel = "▶  Play"
				}
				if auroraPrimaryButton(c, playLabel).Clicked() {
					a.togglePlayPause()
				}
				if auroraQuietButton(c, "⏭").Tooltip("Next").Clicked() {
					a.next()
				}
				if auroraQuietButton(c, "■").Tooltip("Stop").Clicked() && a.player != nil {
					if err := a.player.Stop(); err != nil {
						a.status = err.Error()
					} else {
						a.state.Position = 0
						a.state.Paused = true
						a.status = "Stopped"
					}
				}
				ui.Spacer(c)
				ui.Text(c, "VOLUME").FontSize(8).LetterSpacing(1).TextColor(ui.Hex(auroraMuted))
				volumeSlider := ui.Slider(c, &a.state.Volume, 0, 100).Width(145).Height(23)
				if volumeSlider.Changed() && a.player != nil {
					if err := a.player.SetVolume(a.state.Volume); err != nil {
						a.status = err.Error()
					}
				}
				ui.Text(c, fmt.Sprintf("%.0f%%", a.state.Volume)).Width(38).FontSize(10).TextColor(ui.Hex(auroraText))
				if auroraQuietButton(c, "⛶").Tooltip("Fullscreen").Clicked() {
					a.toggleFullscreen()
				}
			})
			ui.Text(c, "F / F11  Fullscreen     ESC  Exit     Double-click video  Fullscreen").Shrink(0).FontSize(8).TextColor(ui.Hex("#778194"))
		})
	})

	// Handle.Bounds is committed after layout; one follow-up frame exposes the
	// new rectangle to the Win32 child-window adapter.
	if !a.videoLayoutFramePending {
		a.videoLayoutFramePending = true
		c.After(16 * time.Millisecond)
	}
}
