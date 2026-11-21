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
	filmstripVisibleFrames  = 9
	filmstripBitmapLimit    = 12
	filmstripImageMaxBytes  = int64(256 << 10)
	filmstripBrowseStep     = 3
	filmstripWheelThreshold = float32(99)
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
	preferredPosition := time.Duration(clampPlaybackPosition(a.filmstripPreviewPosition, duration.Seconds()) * float64(time.Second))
	a.filmstripGenerating = true
	a.filmstripProgressCompleted = 0
	a.filmstripProgressTotal = 0
	a.filmstripGeneratingFor = mediaPath
	a.filmstripError = ""

	go func() {
		var strip domain.Filmstrip
		var err error
		progressUpdate := func(partial domain.Filmstrip, completed, total int) {
			if window == nil || window.IsDestroyed() {
				return
			}
			window.Update(func() {
				if a.win != window || window.IsDestroyed() || token != a.filmstripGenerationToken || a.path != mediaPath {
					return
				}
				// The adapter sends a full-length frame slice with empty paths
				// for samples that are still being generated. This keeps tile
				// positions stable while completed thumbnails appear in place.
				a.filmstripManifest = partial
				a.filmstripManifestPath = mediaPath
				a.filmstripProgressCompleted = completed
				a.filmstripProgressTotal = total
				window.Invalidate()
				if a.controller != nil && !a.controller.IsDestroyed() {
					a.controller.Invalidate()
				}
			})
		}
		if progressiveGenerator, ok := generator.(interface {
			EnsureProgressive(context.Context, string, time.Duration, time.Duration, func(domain.Filmstrip, int, int)) (domain.Filmstrip, error)
		}); ok {
			strip, err = progressiveGenerator.EnsureProgressive(ctx, mediaPath, duration, preferredPosition, progressUpdate)
		} else if progressGenerator, ok := generator.(interface {
			EnsureWithProgress(context.Context, string, time.Duration, func(completed, total int)) (domain.Filmstrip, error)
		}); ok {
			strip, err = progressGenerator.EnsureWithProgress(ctx, mediaPath, duration, func(completed, total int) {
				progressUpdate(domain.Filmstrip{Duration: duration}, completed, total)
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
				// Failed or canceled generations remove their staging cache
				// directory, so discard every partial path from the UI too.
				a.releaseFilmstripBitmaps()
				a.filmstripManifest = domain.Filmstrip{}
				a.filmstripManifestPath = ""
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
	a.filmstripSelectedFrame = 0
	a.filmstripHasSelectedFrame = false
	a.filmstripWheelRemainder = 0
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
	a.filmstripSelectedFrame = 0
	a.filmstripHasSelectedFrame = false
	a.filmstripWheelRemainder = 0
	a.filmstripBitmaps = nil
	a.filmstripBitmapClock = 0
}

func (a *App) closeFilmstrip() {
	if a == nil {
		return
	}
	a.filmstripOpen = false
	wasGenerating := a.filmstripGenerating
	if a.filmstripGenerationCancel != nil {
		a.filmstripGenerationCancel()
		a.filmstripGenerationCancel = nil
	}
	a.filmstripGenerationToken++
	a.filmstripGenerating = false
	a.filmstripProgressCompleted = 0
	a.filmstripProgressTotal = 0
	a.filmstripGeneratingFor = ""
	if wasGenerating {
		// A progressive manifest points at files in the generation directory.
		// Cancellation removes that directory, so never reuse these paths when
		// the user opens the filmstrip again.
		a.filmstripManifest = domain.Filmstrip{}
		a.filmstripManifestPath = ""
		a.filmstripGenerationAttemptedFor = ""
	}
	a.filmstripDragging = false
	a.filmstripHoverPinned = false
	a.filmstripHoverStart = 0
	a.filmstripSelectedFrame = 0
	a.filmstripHasSelectedFrame = false
	a.filmstripWheelRemainder = 0
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

// filmstripShiftWindowStart moves the visible window by three thumbnails
// per navigation step and clamps it to the first or last available window.
func filmstripShiftWindowStart(frameCount, currentStart, steps int) int {
	visibleCount := min(frameCount, filmstripVisibleFrames)
	if visibleCount <= 0 {
		return 0
	}
	maxStart := frameCount - visibleCount
	return max(0, min(currentStart+steps*filmstripBrowseStep, maxStart))
}

// filmstripWheelSteps turns wheel motion into browsing steps. Standard mouse
// wheel messages advance one step per event; precise touchpad deltas accumulate
// until they cover roughly one default Windows wheel notch.
func filmstripWheelSteps(delta, remainder float32, precise bool) (int, float32) {
	if delta == 0 || math.IsNaN(float64(delta)) || math.IsInf(float64(delta), 0) {
		return 0, remainder
	}
	if !precise {
		if delta < 0 {
			return -1, 0
		}
		return 1, 0
	}
	total := remainder + delta
	steps := int(total / filmstripWheelThreshold)
	return steps, total - float32(steps)*filmstripWheelThreshold
}

func (a *App) currentFilmstripWindowStart() int {
	if a == nil {
		return 0
	}
	frameCount := len(a.filmstripManifest.Frames)
	if a.filmstripHoverPinned {
		visibleCount := min(frameCount, filmstripVisibleFrames)
		return max(0, min(a.filmstripHoverStart, max(0, frameCount-visibleCount)))
	}
	return filmstripWindowStart(
		frameCount,
		a.filmstripPreviewPosition,
		a.filmstripManifest.Duration.Seconds(),
	)
}

// shiftFilmstripWindow browses without changing playback or the preview seek
// position. The next pointer movement over the strip selects the frame under it.
// filmstripFrameIndexAtX maps a pointer position to the actual tile layout.
// Unlike dividing the whole row into equal fractions, this accounts for the
// 3-DIP gaps between the nine tiles. A pointer on the final tile therefore
// always selects the final frame rather than a neighboring logical slot.
func filmstripFrameIndexAtX(x, rowWidth float32, frameCount int) int {
	if frameCount <= 0 {
		return -1
	}
	const padding = float32(3)
	const gap = float32(3)
	contentWidth := rowWidth - 2*padding
	if contentWidth <= 0 {
		return 0
	}
	contentX := max(float32(0), min(x-padding, contentWidth))
	tileWidth := (contentWidth - gap*float32(frameCount-1)) / float32(frameCount)
	if tileWidth <= 0 {
		return filmstripFrameIndexAtRatio(float64(contentX/contentWidth), frameCount)
	}
	pitch := tileWidth + gap
	index := int(math.Floor(float64(contentX / pitch)))
	index = max(0, min(index, frameCount-1))
	offset := contentX - float32(index)*pitch
	// A gap belongs to whichever neighboring tile center is nearer.
	if index < frameCount-1 && offset > tileWidth && offset-tileWidth > gap/2 {
		index++
	}
	return index
}

func (a *App) shiftFilmstripWindow(steps int) {
	if a == nil || steps == 0 {
		return
	}
	frameCount := len(a.filmstripManifest.Frames)
	if frameCount == 0 {
		return
	}
	start := a.currentFilmstripWindowStart()
	a.filmstripHoverStart = filmstripShiftWindowStart(frameCount, start, steps)
	a.filmstripHoverPinned = true
}

// handleFilmstripInput consumes wheel events only while the pointer is over
// the filmstrip row. It deliberately does not seek the player.
func (a *App) handleFilmstripInput(ev ui.InputEvent) bool {
	if ev.Kind != ui.InputScroll {
		return false
	}
	if a == nil || !a.filmstripOpen || a.filmstripDragging {
		return true
	}
	delta := ev.DY
	if delta == 0 {
		delta = ev.DX
	}
	steps, remainder := filmstripWheelSteps(delta, a.filmstripWheelRemainder, ev.Precise)
	a.filmstripWheelRemainder = remainder
	if steps != 0 {
		a.shiftFilmstripWindow(steps)
	}
	return true
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
		if frame.Path == "" {
			continue
		}
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

			windowStart := a.currentFilmstripWindowStart()
			maxWindowStart := max(0, len(a.filmstripManifest.Frames)-min(len(a.filmstripManifest.Frames), filmstripVisibleFrames))
			ui.Row(c).FillWidth().Height(72).Gap(6).AlignItems(ui.Center).Children(func() {
				previous := ui.Button(c, "‹").Width(30).MinWidth(30).Shrink(0).Height(72).Padding(0).
					Radius(6).Background(ui.Hex("#191A1B")).
					Border(1, ui.Hex(auroraBorderSoft)).
					FontSize(14).TextColor(ui.Hex(auroraText)).
					Disabled(windowStart <= 0).Tooltip("Previous thumbnails")
				if previous.Clicked() {
					a.shiftFilmstripWindow(-1)
				}

				strip := ui.Row(c).Grow(1).FillHeight().MinWidth(0).Gap(3).Padding(3).
					Background(ui.Hex("#080909")).Radius(5)
				strip.HandleInput(a.handleFilmstripInput)
				x, _, over := strip.PointerPosition()
				bounds := strip.Bounds()
				pressedInside := strip.PressedInside()
				pressed := strip.Pressed()
				if pressedInside {
					a.filmstripDragging = true
				}

				pointerActive := over || pressed || a.filmstripDragging
				hoveredIndex := -1
				if strip.PressCanceled() {
					a.filmstripDragging = false
					a.filmstripHoverPinned = false
					a.filmstripHoverStart = 0
					a.filmstripWheelRemainder = 0
					a.filmstripHasSelectedFrame = false
					a.filmstripPreviewPosition = a.state.Position
					pointerActive = false
				} else {
					if pointerActive && !a.filmstripHoverPinned {
						a.filmstripHoverStart = filmstripWindowStart(
							len(a.filmstripManifest.Frames),
							a.filmstripPreviewPosition,
							a.filmstripManifest.Duration.Seconds(),
						)
						a.filmstripHoverPinned = true
					}

					frames := a.visibleFilmstripFrames()
					if a.filmstripDragging && !over && len(frames) > 0 && bounds.W > 6 {
						// If a drag leaves the strip, clamp to its nearest edge
						// thumbnail. Inside the strip, the tile itself handles hits.
						hoveredIndex = filmstripFrameIndexAtX(x, bounds.W, len(frames))
						a.filmstripSelectedFrame = a.currentFilmstripWindowStart() + hoveredIndex
						a.filmstripHasSelectedFrame = true
						a.filmstripPreviewPosition = frames[hoveredIndex].At.Seconds()
					}
					// Do not reset the preview when the pointer leaves the
					// thumbnails. It is the user's current selection and must
					// remain highlighted while they move to Previous/Next or
					// browse with the wheel. Playback position stays separate.

					if !pressed && a.filmstripDragging {
						a.filmstripDragging = false
						a.seekToFilmstripPreview()
					}
				}

				frames := a.visibleFilmstripFrames()
				a.requestVisibleFilmstripBitmaps(frames)
				visibleStart := a.currentFilmstripWindowStart()
				requestedFrames := frames
				strip.Children(func() {
					for i, frame := range requestedFrames {
						tile := ui.Box(c).Key(fmt.Sprintf("filmstrip-frame-%d", visibleStart+i)).Grow(1).FillHeight().MinWidth(0).Clip().
							Radius(4).Background(ui.Hex("#202122"))

						// Ask the tile itself whether it is hovered or was clicked.
						// This avoids parent-row coordinate rounding at the last tile.
						tileHovered := tile.Hovered()
						tileClicked := tile.Clicked()
						if tileHovered || tileClicked {
							a.filmstripSelectedFrame = visibleStart + i
							a.filmstripHasSelectedFrame = true
							a.filmstripPreviewPosition = frame.At.Seconds()
						}

						selectedIndex := filmstripSelectedIndex(
							requestedFrames,
							visibleStart,
							a.filmstripSelectedFrame,
							a.filmstripHasSelectedFrame,
							a.filmstripPreviewPosition,
						)
						highlight := i == selectedIndex || tileHovered || tileClicked
						border := auroraBorderSoft
						if highlight {
							border = auroraAccent
						}
						tile = tile.Border(1, ui.Hex(border))
						tile.Children(func() {
							if frame.Path != "" {
								if cached := a.filmstripBitmaps[frame.Path]; cached != nil && cached.bitmap != nil {
									ui.Image(c, cached.bitmap).FillWidth().FillHeight().Fit(ui.Cover)
								} else {
									ui.Column(c).Fill().Center().Children(func() {
										ui.Text(c, "· · ·").FontSize(10).TextColor(ui.Hex(auroraMuted))
									})
								}
							} else {
								ui.Column(c).Fill().Center().Children(func() {
									ui.Text(c, "…").FontSize(10).TextColor(ui.Hex(auroraMuted))
								})
							}
						})
					}
				})

				next := ui.Button(c, "›").Width(30).MinWidth(30).Shrink(0).Height(72).Padding(0).
					Radius(6).Background(ui.Hex("#191A1B")).
					Border(1, ui.Hex(auroraBorderSoft)).
					FontSize(14).TextColor(ui.Hex(auroraText)).
					Disabled(windowStart >= maxWindowStart).Tooltip("Next thumbnails")
				if next.Clicked() {
					a.shiftFilmstripWindow(1)
				}
			})
			hint := "Scroll to browse thumbnails · drag across and release to seek"
			if a.filmstripGenerating && a.filmstripProgressTotal > 0 {
				hint = fmt.Sprintf("Generating thumbnails %d/%d… · scroll to browse and drag to seek",
					a.filmstripProgressCompleted, a.filmstripProgressTotal)
			}
			ui.Text(c, hint).FontSize(8).TextColor(ui.Hex(auroraMuted))
		})
}

// filmstripFrameIndexForPosition returns the selected tile in the current
// window, or -1 when the retained preview position is outside that window.
func filmstripFrameIndexForPosition(frames []domain.FilmstripFrame, positionSeconds float64) int {
	if len(frames) == 0 || positionSeconds < 0 || math.IsNaN(positionSeconds) || math.IsInf(positionSeconds, 0) {
		return -1
	}

	selected := nearestFilmstripFrame(frames, positionSeconds)
	selectedIndex := -1
	for i := range frames {
		if frames[i].At == selected.At {
			selectedIndex = i
			break
		}
	}
	if selectedIndex < 0 {
		return -1
	}

	target := time.Duration(math.Max(0, positionSeconds) * float64(time.Second))
	distance := absDuration(selected.At - target)

	// A sampled frame represents the interval halfway to its neighbors.
	// Allow that half-interval beyond the first/last sample, since the video
	// duration usually extends beyond the timestamp of the final thumbnail.
	// Still return no selection when the preview is farther away than the
	// selected frame's nearest sampling boundary.
	var nearestGap time.Duration
	switch {
	case len(frames) == 1:
		// With a single thumbnail there is no neighbor from which to infer
		// its interval, so only highlight it at its own timestamp.
		if distance == 0 {
			return selectedIndex
		}
		return -1
	case selectedIndex == 0:
		nearestGap = absDuration(frames[1].At - frames[0].At)
	case selectedIndex == len(frames)-1:
		nearestGap = absDuration(frames[selectedIndex].At - frames[selectedIndex-1].At)
	default:
		before := absDuration(frames[selectedIndex].At - frames[selectedIndex-1].At)
		after := absDuration(frames[selectedIndex+1].At - frames[selectedIndex].At)
		nearestGap = min(before, after)
	}
	if distance > nearestGap/2 {
		return -1
	}
	return selectedIndex
}

// filmstripSelectedIndex resolves the persistent manifest-frame selection
// into the current visible window. An explicit selection outside the window
// stays hidden; it must not silently jump to the playback position.
func filmstripSelectedIndex(frames []domain.FilmstripFrame, windowStart, selectedFrame int, hasSelection bool, playbackPosition float64) int {
	if len(frames) == 0 {
		return -1
	}
	if hasSelection {
		if selectedFrame < windowStart || selectedFrame >= windowStart+len(frames) {
			return -1
		}
		return selectedFrame - windowStart
	}
	return filmstripFrameIndexForPosition(frames, playbackPosition)
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
