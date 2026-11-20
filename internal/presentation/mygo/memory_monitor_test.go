package presentation

import (
	"strings"
	"testing"
)

func TestFormatMemoryBytes(t *testing.T) {
	got := formatMemoryBytes(2 * 1024 * 1024)
	if got != "2.0 MiB" {
		t.Fatalf("formatMemoryBytes() = %q, want 2.0 MiB", got)
	}
}

func TestMemorySparkline(t *testing.T) {
	if got := memorySparkline(nil, func(memorySample) uint64 { return 0 }); got != "—" {
		t.Fatalf("empty sparkline = %q", got)
	}
	samples := []memorySample{{WorkingSetBytes: 10}, {WorkingSetBytes: 20}, {WorkingSetBytes: 30}}
	got := memorySparkline(samples, func(s memorySample) uint64 { return s.WorkingSetBytes })
	if len([]rune(got)) != len(samples) {
		t.Fatalf("sparkline length = %d, want %d", len([]rune(got)), len(samples))
	}
	if strings.TrimSpace(got) == "" {
		t.Fatal("sparkline is empty")
	}
}
