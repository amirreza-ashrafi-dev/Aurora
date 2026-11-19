package application

import (
	"errors"
	"path/filepath"
	"os"
	"strings"
	"sort"
	"strings"

	"my-go/internal/domain"
)

// SubtitlePlayer is a focused capability port. Keeping it separate from
// Player avoids making subtitle operations mandatory for unrelated players.
type SubtitlePlayer interface {
	LoadExternalSubtitle(path string) error
	DisableSubtitles() error
	SetSubtitleDelay(seconds float64) error
	SetSubtitleStyle(style domain.SubtitleStyle) error
	ActiveSubtitleLanguage() (string, error)
}

// SubtitlePreferencesRepository persists global and per-language appearance.
type SubtitlePreferencesRepository interface {
	Load() (domain.SubtitlePreferences, error)
	SavePreferences(domain.SubtitlePreferences) error
}

// SubtitleAssignmentRepository persists external subtitle choices per video.
type SubtitleAssignmentRepository interface {
	LoadAll() ([]domain.SubtitleAssignment, error)
	SaveAssignment(domain.SubtitleAssignment) error
	Delete(mediaPath string) error
}

// NormalizeSubtitleLanguage canonicalizes common language tags for profile
// lookup. Unknown/empty metadata becomes "und" and uses global preferences.
func NormalizeSubtitleLanguage(language string) string {
	language = strings.TrimSpace(strings.ReplaceAll(language, "_", "-"))
	if language == "" {
		return "und"
	}
	parts := strings.Split(language, "-")
	for i, part := range parts {
		if part == "" {
			return "und"
		}
		if i == 0 {
			parts[i] = strings.ToLower(part)
		} else if len(part) == 2 || len(part) == 3 {
			parts[i] = strings.ToUpper(part)
		} else {
			parts[i] = strings.ToLower(part)
		}
	}
	return strings.Join(parts, "-")
}

// SubtitleStyleForLanguage returns a saved language style or the global
// fallback. It never mutates the supplied preferences.
func SubtitleStyleForLanguage(prefs domain.SubtitlePreferences, language string) domain.SubtitleStyle {
	key := NormalizeSubtitleLanguage(language)
	if key != "und" {
		// Prefer the most specific profile first, then progressively fall
		// back to the base language (for example fa-IR -> fa).
		candidates := []string{key}
		for candidate := key; strings.Contains(candidate, "-"); {
			candidate = candidate[:strings.LastIndex(candidate, "-")]
			candidates = append(candidates, candidate)
		}
		for _, candidate := range candidates {
			for _, profile := range prefs.Languages {
				if NormalizeSubtitleLanguage(profile.Language) == candidate {
					return profile.Style
				}
			}
		}
	}
	if prefs.Global.FontFamily == "" {
		return domain.DefaultSubtitleStyle()
	}
	return prefs.Global
}

// ValidateSubtitleStyle rejects values that could cause unusable or unsafe
// backend settings before they reach libmpv.
func ValidateSubtitleStyle(style domain.SubtitleStyle) error {
	if strings.TrimSpace(style.FontFamily) == "" {
		return errors.New("subtitle font family is empty")
	}
	if style.FontSize < 8 || style.FontSize > 120 {
		return errors.New("subtitle font size must be between 8 and 120")
	}
	if style.OutlineSize < 0 || style.OutlineSize > 10 {
		return errors.New("subtitle outline size must be between 0 and 10")
	}
	if style.VerticalPosition < 0 || style.VerticalPosition > 100 {
		return errors.New("subtitle vertical position must be between 0 and 100")
	}
	if !validSubtitleColor(style.TextColor) || !validSubtitleColor(style.OutlineColor) {
		return errors.New("subtitle colors must use #RRGGBB or #AARRGGBB")
	}
	return nil
}

func validSubtitleColor(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 7 && len(value) != 9 {
		return false
	}
	if value[0] != '#' {
		return false
	}
	for _, r := range value[1:] {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
			return false
		}
	}
	return true
}

// MatchSubtitleFiles returns subtitle files whose filename stem belongs to
// the video. It prefers exact basename matches and then language-suffixed
// forms such as Movie.en.srt. It deliberately avoids fuzzy matching.
func MatchSubtitleFiles(videoPath string, candidates []string) []string {
	videoName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	videoKey := strings.ToLower(videoName)
	matches := make([]string, 0)
	for _, candidate := range candidates {
		ext := strings.ToLower(filepath.Ext(candidate))
		if ext != ".srt" && ext != ".ass" && ext != ".ssa" && ext != ".vtt" && ext != ".sub" {
			continue
		}
		stem := strings.TrimSuffix(filepath.Base(candidate), filepath.Ext(candidate))
		key := strings.ToLower(stem)
		if key == videoKey || strings.HasPrefix(key, videoKey+".") || strings.HasPrefix(key, videoKey+"_") || strings.HasPrefix(key, videoKey+" ") {
			matches = append(matches, candidate)
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		ai := strings.TrimSuffix(filepath.Base(matches[i]), filepath.Ext(matches[i]))
		aj := strings.TrimSuffix(filepath.Base(matches[j]), filepath.Ext(matches[j]))
		exactI, exactJ := strings.EqualFold(ai, videoName), strings.EqualFold(aj, videoName)
		if exactI != exactJ {
			return exactI
		}
		return strings.ToLower(matches[i]) < strings.ToLower(matches[j])
	})
	return matches
}


// DiscoverSubtitleFiles scans only the video's own directory and accepts
// strict basename matches. It never performs fuzzy matching across folders.
func DiscoverSubtitleFiles(videoPath string) ([]string, error) {
	dir := filepath.Dir(videoPath)
	entries, err := os.ReadDir(dir)
	if err != nil { return nil, err }
	candidates := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() { continue }
		candidates = append(candidates, filepath.Join(dir, entry.Name()))
	}
	return MatchSubtitleFiles(videoPath, candidates), nil
}

// SubtitleLanguageFromFilename extracts a likely BCP-47 suffix from names
// such as Movie.en.srt or Movie_pt-BR.ass. It returns "und" when absent.
func SubtitleLanguageFromFilename(videoPath, subtitlePath string) string {
	video := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	sub := strings.TrimSuffix(filepath.Base(subtitlePath), filepath.Ext(subtitlePath))
	if len(sub) <= len(video) { return "und" }
	if !strings.EqualFold(sub[:len(video)], video) { return "und" }
	suffix := strings.TrimLeft(sub[len(video):], " ._-")
	if suffix == "" { return "und" }
	parts := strings.Split(suffix, ".")
	candidate := parts[0]
	if len(candidate) < 2 || len(candidate) > 8 { return "und" }
	for _, r := range candidate {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') { return "und" }
	}
	return NormalizeSubtitleLanguage(candidate)
}
