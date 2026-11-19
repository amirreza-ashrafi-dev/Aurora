package presentation

import (
	"path/filepath"
	"time"
)

func (a *App) startPolling() {
	a.pollStop = make(chan struct{})
	a.pollDone = make(chan struct{})
	go func() {
		defer close(a.pollDone)
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		lastProgressSample := time.Now()
		lastProgressWrite := time.Now()
		for {
			select {
			case <-a.pollStop:
				return
			case <-ticker.C:
				if a.player == nil || a.win == nil {
					continue
				}
				position, posErr := a.player.GetDouble("time-pos")
				duration, durErr := a.player.GetDouble("duration")
				playlistPos, playlistErr := a.player.GetDouble("playlist-pos")
				a.win.Update(func() {
					if a.win.IsDestroyed() {
						return
					}
					// Detect automatic queue advancement before applying MPV's
					// newly reported time-pos to the outgoing item's state.
					if playlistErr == nil && playlistPos >= 0 {
						index := int(playlistPos)
						if index >= 0 && index < len(a.playlist) && index != a.playlistPos {
							a.saveCurrentProgressImmediately()
							a.pendingResume = nil
							a.playlistPos = index
							a.path = a.playlist[index]
							a.state.Position = 0
							a.state.Duration = 0
							a.state.Paused = false
							a.status = "Playing: " + filepath.Base(a.path)
						}
					}
					if !a.scrubbing && posErr == nil {
						a.state.Position = position
					}
					if durErr == nil && duration > 0 {
						a.state.Duration = duration
					}
					now := time.Now()
					if !a.state.Paused && !a.scrubbing && now.Sub(lastProgressSample) >= 5*time.Second {
						lastProgressSample = now
						if record, completed := a.progressSnapshot(now); completed {
							a.clearCurrentProgress()
						} else if record != nil {
							a.rememberProgress(*record)
							if now.Sub(lastProgressWrite) >= 30*time.Second {
								a.sendProgressRequest(progressWriteRequest{record: record})
								lastProgressWrite = now
							}
						}
					}
					if a.controller != nil && !a.controller.IsDestroyed() && a.controller.IsVisible() {
						a.controller.Invalidate()
					}
				})
			}
		}
	}()
}

func (a *App) stopPolling() {
	if a.pollStop == nil {
		return
	}
	close(a.pollStop)
	if a.pollDone != nil {
		<-a.pollDone
	}
	a.pollStop, a.pollDone = nil, nil
}
