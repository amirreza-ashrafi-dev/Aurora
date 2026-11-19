package domain

import "time"

// PlaybackState is the presentation-independent playback state of one item.
type PlaybackState struct {
	Position float64
	Duration float64
	Volume   float64
	Paused   bool
}

// PlaybackProgress stores the last useful position for a local media file.
type PlaybackProgress struct {
	MediaPath   string    `json:"media_path"`
	Position    float64   `json:"position"`
	Duration    float64   `json:"duration"`
	LastUpdated time.Time `json:"last_updated"`
}

// Rect describes a rectangle in logical window coordinates (DIPs).
type Rect struct {
	X, Y, W, H float32
}

// Point is a screen-space point in physical pixels.
type Point struct {
	X, Y int32
}
