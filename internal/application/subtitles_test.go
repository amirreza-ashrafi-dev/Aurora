package application

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"my-go/internal/domain"
)

func TestNormalizeSubtitleLanguage(t *testing.T) {
	if got := NormalizeSubtitleLanguage("EN_us"); got != "en-US" {
		t.Fatalf("NormalizeSubtitleLanguage() = %q, want en-US", got)
	}
	if got := NormalizeSubtitleLanguage(""); got != "und" {
		t.Fatalf("empty language = %q, want und", got)
	}
}

func TestSubtitleStyleForLanguage(t *testing.T) {
	global := domain.DefaultSubtitleStyle()
	persian := global
	persian.FontFamily = "Tahoma"
	prefs := domain.SubtitlePreferences{
		Global: global,
		Languages: []domain.SubtitleLanguageProfile{{Language: "fa", Style: persian}},
	}
	if got := SubtitleStyleForLanguage(prefs, "fa-IR"); got.FontFamily != "Tahoma" {
		t.Fatalf("language profile font = %q, want Tahoma", got.FontFamily)
	}
	if got := SubtitleStyleForLanguage(prefs, "ja"); got.FontFamily != global.FontFamily {
		t.Fatalf("fallback font = %q, want %q", got.FontFamily, global.FontFamily)
	}
}

func TestMatchSubtitleFilesUsesStrictVideoStem(t *testing.T) {
	got := MatchSubtitleFiles("C:/Movies/Movie.mkv", []string{
		"C:/Movies/Movie.en.srt",
		"C:/Movies/Movie.fa.ass",
		"C:/Movies/Other.en.srt",
		"C:/Movies/Movie.mp4",
	})
	want := []string{"C:/Movies/Movie.en.srt", "C:/Movies/Movie.fa.ass"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MatchSubtitleFiles() = %#v, want %#v", got, want)
	}
}

func TestSubtitleLanguageFromFilename(t *testing.T) {
	if got := SubtitleLanguageFromFilename("Movie.mkv", "Movie.fa-IR.srt"); got != "fa-IR" { t.Fatalf("language = %q", got) }
	if got := SubtitleLanguageFromFilename("Movie.mkv", "Other.en.srt"); got != "und" { t.Fatalf("unrelated file language = %q", got) }
}


func TestIsSubtitleFile(t *testing.T) {
	for _, path := range []string{"movie.srt", "movie.ASS", "movie.ssa", "movie.vtt", "movie.sub", "movie.idx"} {
		if !IsSubtitleFile(path) {
			t.Errorf("IsSubtitleFile(%q) = false, want true", path)
		}
	}
	for _, path := range []string{"movie.mp4", "movie.txt", "movie"} {
		if IsSubtitleFile(path) {
			t.Errorf("IsSubtitleFile(%q) = true, want false", path)
		}
	}

	folderPath := filepath.Join(t.TempDir(), "folder.srt")
	if err := os.Mkdir(folderPath, 0o755); err != nil {
		t.Fatalf("create subtitle-named directory: %v", err)
	}
	if IsSubtitleFile(folderPath) {
		t.Errorf("IsSubtitleFile(%q) = true, want false for a directory", folderPath)
	}
}
