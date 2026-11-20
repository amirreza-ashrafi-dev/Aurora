package presentation

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	mygo "github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const memorySampleLimit = 120

// memorySample combines process-wide measurements with explicitly labelled
// estimates for memory retained by Aurora-owned data structures.
type memorySample struct {
	CapturedAt time.Time

	WorkingSetBytes     uint64
	PrivateBytes        uint64
	PeakWorkingSetBytes uint64
	ProcessAvailable    bool
	ProcessError        string

	GoHeapAllocBytes uint64
	GoHeapInuseBytes uint64
	GoHeapSysBytes   uint64
	GoStackInuseBytes uint64
	GoStackSysBytes   uint64
	GoSysBytes        uint64
	GoTotalAllocBytes uint64
	GoGCCycles        uint32
	Goroutines        int

	ThumbnailCount          int
	ThumbnailRasterBytes    uint64
	ThumbnailEncodedBytes   uint64
	PlaylistCount           int
	PlaylistEstimateBytes   uint64
	SubtitleCandidateCount int
	SubtitleAssignmentCount int
	SubtitleEstimateBytes   uint64
	HistoryRecordCount      int
	HistoryEstimateBytes    uint64
}

func (a *App) toggleMemoryMonitor() {
	if a == nil || a.win == nil || a.win.IsDestroyed() {
		return
	}
	if a.memoryWindow != nil && !a.memoryWindow.IsDestroyed() {
		a.memoryWindow.Show()
		return
	}

	a.memorySamples = nil
	a.memoryExportPath = ""
	a.memoryWindow = mygo.NewWindow(mygo.WindowOptions{
		Title:      "Aurora Memory Diagnostics",
		Width:      920,
		Height:     760,
		MinWidth:   780,
		MinHeight:  620,
		Content:    ui.View(a.memoryMonitorView),
	})
	window := a.memoryWindow
	window.OnClosed(func() {
		if a.memoryWindow == window {
			a.stopMemoryMonitor()
			a.memoryWindow = nil
		}
	})
	a.captureMemorySample()
	a.startMemoryMonitor(window)
}

func (a *App) startMemoryMonitor(window *mygo.Window) {
	a.stopMemoryMonitor()
	stop := make(chan struct{})
	a.memoryMonitorStop = stop

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				window.Update(func() {
					if a.memoryWindow == nil || a.memoryWindow != window || window.IsDestroyed() {
						return
					}
					a.captureMemorySample()
					window.Invalidate()
				})
			}
		}
	}()
}

func (a *App) stopMemoryMonitor() {
	if a.memoryMonitorStop != nil {
		close(a.memoryMonitorStop)
		a.memoryMonitorStop = nil
	}
}

func (a *App) captureMemorySample() {
	if a == nil {
		return
	}

	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	process, processErr := readProcessMemorySnapshot()

	sample := memorySample{
		CapturedAt:          time.Now(),
		ProcessAvailable:    processErr == nil,
		GoHeapAllocBytes:    stats.HeapAlloc,
		GoHeapInuseBytes:    stats.HeapInuse,
		GoHeapSysBytes:      stats.HeapSys,
		GoStackInuseBytes:   stats.StackInuse,
		GoStackSysBytes:     stats.StackSys,
		GoSysBytes:          stats.Sys,
		GoTotalAllocBytes:   stats.TotalAlloc,
		GoGCCycles:          stats.NumGC,
		Goroutines:          runtime.NumGoroutine(),
		ThumbnailCount:      len(a.queueThumbnails),
		PlaylistCount:       len(a.playlist),
		SubtitleCandidateCount: len(a.subtitleCandidates),
		SubtitleAssignmentCount: len(a.subtitleAssignmentRecords),
		HistoryRecordCount:  len(a.historyRecords),
	}
	if processErr == nil {
		sample.WorkingSetBytes = process.WorkingSetBytes
		sample.PrivateBytes = process.PrivateBytes
		sample.PeakWorkingSetBytes = process.PeakWorkingSetBytes
	} else {
		sample.ProcessError = processErr.Error()
	}

	// Thumbnail raster size is estimated from the PNG dimensions as RGBA
	// (width * height * 4). It is not a claim about the graphics driver's
	// actual texture allocation or MyGo's internal pixel format.
	for _, size := range a.thumbnailRasterBytes {
		sample.ThumbnailRasterBytes += size
	}
	for _, size := range a.thumbnailEncodedBytes {
		sample.ThumbnailEncodedBytes += size
	}

	// These are deliberately approximate retained-data estimates. The Go
	// heap metrics above already include Go-managed portions, so do not add
	// these estimates to HeapAlloc when calculating totals.
	for _, path := range a.playlist {
		sample.PlaylistEstimateBytes += uint64(len(path) + 16)
	}
	for _, path := range a.subtitleCandidates {
		sample.SubtitleEstimateBytes += uint64(len(path) + 16)
	}
	for key, record := range a.subtitleAssignmentRecords {
		sample.SubtitleEstimateBytes += uint64(len(key) + len(record.MediaPath) + len(record.ExternalPath) + len(record.Language) + 64)
	}
	for _, profile := range a.subtitlePrefs.Languages {
		sample.SubtitleEstimateBytes += uint64(len(profile.Language) + len(profile.Style.FontFamily) + len(profile.Style.TextColor) + len(profile.Style.OutlineColor) + 48)
	}
	for key, record := range a.historyRecords {
		sample.HistoryEstimateBytes += uint64(len(key) + len(record.MediaPath) + 64)
	}

	if len(a.memorySamples) >= memorySampleLimit {
		copy(a.memorySamples, a.memorySamples[1:])
		a.memorySamples = a.memorySamples[:memorySampleLimit-1]
	}
	a.memorySamples = append(a.memorySamples, sample)
}

func (a *App) memoryMonitorView(c *ui.Context) {
	applyAuroraTheme(c)
	latest, hasSample := memorySample{}, len(a.memorySamples) > 0
	if hasSample {
		latest = a.memorySamples[len(a.memorySamples)-1]
	}

	ui.Column(c).Fill().Padding(18).Gap(12).Background(ui.Hex(auroraBackground)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Column(c).Gap(2).Children(func() {
				ui.Text(c, "MEMORY DIAGNOSTICS").FontSize(12).Bold().LetterSpacing(1.4).TextColor(ui.Hex(auroraAccent))
				ui.Text(c, "Live process and runtime measurements · 1-second sampling").FontSize(9).TextColor(ui.Hex(auroraMuted))
			})
			ui.Spacer(c)
			if ui.Button(c, "Refresh").Clicked() {
				a.captureMemorySample()
				if a.memoryWindow != nil && !a.memoryWindow.IsDestroyed() { a.memoryWindow.Invalidate() }
			}
			if ui.Button(c, "Export CSV").Clicked() {
				a.exportMemorySamples()
				if a.memoryWindow != nil && !a.memoryWindow.IsDestroyed() { a.memoryWindow.Invalidate() }
			}
		})

		ui.Row(c).FillWidth().Gap(10).Children(func() {
			workingSet := "Unavailable"
			privateBytes := "Unavailable"
			if latest.ProcessAvailable {
				workingSet = formatMemoryBytes(latest.WorkingSetBytes)
				privateBytes = formatMemoryBytes(latest.PrivateBytes)
			}
			memoryMetricCard(c, "PROCESS WORKING SET", workingSet, "Resident physical memory")
			memoryMetricCard(c, "PROCESS PRIVATE BYTES", privateBytes, "Private committed memory")
			memoryMetricCard(c, "GO HEAP ALLOCATED", formatMemoryBytes(latest.GoHeapAllocBytes), "Live Go heap allocation")
		})

		memoryPanel(c, "PROCESS MEMORY TREND · LAST 120 SECONDS", func() {
			if !hasSample {
				ui.Text(c, "Waiting for the first sample…").FontSize(10).TextColor(ui.Hex(auroraMuted))
				return
			}
			if latest.ProcessAvailable {
				ui.Text(c, "Working set  "+memorySparkline(a.memorySamples, func(s memorySample) uint64 { return s.WorkingSetBytes })).
					FontSize(15).TextColor(ui.Hex(auroraAccent))
				ui.Text(c, "Private bytes "+memorySparkline(a.memorySamples, func(s memorySample) uint64 { return s.PrivateBytes })).
					FontSize(15).TextColor(ui.Hex(auroraText))
				ui.Text(c, "Peak working set: "+formatMemoryBytes(latest.PeakWorkingSetBytes)).
					FontSize(9).TextColor(ui.Hex(auroraMuted))
			} else {
				ui.Text(c, "Windows process counters unavailable: "+latest.ProcessError).FontSize(9).TextColor(ui.Hex(auroraMuted))
			}
			ui.Text(c, fmt.Sprintf("%d samples · latest at %s", len(a.memorySamples), latest.CapturedAt.Format("15:04:05"))).
				FontSize(8).TextColor(ui.Hex(auroraMuted))
		})

		memoryPanel(c, "GO RUNTIME · EXACT RUNTIME COUNTERS", func() {
			memoryValueRow(c, "Heap allocated", formatMemoryBytes(latest.GoHeapAllocBytes))
			memoryValueRow(c, "Heap in use", formatMemoryBytes(latest.GoHeapInuseBytes))
			memoryValueRow(c, "Heap reserved by runtime", formatMemoryBytes(latest.GoHeapSysBytes))
			memoryValueRow(c, "Stack in use / reserved", formatMemoryBytes(latest.GoStackInuseBytes)+" / "+formatMemoryBytes(latest.GoStackSysBytes))
			memoryValueRow(c, "Runtime memory obtained from OS (Sys)", formatMemoryBytes(latest.GoSysBytes))
			memoryValueRow(c, "Total bytes allocated since start", formatMemoryBytes(latest.GoTotalAllocBytes))
			memoryValueRow(c, "Garbage collections / goroutines", fmt.Sprintf("%d / %d", latest.GoGCCycles, latest.Goroutines))
		})

		memoryPanel(c, "AURORA-OWNED DATA · ESTIMATES, NOT ADDITIVE", func() {
			memoryValueRow(c, "Cached thumbnails", fmt.Sprintf("%d images · %s estimated RGBA raster", latest.ThumbnailCount, formatMemoryBytes(latest.ThumbnailRasterBytes)))
			memoryValueRow(c, "Thumbnail PNG files read", formatMemoryBytes(latest.ThumbnailEncodedBytes))
			memoryValueRow(c, "Playlist", fmt.Sprintf("%d items · ~%s string/header estimate", latest.PlaylistCount, formatMemoryBytes(latest.PlaylistEstimateBytes)))
			memoryValueRow(c, "Subtitle metadata", fmt.Sprintf("%d candidates, %d assignments · ~%s", latest.SubtitleCandidateCount, latest.SubtitleAssignmentCount, formatMemoryBytes(latest.SubtitleEstimateBytes)))
			memoryValueRow(c, "Playback history", fmt.Sprintf("%d records · ~%s", latest.HistoryRecordCount, formatMemoryBytes(latest.HistoryEstimateBytes)))
		})

		memoryPanel(c, "INTERPRETATION", func() {
			ui.Text(c, "Working set and private bytes cover the whole Aurora process: Go, MyGo, libmpv, native libraries and graphics allocations.").FontSize(9).TextColor(ui.Hex(auroraText))
			ui.Text(c, "libmpv does not expose a reliable per-component RAM total through the properties currently used by Aurora. MPV and GPU memory therefore cannot be separated from the process totals in this first version.").FontSize(9).TextColor(ui.Hex(auroraMuted))
			ui.Text(c, "Thumbnail raster and metadata figures are estimates for diagnosis. They overlap with Go heap memory where Go owns the backing data; do not sum these rows with Heap Allocated. GPU texture allocations may not be included in process working-set figures.").FontSize(9).TextColor(ui.Hex(auroraMuted))
			if latest.ProcessError != "" {
				ui.Text(c, "Process metric error: "+latest.ProcessError).FontSize(8).TextColor(ui.Hex(auroraMuted))
			}
			if a.memoryExportPath != "" {
				ui.Text(c, "CSV saved to: "+a.memoryExportPath).FontSize(8).TextColor(ui.Hex(auroraAccent))
			}
		})
	})
}

func memoryMetricCard(c *ui.Context, label, value, detail string) {
	ui.Column(c).Grow(1).Padding(12).Gap(5).
		Background(ui.Hex(auroraPanel)).Border(1, ui.Hex(auroraBorderSoft)).Radius(10).
		Children(func() {
			ui.Text(c, label).FontSize(8).Bold().LetterSpacing(0.8).TextColor(ui.Hex(auroraMuted))
			ui.Text(c, value).FontSize(20).Bold().TextColor(ui.Hex(auroraText))
			ui.Text(c, detail).FontSize(8).TextColor(ui.Hex(auroraMuted))
		})
}

func memoryPanel(c *ui.Context, title string, children func()) {
	ui.Column(c).FillWidth().Padding(12).Gap(8).
		Background(ui.Hex(auroraPanel)).Border(1, ui.Hex(auroraBorderSoft)).Radius(10).
		Children(func() {
			ui.Text(c, title).FontSize(9).Bold().LetterSpacing(0.8).TextColor(ui.Hex(auroraAccent))
			children()
		})
}

func memoryValueRow(c *ui.Context, label, value string) {
	ui.Row(c).FillWidth().Gap(12).Children(func() {
		ui.Text(c, label).Grow(1).FontSize(9).TextColor(ui.Hex(auroraMuted))
		ui.Text(c, value).FontSize(9).Bold().TextColor(ui.Hex(auroraText))
	})
}

func formatMemoryBytes(value uint64) string {
	return fmt.Sprintf("%.1f MiB", float64(value)/(1024*1024))
}

func memorySparkline(samples []memorySample, value func(memorySample) uint64) string {
	if len(samples) == 0 {
		return "—"
	}
	const bars = "▁▂▃▄▅▆▇█"
	values := make([]uint64, len(samples))
	low, high := ^uint64(0), uint64(0)
	for i, sample := range samples {
		values[i] = value(sample)
		if values[i] < low { low = values[i] }
		if values[i] > high { high = values[i] }
	}
	if high == low {
		return strings.Repeat("▄", len(values))
	}
	runes := []rune(bars)
	var result strings.Builder
	for _, v := range values {
		index := int((v-low)*uint64(len(runes)-1)/(high-low))
		result.WriteRune(runes[index])
	}
	return result.String()
}

func (a *App) exportMemorySamples() {
	if a == nil || len(a.memorySamples) == 0 {
		return
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		a.memoryExportPath = "Export failed: " + err.Error()
		return
	}
	dir := filepath.Join(cacheDir, "Aurora Player", "diagnostics")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		a.memoryExportPath = "Export failed: " + err.Error()
		return
	}
	path := filepath.Join(dir, "memory-monitor-"+time.Now().Format("20060102-150405")+".csv")
	file, err := os.Create(path)
	if err != nil {
		a.memoryExportPath = "Export failed: " + err.Error()
		return
	}
	writer := csv.NewWriter(file)
	header := []string{
		"timestamp", "process_working_set_bytes", "process_private_bytes", "peak_working_set_bytes",
		"go_heap_alloc_bytes", "go_heap_inuse_bytes", "go_heap_sys_bytes", "go_stack_inuse_bytes",
		"go_stack_sys_bytes", "go_sys_bytes", "go_total_alloc_bytes", "gc_cycles", "goroutines",
		"thumbnail_count", "thumbnail_rgba_estimate_bytes", "thumbnail_png_bytes",
		"playlist_count", "playlist_estimate_bytes", "subtitle_candidate_count", "subtitle_assignment_count",
		"subtitle_estimate_bytes", "history_record_count", "history_estimate_bytes",
	}
	writeErr := writer.Write(header)
	for _, s := range a.memorySamples {
		if writeErr != nil { break }
		row := []string{
			s.CapturedAt.Format(time.RFC3339Nano), strconv.FormatUint(s.WorkingSetBytes, 10),
			strconv.FormatUint(s.PrivateBytes, 10), strconv.FormatUint(s.PeakWorkingSetBytes, 10),
			strconv.FormatUint(s.GoHeapAllocBytes, 10), strconv.FormatUint(s.GoHeapInuseBytes, 10),
			strconv.FormatUint(s.GoHeapSysBytes, 10), strconv.FormatUint(s.GoStackInuseBytes, 10),
			strconv.FormatUint(s.GoStackSysBytes, 10), strconv.FormatUint(s.GoSysBytes, 10),
			strconv.FormatUint(s.GoTotalAllocBytes, 10), strconv.FormatUint(uint64(s.GoGCCycles), 10),
			strconv.Itoa(s.Goroutines), strconv.Itoa(s.ThumbnailCount), strconv.FormatUint(s.ThumbnailRasterBytes, 10),
			strconv.FormatUint(s.ThumbnailEncodedBytes, 10), strconv.Itoa(s.PlaylistCount),
			strconv.FormatUint(s.PlaylistEstimateBytes, 10), strconv.Itoa(s.SubtitleCandidateCount),
			strconv.Itoa(s.SubtitleAssignmentCount), strconv.FormatUint(s.SubtitleEstimateBytes, 10),
			strconv.Itoa(s.HistoryRecordCount), strconv.FormatUint(s.HistoryEstimateBytes, 10),
		}
		writeErr = writer.Write(row)
	}
	writer.Flush()
	if writeErr == nil { writeErr = writer.Error() }
	closeErr := file.Close()
	if writeErr != nil {
		_ = os.Remove(path)
		a.memoryExportPath = "Export failed: " + writeErr.Error()
		return
	}
	if closeErr != nil {
		a.memoryExportPath = "Export may be incomplete: " + closeErr.Error()
		return
	}
	a.memoryExportPath = path
}
