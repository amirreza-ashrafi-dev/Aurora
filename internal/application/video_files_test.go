package application

import (
	"reflect"
	"testing"
)

func TestFilterVideoLaunchPaths(t *testing.T) {
	input := []string{
		`C:\Movies\First Movie.MKV`,
		`C:\Movies\second.mp4`,
		`C:\Documents\notes.txt`,
		"",
		"--some-option",
		`C:\Movies\third.3GP`,
	}
	want := []string{
		`C:\Movies\First Movie.MKV`,
		`C:\Movies\second.mp4`,
		`C:\Movies\third.3GP`,
	}
	if got := FilterVideoLaunchPaths(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("FilterVideoLaunchPaths() = %#v, want %#v", got, want)
	}
}

func TestVideoFileDialogExtensionsMatchSupportedExtensions(t *testing.T) {
	extensions := SupportedVideoExtensions()
	dialogExtensions := VideoFileDialogExtensions()
	if len(extensions) != len(dialogExtensions) {
		t.Fatalf("extension counts differ: supported=%d dialog=%d", len(extensions), len(dialogExtensions))
	}
	for i, extension := range extensions {
		if extension != "."+dialogExtensions[i] {
			t.Errorf("dialog extension %d = %q, want %q", i, dialogExtensions[i], extension[1:])
		}
	}
	extensions[0] = ".changed"
	if SupportedVideoExtensions()[0] == ".changed" {
		t.Fatal("SupportedVideoExtensions returned the shared backing slice")
	}
}

func TestIsSupportedVideoFile(t *testing.T) {
	for _, path := range []string{"movie.mp4", "movie.MKV", "clip.webm"} {
		if !IsSupportedVideoFile(path) {
			t.Errorf("IsSupportedVideoFile(%q) = false, want true", path)
		}
	}
	for _, path := range []string{"movie.txt", "movie", "", "folder.mp4/"} {
		if IsSupportedVideoFile(path) {
			t.Errorf("IsSupportedVideoFile(%q) = true, want false", path)
		}
	}
}
