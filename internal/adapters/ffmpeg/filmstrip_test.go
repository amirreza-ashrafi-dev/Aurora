package ffmpeg

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestSampleIntervalSecondsBoundsFilmstripDensity(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     int64
	}{
		{name: "short clip", duration: 3 * time.Second, want: 1},
		{name: "feature length", duration: 90 * time.Minute, want: 115},
		{name: "long recording", duration: 4 * time.Hour, want: 307},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := sampleIntervalSeconds(test.duration); got != test.want {
				t.Fatalf("sampleIntervalSeconds(%s) = %d, want %d", test.duration, got, test.want)
			}
		})
	}
}

func TestVideoFilterBoundsOutputDimensions(t *testing.T) {
	got := videoFilter()
	want := "scale=160:90:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=160:90:(ow-iw)/2:(oh-ih)/2,setsar=1"
	if got != want {
		t.Fatalf("videoFilter() = %q, want %q", got, want)
	}
}

func TestSampleFrameCountNeverExceedsTarget(t *testing.T) {
	tests := []struct {
		name string
		duration time.Duration
		interval int64
		want int
	}{
		{name: "short clip", duration: 3 * time.Second, interval: 1, want: 3},
		{name: "two hour movie", duration: 2 * time.Hour, interval: 153, want: 48},
		{name: "invalid interval", duration: time.Minute, interval: 0, want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := sampleFrameCount(test.duration, test.interval); got != test.want {
				t.Fatalf("sampleFrameCount() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestFFmpegFrameArgsUseInputSeekBeforeSource(t *testing.T) {
	args := ffmpegFrameArgs("movie.mkv", "frame.jpg", 90*time.Second)
	ss, input := -1, -1
	for i, arg := range args {
		if arg == "-ss" { ss = i }
		if arg == "-i" { input = i }
	}
	if ss < 0 || input < 0 || ss >= input {
		t.Fatalf("expected input seek before source input, args: %#v", args)
	}
	if got := args[ss+1]; got != "90.000" {
		t.Fatalf("seek timestamp = %q, want 90.000", got)
	}
}

func TestCacheKeyChangesWhenSourceFingerprintChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "movie.mkv")
	if err := os.WriteFile(path, []byte("movie"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	first := cacheKey(path, info, time.Hour)
	if got := cacheKey(path, info, time.Hour); got != first {
		t.Fatal("same source fingerprint produced a different cache key")
	}
	if got := cacheKey(path, info, 2*time.Hour); got == first {
		t.Fatal("duration change did not invalidate the cache key")
	}
}

func TestLoadCachedFilmstripValidatesEveryImage(t *testing.T) {
	dir := t.TempDir()
	imagePath := filepath.Join(dir, "frame_000.jpg")
	if err := os.WriteFile(imagePath, []byte("jpeg"), 0o600); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(t.TempDir(), "source.mkv")
	if err := os.WriteFile(sourcePath, []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	duration := 10 * time.Minute
	manifest := diskManifest{
		Version:             manifestVersion,
		SourceSize:          sourceInfo.Size(),
		SourceModTimeUnixNS: sourceInfo.ModTime().UnixNano(),
		DurationNS:          duration.Nanoseconds(),
		IntervalSeconds:     30,
		Width:               thumbnailWidth,
		Height:              thumbnailHeight,
		Frames: []diskFrame{{
			File:      filepath.Base(imagePath),
			AtNS:      0,
			SizeBytes: int64(len("jpeg")),
		}},
	}
	writeManifest(t, dir, manifest)

	strip, ok := loadCachedFilmstrip(dir, sourceInfo, duration)
	if !ok {
		t.Fatal("expected a valid cached filmstrip")
	}
	if len(strip.Frames) != 1 || strip.Frames[0].Path != imagePath {
		t.Fatalf("unexpected cached frames: %#v", strip.Frames)
	}

	if err := os.Remove(imagePath); err != nil {
		t.Fatal(err)
	}
	if _, ok := loadCachedFilmstrip(dir, sourceInfo, duration); ok {
		t.Fatal("cache remained valid after a referenced image was removed")
	}
}

func writeManifest(t *testing.T, dir string, manifest diskManifest) {
	t.Helper()
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifestName), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestPrioritizedSampleOrderStartsNearPreferredPosition(t *testing.T) {
	got := prioritizedSampleOrder(10, 10*time.Second, 43*time.Second)
	if len(got) != 10 {
		t.Fatalf("sample order length = %d, want 10", len(got))
	}
	if got[0] != 4 || got[1] != 5 {
		t.Fatalf("first samples = %v, want nearest indexes [4 5]", got[:2])
	}
}

func TestPrioritizedSampleOrderUsesActualTimestampDistance(t *testing.T) {
	tests := []struct {
		name string
		position time.Duration
		want []int
	}{
		{name: "playhead nearer next sample", position: 43 * time.Second, want: []int{4, 5, 3}},
		{name: "playhead nearer previous sample", position: 47 * time.Second, want: []int{5, 4, 6}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := prioritizedSampleOrder(10, 10*time.Second, test.position)
			for i, want := range test.want {
				if got[i] != want {
					t.Fatalf("sample order prefix = %v, want %v", got[:len(test.want)], test.want)
				}
			}
		})
	}
}

func TestRunBoundedJobsNeverExceedsTwoWorkers(t *testing.T) {
	jobs := []int{0, 1, 2, 3, 4, 5, 6, 7}
	var active atomic.Int32
	var peak atomic.Int32
	completed := make([]int, 0, len(jobs))

	err := runBoundedJobs(context.Background(), jobs, maxConcurrentFFmpeg,
		func(ctx context.Context, _ int) error {
			current := active.Add(1)
			for {
				old := peak.Load()
				if current <= old || peak.CompareAndSwap(old, current) {
					break
				}
			}
			select {
			case <-time.After(20 * time.Millisecond):
				active.Add(-1)
				return nil
			case <-ctx.Done():
				active.Add(-1)
				return ctx.Err()
			}
		},
		func(index, _ int) {
			completed = append(completed, index)
		},
	)
	if err != nil {
		t.Fatalf("runBoundedJobs() error = %v", err)
	}
	if peak.Load() != 2 {
		t.Fatalf("peak concurrent workers = %d, want 2", peak.Load())
	}
	if len(completed) != len(jobs) {
		t.Fatalf("completed jobs = %d, want %d", len(completed), len(jobs))
	}
}
