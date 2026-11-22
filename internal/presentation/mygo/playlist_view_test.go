package presentation

import "testing"

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
