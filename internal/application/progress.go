package application

import (
	"math"
	"path/filepath"
	"runtime"
	"strings"

	"my-go/internal/domain"
)

// ProgressKey returns a stable key for a local media path. Windows paths are
// compared case-insensitively, matching Windows filesystem semantics.
func ProgressKey(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = filepath.Clean(path)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	key := filepath.Clean(abs)
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	return key
}

// FindProgress returns the most recently saved progress for path.
func FindProgress(records []domain.PlaybackProgress, path string) (domain.PlaybackProgress, bool) {
	key := ProgressKey(path)
	if key == "" {
		return domain.PlaybackProgress{}, false
	}
	var found domain.PlaybackProgress
	ok := false
	for _, record := range records {
		if ProgressKey(record.MediaPath) == key && (!ok || record.LastUpdated.After(found.LastUpdated)) {
			found, ok = record, true
		}
	}
	return found, ok
}

// CanResumeProgress rejects unusable positions and points very close to the
// end, so completed videos do not produce a pointless resume prompt.
func CanResumeProgress(record domain.PlaybackProgress) bool {
	if strings.TrimSpace(record.MediaPath) == "" ||
		math.IsNaN(record.Position) || math.IsInf(record.Position, 0) ||
		math.IsNaN(record.Duration) || math.IsInf(record.Duration, 0) ||
		record.Position < 10 || record.Duration < 0 {
		return false
	}
	if record.Duration > 0 {
		if record.Position > record.Duration || record.Duration-record.Position <= 30 {
			return false
		}
	}
	return true
}
