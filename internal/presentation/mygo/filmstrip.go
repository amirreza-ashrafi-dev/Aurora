package presentation

import (
	"context"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/egoist/mygo/ui"

	"my-go/internal/domain"
)

const (
	filmstripVisibleFrames = 9
	filmstripBitmapLimit   = 12
	filmstripImageMaxBytes = int64(256 << 10)
)

type filmstripBitmap struct {
	bitmap       *ui.Bitmap
	rasterBytes  uint64
	encodedBytes uint64
	lastUsed     uint64
}

func (a *App) filmstripToggleButton(c *ui.Context) {
	if a == nil || a.path == "" {
		return
	}
	label := "▤"
	tooltip := "Show thumbnail filmstrip"
	if a.filmstripOpen {
		label = "▤  Hide"
		tooltip = "Hide thumbnail filmstrip"
	}
	if auroraQuietButton(c, label).Tooltip(tooltip).Clicked() {
		a.toggleFilmstrip()
	}
}

func (a *App) toggleFilmstrip() {
	if a == nil {
		return
	}
	if a.path == "" {
		return
	}
	if a.filmstripOpen {
		a.closeFilmstrip()
	} else {
		a.filmstripOpen = true
		a.filmstripError = ""
		a.filmstripGenerationAttemptedFor = ""
		a.filmstripPreviewPosition = a.state.Position
		a.ensureFilmstripGeneration()
	}
	a.updateFullscreenControllerSize()
	if a.win != nil && !a.win.IsDestroyed() {
		a.win.Invalidate()
	}
}

func (a *App) ensureFilmstripGeneration() {
	if a == nil || !a.filmstripOpen || a.path == "" || a.filmstripGenerating {
		return
	}
	if len(a.filmstripManifest.Frames) > 0 && a.filmstripManifestPath == a.path {
		return
	}
	if a.filmstripGenerationAttemptedFor == a.path && a.filmstripError != "" {
		return
	}
	if a.filmstripGenerator == nil {
		a.filmstripGenerationAttemptedFor = a.path
		a.filmstripError = "Thumbnail extraction is not configured."
		return
	}
	if a.state.Duration <= 0 || math.IsNaN(a.state.Duration) || math.IsInf(a.state.Duration, 0) {
		return
	}
	duration := time.Duration(a.state.Duration * float64(time.Second))
	if duration <= 0 {
		return
	}

	if a.filmstripGenerationCancel != nil {
		a.filmstripGenerationCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.filmstripGenerationCancel = cancel
	a.filmstripGenerationToken++
	token := a.filmstripGenerationToken
	mediaPath := a.path
	a.filmstripGenerationAttemptedFor = mediaPath
	generator := a.filmstripGenerator
	window := a.win
	a.filmstripGenerating = true
	a.filmstripProgressCompleted = 0
	a.filmstripProgressTotal = 0
	a.filmstripGeneratingFor = mediaPath
	a.filmstripError = ""

	go func() {
		var strip domain.Filmstrip
		var err error
		if progressGenerator, ok := generator.(interface {
			EnsureWithProgress(context.Context, string, time.Duration, func(completed, total int)) (domain.Filmstrip, error)
		}); ok {
			strip, err = progressGenerator.EnsureWithProgress(ctx, mediaPath, duration, func(completed, total int) {
				if window == nil || window.IsDestroyed() {
					return
				}
				window.Update(func() {
					if a.win != window || window.IsDestroyed() || token != a.filmstripGenerationToken || a.path != mediaPath {
						return
					}
					a.filmstripProgressCompleted = completed
					a.filmstripProgressTotal = total
					window.Invalidate()
				})
			})
		} else {
			strip, err = generator.Ensure(ctx, mediaPath, duration)
		}
		if window == nil || window.IsDestroyed() {
			return
		}
		window.Update(func() {
			if a.win != window || window.IsDestroyed() || token != a.filmstripGenerationToken || a.path != mediaPath {
				return
			}
			a.filmstripGenerating = false
			a.filmstripProgressCompleted = 0
			a.filmstripProgressTotal = 0
			a.filmstripGeneratingFor = ""
			a.filmstripGenerationCancel = nil
			if err != nil {
				if ctx.Err() == nil {
					a.filmstripError = err.Error()
				}
				window.Invalidate()
				if a.controller != nil && !a.controller.IsDestroyed() {
					a.controller.Invalidate()
				}
				return
			}
			a.filmstripManifest = strip
			a.filmstripManifestPath = mediaPath
			a.filmstripPreviewPosition = clampPlaybackPosition(a.filmstripPreviewPosition, strip.Duration.Seconds())
			window.Invalidate()
			if a.controller != nil && !a.controller.IsDestroyed() {
				a.controller.Invalidate()
			}
		})
	}()
}

func (a *App) resetFilmstripForMedia() {
	if a == nil {
		return
	}
	if a.filmstripGenerationCancel != nil {
		a.filmstripGenerationCancel()
		a.filmstripGenerationCancel = nil
	}
	a.filmstripGenerationToken++
	a.filmstripManifest = domain.Filmstrip{}
	a.filmstripManifestPath = ""
	a.filmstripGenerating = false
	a.filmstripProgressCompleted = 0
	a.filmstripProgressTotal = 0
	a.filmstripGeneratingFor = ""
	a.filmstripGenerationAttemptedFor = ""
	a.filmstripError = ""
	a.filmstripPreviewPosition = 0
	a.timelinePreviewPosition = 0
	a.filmstripDragging = false
	a.filmstripHoverPinned = false
	a.filmstripHoverStart = 0
	a.releaseFilmstripBitmaps()
}


// abandonFilmstripAfterShutdown cancels background work and drops references
// after the MyGo event loop has stopped. It deliberately does not call
// Bitmap.ReleasePixels here: that API must be used on the UI thread.
func (a *App) abandonFilmstripAfterShutdown() {
	if a == nil {
		return
	}
	a.filmstripOpen = false
	if a.filmstripGenerationCancel != nil {
		a.filmstripGenerationCancel()
		a.filmstripGenerationCancel = nil
	}
	a.filmstripGenerationToken++
	a.filmstripLoadToken++
	a.filmstripGenerating = false
	a.filmstripGeneratingFor = ""
	a.filmstripLoadInFlight = false
	a.filmstripDragging = false
	a.filmstripHoverPinned = false
	a.filmstripHoverStart = 0
	a.filmstripBitmaps = nil
	a.filmstripBitmapClock = 0
}

func (a *App) closeFilmstrip() {
	if a == nil {
		return
	}
	a.filmstripOpen = false
	if a.filmstripGenerationCancel != nil {
		a.filmstripGenerationCancel()
		a.filmstripGenerationCancel = nil
	}
	a.filmstripGenerationToken++
	a.filmstripGenerating = false
	a.filmstripGeneratingFor = ""
	a.filmstripDragging = false
	a.filmstripHoverPinned = false
	a.filmstripHoverStart = 0
	a.filmstripPreviewPosition = a.state.Position
	a.releaseFilmstripBitmaps()
}

func (a *App) releaseFilmstripBitmaps() {
	if a == nil {
		return
	}
	for _, cached := range a.filmstripBitmaps {
		if cached != nil && cached.bitmap != nil {
			cached.bitmap.ReleasePixels()
		}
	}
	a.filmstripBitmaps = nil
	a.filmstripBitmapClock = 0
	a.filmstripLoadToken++
	a.filmstripLoadInFlight = false
}

func (a *App) visibleFilmstripFrames() []domain.FilmstripFrame {
	if a == nil || len(a.filmstripManifest.Frames) == 0 {
		return nil
	}
	frames := a.filmstripManifest.Frames
	start := 0
	if a.filmstripHoverPinned {
		start = a.filmstripHoverStart
	} else {
		start = filmstripWindowStart(len(frames), a.filmstripPreviewPosition, a.filmstripManifest.Duration.Seconds())
	}
	return filmstripFramesAtStart(frames, start)
}

// filmstripWindowStart chooses the fixed thumbnail window centered around a
// media position. The window is pinned while the pointer is over the strip so
// its tiles cannot slide underneath the pointer during hover or dragging.
func filmstripWindowStart(frameCount int, positionSeconds, durationSeconds float64) int {
	visibleCount := min(frameCount, filmstripVisibleFrames)
	if visibleCount <= 0 || frameCount <= visibleCount || durationSeconds <= 0 {
		return 0
	}
	ratio := clampPlaybackPosition(positionSeconds, durationSeconds) / durationSeconds
	center := int(math.Round(ratio * float64(frameCount-1)))
	start := center - visibleCount/2
	return max(0, min(start, frameCount-visibleCount))
}

func filmstripFramesAtStart(frames []domain.FilmstripFrame, start int) []domain.FilmstripFrame {
	visibleCount := min(len(frames), filmstripVisibleFrames)
	if visibleCount == 0 {
		return nil
	}
	start = max(0, min(start, len(frames)-visibleCount))
	return frames[start : start+visibleCount]
}

// filmstripFrameIndexAtRatio maps the pointer's horizontal position to the
// thumbnail tile beneath it. Each tile gets an equal share of the strip width.
func filmstripFrameIndexAtRatio(ratio float64, frameCount int) int {
	if frameCount <= 0 {
		return -1
	}
	if math.IsNaN(ratio) || math.IsInf(ratio, 0) {
		ratio = 0
	}
	ratio = math.Max(0, math.Min(1, ratio))
	index := int(math.Floor(ratio * float64(frameCount)))
	return min(index, frameCount-1)
}

func (a *App) requestVisibleFilmstripBitmaps(frames []domain.FilmstripFrame) {
	if a == nil || !a.filmstripOpen || a.win == nil || a.win.IsDestroyed() || len(frames) == 0 {
		return
	}
	if a.filmstripBitmaps == nil {
		a.filmstripBitmaps = make(map[string]*filmstripBitmap)
	}
	keep := make(map[string]struct{}, len(frames))
	missing := make([]domain.FilmstripFrame, 0, len(frames))
	for _, frame := range frames {
		keep[frame.Path] = struct{}{}
		if cached := a.filmstripBitmaps[frame.Path]; cached != nil {
			a.filmstripBitmapClock++
			cached.lastUsed = a.filmstripBitmapClock
			continue
		}
		missing = append(missing, frame)
	}
	a.evictFilmstripBitmaps(keep)
	if len(missing) == 0 || a.filmstripLoadInFlight {
		return
	}

	a.filmstripLoadInFlight = true
	a.filmstripLoadToken++
	token := a.filmstripLoadToken
	mediaPath := a.path
	window := a.win
	go func() {
		type loadedFrame struct {
			frame domain.FilmstripFrame
			data  []byte
			err   error
		}
		loaded := make([]loadedFrame, 0, len(missing))
		for _, frame := range missing {
			item := loadedFrame{frame: frame}
			info, err := os.Stat(frame.Path)
			switch {
			case err != nil:
				item.err = err
			case !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > filmstripImageMaxBytes:
				item.err = fmt.Errorf("thumbnail has an unexpected size")
			default:
				item.data, item.err = os.ReadFile(frame.Path)
			}
			loaded = append(loaded, item)
		}
		if window == nil || window.IsDestroyed() {
			return
		}
		window.Update(func() {
			if token != a.filmstripLoadToken {
				return
			}
			a.filmstripLoadInFlight = false
			if a.win != window || window.IsDestroyed() || !a.filmstripOpen || a.path != mediaPath {
				return
			}
			if a.filmstripBitmaps == nil {
				a.filmstripBitmaps = make(map[string]*filmstripBitmap)
			}
			for _, item := range loaded {
				if item.err != nil || len(item.data) == 0 {
					continue
				}
				if a.filmstripBitmaps[item.frame.Path] != nil {
					continue
				}
				bitmap, err := ui.DecodeBitmap(item.data)
				if err != nil {
					continue
				}
				width, height := bitmap.Size()
				a.filmstripBitmapClock++
				a.filmstripBitmaps[item.frame.Path] = &filmstripBitmap{
					bitmap:       bitmap,
					rasterBytes:  uint64(width) * uint64(height) * 4,
					encodedBytes: uint64(len(item.data)),
					lastUsed:     a.filmstripBitmapClock,
				}
			}
			a.evictFilmstripBitmaps(keep)
			window.Invalidate()
			if a.controller != nil && !a.controller.IsDestroyed() {
				a.controller.Invalidate()
			}
		})
	}()
}

func (a *App) evictFilmstripBitmaps(keep map[string]struct{}) {
	if a == nil {
		return
	}
	for len(a.filmstripBitmaps) > filmstripBitmapLimit {
		oldestPath := ""
		oldestUse := ^uint64(0)
		for path, cached := range a.filmstripBitmaps {
			if _, visible := keep[path]; visible || cached == nil {
				continue
			}
			if oldestPath == "" || cached.lastUsed < oldestUse {
				oldestPath, oldestUse = path, cached.lastUsed
			}
		}
		if oldestPath == "" {
			break
		}
		if cached := a.filmstripBitmaps[oldestPath]; cached != nil && cached.bitmap != nil {
			cached.bitmap.ReleasePixels()
		}
		delete(a.filmstripBitmaps, oldestPath)
	}
}

func (a *App) seekToFilmstripPreview() {
	if a == nil {
		return
	}
	a.seekToPosition(a.filmstripPreviewPosition)
}

func (a *App) filmstripView(c *ui.Context) {
	if a == nil || !a.filmstripOpen {
		return
	}
	a.ensureFilmstripGeneration()

	ui.Column(c).FillWidth().Gap(5).Padding(8, 10).
		Background(ui.Hex("#121314")).
		Border(1, ui.Hex(auroraBorderSoft)).
		Radius(9).
		Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Text(c, "VISUAL SEEK").FontSize(8).Bold().
					LetterSpacing(1.1).TextColor(ui.Hex(auroraAccent))
				ui.Spacer(c)
				ui.Text(c, formatTime(a.filmstripPreviewPosition)).
					FontSize(9).Bold().TextColor(ui.Hex(auroraText))
				if auroraQuietButton(c, "×").Tooltip("Close thumbnail filmstrip").Clicked() {
					a.toggleFilmstrip()
				}
			})

			if len(a.filmstripManifest.Frames) == 0 {
				message := "Preparing video thumbnails…"
				if a.filmstripGenerating && a.filmstripProgressTotal > 0 {
					message = fmt.Sprintf("Generating thumbnails %d/%d…", a.filmstripProgressCompleted, a.filmstripProgressTotal)
				}
				if a.filmstripError != "" {
					message = a.filmstripError
				} else if !a.filmstripGenerating && (a.state.Duration <= 0 || math.IsNaN(a.state.Duration)) {
					message = "Waiting for video duration…"
				}
				ui.Box(c).FillWidth().Height(56).Center().Padding(8).
					Background(ui.Hex("#191A1B")).Radius(5).
					Children(func() {
						ui.Text(c, message).FontSize(9).TextColor(ui.Hex(auroraMuted))
					})
				return
			}

			strip := ui.Row(c).FillWidth().Height(72).Gap(3).Padding(3).
				Background(ui.Hex("#080909")).Radius(5)
			x, _, over := strip.PointerPosition()
			bounds := strip.Bounds()
			pressedInside := strip.PressedInside()
			pressed := strip.Pressed()
			if pressedInside {
				a.filmstripDragging = true
			}

			if strip.PressCanceled() {
				a.filmstripDragging = false
				a.filmstripHoverPinned = false
				a.filmstripHoverStart = 0
				a.filmstripPreviewPosition = a.state.Position
			} else {
				pointerActive := over || pressed || a.filmstripDragging
				if pointerActive && !a.filmstripHoverPinned {
					a.filmstripHoverStart = filmstripWindowStart(
						len(a.filmstripManifest.Frames),
						a.filmstripPreviewPosition,
						a.filmstripManifest.Duration.Seconds(),
					)
					a.filmstripHoverPinned = true
				}

				frames := a.visibleFilmstripFrames()
				if pointerActive && len(frames) > 0 && bounds.W > 6 {
					// PointerPosition is local to the row's outer bounds.
					// Exclude the row's 3-DIP left/right padding so the
					// first and last thumbnail centers match the pointer.
					contentWidth := bounds.W - 6
					contentX := x - 3
					ratio := float64(contentX / contentWidth)
					index := filmstripFrameIndexAtRatio(ratio, len(frames))
					// Selecting the frame under the pointer keeps the visual
					// focus, timestamp, and eventual seek on the same frame.
					a.filmstripPreviewPosition = frames[index].At.Seconds()
				} else if !pointerActive {
					a.filmstripHoverPinned = false
					a.filmstripHoverStart = 0
					a.filmstripPreviewPosition = a.state.Position
				}

				if !pressed && a.filmstripDragging {
					a.filmstripDragging = false
					a.seekToFilmstripPreview()
				}
			}

			frames := a.visibleFilmstripFrames()
			a.requestVisibleFilmstripBitmaps(frames)
			selected := nearestFilmstripFrame(frames, a.filmstripPreviewPosition)
			strip.Children(func() {
				for _, frame := range frames {
					highlight := frame.At == selected.At
					border := auroraBorderSoft
					if highlight {
						border = auroraAccent
					}
					tile := ui.Box(c).Grow(1).FillHeight().MinWidth(0).Clip().
						Radius(4).Border(1, ui.Hex(border)).
						Background(ui.Hex("#202122"))
					tile.Children(func() {
						if cached := a.filmstripBitmaps[frame.Path]; cached != nil && cached.bitmap != nil {
							ui.Image(c, cached.bitmap).FillWidth().FillHeight().Fit(ui.Cover)
						} else {
							ui.Column(c).Fill().Center().Children(func() {
								ui.Text(c, "· · ·").FontSize(10).TextColor(ui.Hex(auroraMuted))
							})
						}
					})
				}
			})
			ui.Text(c, "Drag across the thumbnails and release to seek").
				FontSize(8).TextColor(ui.Hex(auroraMuted))
		})
}

func nearestFilmstripFrame(frames []domain.FilmstripFrame, positionSeconds float64) domain.FilmstripFrame {
	if len(frames) == 0 {
		return domain.FilmstripFrame{}
	}
	target := time.Duration(math.Max(0, positionSeconds) * float64(time.Second))
	best := frames[0]
	bestDistance := absDuration(best.At - target)
	for _, frame := range frames[1:] {
		distance := absDuration(frame.At - target)
		if distance < bestDistance {
			best, bestDistance = frame, distance
		}
	}
	return best
}

func absDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}

func clampPlaybackPosition(position, duration float64) float64 {
	if math.IsNaN(position) || math.IsInf(position, 0) || position < 0 {
		return 0
	}
	if duration > 0 && position > duration {
		return duration
	}
	return position
}
