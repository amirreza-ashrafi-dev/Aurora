package presentation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
)

type thumbnailCapturer interface {
	CaptureThumbnail(path string) error
}

func queueThumbnailPath(mediaPath string) (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(mediaPath))))
	name := hex.EncodeToString(sum[:]) + ".png"
	return filepath.Join(cacheDir, "Aurora Player", "thumbnails", name), nil
}

// captureQueueThumbnail is called on the UI thread. MPV writes a frame to a
// stable per-media cache file; the bitmap is decoded only after capture ends.
func (a *App) captureQueueThumbnail(mediaPath string) {
	if a == nil || a.win == nil || a.player == nil || mediaPath == "" {
		return
	}
	if a.queueThumbnails == nil {
		a.queueThumbnails = make(map[string]*ui.Bitmap)
	}
	if a.thumbnailCaptured == nil {
		a.thumbnailCaptured = make(map[string]bool)
	}
	if a.thumbnailCaptureTokens == nil {
		a.thumbnailCaptureTokens = make(map[string]uint64)
	}
	if a.thumbnailCaptured[mediaPath] {
		return
	}
	capturer, ok := a.player.(thumbnailCapturer)
	if !ok {
		a.thumbnailCaptured[mediaPath] = true
		return
	}
	path, err := queueThumbnailPath(mediaPath)
	if err != nil {
		return
	}

	// A generation token prevents a late screenshot from resurrecting a
	// thumbnail after its queue item has been removed (or re-added).
	a.thumbnailCaptureSequence++
	token := a.thumbnailCaptureSequence
	a.thumbnailCaptureTokens[mediaPath] = token
	a.thumbnailCaptured[mediaPath] = true
	window := a.win

	go func() {
		data, captureErr := []byte(nil), capturer.CaptureThumbnail(path)
		if captureErr == nil {
			data, captureErr = os.ReadFile(path)
		}
		window.Update(func() {
			if a.win == nil || a.win.IsDestroyed() {
				return
			}
			if a.thumbnailCaptureTokens[mediaPath] != token {
				return
			}
			if captureErr != nil {
				delete(a.thumbnailCaptureTokens, mediaPath)
				delete(a.thumbnailCaptured, mediaPath)
				return
			}
			if !a.playlistContainsPath(mediaPath) {
				delete(a.thumbnailCaptureTokens, mediaPath)
				delete(a.thumbnailCaptured, mediaPath)
				return
			}
			bitmap, decodeErr := ui.DecodeBitmap(data)
			if decodeErr != nil {
				delete(a.thumbnailCaptureTokens, mediaPath)
				delete(a.thumbnailCaptured, mediaPath)
				return
			}
			if old := a.queueThumbnails[mediaPath]; old != nil {
				old.ReleasePixels()
			}
			width, height := bitmap.Size()
			a.thumbnailRasterBytes[mediaPath] = uint64(width) * uint64(height) * 4
			a.thumbnailEncodedBytes[mediaPath] = uint64(len(data))
			a.queueThumbnails[mediaPath] = bitmap
			delete(a.thumbnailCaptureTokens, mediaPath)
			a.win.Invalidate()
		})
	}()
}

// playlistContainsPath reports whether a media path still has a visible queue
// entry. Playlist paths are normalized by the application when they are added.
func (a *App) playlistContainsPath(mediaPath string) bool {
	for _, path := range a.playlist {
		if strings.EqualFold(filepath.Clean(path), filepath.Clean(mediaPath)) {
			return true
		}
	}
	return false
}

// evictQueueThumbnail releases the bitmap's CPU-side pixels and drops every
// Aurora-owned reference and estimate for a removed queue item. A pending
// capture becomes stale because its generation token is removed.
func (a *App) evictQueueThumbnail(mediaPath string) {
	if a == nil || mediaPath == "" {
		return
	}
	delete(a.thumbnailCaptureTokens, mediaPath)
	delete(a.thumbnailCaptured, mediaPath)
	if bitmap := a.queueThumbnails[mediaPath]; bitmap != nil {
		bitmap.ReleasePixels()
		delete(a.queueThumbnails, mediaPath)
	}
	delete(a.thumbnailRasterBytes, mediaPath)
	delete(a.thumbnailEncodedBytes, mediaPath)
}


func queueCardTitle(path string) string {
	name := strings.TrimSpace(filepath.Base(path))
	if name == "" || name == "." {
		return "Untitled video"
	}
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func queueCardFormat(path string) string {
	ext := strings.TrimPrefix(strings.ToUpper(filepath.Ext(path)), ".")
	if ext == "" {
		return "VIDEO"
	}
	return fmt.Sprintf("%s VIDEO", ext)
}
