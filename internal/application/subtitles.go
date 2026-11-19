package application

import (
	"errors"
	"path/filepath"
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
	Save(domain.SubtitlePreferences) error
}

// SubtitleAssignmentRepository persists external subtitle choices per video.
type SubtitleAssignmentRepository interface {
	LoadAll() ([]domain.SubtitleAssignment, error)
	Save(domain.SubtitleAssignment) error
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
		for _, profile := range prefs.Languages {
			if NormalizeSubtitleLanguage(profile.Language) == key {
				return profile.Style
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
