package history

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"my-go/internal/domain"
)

const (
	fileVersion  = 1
	maxRecords   = 500
	maxFileBytes = 1 << 20
)

type fileData struct {
	Version int                       `json:"version"`
	Records []domain.PlaybackProgress `json:"records"`
}

// JSONRepository stores playback progress in a small, versioned JSON file.
// Each mutation is written to a temporary file and atomically renamed.
type JSONRepository struct {
	path string
	mu   sync.Mutex
}

func NewJSONRepository(path string) (*JSONRepository, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("history file path is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve history file path: %w", err)
	}
	return &JSONRepository{path: abs}, nil
}

func (r *JSONRepository) Load() ([]domain.PlaybackProgress, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.loadUnlocked()
}

func (r *JSONRepository) Save(record domain.PlaybackProgress) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	record.MediaPath = normalizeMediaPath(record.MediaPath)
	if record.MediaPath == "" ||
		math.IsNaN(record.Position) || math.IsInf(record.Position, 0) ||
		math.IsNaN(record.Duration) || math.IsInf(record.Duration, 0) ||
		record.Position < 0 || record.Duration < 0 {
		return errors.New("invalid playback progress record")
	}
	if record.LastUpdated.IsZero() {
		record.LastUpdated = time.Now().UTC()
	} else {
		record.LastUpdated = record.LastUpdated.UTC()
	}

	records, err := r.loadUnlocked()
	if err != nil {
		return err
	}
	key := pathKey(record.MediaPath)
	updated := make([]domain.PlaybackProgress, 0, len(records)+1)
	for _, existing := range records {
		if pathKey(existing.MediaPath) != key {
			updated = append(updated, existing)
		}
	}
	updated = append(updated, record)
	return r.writeUnlocked(updated)
}

func (r *JSONRepository) Delete(mediaPath string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := pathKey(mediaPath)
	if key == "" {
		return nil
	}
	records, err := r.loadUnlocked()
	if err != nil {
		return err
	}
	updated := make([]domain.PlaybackProgress, 0, len(records))
	for _, record := range records {
		if pathKey(record.MediaPath) != key {
			updated = append(updated, record)
		}
	}
	return r.writeUnlocked(updated)
}

func (r *JSONRepository) loadUnlocked() ([]domain.PlaybackProgress, error) {
	data, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read playback history: %w", err)
	}
	if len(data) > maxFileBytes {
		return nil, fmt.Errorf("playback history exceeds the %d-byte safety limit", maxFileBytes)
	}
	var decoded fileData
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("decode playback history: %w", err)
	}
	if decoded.Version != fileVersion {
		return nil, fmt.Errorf("unsupported playback history version %d", decoded.Version)
	}

	// Discard invalid rows and collapse duplicate paths without touching media
	// files. Missing-file cleanup happens only when a user selects that item.
	byPath := make(map[string]domain.PlaybackProgress, len(decoded.Records))
	for _, record := range decoded.Records {
		record.MediaPath = normalizeMediaPath(record.MediaPath)
		if record.MediaPath == "" ||
			math.IsNaN(record.Position) || math.IsInf(record.Position, 0) ||
			math.IsNaN(record.Duration) || math.IsInf(record.Duration, 0) ||
			record.Position < 0 || record.Duration < 0 {
			continue
		}
		key := pathKey(record.MediaPath)
		previous, exists := byPath[key]
		if !exists || record.LastUpdated.After(previous.LastUpdated) {
			byPath[key] = record
		}
	}
	records := make([]domain.PlaybackProgress, 0, len(byPath))
	for _, record := range byPath {
		records = append(records, record)
	}
	sortRecordsNewestFirst(records)
	if len(records) > maxRecords {
		records = records[:maxRecords]
	}
	return records, nil
}

func (r *JSONRepository) writeUnlocked(records []domain.PlaybackProgress) error {
	sortRecordsNewestFirst(records)
	if len(records) > maxRecords {
		records = records[:maxRecords]
	}
	encode := func() ([]byte, error) {
		return json.MarshalIndent(fileData{Version: fileVersion, Records: records}, "", "  ")
	}
	data, err := encode()
	if err != nil {
		return fmt.Errorf("encode playback history: %w", err)
	}
	// Keep the newest entries and evict the least recently watched until both
	// the record and byte limits are satisfied.
	for len(data) > maxFileBytes && len(records) > 0 {
		records = records[:len(records)-1]
		data, err = encode()
		if err != nil {
			return fmt.Errorf("encode playback history: %w", err)
		}
	}
	if len(data) > maxFileBytes {
		return errors.New("a playback history record exceeds the file size limit")
	}

	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create playback history directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".playback-history-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary playback history: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set playback history permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write playback history: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("flush playback history: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close playback history: %w", err)
	}
	if err := os.Rename(tmpPath, r.path); err != nil {
		return fmt.Errorf("replace playback history: %w", err)
	}
	return nil
}

func normalizeMediaPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	return filepath.Clean(abs)
}

func pathKey(path string) string {
	key := filepath.Clean(normalizeMediaPath(path))
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	return key
}

func sortRecordsNewestFirst(records []domain.PlaybackProgress) {
	sort.Slice(records, func(i, j int) bool {
		return records[i].LastUpdated.After(records[j].LastUpdated)
	})
}
