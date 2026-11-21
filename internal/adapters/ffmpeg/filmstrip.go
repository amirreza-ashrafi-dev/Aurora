package ffmpeg

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"my-go/internal/domain"
)

const (
	manifestVersion     = 1
	thumbnailWidth      = 160
	thumbnailHeight     = 90
	targetFrames        = 48
	maxConcurrentFFmpeg = 2
	maxDiskCache        = int64(512 << 20)
	maxImageBytes       = int64(256 << 10)
	maxManifestBytes    = int64(64 << 10)
)

const manifestName = "manifest.json"

type FilmstripGenerator struct {
	ffmpegPath string
	cacheRoot  string

	// Serialize cache publication so two requests cannot write the same
	// fingerprint directory at the same time.
	generation chan struct{}
	// A single filmstrip may use at most two FFmpeg child processes.
	// Each child is independently limited to one decoder and filter thread.
	workers chan struct{}
}

type diskManifest struct {
	Version             int
	SourceSize          int64
	SourceModTimeUnixNS int64
	DurationNS          int64
	IntervalSeconds     int64
	Width               int
	Height              int
	Frames              []diskFrame
}

type diskFrame struct {
	File      string
	AtNS      int64
	SizeBytes int64
}

// NewFilmstripGenerator locates FFmpeg in the packaged resources directory,
// beside the executable, in the development third_party folder, or on PATH.
// A missing executable is reported when the user opens the filmstrip; it does
// not prevent Aurora from starting or playing videos.
func NewFilmstripGenerator(resourcesDir string) *FilmstripGenerator {
	cacheRoot := ""
	if base, err := os.UserCacheDir(); err == nil {
		cacheRoot = filepath.Join(base, "Aurora Player", "filmstrip")
	}
	return NewFilmstripGeneratorAt(resolveFFmpeg(resourcesDir), cacheRoot)
}

// NewFilmstripGeneratorAt creates a generator with explicit paths. It is
// exported to make cache behaviour straightforward to test.
func NewFilmstripGeneratorAt(ffmpegPath, cacheRoot string) *FilmstripGenerator {
	return &FilmstripGenerator{
		ffmpegPath: ffmpegPath,
		cacheRoot:  cacheRoot,
		generation: make(chan struct{}, 1),
		workers:    make(chan struct{}, maxConcurrentFFmpeg),
	}
}

// Ensure returns a complete disk-cached filmstrip, generating it only when
// the source fingerprint or thumbnail format changes.
func (g *FilmstripGenerator) Ensure(ctx context.Context, mediaPath string, duration time.Duration) (domain.Filmstrip, error) {
	return g.EnsureProgressive(ctx, mediaPath, duration, 0, nil)
}

// EnsureWithProgress preserves the count-only progress API for callers that do
// not need partial frame paths.
func (g *FilmstripGenerator) EnsureWithProgress(
	ctx context.Context,
	mediaPath string,
	duration time.Duration,
	progress func(completed, total int),
) (domain.Filmstrip, error) {
	var update func(domain.Filmstrip, int, int)
	if progress != nil {
		update = func(_ domain.Filmstrip, completed, total int) {
			progress(completed, total)
		}
	}
	return g.EnsureProgressive(ctx, mediaPath, duration, 0, update)
}

// EnsureProgressive generates thumbnails around the preferred position first,
// then fills the rest of the filmstrip. Completed JPEGs are written to stable
// paths and reported immediately; the cache is considered valid only after
// the complete manifest is atomically published.
func (g *FilmstripGenerator) EnsureProgressive(
	ctx context.Context,
	mediaPath string,
	duration time.Duration,
	preferredPosition time.Duration,
	update func(partial domain.Filmstrip, completed, total int),
) (domain.Filmstrip, error) {
	if g == nil {
		return domain.Filmstrip{}, fmt.Errorf("FFmpeg filmstrip generator is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(mediaPath) == "" {
		return domain.Filmstrip{}, fmt.Errorf("filmstrip source path is empty")
	}
	if duration <= 0 {
		return domain.Filmstrip{}, fmt.Errorf("video duration is not available yet")
	}
	if g.ffmpegPath == "" {
		return domain.Filmstrip{}, fmt.Errorf("FFmpeg was not found; install ffmpeg.exe in third_party/ffmpeg or add it to PATH")
	}
	if g.cacheRoot == "" {
		return domain.Filmstrip{}, fmt.Errorf("could not resolve the Aurora user cache directory")
	}

	absolutePath, err := filepath.Abs(mediaPath)
	if err != nil {
		return domain.Filmstrip{}, fmt.Errorf("resolve video path: %w", err)
	}
	info, err := os.Stat(absolutePath)
	if err != nil {
		return domain.Filmstrip{}, fmt.Errorf("inspect video file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return domain.Filmstrip{}, fmt.Errorf("filmstrip source is not a regular file")
	}

	key := cacheKey(absolutePath, info, duration)
	targetDir := filepath.Join(g.cacheRoot, key)
	if strip, ok := loadCachedFilmstrip(targetDir, info, duration); ok {
		touchDirectory(targetDir)
		if update != nil {
			update(strip, len(strip.Frames), len(strip.Frames))
		}
		return strip, nil
	}

	select {
	case g.generation <- struct{}{}:
		defer func() { <-g.generation }()
	case <-ctx.Done():
		return domain.Filmstrip{}, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return domain.Filmstrip{}, err
	}

	// Another request may have finished while this call waited for the
	// generator lock.
	if strip, ok := loadCachedFilmstrip(targetDir, info, duration); ok {
		touchDirectory(targetDir)
		if update != nil {
			update(strip, len(strip.Frames), len(strip.Frames))
		}
		return strip, nil
	}

	if err := os.MkdirAll(g.cacheRoot, 0o700); err != nil {
		return domain.Filmstrip{}, fmt.Errorf("create filmstrip cache: %w", err)
	}
	// An invalid or abandoned cache directory has no published manifest.
	// Remove it before reusing the stable frame paths for this generation.
	if err := os.RemoveAll(targetDir); err != nil {
		return domain.Filmstrip{}, fmt.Errorf("remove incomplete filmstrip cache: %w", err)
	}
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return domain.Filmstrip{}, fmt.Errorf("create filmstrip cache directory: %w", err)
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(targetDir)
		}
	}()

	stepSeconds := sampleIntervalSeconds(duration)
	interval := time.Duration(stepSeconds) * time.Second
	frameCount := sampleFrameCount(duration, stepSeconds)
	if frameCount == 0 {
		return domain.Filmstrip{}, fmt.Errorf("video duration is too short to sample thumbnails")
	}

	manifest := diskManifest{
		Version:             manifestVersion,
		SourceSize:          info.Size(),
		SourceModTimeUnixNS: info.ModTime().UnixNano(),
		DurationNS:          duration.Nanoseconds(),
		IntervalSeconds:     stepSeconds,
		Width:               thumbnailWidth,
		Height:              thumbnailHeight,
		Frames:              make([]diskFrame, frameCount),
	}
	partialFrames := make([]domain.FilmstripFrame, frameCount)
	for index := range partialFrames {
		at := time.Duration(int64(index)*stepSeconds) * time.Second
		partialFrames[index] = domain.FilmstripFrame{At: at}
	}
	if update != nil {
		update(domain.Filmstrip{
			Duration: duration,
			Interval: interval,
			Frames:   append([]domain.FilmstripFrame(nil), partialFrames...),
		}, 0, frameCount)
	}

	// Generate the frames nearest the current playhead first, so the part of
	// the filmstrip the user is most likely to inspect appears first.
	preferredPosition = clampDuration(preferredPosition, duration)
	jobs := prioritizedSampleOrder(frameCount, interval, preferredPosition)

	var partialMu sync.RWMutex
	err = runBoundedJobs(ctx, jobs, maxConcurrentFFmpeg, func(workerCtx context.Context, index int) error {
		at := time.Duration(int64(index)*stepSeconds) * time.Second
		outputPath := filepath.Join(targetDir, fmt.Sprintf("frame_%03d.jpg", index))

		select {
		case g.workers <- struct{}{}:
		case <-workerCtx.Done():
			return workerCtx.Err()
		}
		defer func() { <-g.workers }()

		args := ffmpegFrameArgs(absolutePath, outputPath, at)
		cmd := exec.CommandContext(workerCtx, g.ffmpegPath, args...)
		configureFFmpegCommand(cmd)
		cmd.Stdout = io.Discard
		stderr := &limitedBuffer{limit: 16 << 10}
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			if workerCtx.Err() != nil {
				return workerCtx.Err()
			}
			detail := strings.TrimSpace(stderr.String())
			if detail == "" {
				detail = err.Error()
			}
			return fmt.Errorf("FFmpeg thumbnail %d/%d at %s failed: %s",
				index+1, frameCount, formatSeekTimestamp(at), detail)
		}

		frameInfo, err := os.Stat(outputPath)
		if err != nil {
			return fmt.Errorf("FFmpeg did not create thumbnail %d/%d: %w", index+1, frameCount, err)
		}
		if !frameInfo.Mode().IsRegular() || frameInfo.Size() <= 0 || frameInfo.Size() > maxImageBytes {
			return fmt.Errorf("thumbnail %d/%d has an unexpected size: %d bytes",
				index+1, frameCount, frameInfo.Size())
		}

		partialMu.Lock()
		manifest.Frames[index] = diskFrame{
			File:      filepath.Base(outputPath),
			AtNS:      at.Nanoseconds(),
			SizeBytes: frameInfo.Size(),
		}
		partialFrames[index] = domain.FilmstripFrame{
			At:        at,
			Path:      outputPath,
			SizeBytes: frameInfo.Size(),
		}
		partialMu.Unlock()
		return nil
	}, func(_ int, completed int) {
		if update == nil {
			return
		}
		partialMu.RLock()
		snapshot := domain.Filmstrip{
			Duration: duration,
			Interval: interval,
			Frames:   append([]domain.FilmstripFrame(nil), partialFrames...),
		}
		partialMu.RUnlock()
		update(snapshot, completed, frameCount)
	})
	if err != nil {
		return domain.Filmstrip{}, err
	}

	for index, frame := range manifest.Frames {
		if frame.File == "" {
			return domain.Filmstrip{}, fmt.Errorf("thumbnail generation stopped before frame %d/%d", index+1, frameCount)
		}
	}

	encodedManifest, err := json.Marshal(manifest)
	if err != nil {
		return domain.Filmstrip{}, fmt.Errorf("encode filmstrip manifest: %w", err)
	}
	manifestTempPath := filepath.Join(targetDir, manifestName+".tmp")
	if err := os.WriteFile(manifestTempPath, encodedManifest, 0o600); err != nil {
		return domain.Filmstrip{}, fmt.Errorf("write filmstrip manifest: %w", err)
	}
	if err := os.Rename(manifestTempPath, filepath.Join(targetDir, manifestName)); err != nil {
		return domain.Filmstrip{}, fmt.Errorf("publish filmstrip manifest: %w", err)
	}

	strip, ok := loadCachedFilmstrip(targetDir, info, duration)
	if !ok {
		return domain.Filmstrip{}, fmt.Errorf("published filmstrip cache failed validation")
	}
	published = true
	touchDirectory(targetDir)
	_ = trimDiskCache(g.cacheRoot, targetDir, maxDiskCache)
	return strip, nil
}

// clampDuration bounds a preferred sampling position to the source video's
// valid timeline.
func clampDuration(position, duration time.Duration) time.Duration {
	if position < 0 {
		return 0
	}
	if position > duration {
		return duration
	}
	return position
}

// prioritizedSampleOrder starts with the sample nearest the current playhead.
// Stable sorting makes ties deterministic and keeps frame indexes reproducible.
func prioritizedSampleOrder(frameCount int, interval, preferredPosition time.Duration) []int {
	if frameCount <= 0 {
		return nil
	}
	order := make([]int, frameCount)
	for index := range order {
		order[index] = index
	}
	if interval <= 0 {
		return order
	}
	sort.SliceStable(order, func(i, j int) bool {
		left := sampleTimeDistance(order[i], interval, preferredPosition)
		right := sampleTimeDistance(order[j], interval, preferredPosition)
		return left < right
	})
	return order
}


func sampleTimeDistance(index int, interval, preferredPosition time.Duration) time.Duration {
	sampleTime := time.Duration(index) * interval
	if sampleTime < preferredPosition {
		return preferredPosition - sampleTime
	}
	return sampleTime - preferredPosition
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// runBoundedJobs runs at most workerCount jobs simultaneously. Completion
// callbacks run serially on the caller goroutine, not on worker goroutines.
func runBoundedJobs(
	ctx context.Context,
	jobs []int,
	workerCount int,
	run func(context.Context, int) error,
	onComplete func(index, completed int),
) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(jobs) == 0 {
		return nil
	}
	if workerCount < 1 {
		return fmt.Errorf("worker count must be positive")
	}

	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	queue := make(chan int, len(jobs))
	for _, job := range jobs {
		queue <- job
	}
	close(queue)

	completedJobs := make(chan int, len(jobs))
	var wg sync.WaitGroup
	var errMu sync.Mutex
	var firstErr error

	for worker := 0; worker < min(workerCount, len(jobs)); worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				var job int
				select {
				case <-workCtx.Done():
					return
				case next, ok := <-queue:
					if !ok {
						return
					}
					job = next
				}

				if err := run(workCtx, job); err != nil {
					if workCtx.Err() == nil {
						errMu.Lock()
						if firstErr == nil {
							firstErr = err
						}
						errMu.Unlock()
						cancel()
					}
					return
				}

				select {
				case completedJobs <- job:
				case <-workCtx.Done():
					return
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(completedJobs)
	}()

	completed := 0
	for job := range completedJobs {
		if workCtx.Err() != nil {
			continue
		}
		completed++
		if onComplete != nil {
			onComplete(job, completed)
		}
	}

	errMu.Lock()
	jobErr := firstErr
	errMu.Unlock()
	if jobErr != nil {
		return jobErr
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if completed != len(jobs) {
		return fmt.Errorf("thumbnail extraction stopped after %d of %d frames", completed, len(jobs))
	}
	return nil
}

func sampleFrameCount(duration time.Duration, intervalSeconds int64) int {
	if duration <= 0 || intervalSeconds <= 0 {
		return 0
	}
	count := int(math.Ceil(duration.Seconds() / float64(intervalSeconds)))
	if count < 1 {
		return 1
	}
	if count > targetFrames {
		return targetFrames
	}
	return count
}

func formatSeekTimestamp(position time.Duration) string {
	return strconv.FormatFloat(position.Seconds(), 'f', 3, 64)
}

func ffmpegFrameArgs(mediaPath, outputPath string, position time.Duration) []string {
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-nostdin",
		"-threads", "1",
		"-filter_threads", "1",
		"-accurate_seek",
	}
	if position > 0 {
		// Input seeking jumps to the nearest indexed seek point. FFmpeg then
		// decodes only the short GOP tail needed to reach the requested time.
		args = append(args, "-ss", formatSeekTimestamp(position))
	}
	args = append(args,
		"-i", mediaPath,
		"-map", "0:V:0",
		"-an",
		"-sn",
		"-dn",
		"-vf", videoFilter(),
		"-frames:v", "1",
		"-q:v", "6",
		"-f", "image2",
		"-y", outputPath,
	)
	return args
}

func loadCachedFilmstrip(dir string, sourceInfo os.FileInfo, duration time.Duration) (domain.Filmstrip, bool) {
	manifestPath := filepath.Join(dir, manifestName)
	manifestInfo, err := os.Stat(manifestPath)
	if err != nil || !manifestInfo.Mode().IsRegular() || manifestInfo.Size() <= 0 || manifestInfo.Size() > maxManifestBytes {
		return domain.Filmstrip{}, false
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return domain.Filmstrip{}, false
	}
	var manifest diskManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return domain.Filmstrip{}, false
	}
	if manifest.Version != manifestVersion ||
		manifest.SourceSize != sourceInfo.Size() ||
		manifest.SourceModTimeUnixNS != sourceInfo.ModTime().UnixNano() ||
		manifest.DurationNS != duration.Nanoseconds() ||
		manifest.IntervalSeconds <= 0 ||
		manifest.Width != thumbnailWidth ||
		manifest.Height != thumbnailHeight ||
		len(manifest.Frames) == 0 || len(manifest.Frames) > targetFrames {
		return domain.Filmstrip{}, false
	}

	frames := make([]domain.FilmstripFrame, 0, len(manifest.Frames))
	for _, cached := range manifest.Frames {
		// Only accept a filename, never a path supplied by the manifest.
		if cached.File == "" || filepath.Base(cached.File) != cached.File || cached.SizeBytes <= 0 || cached.SizeBytes > maxImageBytes {
			return domain.Filmstrip{}, false
		}
		path := filepath.Join(dir, cached.File)
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() != cached.SizeBytes {
			return domain.Filmstrip{}, false
		}
		frames = append(frames, domain.FilmstripFrame{
			At:        time.Duration(cached.AtNS),
			Path:      path,
			SizeBytes: cached.SizeBytes,
		})
	}
	return domain.Filmstrip{
		Duration: duration,
		Interval: time.Duration(manifest.IntervalSeconds) * time.Second,
		Frames:   frames,
	}, true
}

func cacheKey(path string, info os.FileInfo, duration time.Duration) string {
	normalized := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		normalized = strings.ToLower(normalized)
	}
	payload := fmt.Sprintf("%s\n%d\n%d\n%d\n%d\n%d\n%d\n",
		normalized,
		info.Size(),
		info.ModTime().UnixNano(),
		duration.Nanoseconds(),
		manifestVersion,
		thumbnailWidth,
		thumbnailHeight,
	)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func sampleIntervalSeconds(duration time.Duration) int64 {
	seconds := duration.Seconds()
	step := int64(math.Ceil(seconds / float64(targetFrames-1)))
	if step < 1 {
		return 1
	}
	return step
}

func videoFilter() string {
	return fmt.Sprintf(
		"scale=%d:%d:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,setsar=1",
		thumbnailWidth, thumbnailHeight,
		thumbnailWidth, thumbnailHeight,
	)
}

func resolveFFmpeg(resourcesDir string) string {
	candidates := make([]string, 0, 8)
	if resourcesDir != "" {
		candidates = append(candidates,
			filepath.Join(resourcesDir, "ffmpeg.exe"),
			filepath.Join(resourcesDir, "ffmpeg", "ffmpeg.exe"),
			filepath.Join(resourcesDir, "third_party", "ffmpeg", "ffmpeg.exe"),
		)
	}
	if executable, err := os.Executable(); err == nil {
		executableDir := filepath.Dir(executable)
		candidates = append(candidates,
			filepath.Join(executableDir, "ffmpeg.exe"),
			filepath.Join(executableDir, "third_party", "ffmpeg", "ffmpeg.exe"),
		)
	}
	if workingDir, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(workingDir, "third_party", "ffmpeg", "ffmpeg.exe"))
	}
	candidates = append(candidates, "ffmpeg.exe", "ffmpeg")
	for _, candidate := range candidates {
		if candidate == "ffmpeg.exe" || candidate == "ffmpeg" {
			if path, err := exec.LookPath(candidate); err == nil {
				return path
			}
			continue
		}
		info, err := os.Stat(candidate)
		if err == nil && info.Mode().IsRegular() {
			return candidate
		}
	}
	return ""
}

func touchDirectory(path string) {
	now := time.Now()
	_ = os.Chtimes(path, now, now)
}

func trimDiskCache(root, keepDir string, budget int64) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	type cacheEntry struct {
		path    string
		size    int64
		modTime time.Time
	}
	caches := make([]cacheEntry, 0, len(entries))
	var total int64
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name())
		if strings.HasPrefix(entry.Name(), ".filmstrip-") {
			// A crash can leave a temporary extraction directory behind.
			// Keep fresh work from another process, but remove abandoned jobs.
			if info, err := entry.Info(); err == nil && time.Since(info.ModTime()) > time.Hour {
				_ = os.RemoveAll(path)
			}
			continue
		}
		if _, err := os.Stat(filepath.Join(path, manifestName)); err != nil {
			// Incomplete stable-path directories are left without a manifest
			// after a crash. Remove them once they are no longer fresh work.
			if info, statErr := entry.Info(); statErr == nil && time.Since(info.ModTime()) > time.Hour {
				_ = os.RemoveAll(path)
			}
			continue
		}
		var size int64
		_ = filepath.WalkDir(path, func(file string, d os.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() {
				return nil
			}
			if info, err := d.Info(); err == nil {
				size += info.Size()
			}
			return nil
		})
		info, err := entry.Info()
		if err != nil {
			continue
		}
		total += size
		caches = append(caches, cacheEntry{path: path, size: size, modTime: info.ModTime()})
	}
	sort.Slice(caches, func(i, j int) bool { return caches[i].modTime.Before(caches[j].modTime) })
	for _, item := range caches {
		if total <= budget {
			break
		}
		if item.path == keepDir {
			continue
		}
		if err := os.RemoveAll(item.path); err != nil {
			continue
		}
		total -= item.size
	}
	return nil
}

type limitedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := b.limit - b.buffer.Len()
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = b.buffer.Write(p)
	}
	return n, nil
}

func (b *limitedBuffer) String() string {
	return b.buffer.String()
}
