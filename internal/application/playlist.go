package application

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// AddSummary reports how many candidate paths were accepted or rejected.
type AddSummary struct {
	Valid      []string
	Duplicates int
	Invalid    int
}

// FilterNewMedia validates candidate files and removes duplicates against both
// the existing queue and earlier candidates. It detects path aliases, symlinks,
// and hard links while leaving playback/UI decisions to the caller.
func FilterNewMedia(existing, candidates []string) AddSummary {
	seenPaths := make(map[string]struct{}, len(existing)+len(candidates))
	seenFiles := make([]os.FileInfo, 0, len(existing)+len(candidates))
	for _, queuedPath := range existing {
		if key := mediaPathKey(queuedPath); key != "" {
			seenPaths[key] = struct{}{}
		}
		if info, err := os.Stat(queuedPath); err == nil {
			seenFiles = append(seenFiles, info)
		}
	}

	result := AddSummary{Valid: make([]string, 0, len(candidates))}
	for _, path := range candidates {
		path = strings.TrimSpace(path)
		if path == "" {
			result.Invalid++
			continue
		}

		abs, err := filepath.Abs(path)
		if err != nil {
			result.Invalid++
			continue
		}
		info, err := os.Stat(abs)
		if err != nil || !info.Mode().IsRegular() {
			result.Invalid++
			continue
		}

		if resolved, resolveErr := filepath.EvalSymlinks(abs); resolveErr == nil {
			abs = resolved
		}
		key := mediaPathKey(abs)
		_, duplicate := seenPaths[key]
		if !duplicate {
			for _, queuedInfo := range seenFiles {
				if os.SameFile(info, queuedInfo) {
					duplicate = true
					break
				}
			}
		}
		if duplicate {
			result.Duplicates++
			continue
		}

		seenPaths[key] = struct{}{}
		seenFiles = append(seenFiles, info)
		result.Valid = append(result.Valid, abs)
	}
	return result
}

func mediaPathKey(path string) string {
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
