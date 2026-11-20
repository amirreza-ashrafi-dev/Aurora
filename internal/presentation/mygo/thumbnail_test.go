package presentation

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
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

func TestResizeThumbnailPNGDownscalesLargeImage(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 0; y < 1080; y++ {
		for x := 0; x < 1920; x++ {
			source.SetRGBA(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 90, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatal(err)
	}

	data, width, height, err := resizeThumbnailPNG(encoded.Bytes(), thumbnailMaxDimension)
	if err != nil {
		t.Fatal(err)
	}
	if width != 320 || height != 180 {
		t.Fatalf("resized dimensions = %dx%d, want 320x180", width, height)
	}
	if len(data) == 0 {
		t.Fatal("resized PNG is empty")
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("resized output is not a valid PNG: %v", err)
	}
}

func TestResizeThumbnailPNGLeavesSmallImageUnchanged(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 160, 90))
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatal(err)
	}
	original := append([]byte(nil), encoded.Bytes()...)

	data, width, height, err := resizeThumbnailPNG(original, thumbnailMaxDimension)
	if err != nil {
		t.Fatal(err)
	}
	if width != 160 || height != 90 {
		t.Fatalf("dimensions = %dx%d, want 160x90", width, height)
	}
	if !bytes.Equal(data, original) {
		t.Fatal("small thumbnail should not be re-encoded")
	}
}

func TestResizeThumbnailPNGRejectsInvalidInput(t *testing.T) {
	if _, _, _, err := resizeThumbnailPNG([]byte("not a png"), thumbnailMaxDimension); err == nil {
		t.Fatal("expected invalid PNG error")
	}
	if _, _, _, err := resizeThumbnailPNG([]byte{}, 0); err == nil {
		t.Fatal("expected invalid maximum dimension error")
	}
}
