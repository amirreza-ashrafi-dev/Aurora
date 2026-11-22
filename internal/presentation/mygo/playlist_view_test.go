package presentation

import "testing"

func TestNewAppStartsWithPlaylistSidebarOpen(t *testing.T) {
	app := New(nil, nil, nil, nil, nil)
	if !app.playlistSidebarOpen {
		t.Fatal("new app should start with the playlist sidebar open")
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
