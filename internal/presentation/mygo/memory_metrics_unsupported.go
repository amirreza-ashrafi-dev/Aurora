//go:build !windows

package presentation

import "errors"

type processMemorySnapshot struct {
	WorkingSetBytes     uint64
	PrivateBytes        uint64
	PeakWorkingSetBytes uint64
}

func readProcessMemorySnapshot() (processMemorySnapshot, error) {
	return processMemorySnapshot{}, errors.New("Windows process counters are only available on Windows")
}
