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
