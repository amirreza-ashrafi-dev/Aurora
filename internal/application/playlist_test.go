package application

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFilterNewMedia(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.mp4")
	newFile := filepath.Join(dir, "new.mp4")
	if err := os.WriteFile(existing, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newFile, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}

	result := FilterNewMedia([]string{existing}, []string{existing, newFile, dir, ""})
	if result.Duplicates != 1 {
		t.Errorf("Duplicates = %d, want 1", result.Duplicates)
	}
	if result.Invalid != 2 {
		t.Errorf("Invalid = %d, want 2", result.Invalid)
	}
	if len(result.Valid) != 1 || result.Valid[0] != newFile {
		t.Errorf("Valid = %#v, want [%q]", result.Valid, newFile)
	}
}

func TestFilterNewMediaDetectsDuplicateWithinSelection(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "same.mkv")
	if err := os.WriteFile(file, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := FilterNewMedia(nil, []string{file, file})
	if len(result.Valid) != 1 || result.Duplicates != 1 {
		t.Fatalf("unexpected result: valid=%v duplicates=%d", result.Valid, result.Duplicates)
	}
}
