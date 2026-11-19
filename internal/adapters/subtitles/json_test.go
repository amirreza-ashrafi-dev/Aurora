package subtitles

import (
	"path/filepath"
	"testing"

	"my-go/internal/domain"
)

func TestJSONRepositoryPersistsPreferencesAndAssignments(t *testing.T) {
	repo, err := NewJSONRepository(t.TempDir())
	if err != nil { t.Fatal(err) }

	prefs := domain.SubtitlePreferences{
		Global: domain.DefaultSubtitleStyle(),
		Languages: []domain.SubtitleLanguageProfile{{Language: "fa", Style: domain.SubtitleStyle{FontFamily: "Tahoma", FontSize: 42, TextColor: "#FFFFFFFF", OutlineColor: "#FF000000", OutlineSize: 2, VerticalPosition: 95}}},
	}
	if err := repo.SavePreferences(prefs); err != nil { t.Fatal(err) }
	loadedPrefs, err := repo.Load()
	if err != nil { t.Fatal(err) }
	if len(loadedPrefs.Languages) != 1 || loadedPrefs.Languages[0].Style.FontFamily != "Tahoma" {
		t.Fatalf("Load() preferences = %#v", loadedPrefs)
	}

	video := filepath.Join(t.TempDir(), "Movie.mkv")
	subtitle := filepath.Join(t.TempDir(), "Movie.fa.srt")
	if err := repo.SaveAssignment(domain.SubtitleAssignment{MediaPath: video, ExternalPath: subtitle, Language: "fa"}); err != nil { t.Fatal(err) }
	assignments, err := repo.LoadAll()
	if err != nil { t.Fatal(err) }
	if len(assignments) != 1 || assignments[0].ExternalPath != subtitle {
		t.Fatalf("LoadAll() = %#v", assignments)
	}
	if err := repo.Delete(video); err != nil { t.Fatal(err) }
	assignments, err = repo.LoadAll()
	if err != nil { t.Fatal(err) }
	if len(assignments) != 0 { t.Fatalf("LoadAll() after Delete = %#v", assignments) }
}
