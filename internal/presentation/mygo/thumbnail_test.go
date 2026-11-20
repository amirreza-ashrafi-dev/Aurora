package presentation

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestPlaylistContainsPath(t *testing.T) {
	app := &App{playlist: []string{`C:\Videos\Movie.mkv`}}

	if !app.playlistContainsPath(`c:\videos\movie.mkv`) {
		t.Fatal("playlistContainsPath() should compare Windows paths case-insensitively")
	}
	if app.playlistContainsPath(`C:\Videos\Other.mkv`) {
		t.Fatal("playlistContainsPath() unexpectedly found an unrelated path")
	}
}

func TestEvictQueueThumbnailClearsAllTrackedState(t *testing.T) {
	app := &App{
		queueThumbnails:       map[string]*ui.Bitmap{"movie": nil},
		thumbnailCaptured:     map[string]bool{"movie": true},
		thumbnailCaptureTokens: map[string]uint64{"movie": 42},
		thumbnailRasterBytes:  map[string]uint64{"movie": 64 * 1024 * 1024},
		thumbnailEncodedBytes: map[string]uint64{"movie": 1024},
	}

	app.evictQueueThumbnail("movie")

	if _, ok := app.queueThumbnails["movie"]; ok {
		t.Fatal("thumbnail bitmap reference was not removed")
	}
	if _, ok := app.thumbnailCaptured["movie"]; ok {
		t.Fatal("thumbnail captured marker was not cleared")
	}
	if _, ok := app.thumbnailCaptureTokens["movie"]; ok {
		t.Fatal("in-flight capture token was not invalidated")
	}
	if _, ok := app.thumbnailRasterBytes["movie"]; ok {
		t.Fatal("thumbnail raster estimate was not cleared")
	}
	if _, ok := app.thumbnailEncodedBytes["movie"]; ok {
		t.Fatal("thumbnail encoded estimate was not cleared")
	}
}
