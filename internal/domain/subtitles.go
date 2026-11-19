package domain

import "time"

// SubtitleStyle contains backend-neutral styling preferences for text subtitles.
// Color values use #RRGGBB or #AARRGGBB notation.
type SubtitleStyle struct {
	FontFamily    string  `json:"font_family"`
	FontSize      float64 `json:"font_size"`
	TextColor     string  `json:"text_color"`
	OutlineColor  string  `json:"outline_color"`
	OutlineSize   float64 `json:"outline_size"`
	VerticalPosition int `json:"vertical_position"`
}

// SubtitleLanguageProfile stores the appearance selected for one normalized
// language tag. Delay is intentionally not included: synchronization is per
// video/track, not a language preference.
type SubtitleLanguageProfile struct {
	Language string        `json:"language"`
	Style    SubtitleStyle `json:"style"`
}

// SubtitlePreferences stores global defaults and language-specific overrides.
type SubtitlePreferences struct {
	Version   int                         `json:"version"`
	Global    SubtitleStyle               `json:"global"`
	Languages []SubtitleLanguageProfile   `json:"languages,omitempty"`
}

// SubtitleTrack describes one subtitle track reported by the playback engine.
// Track IDs are scoped to the owning media item and must never be reused for
// a different playlist entry.
type SubtitleTrack struct {
	ID           int64
	Title        string
	Language     string
	Codec        string
	External     bool
	ExternalPath string
	Selected     bool
}

// SubtitleAssignment records the subtitle choice for one media file. An
// external path takes precedence; otherwise TrackID identifies an embedded
// track within this media item. Disabled explicitly remembers subtitles off.
type SubtitleAssignment struct {
	MediaPath       string    `json:"media_path"`
	ExternalPath    string    `json:"external_path,omitempty"`
	TrackID         int64     `json:"track_id,omitempty"`
	HasTrackID      bool      `json:"has_track_id,omitempty"`
	Language        string    `json:"language,omitempty"`
	Disabled        bool      `json:"disabled,omitempty"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// DefaultSubtitleStyle provides readable defaults at the mpv 720p reference
// scale. The adapter maps these values to mpv's sub-* properties.
func DefaultSubtitleStyle() SubtitleStyle {
	return SubtitleStyle{
		FontFamily: "Segoe UI",
		FontSize: 38,
		TextColor: "#FFFFFFFF",
		OutlineColor: "#CC000000",
		OutlineSize: 2,
		VerticalPosition: 95,
	}
}
