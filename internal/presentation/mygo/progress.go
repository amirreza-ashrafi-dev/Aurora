package presentation

import (
	"log"
	"math"
	"os"
	"path/filepath"
	"time"

	mygo "github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"my-go/internal/application"
	"my-go/internal/domain"
)

type progressWriteRequest struct {
	record     *domain.PlaybackProgress
	deletePath string
	debounce   bool
	cancel     bool
}

func (a *App) loadProgressHistory() {
	if a.history == nil { return }
	records, err := a.history.Load()
	if err != nil {
		log.Printf("load playback history: %v", err)
		a.status = "Playback history unavailable: " + err.Error()
		return
	}
	a.historyRecords = make(map[string]domain.PlaybackProgress, len(records))
	for _, record := range records {
		a.historyRecords[application.ProgressKey(record.MediaPath)] = record
	}
	a.startProgressWorker()
}

func (a *App) startProgressWorker() {
	if a.history == nil || a.progressRequests != nil { return }
	a.progressRequests = make(chan progressWriteRequest, 1)
	a.progressStop = make(chan struct{})
	a.progressDone = make(chan struct{})
	go func() {
		defer close(a.progressDone)
		var pending *progressWriteRequest
		timer := time.NewTimer(time.Hour)
		if !timer.Stop() { <-timer.C }
		var timerC <-chan time.Time
		persist := func(req progressWriteRequest) {
			var err error
			switch {
			case req.record != nil: err = a.history.Save(*req.record)
			case req.deletePath != "": err = a.history.Delete(req.deletePath)
			}
			if err != nil { log.Printf("save playback history: %v", err) }
		}
		stopTimer := func() {
			if timerC != nil {
				if !timer.Stop() { select { case <-timer.C: default: } }
				timerC = nil
			}
		}
		for {
			select {
			case req := <-a.progressRequests:
				if req.cancel {
					pending = nil
					stopTimer()
					continue
				}
				if req.debounce && req.record != nil {
					copy := req
					pending = &copy
					stopTimer()
					timer.Reset(2 * time.Second)
					timerC = timer.C
					continue
				}
				if pending != nil {
					persist(*pending)
					pending = nil
				}
				stopTimer()
				persist(req)
			case <-timerC:
				if pending != nil { persist(*pending); pending = nil }
				timerC = nil
			case <-a.progressStop:
				// Drain requests queued just before shutdown so the final snapshot
				// cannot lose a race with the stop signal.
				for {
					select {
					case req := <-a.progressRequests:
						switch {
						case req.record != nil:
							persist(req)
							pending = nil
						case req.deletePath != "":
							persist(req)
						case req.cancel:
							pending = nil
						}
					default:
						if pending != nil { persist(*pending) }
						return
					}
				}
			}
		}
	}()
}

func (a *App) sendProgressRequest(req progressWriteRequest) {
	if a.progressRequests == nil { return }
	select {
	case a.progressRequests <- req:
	default:
		select { case <-a.progressRequests: default: }
		select { case a.progressRequests <- req: default: }
	}
}

func (a *App) stopProgressWorker() {
	if a.progressStop == nil { return }
	close(a.progressStop)
	if a.progressDone != nil { <-a.progressDone }
	a.progressRequests, a.progressStop, a.progressDone = nil, nil, nil
}

func (a *App) progressSnapshot(now time.Time) (*domain.PlaybackProgress, bool) {
	if a.path == "" || a.pendingResume != nil { return nil, false }
	position, duration := a.state.Position, a.state.Duration
	if math.IsNaN(position) || math.IsInf(position, 0) || math.IsNaN(duration) || math.IsInf(duration, 0) ||
		position < 10 || duration < 0 { return nil, false }
	if duration > 0 {
		if position > duration { position = duration }
		if duration-position <= 30 { return nil, true }
	}
	return &domain.PlaybackProgress{MediaPath:a.path, Position:position, Duration:duration, LastUpdated:now.UTC()}, false
}

func (a *App) rememberProgress(record domain.PlaybackProgress) {
	if a.historyRecords == nil {
		a.historyRecords = make(map[string]domain.PlaybackProgress)
	}
	key := application.ProgressKey(record.MediaPath)
	if key == "" {
		return
	}
	a.historyRecords[key] = record
	// Keep the in-memory index bounded just like the persisted history.
	for len(a.historyRecords) > 500 {
		oldestKey := ""
		var oldest time.Time
		for candidate, item := range a.historyRecords {
			if oldestKey == "" || item.LastUpdated.Before(oldest) {
				oldestKey, oldest = candidate, item.LastUpdated
			}
		}
		delete(a.historyRecords, oldestKey)
	}
}

func (a *App) scheduleCurrentProgressSave() {
	record, completed := a.progressSnapshot(time.Now())
	if completed { a.clearCurrentProgress(); return }
	if record == nil { return }
	a.rememberProgress(*record)
	a.sendProgressRequest(progressWriteRequest{record:record, debounce:true})
}

func (a *App) saveCurrentProgressImmediately() {
	record, completed := a.progressSnapshot(time.Now())
	if completed { a.clearCurrentProgress(); return }
	if record == nil { return }
	a.rememberProgress(*record)
	a.sendProgressRequest(progressWriteRequest{record:record})
}

func (a *App) clearProgress(path string) {
	if path == "" { return }
	delete(a.historyRecords, application.ProgressKey(path))
	a.sendProgressRequest(progressWriteRequest{deletePath:path})
}

func (a *App) clearCurrentProgress() {
	if a.path != "" { a.clearProgress(a.path) }
	a.pendingResume = nil
	a.closeResumeBanner()
}

func (a *App) cancelPendingProgressSave() {
	a.sendProgressRequest(progressWriteRequest{cancel:true})
}

func (a *App) offerResumeForCurrent() {
	if a.player == nil || a.path == "" { return }
	record, ok := a.historyRecords[application.ProgressKey(a.path)]
	if !ok { return }
	if _, err := os.Stat(a.path); err != nil || !application.CanResumeProgress(record) {
		a.clearProgress(a.path)
		return
	}
	if err := a.player.SetPaused(true); err != nil {
		a.status = "Could not pause for resume choice: " + err.Error()
		return
	}
	a.pendingResume = &record
	a.state.Position = 0
	a.state.Duration = record.Duration
	a.state.Paused = true
	a.status = "Choose whether to continue watching"
	if a.win != nil && !a.win.IsDestroyed() { a.win.Invalidate() }
	if a.fullscreen || a.cinematicMode {
		a.showController()
		a.showResumeBanner()
		if a.controller != nil && !a.controller.IsDestroyed() {
			a.controller.Invalidate()
		}
	}
}

func (a *App) chooseResume() {
	if a.pendingResume == nil || a.player == nil { return }
	record := *a.pendingResume
	if err := a.player.Seek(record.Position); err != nil { a.status = "Resume failed: " + err.Error(); return }
	if err := a.player.SetPaused(false); err != nil { a.status = "Resume failed: " + err.Error(); return }
	a.pendingResume = nil
	a.closeResumeBanner()
	a.state.Position, a.state.Duration, a.state.Paused = record.Position, record.Duration, false
	a.status = "Resuming from " + formatTime(record.Position)
}

func (a *App) chooseStartOver() {
	if a.pendingResume == nil || a.player == nil { return }
	path := a.pendingResume.MediaPath
	a.pendingResume = nil
	a.closeResumeBanner()
	a.clearProgress(path)
	if err := a.player.Seek(0); err != nil { a.status = "Could not restart video: " + err.Error(); return }
	if err := a.player.SetPaused(false); err != nil { a.status = "Could not restart video: " + err.Error(); return }
	a.state.Position, a.state.Paused = 0, false
	a.status = "Playing from the beginning"
}

func (a *App) updateTimeline(dragging, changed bool) {
	wasScrubbing := a.scrubbing
	a.scrubbing = dragging
	if changed && a.player != nil {
		if err := a.player.Seek(a.state.Position); err != nil { a.status = err.Error() }
	}
	if !dragging && (wasScrubbing || changed) { a.scheduleCurrentProgressSave() }
}

// resumePrompt keeps the ordinary-window Smart Resume choice inside the
// main layout. Fullscreen and Cinematic Mode use the separate floating banner.
func (a *App) resumePrompt(c *ui.Context) {
	if a == nil || a.pendingResume == nil {
		return
	}
	if a.fullscreen || a.cinematicMode || (a.win != nil && !a.win.IsDestroyed() && a.win.IsFullScreen()) {
		return
	}

	record := *a.pendingResume
	ui.Row(c).FillWidth().Gap(10).Padding(10, 12).
		Background(ui.Hex("#1B1810")).
		Border(1, ui.Hex("#80672D")).
		Radius(10).
		AlignItems(ui.Center).
		Children(func() {
			fullTitle := filepath.Base(record.MediaPath)
			displayTitle := fullTitle+"  ·  "+formatTime(record.Position)
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, "CONTINUE WATCHING?").
					FontSize(9).Bold().LetterSpacing(1.1).
					TextColor(ui.Hex(auroraAccent))
				ui.Text(c, displayTitle).FillWidth().MinWidth(0).NoWrap().
					FontSize(12).Bold().
					TextColor(ui.Hex(auroraText)).
					Ellipsis("…").Tooltip(fullTitle)
			})
			if auroraQuietButton(c, "Start over").Clicked() {
				a.chooseStartOver()
			}
			if auroraPrimaryButton(c, "▶  Resume").Clicked() {
				a.chooseResume()
			}
		})
}

func (a *App) showResumeBanner() {
	if a == nil || a.pendingResume == nil || a.win == nil || a.win.IsDestroyed() {
		return
	}
	if !a.win.IsFullScreen() && !a.cinematicMode {
		return
	}
	bounds := a.win.ContentBounds()
	if bounds.Width < 1 {
		return
	}
	anchor := mygo.Rectangle{X: bounds.Width / 2, Y: 84, Width: 1, Height: 1}
	if a.resumeBanner != nil && !a.resumeBanner.IsDestroyed() {
		if anchor != a.resumeBannerAnchor {
			a.resumeBanner.SetAnchor(anchor)
			a.resumeBannerAnchor = anchor
		}
		if !a.resumeBanner.IsVisible() {
			a.resumeBanner.ShowInactive()
		}
		return
	}
	a.resumeBanner = mygo.NewFlyout(mygo.FlyoutOptions{
		Parent: a.win, Anchor: anchor, Placement: mygo.PlacementBottom,
		Gap: 12, Width: 672, Height: 77, Focusable: false,
		Hidden: true, Shadow: true, Content: ui.View(a.resumeBannerView),
	})
	a.resumeBannerAnchor = anchor
	a.resumeBanner.ShowInactive()
}

func (a *App) closeResumeBanner() {
	if a.resumeBanner != nil {
		if !a.resumeBanner.IsDestroyed() {
			a.resumeBanner.Close()
		}
		a.resumeBanner = nil
		a.resumeBannerAnchor = mygo.Rectangle{}
	}
}

func (a *App) resumeBannerView(c *ui.Context) {
	applyAuroraTheme(c)
	c.Root().Background(ui.Transparent)
	if a.pendingResume == nil {
		return
	}
	record := *a.pendingResume
	ui.Row(c).Fill().Gap(13).Padding(14, 18).
		Background(ui.Hex("#151718")).
		Border(1, ui.Hex("#80672D")).Radius(14).
		AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(42, 42).Radius(12).
				Background(ui.Hex("#292315")).Center().
				Children(func() {
					ui.Text(c, "◷").FontSize(22).TextColor(ui.Hex(auroraAccent))
				})
			fullTitle := filepath.Base(record.MediaPath)
			displayTitle := fullTitle+"  ·  "+formatTime(record.Position)
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(5).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "CONTINUE WATCHING?").FontSize(9).Bold().
						LetterSpacing(1.2).TextColor(ui.Hex(auroraAccent))
					ui.Text(c, "SAVED POINT").FontSize(7).LetterSpacing(0.6).
						Padding(4, 7).Radius(5).
						Background(ui.Hex("#292315")).TextColor(ui.Hex("#D8BE79"))
				})
				ui.Text(c, displayTitle).FillWidth().MinWidth(0).NoWrap().
					FontSize(12).Bold().TextColor(ui.Hex(auroraText)).
					Ellipsis("…").Tooltip(fullTitle)
			})
			if auroraQuietButton(c, "Start over").Clicked() {
				a.chooseStartOver()
			}
			if auroraPrimaryButton(c, "▶  Resume").Clicked() {
				a.chooseResume()
			}
		})
}
