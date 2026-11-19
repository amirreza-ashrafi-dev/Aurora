package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"my-go/internal/domain"
)

func TestJSONRepositoryRoundTripAndDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Aurora", "playback_history.json")
	repo, err := NewJSONRepository(path)
	if err != nil {
		t.Fatal(err)
	}
	record := domain.PlaybackProgress{
		MediaPath: filepath.Join(t.TempDir(), "movie.mkv"),
		Position:  125.5,
		Duration:  900,
		LastUpdated: time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC),
	}
	if err := repo.Save(record); err != nil {
		t.Fatal(err)
	}
	records, err := repo.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Position != record.Position {
		t.Fatalf("Load() = %#v, want one record at %v", records, record.Position)
	}
	if err := repo.Delete(record.MediaPath); err != nil {
		t.Fatal(err)
	}
	records, err = repo.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Fatalf("Load() after Delete = %#v, want empty", records)
	}
}

func TestJSONRepositoryKeepsNewest500Records(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	repo, err := NewJSONRepository(path)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	records := make([]domain.PlaybackProgress, 0, maxRecords+2)
	for i := 0; i < maxRecords+2; i++ {
		records = append(records, domain.PlaybackProgress{
			MediaPath: filepath.Join(t.TempDir(), fmt.Sprintf("video-%03d.mkv", i)),
			Position:  float64(i + 10),
			LastUpdated: base.Add(time.Duration(i) * time.Minute),
		})
	}
	if err := repo.writeUnlocked(records); err != nil {
		t.Fatal(err)
	}
	loaded, err := repo.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != maxRecords {
		t.Fatalf("record count = %d, want %d", len(loaded), maxRecords)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > maxFileBytes {
		t.Fatalf("file size = %d, want <= %d", len(data), maxFileBytes)
	}
}

func TestJSONRepositoryRejectsUnsupportedVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	if err := os.WriteFile(path, []byte("{\"version\":99,\"records\":[]}"), 0o600); err != nil {
		t.Fatal(err)
	}
	repo, err := NewJSONRepository(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Load(); err == nil {
		t.Fatal("Load() error = nil, want unsupported version error")
	}
}

func TestJSONFileUsesVersionedEnvelope(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	repo, err := NewJSONRepository(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(domain.PlaybackProgress{MediaPath: filepath.Join(t.TempDir(), "x.mkv"), Position: 20}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded fileData
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Version != fileVersion {
		t.Fatalf("version = %d, want %d", decoded.Version, fileVersion)
	}
}
