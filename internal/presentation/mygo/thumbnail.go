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
		a.thumbnailCaptured[mediaPath] = true
		return
	}
	a.thumbnailCaptured[mediaPath] = true
	go func() {
		if err := capturer.CaptureThumbnail(path); err != nil {
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		a.win.Update(func() {
			if a.win == nil || a.win.IsDestroyed() {
				return
			}
			bitmap, decodeErr := ui.DecodeBitmap(data)
			if decodeErr != nil {
				return
			}
			a.queueThumbnails[mediaPath] = bitmap
			a.win.Invalidate()
		})
	}()
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
