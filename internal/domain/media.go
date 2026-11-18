package domain

import "path/filepath"

// Media identifies a local media item without depending on a playback backend.
type Media struct {
	Path string
}

// Title returns the filename shown in the player interface.
func (m Media) Title() string {
	if m.Path == "" {
		return "Nothing playing"
	}
	return filepath.Base(m.Path)
}
