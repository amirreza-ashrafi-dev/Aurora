package presentation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
	xdraw "golang.org/x/image/draw"
)

const thumbnailMaxDimension = 320

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
	a.playerCommandMu.RLock()
	player := a.player
	a.playerCommandMu.RUnlock()
	if player == nil {
		return
	}
	capturer, ok := player.(thumbnailCapturer)
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
		// Hold a read lock only for the native MPV call. closePlayer takes the
		// write lock, so it cannot destroy the context or unload libmpv mid-call.
		a.playerCommandMu.RLock()
		if a.player != player {
			a.playerCommandMu.RUnlock()
			return
		}
		captureErr := capturer.CaptureThumbnail(path)
		a.playerCommandMu.RUnlock()
		data := []byte(nil)
		width, height := 0, 0
		if captureErr == nil {
			data, captureErr = os.ReadFile(path)
		}
		if captureErr == nil {
			data, width, height, captureErr = resizeThumbnailPNG(data, thumbnailMaxDimension)
			if captureErr == nil {
				// Keep the on-disk cache compact too. A failed cache rewrite does
				// not prevent displaying the already-resized in-memory thumbnail.
				_ = os.WriteFile(path, data, 0o600)
			}
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
			if width <= 0 || height <= 0 {
				width, height = bitmap.Size()
			}
			a.thumbnailRasterBytes[mediaPath] = uint64(width) * uint64(height) * 4
			a.thumbnailEncodedBytes[mediaPath] = uint64(len(data))
			a.queueThumbnails[mediaPath] = bitmap
			delete(a.thumbnailCaptureTokens, mediaPath)
			a.win.Invalidate()
		})
	}()
}

// resizeThumbnailPNG bounds queue-card images independently of source video
// resolution. This affects only the small preview shown in the playlist, never
// the decoded video or its playback quality.
func resizeThumbnailPNG(data []byte, maxDimension int) ([]byte, int, int, error) {
	if maxDimension <= 0 {
		return nil, 0, 0, fmt.Errorf("thumbnail maximum dimension must be positive")
	}
	source, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decode captured thumbnail: %w", err)
	}
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, 0, 0, fmt.Errorf("captured thumbnail has invalid dimensions")
	}
	longest := max(width, height)
	if longest <= maxDimension {
		return data, width, height, nil
	}

	scale := float64(maxDimension) / float64(longest)
	targetWidth := max(1, int(float64(width)*scale+0.5))
	targetHeight := max(1, int(float64(height)*scale+0.5))
	target := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	xdraw.CatmullRom.Scale(target, target.Bounds(), source, bounds, xdraw.Over, nil)

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, target); err != nil {
		return nil, 0, 0, fmt.Errorf("encode resized thumbnail: %w", err)
	}
	return encoded.Bytes(), targetWidth, targetHeight, nil
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
	}
	// Delete the key even when its value is nil. A nil bitmap can be left by
	// an interrupted or not-yet-completed capture, but it is still tracked
	// queue state and must be evicted with the rest of the media's metadata.
	delete(a.queueThumbnails, mediaPath)
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
