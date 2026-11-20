//go:build windows

package presentation

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

type processMemorySnapshot struct {
	WorkingSetBytes     uint64
	PrivateBytes        uint64
	PeakWorkingSetBytes uint64
}

type processMemoryCountersEx struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
	PrivateUsage               uintptr
}

var (
	memoryPSAPIDLL  = windows.NewLazySystemDLL("psapi.dll")
	memoryKernelDLL = windows.NewLazySystemDLL("kernel32.dll")
	memoryGetProcess = memoryKernelDLL.NewProc("GetCurrentProcess")
	memoryGetProcessMemoryInfo = memoryPSAPIDLL.NewProc("GetProcessMemoryInfo")
)

func readProcessMemorySnapshot() (processMemorySnapshot, error) {
	if err := memoryKernelDLL.Load(); err != nil {
		return processMemorySnapshot{}, fmt.Errorf("load kernel32: %w", err)
	}
	if err := memoryPSAPIDLL.Load(); err != nil {
		return processMemorySnapshot{}, fmt.Errorf("load psapi: %w", err)
	}

	handle, _, _ := memoryGetProcess.Call()
	if handle == 0 {
		return processMemorySnapshot{}, fmt.Errorf("GetCurrentProcess returned a null handle")
	}
	counters := processMemoryCountersEx{CB: uint32(unsafe.Sizeof(processMemoryCountersEx{}))}
	ok, _, callErr := memoryGetProcessMemoryInfo.Call(
		handle,
		uintptr(unsafe.Pointer(&counters)),
		uintptr(unsafe.Sizeof(counters)),
	)
	if ok == 0 {
		return processMemorySnapshot{}, fmt.Errorf("GetProcessMemoryInfo failed: %v", callErr)
	}
	return processMemorySnapshot{
		WorkingSetBytes:     uint64(counters.WorkingSetSize),
		PrivateBytes:        uint64(counters.PrivateUsage),
		PeakWorkingSetBytes: uint64(counters.PeakWorkingSetSize),
	}, nil
}
