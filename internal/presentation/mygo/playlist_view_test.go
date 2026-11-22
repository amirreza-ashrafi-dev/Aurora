package presentation

import "testing"

func TestNewAppStartsWithPlaylistExpanded(t *testing.T) {
	app := New(nil, nil, nil, nil, nil)
	if !app.playlistSidebarOpen {
		t.Fatal("new app should start with the playlist expanded")
	}
}

func TestTogglePlaylistExpanded(t *testing.T) {
	app := &App{playlistSidebarOpen: true}

	app.togglePlaylistSidebar()
	if app.playlistSidebarOpen {
		t.Fatal("playlist should be collapsed after the first toggle")
	}

	app.togglePlaylistSidebar()
	if !app.playlistSidebarOpen {
		t.Fatal("playlist should be expanded after the second toggle")
	}
}

func TestTogglePlaylistExpandedHandlesNilApp(t *testing.T) {
	var app *App
	app.togglePlaylistSidebar()
}
