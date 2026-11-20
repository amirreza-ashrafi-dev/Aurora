package presentation

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	mygo "github.com/egoist/mygo"

	"my-go/internal/application"
)

func (a *App) chooseFiles() {
	if a.win == nil {
		return
	}
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:   a.win,
			Title:    "Add videos to playlist",
			Multiple: true,
			Filters: []mygo.FileFilter{
				{Name: "Video files", Extensions: []string{"mp4", "mkv", "avi", "mov", "wmv", "webm", "m4v", "mpg", "mpeg", "ts", "m2ts", "flv", "ogv", "3gp"}},
				{Name: "All files", Extensions: []string{"*"}},
			},
		})
		if err != nil {
			if a.win != nil && !a.win.IsDestroyed() {
				a.win.Update(func() { a.status = "File picker: " + err.Error() })
			}
			return
		}
		if len(paths) == 0 || a.win == nil || a.win.IsDestroyed() {
			return
		}
		a.win.Update(func() { a.addFiles(paths) })
	}()
}

func (a *App) addFiles(paths []string) {
	filtered := application.FilterNewMedia(a.playlist, paths)
	validationNote := ""
	if filtered.Duplicates > 0 || filtered.Invalid > 0 {
		parts := make([]string, 0, 2)
		if filtered.Duplicates > 0 {
			parts = append(parts, fmt.Sprintf("skipped %d duplicate(s)", filtered.Duplicates))
		}
		if filtered.Invalid > 0 {
			parts = append(parts, fmt.Sprintf("ignored %d invalid file(s)", filtered.Invalid))
		}
		validationNote = "; " + strings.Join(parts, ", ")
	}
	valid := filtered.Valid
	if len(valid) == 0 {
		if filtered.Duplicates > 0 && filtered.Invalid == 0 {
			a.status = "Those video(s) are already in your queue" + validationNote
		} else {
			a.status = "No new valid videos to add" + validationNote
		}
		return
	}
	if err := a.ensurePlayer(); err != nil {
		a.status = "Initialize playback: " + err.Error()
		return
	}

	startIndex := len(a.playlist)
	if len(a.playlist) == 0 {
		// The native host must be visible before MPV creates its video output.
		a.revealVideoHost()
		if err := a.player.Load(valid[0]); err != nil {
			a.hideVideoHost()
			a.status = err.Error()
			return
		}
		a.playlist = append(a.playlist, valid[0])
		a.path = valid[0]
		a.resetSubtitleDelay()
		a.playlistPos = 0
		for _, path := range valid[1:] {
			if err := a.player.Append(path); err != nil {
				log.Printf("append %q to MPV playlist: %v", path, err)
				continue
			}
			a.playlist = append(a.playlist, path)
		}
		a.state.Position = 0
		a.state.Duration = 0
		a.state.Paused = false
		a.status = "Playing: " + filepath.Base(a.path) + validationNote
		a.captureMemorySample("playback_started")
		a.offerResumeForCurrent()
		a.restoreSubtitleForCurrent()
		return
	}

	for _, path := range valid {
		if err := a.player.Append(path); err != nil {
			log.Printf("append %q to MPV playlist: %v", path, err)
			continue
		}
		a.playlist = append(a.playlist, path)
	}
	added := len(a.playlist) - startIndex
	a.status = fmt.Sprintf("Added %d video(s)%s", added, validationNote)
	if added == 0 && filtered.Duplicates > 0 && filtered.Invalid == 0 {
		a.status = "Those video(s) are already in your queue" + validationNote
	}
}

func (a *App) removePlaylistIndex(index int) {
	if a.player == nil || index < 0 || index >= len(a.playlist) {
		return
	}

	removedPath := a.playlist[index]
	if index == a.playlistPos {
		a.saveCurrentProgressImmediately()
		a.pendingResume = nil
	}
	if err := a.player.RemovePlaylistItem(index); err != nil {
		a.status = "Remove from queue: " + err.Error()
		return
	}

	updated, nextCurrent, removedCurrent := application.RemovePlaylistEntry(
		a.playlist, a.playlistPos, index,
	)
	a.playlist = updated
	a.playlistPos = nextCurrent
	if !a.playlistContainsPath(removedPath) {
		a.evictQueueThumbnail(removedPath)
		if a.win != nil && !a.win.IsDestroyed() {
			a.win.Invalidate()
		}
	}

	if !removedCurrent {
		a.status = fmt.Sprintf("Removed %q from queue", filepath.Base(removedPath))
		return
	}

	if len(a.playlist) == 0 {
		a.path = ""
		a.state.Position = 0
		a.state.Duration = 0
		a.state.Paused = true
		a.status = "Queue is empty"
		a.subtitleCandidates = nil
		a.hideVideoHost()
		a.stopPolling()
		// Stop and unload the media but retain libmpv's context and DLL.
		if err := a.unloadPlayerMedia(); err != nil {
			log.Printf("unload MPV media after queue emptied: %v", err)
		}
		a.captureMemorySample("playback_stopped_media_unloaded")
		return
	}

	a.path = a.playlist[nextCurrent]
	a.resetSubtitleDelay()
	a.state.Position = 0
	a.state.Duration = 0
	a.state.Paused = false

	// Explicitly select the resulting current item after removing the
	// playing entry. This keeps Aurora's selected index and mpv in sync,
	// regardless of mpv's automatic playlist advancement behavior.
	if err := a.player.PlayIndex(nextCurrent); err != nil {
		a.status = fmt.Sprintf("Removed %q; could not start %q: %v",
			filepath.Base(removedPath), filepath.Base(a.path), err)
		return
	}

	a.status = fmt.Sprintf("Removed %q; playing %q",
		filepath.Base(removedPath), filepath.Base(a.path))
	a.restoreSubtitleForCurrent()
}

func (a *App) playPlaylistIndex(index int) {
	if a.player == nil || index < 0 || index >= len(a.playlist) {
		return
	}
	a.saveCurrentProgressImmediately()
	a.pendingResume = nil
	a.revealVideoHost()
	if err := a.player.PlayIndex(index); err != nil {
		a.status = err.Error()
		return
	}
	a.playlistPos = index
	a.path = a.playlist[index]
	a.resetSubtitleDelay()
	a.state.Position = 0
	a.state.Duration = 0
	a.state.Paused = false
	a.status = "Playing: " + filepath.Base(a.path)
	a.captureMemorySample("playback_started")
	a.offerResumeForCurrent()
	a.restoreSubtitleForCurrent()
}

func (a *App) next() {
	if a.player == nil || len(a.playlist) == 0 {
		return
	}
	if a.playlistPos+1 < len(a.playlist) {
		a.playPlaylistIndex(a.playlistPos + 1)
	}
}

func (a *App) previous() {
	if a.player == nil || len(a.playlist) == 0 {
		return
	}
	if a.state.Position > 3 {
		a.seekBy(-a.state.Position)
		return
	}
	if a.playlistPos > 0 {
		a.playPlaylistIndex(a.playlistPos - 1)
	}
}
