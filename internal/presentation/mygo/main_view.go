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

	if a.cinematicMode {
		a.cinematicView(c)
	} else {
		a.normalView(c)
	}

	// Handle.Bounds is committed after layout; a follow-up frame exposes the
	// new rectangle to the Win32 child-window adapter.
	if !a.videoLayoutFramePending {
		a.videoLayoutFramePending = true
		c.After(16 * time.Millisecond)
	}
}

func (a *App) normalView(c *ui.Context) {
	ui.Column(c).Fill().Background(ui.Hex(auroraBackground)).Children(func() {
		a.windowTitleBar(c)
		a.resumePrompt(c)
		ui.Column(c).Fill().Padding(16).Gap(12).Background(ui.Hex(auroraBackground)).Children(func() {
			// The reference uses a 912 / 320 split at 1280 px, with a 16 px gap.
			// Grow keeps the video stage responsive while the queue stays fixed.
			ui.Row(c).
				Basis(0).Grow(1).Shrink(1).FillWidth().
				MinHeight(auroraStageMinHeight).
				AlignItems(ui.Stretch).
				Gap(auroraStageGap).
				Children(func() {
					videoStage := ui.Box(c).
						Grow(1).FillHeight().MinWidth(0).MinHeight(auroraStageMinHeight).
						Radius(16).Background(ui.Hex("#000000")).
						Border(1, ui.Hex(auroraBorderSoft))
					if strings.TrimSpace(a.path) == "" {
						videoStage = videoStage.Center().Padding(28).Gap(12).
							Background(ui.Hex(auroraPanel)).
							Children(func() {
								// The final artwork is supplied by the user. Until the
								// binary asset can be committed, keep the empty-state
								// composition restrained rather than inventing a mark.
								ui.Text(c, "YOUR CINEMA, READY").
									FontSize(9).Bold().LetterSpacing(2.2).
									TextColor(ui.Hex(auroraAccent))
								ui.Text(c, "Nothing playing yet").
									FontSize(26).Bold().
									TextColor(ui.Hex(auroraText))
								ui.Text(c, "Choose a video from your queue to begin watching.").
									FontSize(12).TextColor(ui.Hex(auroraMuted))
							})
					}
					videoStage.Bind(&a.videoStageHandle)

					ui.Column(c).
						Width(auroraQueueWidth).Shrink(0).FillHeight().MinHeight(0).
						Padding(16).Gap(12).
						Background(ui.Hex(auroraPanel)).
						Border(1, ui.Hex(auroraBorderSoft)).
						Radius(16).Clip().
						Children(func() {
							ui.Row(c).Shrink(0).Gap(8).Children(func() {
								ui.Column(c).Gap(2).Children(func() {
									ui.Text(c, "PLAY QUEUE").
										FontSize(11).Bold().LetterSpacing(1.2).
										TextColor(ui.Hex(auroraText))
									ui.Text(c, "UP NEXT").
										FontSize(8).LetterSpacing(1.5).
										TextColor(ui.Hex(auroraMuted))
								})
								ui.Spacer(c)
								ui.Text(c, fmt.Sprintf("%02d", len(a.playlist))).
									FontSize(10).Bold().Padding(6, 9).
									Radius(8).Background(ui.Hex(auroraAccentSoft)).
									TextColor(ui.Hex(auroraAccent))
							})

							if len(a.playlist) == 0 {
								ui.Column(c).
									Basis(0).Grow(1).Shrink(1).FillWidth().MinHeight(0).
									Padding(14).Gap(7).
									Background(ui.Hex("#181611")).
									Border(1, ui.Hex(auroraBorderSoft)).
									Radius(11).Children(func() {
										ui.Text(c, "Your queue is empty").
											FontSize(13).Bold().TextColor(ui.Hex(auroraText))
										ui.Text(c, "Add a few videos to start watching.").
											FontSize(10).TextColor(ui.Hex(auroraMuted))
									})
							} else {
								ui.List(c, nil, len(a.playlist), func(i int) {
									index := i
									mediaPath := a.playlist[i]
									title := queueCardTitle(mediaPath)
									active := i == a.playlistPos
									cardBorder := auroraBorderSoft
									if active {
										cardBorder = "#80672D"
									}
									ui.Row(c).FillWidth().Height(84).Gap(7).
										Padding(7).AlignItems(ui.Center).
										Background(ui.Hex("#171510")).
										Border(1, ui.Hex(cardBorder)).
										Radius(10).Children(func() {
											play := ui.Button(c, "").
												Label("Play "+title).
												Grow(1).Height(68).
												Padding(0).Radius(6).
												Background(ui.Transparent).
												Border(0, ui.Transparent)
											play.Children(func() {
												ui.Row(c).Fill().Gap(9).
													AlignItems(ui.Center).Children(func() {
														ui.Box(c).Width(86).Height(54).
															Radius(6).Clip().
															Background(ui.Hex("#090807")).
															Children(func() {
																if bitmap := a.queueThumbnails[mediaPath]; bitmap != nil {
																	ui.Image(c, bitmap).FillWidth().FillHeight().
																		Fit(ui.Cover).Radius(6)
																} else {
																	ui.Column(c).Fill().Center().Gap(3).
																		Children(func() {
																			ui.Text(c, "AURORA").
																				FontSize(7).Bold().LetterSpacing(1.6).
																				TextColor(ui.Hex("#8D7338"))
																			ui.Text(c, "▶").FontSize(15).
																				TextColor(ui.Hex("#DDBB53"))
																		})
																}
															})
														ui.Column(c).Grow(1).Gap(4).
															AlignItems(ui.Start).Children(func() {
																titleColor := auroraText
																if active { titleColor = "#F2D995" }
																ui.Text(c, title).FillWidth().
																	FontSize(10).Bold().
																	TextColor(ui.Hex(titleColor)).
																	Ellipsis("…")
																meta := strings.ToUpper(strings.TrimPrefix(filepath.Ext(mediaPath), "."))
																if meta == "" { meta = "VIDEO" }
																if active && a.state.Duration > 0 {
																	resolution := ""
																	if a.videoHeight > 0 {
																		resolution = fmt.Sprintf("%dp · ", a.videoHeight)
																	}
																	meta = resolution + formatTime(a.state.Position) +
																		" / " + formatTime(a.state.Duration)
																}
																ui.Text(c, meta).FillWidth().
																	FontSize(7).LetterSpacing(0.35).
																	TextColor(ui.Hex(auroraMuted)).
																	Ellipsis("…")
																ui.Row(c).Gap(5).AlignItems(ui.Center).
																	Children(func() {
																		if active {
																			ui.Box(c).Width(5).Height(5).
																				Radius(3).
																				Background(ui.Hex(auroraAccent))
																			ui.Text(c, "PLAYING").
																				FontSize(7).Bold().
																				LetterSpacing(0.7).
																				TextColor(ui.Hex(auroraAccent))
																		} else {
																			ui.Text(c, "IN QUEUE").
																				FontSize(7).LetterSpacing(0.5).
																				TextColor(ui.Hex(auroraMuted))
																		}
																		extension := strings.ToUpper(strings.TrimPrefix(filepath.Ext(mediaPath), "."))
																		if extension != "" {
																			ui.Text(c, "."+extension).
																				FontSize(7).LetterSpacing(0.4).
																				TextColor(ui.Hex(auroraMuted))
																		}
																	})
															})
													})
											})
											if play.Clicked() {
												a.playPlaylistIndex(index)
											}
											remove := ui.Button(c, "×").
												Width(22).Height(28).
												Padding(0).Radius(6).
												Background(ui.Transparent).
												Border(0, ui.Transparent).
												FontSize(15).
												TextColor(ui.Hex(auroraMuted)).
												Tooltip("Remove from queue")
											if remove.Clicked() {
												a.removePlaylistIndex(index)
											}
										})
								}).Basis(0).Grow(1).Shrink(1).FillWidth().MinHeight(0).Gap(8)
							}

							ui.Column(c).Shrink(0).Gap(8).Children(func() {
								if auroraAddVideosButton(c).Clicked() {
									a.chooseFiles()
								}
								ui.Text(c, "SPACE  Play / Pause").
									FontSize(8).TextColor(ui.Hex(auroraMuted))
								ui.Text(c, "← →  Seek     ↑ ↓  Volume").
									FontSize(8).TextColor(ui.Hex(auroraMuted))
							})
						})
				})

			// Playback deck: title/status row, timeline and transport/actions.
			ui.Column(c).Shrink(0).Gap(9).Padding(16, 20).
				Background(ui.Hex(auroraFooter)).
				Border(1, ui.Hex(auroraBorderSoft)).
				Radius(14).Children(func() {
					ui.Row(c).Shrink(0).Gap(10).Children(func() {
						ui.Column(c).Gap(3).Children(func() {
							title := a.currentTitle()
							if title == "" { title = "Nothing playing" }
							ui.Text(c, title).FontSize(15).Bold().
								TextColor(ui.Hex(auroraText)).Ellipsis("…")
							ui.Text(c, a.status).FontSize(9).
								TextColor(ui.Hex(auroraMuted)).Ellipsis("…")
						})
						ui.Spacer(c)
						if strings.TrimSpace(a.path) != "" {
							ui.Text(c, "NOW PLAYING").
								FontSize(8).Bold().LetterSpacing(1.2).
								Padding(7, 10).Radius(7).
								Background(ui.Hex(auroraAccentSoft)).
								TextColor(ui.Hex(auroraAccent))
						} else {
							ui.Text(c, "AURORA  /  CINEMA PLAYER").
								FontSize(8).LetterSpacing(1).
								TextColor(ui.Hex(auroraMuted))
						}
					})

					maxDuration := a.state.Duration
					if maxDuration <= 0 { maxDuration = 1 }
					ui.Row(c).Shrink(0).Gap(10).Children(func() {
						ui.Text(c, formatTime(a.state.Position)).
							Width(48).FontSize(9).TextColor(ui.Hex(auroraMuted))
						timeline := ui.Slider(c, &a.state.Position, 0, maxDuration).
							Fill().Height(20)
						a.updateTimeline(timeline.Dragging(), timeline.Changed())
						ui.Text(c, formatTime(a.state.Duration)).
							Width(48).FontSize(9).TextColor(ui.Hex(auroraMuted))
					})

					ui.Row(c).Shrink(0).Gap(9).Children(func() {
						if auroraQuietButton(c, "⏮").Tooltip("Previous").Clicked() {
							a.previous()
						}
						playLabel := "Ⅱ  Pause"
						if a.state.Paused || strings.TrimSpace(a.path) == "" {
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
								a.clearCurrentProgress()
								a.status = "Stopped"
							}
						}
						ui.Spacer(c)
						ui.Text(c, "VOLUME").FontSize(8).LetterSpacing(1).
							TextColor(ui.Hex(auroraMuted))
						volumeSlider := ui.Slider(c, &a.state.Volume, 0, 100).
							Width(125).Height(21)
						if volumeSlider.Changed() && a.player != nil {
							if err := a.player.SetVolume(a.state.Volume); err != nil {
								a.status = err.Error()
							}
						}
						ui.Text(c, fmt.Sprintf("%.0f%%", a.state.Volume)).
							Width(36).FontSize(9).TextColor(ui.Hex(auroraText))
						a.subtitleStudioFlyoutButton(c)
						if auroraQuietButton(c, "◫").Tooltip("Cinematic mode (C)").Clicked() {
							a.toggleCinematicMode()
						}
						if auroraQuietButton(c, "⛶").Tooltip("Fullscreen").Clicked() {
							a.toggleFullscreen()
						}
					})
					ui.Text(c, "C  CINEMATIC     F / F11  FULLSCREEN     ESC  EXIT MODE").
						Shrink(0).FontSize(8).LetterSpacing(0.5).
						TextColor(ui.Hex("#716A5B"))
				})
		})
	})
}

func (a *App) cinematicView(c *ui.Context) {
	ui.Column(c).Fill().Background(ui.Hex("#050403")).Children(func() {
		videoStage := ui.Box(c).Grow(1).FillWidth().FillHeight().MinWidth(0).MinHeight(0)
		if strings.TrimSpace(a.path) == "" {
			videoStage = videoStage.Center().Padding(24).Gap(10).
				Background(ui.Hex(auroraBackground)).Children(func() {
					ui.Text(c, "YOUR CINEMA, READY").
						FontSize(9).Bold().LetterSpacing(2).TextColor(ui.Hex(auroraAccent))
					ui.Text(c, "Nothing playing yet").
						FontSize(22).Bold().TextColor(ui.Hex(auroraText))
					ui.Text(c, "Add a video to begin watching.").
						FontSize(11).TextColor(ui.Hex(auroraMuted))
				})
		}
		videoStage.Bind(&a.videoStageHandle)
	})
}
