package application

import (
	"testing"
	"time"

	"my-go/internal/domain"
)

func TestCanResumeProgress(t *testing.T) {
	tests := []struct {
		name   string
		record domain.PlaybackProgress
		want   bool
	}{
		{"valid", domain.PlaybackProgress{MediaPath: "movie.mkv", Position: 120, Duration: 600}, true},
		{"too close to beginning", domain.PlaybackProgress{MediaPath: "movie.mkv", Position: 9.9, Duration: 600}, false},
		{"completed", domain.PlaybackProgress{MediaPath: "movie.mkv", Position: 580, Duration: 600}, false},
		{"past duration", domain.PlaybackProgress{MediaPath: "movie.mkv", Position: 601, Duration: 600}, false},
		{"unknown duration", domain.PlaybackProgress{MediaPath: "movie.mkv", Position: 120}, true},
		{"missing path", domain.PlaybackProgress{Position: 120, Duration: 600}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CanResumeProgress(test.record); got != test.want {
				t.Fatalf("CanResumeProgress() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestFindProgressUsesMostRecentMatchingPath(t *testing.T) {
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	records := []domain.PlaybackProgress{
		{MediaPath: "movie.mkv", Position: 30, LastUpdated: older},
		{MediaPath: "movie.mkv", Position: 90, LastUpdated: newer},
	}
	got, ok := FindProgress(records, "movie.mkv")
	if !ok || got.Position != 90 {
		t.Fatalf("FindProgress() = (%+v, %v), want position 90", got, ok)
	}
}
