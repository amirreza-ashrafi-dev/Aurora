package domain

// PlaybackState is the presentation-independent playback state of one item.
type PlaybackState struct {
	Position float64
	Duration float64
	Volume   float64
	Paused   bool
}

// Rect describes a rectangle in logical window coordinates (DIPs).
type Rect struct {
	X, Y, W, H float32
}

// Point is a screen-space point in physical pixels.
type Point struct {
	X, Y int32
}
