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


func TestRemovePlaylistEntry(t *testing.T) {
	tests := []struct {
		name           string
		queue          []string
		currentIndex   int
		removeIndex    int
		wantQueue      []string
		wantCurrent    int
		wantCurrentOut bool
	}{
		{
			name: "remove item before current",
			queue: []string{"a", "b", "c"}, currentIndex: 2, removeIndex: 0,
			wantQueue: []string{"b", "c"}, wantCurrent: 1,
		},
		{
			name: "remove item after current",
			queue: []string{"a", "b", "c"}, currentIndex: 0, removeIndex: 2,
			wantQueue: []string{"a", "b"}, wantCurrent: 0,
		},
		{
			name: "remove current and select next",
			queue: []string{"a", "b", "c"}, currentIndex: 1, removeIndex: 1,
			wantQueue: []string{"a", "c"}, wantCurrent: 1, wantCurrentOut: true,
		},
		{
			name: "remove current final item and select previous",
			queue: []string{"a", "b", "c"}, currentIndex: 2, removeIndex: 2,
			wantQueue: []string{"a", "b"}, wantCurrent: 1, wantCurrentOut: true,
		},
		{
			name: "remove only item",
			queue: []string{"a"}, currentIndex: 0, removeIndex: 0,
			wantQueue: []string{}, wantCurrent: 0, wantCurrentOut: true,
		},
		{
			name: "invalid index leaves queue unchanged",
			queue: []string{"a", "b"}, currentIndex: 1, removeIndex: 3,
			wantQueue: []string{"a", "b"}, wantCurrent: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotQueue, gotCurrent, gotRemovedCurrent := RemovePlaylistEntry(
				tt.queue, tt.currentIndex, tt.removeIndex,
			)
			if len(gotQueue) != len(tt.wantQueue) {
				t.Fatalf("queue length = %d, want %d", len(gotQueue), len(tt.wantQueue))
			}
			for i := range gotQueue {
				if gotQueue[i] != tt.wantQueue[i] {
					t.Errorf("queue[%d] = %q, want %q", i, gotQueue[i], tt.wantQueue[i])
				}
			}
			if gotCurrent != tt.wantCurrent {
				t.Errorf("current index = %d, want %d", gotCurrent, tt.wantCurrent)
			}
			if gotRemovedCurrent != tt.wantCurrentOut {
				t.Errorf("removed current = %v, want %v", gotRemovedCurrent, tt.wantCurrentOut)
			}
		})
	}
}
