package presentation

import (
	"time"

	"my-go/internal/application"
)

// QueueOpenFile receives a video opened through Windows Explorer. MyGo
// delivers one callback per file, so a short debounce keeps multi-select
// opens together as a single playback queue.
func (a *App) QueueOpenFile(path string) {
	if a == nil {
		return
	}
	files := application.FilterVideoLaunchPaths([]string{path})
	if len(files) == 0 {
		return
	}
	a.startupFiles = append(a.startupFiles, files...)
	if !a.windowReady {
		return
	}
	if a.startupFilesTimer != nil {
		a.startupFilesTimer.Stop()
	}
	a.startupFilesTimer = time.AfterFunc(40*time.Millisecond, func() {
		window := a.win
		if window == nil || window.IsDestroyed() {
			return
		}
		window.Update(func() {
			a.startupFilesTimer = nil
			a.openQueuedFiles()
		})
	})
}

func (a *App) openQueuedFiles() {
	if a == nil || !a.windowReady || len(a.startupFiles) == 0 ||
		a.win == nil || a.win.IsDestroyed() {
		return
	}
	if a.startupFilesTimer != nil {
		a.startupFilesTimer.Stop()
		a.startupFilesTimer = nil
	}
	files := a.startupFiles
	a.startupFiles = nil
	a.openFilesFromExplorer(files)
	a.win.Invalidate()
}

// openFilesFromExplorer replaces the current playback queue: opening a file
// from Explorer means "play this", not "append it behind the old queue".
func (a *App) openFilesFromExplorer(paths []string) {
	filtered := application.FilterNewMedia(nil, paths)
	if len(filtered.Valid) == 0 {
		a.status = "The selected video could not be opened"
		return
	}

	a.saveCurrentProgressImmediately()
	a.pendingResume = nil
	a.closeFilmstrip()
	for _, oldPath := range a.playlist {
		a.evictQueueThumbnail(oldPath)
	}
	a.playlist = nil
	a.playlistPos = 0
	a.pendingPlaylistRemoval = nil
	a.path = ""
	a.state.Position = 0
	a.state.Duration = 0
	a.state.Paused = true

	a.addFiles(filtered.Valid)
}

// Activate restores and focuses Aurora when Windows forwards an open request
// from another Explorer launch to the existing single instance.
func (a *App) Activate() {
	if a == nil || a.win == nil || a.win.IsDestroyed() {
		return
	}
	if a.win.IsMinimized() {
		a.win.Restore()
	}
	a.win.Show()
	a.win.Focus()
}
