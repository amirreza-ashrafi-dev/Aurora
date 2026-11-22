package presentation

import "testing"

func TestNewAppStartsWithPlaylistExpanded(t *testing.T) {
	app := New(nil, nil, nil, nil, nil)
	if !app.playlistExpanded {
		t.Fatal("new app should start with the playlist expanded")
	}
}

func TestTogglePlaylistExpanded(t *testing.T) {
	app := &App{playlistExpanded: true}

	app.togglePlaylistExpanded()
	if app.playlistExpanded {
		t.Fatal("playlist should be collapsed after the first toggle")
	}

	app.togglePlaylistExpanded()
	if !app.playlistExpanded {
		t.Fatal("playlist should be expanded after the second toggle")
	}
}

func TestTogglePlaylistExpandedHandlesNilApp(t *testing.T) {
	var app *App
	app.togglePlaylistExpanded()
}
