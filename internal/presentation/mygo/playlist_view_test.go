package presentation

import (
	"testing"

	mygo "github.com/egoist/mygo"
)

func TestNewAppStartsWithPlaylistSidebarClosed(t *testing.T) {
	app := New(nil, nil, nil, nil, nil)
	if app.playlistSidebarOpen {
		t.Fatal("new app should start with the playlist sidebar closed")
	}
}

func TestTogglePlaylistSidebar(t *testing.T) {
	app := &App{playlistSidebarOpen: true}

	app.togglePlaylistSidebar()
	if app.playlistSidebarOpen {
		t.Fatal("playlist sidebar should be closed after the first toggle")
	}

	app.togglePlaylistSidebar()
	if !app.playlistSidebarOpen {
		t.Fatal("playlist sidebar should be open after the second toggle")
	}
}

func TestTogglePlaylistSidebarHandlesNilApp(t *testing.T) {
	var app *App
	app.togglePlaylistSidebar()
}

func TestPlaylistSidebarUsesOverlayInCinematicMode(t *testing.T) {
	app := &App{cinematicMode: true}
	if !app.isPlaylistOverlayMode() {
		t.Fatal("cinematic mode should present the playlist as an overlay")
	}
}

func TestPlaylistSidebarAnchorStaysInsidePlayerBounds(t *testing.T) {
	const width, height = 1920, 1080
	anchor := playlistSidebarAnchor(width, height)

	if anchor.X != width-1 {
		t.Fatalf("anchor X = %d, want %d (one DIP inside the right edge)", anchor.X, width-1)
	}
	if anchor.X+anchor.Width > width {
		t.Fatalf("anchor extends beyond player width: %+v", anchor)
	}
	if anchor.Y < 0 || anchor.Y+anchor.Height > height {
		t.Fatalf("anchor is outside player height: %+v", anchor)
	}
}

func TestPlaylistSidebarAnchorRejectsEmptyBounds(t *testing.T) {
	if anchor := playlistSidebarAnchor(0, 1080); anchor != (mygo.Rectangle{}) {
		t.Fatalf("anchor for empty width = %+v, want zero rectangle", anchor)
	}
	if anchor := playlistSidebarAnchor(1920, 0); anchor != (mygo.Rectangle{}) {
		t.Fatalf("anchor for empty height = %+v, want zero rectangle", anchor)
	}
}
