package domain

import "time"

// Filmstrip describes a persistent set of small preview frames for a media
// file. Frame paths point to compressed images on disk; callers should decode
// only the frames currently visible in the UI.
type Filmstrip struct {
	Duration time.Duration
	Interval time.Duration
	Frames   []FilmstripFrame
}

// FilmstripFrame identifies one compressed preview image and its media time.
type FilmstripFrame struct {
	At        time.Duration
	Path      string
	SizeBytes int64
}
