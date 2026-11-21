package presentation

import (
	"math"
	"testing"
	"time"

	"my-go/internal/domain"
)

func TestNearestFilmstripFrame(t *testing.T) {
	frames := []domain.FilmstripFrame{
		{At: 0},
		{At: 10 * time.Second},
		{At: 20 * time.Second},
	}
	got := nearestFilmstripFrame(frames, 14.9)
	if got.At != 10*time.Second {
		t.Fatalf("nearest frame = %s, want 10s", got.At)
	}
}

func TestVisibleFilmstripFramesCentersAroundPreview(t *testing.T) {
	frames := make([]domain.FilmstripFrame, 30)
	for i := range frames {
		frames[i].At = time.Duration(i) * time.Second
	}
	app := &App{
		filmstripManifest: domain.Filmstrip{
			Duration: 30 * time.Second,
			Frames:   frames,
		},
		filmstripPreviewPosition: 20,
	}
	got := app.visibleFilmstripFrames()
	if len(got) != filmstripVisibleFrames {
		t.Fatalf("visible frame count = %d, want %d", len(got), filmstripVisibleFrames)
	}
	if got[0].At > 20*time.Second || got[len(got)-1].At < 20*time.Second {
		t.Fatalf("visible frame range %s–%s does not include preview 20s", got[0].At, got[len(got)-1].At)
	}
}


func TestFilmstripWindowStartCentersOnPreview(t *testing.T) {
	if got := filmstripWindowStart(48, 24, 47); got != 20 {
		t.Fatalf("filmstripWindowStart() = %d, want 20", got)
	}
	if got := filmstripWindowStart(48, 0, 47); got != 0 {
		t.Fatalf("filmstripWindowStart() at start = %d, want 0", got)
	}
	if got := filmstripWindowStart(48, 47, 47); got != 39 {
		t.Fatalf("filmstripWindowStart() at end = %d, want 39", got)
	}
}

func TestFilmstripFramesAtStartClampsWindow(t *testing.T) {
	frames := make([]domain.FilmstripFrame, 12)
	for i := range frames {
		frames[i].At = time.Duration(i) * time.Second
	}
	got := filmstripFramesAtStart(frames, 99)
	if len(got) != filmstripVisibleFrames {
		t.Fatalf("visible count = %d, want %d", len(got), filmstripVisibleFrames)
	}
	if got[0].At != 3*time.Second || got[len(got)-1].At != 11*time.Second {
		t.Fatalf("unexpected clamped window: %s–%s", got[0].At, got[len(got)-1].At)
	}
}

func TestFilmstripFrameIndexAtRatio(t *testing.T) {
	tests := []struct {
		name string
		ratio float64
		count int
		want int
	}{
		{name: "left edge", ratio: 0, count: 9, want: 0},
		{name: "first tile", ratio: 0.05, count: 9, want: 0},
		{name: "middle tile", ratio: 0.5, count: 9, want: 4},
		{name: "right edge", ratio: 1, count: 9, want: 8},
		{name: "outside right", ratio: 2, count: 9, want: 8},
		{name: "empty strip", ratio: 0.5, count: 0, want: -1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := filmstripFrameIndexAtRatio(test.ratio, test.count); got != test.want {
				t.Fatalf("filmstripFrameIndexAtRatio() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestVisibleFilmstripFramesKeepsPinnedWindow(t *testing.T) {
	frames := make([]domain.FilmstripFrame, 48)
	for i := range frames {
		frames[i].At = time.Duration(i) * time.Second
	}
	app := &App{
		filmstripManifest: domain.Filmstrip{
			Duration: 48 * time.Second,
			Frames: frames,
		},
		filmstripPreviewPosition: 40,
		filmstripHoverPinned: true,
		filmstripHoverStart: 7,
	}
	got := app.visibleFilmstripFrames()
	if got[0].At != 7*time.Second || got[len(got)-1].At != 15*time.Second {
		t.Fatalf("pinned window = %s–%s, want 7s–15s", got[0].At, got[len(got)-1].At)
	}
}


func TestFilmstripSelectedIndexUsesPersistentManifestIndex(t *testing.T) {
	frames := make([]domain.FilmstripFrame, 16)
	for i := range frames {
		frames[i].At = time.Duration(i) * time.Second
	}

	initial := filmstripFramesAtStart(frames, 0)
	if got := filmstripSelectedIndex(initial, 0, 8, true, 0); got != 8 {
		t.Fatalf("last tile selection = %d, want 8", got)
	}

	nextStart := filmstripShiftWindowStart(len(frames), 0, 1)
	next := filmstripFramesAtStart(frames, nextStart)
	if got := filmstripSelectedIndex(next, nextStart, 8, true, 0); got != 5 {
		t.Fatalf("selection after Next = %d, want 5", got)
	}
	if got := filmstripSelectedIndex(filmstripFramesAtStart(frames, 9), 9, 8, true, 0); got != -1 {
		t.Fatalf("selection outside window = %d, want -1", got)
	}
}

func TestFilmstripSelectedIndexFallsBackToPlaybackOnlyWithoutExplicitSelection(t *testing.T) {
	frames := make([]domain.FilmstripFrame, 9)
	for i := range frames {
		frames[i].At = time.Duration(i) * time.Second
	}

	tests := []struct {
		name     string
		position float64
		want     int
	}{
		{name: "inside final sample interval", position: 8.1, want: 8},
		{name: "at final sample", position: 8, want: 8},
		{name: "outside final sample interval", position: 8.6, want: -1},
		{name: "before first sample interval", position: -0.6, want: -1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := filmstripSelectedIndex(frames, 0, 0, false, test.position); got != test.want {
				t.Fatalf("playback fallback selection at %.2fs = %d, want %d", test.position, got, test.want)
			}
		})
	}
}

func TestFilmstripSelectionPersistsAcrossNextWindow(t *testing.T) {
	frames := make([]domain.FilmstripFrame, 16)
	for i := range frames {
		frames[i].At = time.Duration(i) * time.Second
	}

	// The user hovers the last frame in the initial nine-frame window.
	previewPosition := frames[8].At.Seconds()
	initialWindow := filmstripFramesAtStart(frames, 0)
	if got := filmstripFrameIndexForPosition(initialWindow, previewPosition); got != 8 {
		t.Fatalf("initial selected index = %d, want 8", got)
	}

	// Moving to Next changes the visible window but must not replace the
	// selected preview position with the playback position.
	nextWindow := filmstripFramesAtStart(frames, filmstripShiftWindowStart(len(frames), 0, 1))
	if got := filmstripFrameIndexForPosition(nextWindow, previewPosition); got != 5 {
		t.Fatalf("selected index after Next = %d, want 5 (same timestamp)", got)
	}
	if nextWindow[5].At != frames[8].At {
		t.Fatalf("selection changed from %s to %s after browsing",
			frames[8].At, nextWindow[5].At)
	}
}

func TestFilmstripSelectionOutsideWindowHasNoHighlight(t *testing.T) {
	frames := make([]domain.FilmstripFrame, 18)
	for i := range frames {
		frames[i].At = time.Duration(i) * time.Second
	}
	window := filmstripFramesAtStart(frames, 9)
	if got := filmstripFrameIndexForPosition(window, frames[3].At.Seconds()); got != -1 {
		t.Fatalf("selection outside visible window = %d, want -1", got)
	}
}

func TestFilmstripFrameIndexAtXUsesActualTileGaps(t *testing.T) {
	const rowWidth = float32(600)
	const padding = float32(3)
	const gap = float32(3)
	const count = 9
	contentWidth := rowWidth - 2*padding
	tileWidth := (contentWidth - gap*float32(count-1)) / count
	pitch := tileWidth + gap

	for i := 0; i < count; i++ {
		center := padding + float32(i)*pitch + tileWidth/2
		if got := filmstripFrameIndexAtX(center, rowWidth, count); got != i {
			t.Errorf("tile %d center mapped to index %d", i, got)
		}
	}
	if got := filmstripFrameIndexAtX(rowWidth-padding, rowWidth, count); got != count-1 {
		t.Fatalf("rightmost tile edge mapped to index %d, want %d", got, count-1)
	}
	if got := filmstripFrameIndexAtX(rowWidth, rowWidth, count); got != count-1 {
		t.Fatalf("row's right boundary mapped to index %d, want %d", got, count-1)
	}
	if got := filmstripFrameIndexAtX(0, rowWidth, count); got != 0 {
		t.Fatalf("row's left boundary mapped to index %d, want 0", got)
	}
	if got := filmstripFrameIndexAtX(10, rowWidth, 0); got != -1 {
		t.Fatalf("empty strip mapped to index %d, want -1", got)
	}
}

func TestFilmstripShiftWindowStart(t *testing.T) {
	tests := []struct {
		name        string
		frameCount  int
		current     int
		steps       int
		want        int
	}{
		{name: "advance three frames", frameCount: 48, current: 10, steps: 1, want: 13},
		{name: "move back three frames", frameCount: 48, current: 10, steps: -1, want: 7},
		{name: "clamp at beginning", frameCount: 48, current: 1, steps: -1, want: 0},
		{name: "clamp at end", frameCount: 48, current: 39, steps: 1, want: 39},
		{name: "short filmstrip", frameCount: 5, current: 0, steps: 1, want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := filmstripShiftWindowStart(test.frameCount, test.current, test.steps); got != test.want {
				t.Fatalf("filmstripShiftWindowStart() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestFilmstripWheelSteps(t *testing.T) {
	tests := []struct {
		name           string
		delta           float32
		remainder       float32
		precise         bool
		wantSteps       int
		wantRemainder   float32
	}{
		{name: "mouse wheel down", delta: 99, wantSteps: 1},
		{name: "mouse wheel up", delta: -99, wantSteps: -1},
		{name: "touchpad accumulates", delta: -45, precise: true, wantRemainder: -45},
		{name: "touchpad completes step", delta: -54, remainder: -45, precise: true, wantSteps: -1},
		{name: "touchpad keeps remainder", delta: 120, precise: true, wantSteps: 1, wantRemainder: 21},
		{name: "zero delta", remainder: 12, precise: true, wantRemainder: 12},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotSteps, gotRemainder := filmstripWheelSteps(test.delta, test.remainder, test.precise)
			if gotSteps != test.wantSteps || gotRemainder != test.wantRemainder {
				t.Fatalf("filmstripWheelSteps() = (%d, %v), want (%d, %v)",
					gotSteps, gotRemainder, test.wantSteps, test.wantRemainder)
			}
		})
	}
}

func TestClampPlaybackPosition(t *testing.T) {
	tests := []struct {
		name     string
		position float64
		duration float64
		want     float64
	}{
		{name: "negative", position: -1, duration: 20, want: 0},
		{name: "past end", position: 30, duration: 20, want: 20},
		{name: "within range", position: 12.5, duration: 20, want: 12.5},
		{name: "nan", position: math.NaN(), duration: 20, want: 0},
		{name: "infinite", position: math.Inf(1), duration: 20, want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := clampPlaybackPosition(test.position, test.duration); got != test.want {
				t.Fatalf("clampPlaybackPosition() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestProgressiveFilmstripKeepsPendingSlotsInTheirTimelinePositions(t *testing.T) {
	frames := make([]domain.FilmstripFrame, 48)
	for i := range frames {
		frames[i].At = time.Duration(i*10) * time.Second
	}
	// Only the frame nearest the preview position has finished decoding.
	frames[24].Path = "frame_024.jpg"

	app := &App{
		filmstripManifest: domain.Filmstrip{
			Duration: 8 * time.Minute,
			Interval: 10 * time.Second,
			Frames:   frames,
		},
		filmstripPreviewPosition: 240,
	}
	visible := app.visibleFilmstripFrames()
	if len(visible) != filmstripVisibleFrames {
		t.Fatalf("visible progressive frame count = %d, want %d", len(visible), filmstripVisibleFrames)
	}
	if visible[filmstripVisibleFrames/2].Path != "frame_024.jpg" {
		t.Fatalf("center thumbnail path = %q, want generated frame", visible[filmstripVisibleFrames/2].Path)
	}
	if visible[0].Path != "" || visible[len(visible)-1].Path != "" {
		t.Fatal("pending thumbnails should remain as empty slots in the visible timeline")
	}
	if visible[filmstripVisibleFrames/2].At != 240*time.Second {
		t.Fatalf("generated thumbnail timestamp = %s, want 4m", visible[filmstripVisibleFrames/2].At)
	}
}
