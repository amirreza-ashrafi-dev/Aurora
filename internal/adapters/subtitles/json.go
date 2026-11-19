package subtitles

import (
	"encoding/json"
	"errors"
	"fmt"
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
	preferencesVersion = 1
	assignmentsVersion = 1
	maxSubtitleFileSize = 1 << 20
)

type preferencesEnvelope struct {
	Version int `json:"version"`
	Data domain.SubtitlePreferences `json:"data"`
}
type assignmentsEnvelope struct {
	Version int `json:"version"`
	Records []domain.SubtitleAssignment `json:"records"`
}

// JSONRepository stores subtitle preferences and per-video assignments in
// separate versioned files. Each write uses a same-directory temp file and
// atomic replacement.
type JSONRepository struct {
	preferencesPath string
	assignmentsPath string
	mu sync.Mutex
}

func NewJSONRepository(directory string) (*JSONRepository, error) {
	if strings.TrimSpace(directory) == "" {
		return nil, errors.New("subtitle configuration directory is empty")
	}
	abs, err := filepath.Abs(directory)
	if err != nil {
		return nil, fmt.Errorf("resolve subtitle configuration directory: %w", err)
	}
	return &JSONRepository{
		preferencesPath: filepath.Join(abs, "subtitle_preferences.json"),
		assignmentsPath: filepath.Join(abs, "subtitle_assignments.json"),
	}, nil
}

func (r *JSONRepository) Load() (domain.SubtitlePreferences, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	data, err := os.ReadFile(r.preferencesPath)
	if errors.Is(err, os.ErrNotExist) {
		return domain.SubtitlePreferences{Version: preferencesVersion, Global: domain.DefaultSubtitleStyle()}, nil
	}
	if err != nil { return domain.SubtitlePreferences{}, fmt.Errorf("read subtitle preferences: %w", err) }
	if len(data) > maxSubtitleFileSize { return domain.SubtitlePreferences{}, errors.New("subtitle preferences exceed size limit") }
	var envelope preferencesEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil { return domain.SubtitlePreferences{}, fmt.Errorf("decode subtitle preferences: %w", err) }
	if envelope.Version != preferencesVersion { return domain.SubtitlePreferences{}, fmt.Errorf("unsupported subtitle preferences version %d", envelope.Version) }
	envelope.Data.Version = preferencesVersion
	return envelope.Data, nil
}

func (r *JSONRepository) SavePreferences(prefs domain.SubtitlePreferences) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	prefs.Version = preferencesVersion
	data, err := json.MarshalIndent(preferencesEnvelope{Version: preferencesVersion, Data: prefs}, "", "  ")
	if err != nil { return fmt.Errorf("encode subtitle preferences: %w", err) }
	return atomicWrite(r.preferencesPath, data)
}

func (r *JSONRepository) LoadAll() ([]domain.SubtitleAssignment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	data, err := os.ReadFile(r.assignmentsPath)
	if errors.Is(err, os.ErrNotExist) { return nil, nil }
	if err != nil { return nil, fmt.Errorf("read subtitle assignments: %w", err) }
	if len(data) > maxSubtitleFileSize { return nil, errors.New("subtitle assignments exceed size limit") }
	var envelope assignmentsEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil { return nil, fmt.Errorf("decode subtitle assignments: %w", err) }
	if envelope.Version != assignmentsVersion { return nil, fmt.Errorf("unsupported subtitle assignments version %d", envelope.Version) }
	return envelope.Records, nil
}

func (r *JSONRepository) SaveAssignment(record domain.SubtitleAssignment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	record.MediaPath = normalizePath(record.MediaPath)
	record.ExternalPath = normalizePath(record.ExternalPath)
	if record.MediaPath == "" { return errors.New("subtitle assignment media path is empty") }
	if record.UpdatedAt.IsZero() { record.UpdatedAt = time.Now().UTC() }
	data, err := os.ReadFile(r.assignmentsPath)
	var records []domain.SubtitleAssignment
	if err == nil {
		if len(data) > maxSubtitleFileSize { return errors.New("subtitle assignments exceed size limit") }
		var envelope assignmentsEnvelope
		if err := json.Unmarshal(data, &envelope); err != nil { return fmt.Errorf("decode subtitle assignments: %w", err) }
		if envelope.Version != assignmentsVersion { return fmt.Errorf("unsupported subtitle assignments version %d", envelope.Version) }
		records = envelope.Records
	} else if !errors.Is(err, os.ErrNotExist) { return fmt.Errorf("read subtitle assignments: %w", err) }
	key := pathKey(record.MediaPath)
	updated := make([]domain.SubtitleAssignment, 0, len(records)+1)
	for _, item := range records { if pathKey(item.MediaPath) != key { updated = append(updated, item) } }
	updated = append(updated, record)
	sort.Slice(updated, func(i,j int) bool { return updated[i].UpdatedAt.After(updated[j].UpdatedAt) })
	if len(updated) > 500 { updated = updated[:500] }
	encoded, err := json.MarshalIndent(assignmentsEnvelope{Version: assignmentsVersion, Records: updated}, "", "  ")
	if err != nil { return fmt.Errorf("encode subtitle assignments: %w", err) }
	return atomicWrite(r.assignmentsPath, encoded)
}

func (r *JSONRepository) Delete(mediaPath string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	data, err := os.ReadFile(r.assignmentsPath)
	if errors.Is(err, os.ErrNotExist) { return nil }
	if err != nil { return fmt.Errorf("read subtitle assignments: %w", err) }
	var envelope assignmentsEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil { return fmt.Errorf("decode subtitle assignments: %w", err) }
	if envelope.Version != assignmentsVersion { return fmt.Errorf("unsupported subtitle assignments version %d", envelope.Version) }
	key := pathKey(mediaPath)
	updated := make([]domain.SubtitleAssignment, 0, len(envelope.Records))
	for _, item := range envelope.Records { if pathKey(item.MediaPath) != key { updated = append(updated, item) } }
	encoded, err := json.MarshalIndent(assignmentsEnvelope{Version: assignmentsVersion, Records: updated}, "", "  ")
	if err != nil { return fmt.Errorf("encode subtitle assignments: %w", err) }
	return atomicWrite(r.assignmentsPath, encoded)
}

func atomicWrite(path string, data []byte) error {
	if len(data) > maxSubtitleFileSize { return errors.New("subtitle data exceeds size limit") }
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil { return fmt.Errorf("create subtitle configuration directory: %w", err) }
	tmp, err := os.CreateTemp(dir, ".subtitle-*.tmp")
	if err != nil { return fmt.Errorf("create subtitle temporary file: %w", err) }
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil { _ = tmp.Close(); return err }
	if _, err := tmp.Write(data); err != nil { _ = tmp.Close(); return err }
	if err := tmp.Sync(); err != nil { _ = tmp.Close(); return err }
	if err := tmp.Close(); err != nil { return err }
	if err := os.Rename(tmpPath, path); err != nil { return fmt.Errorf("replace subtitle data: %w", err) }
	return nil
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" { return "" }
	abs, err := filepath.Abs(path)
	if err != nil { return "" }
	return filepath.Clean(abs)
}
func pathKey(path string) string {
	key := filepath.Clean(normalizePath(path))
	if runtime.GOOS == "windows" { key = strings.ToLower(key) }
	return key
}
