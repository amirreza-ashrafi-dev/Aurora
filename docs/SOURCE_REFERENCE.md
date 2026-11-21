# Aurora Player — Source Reference

This reference describes the tracked Go source files, their package responsibilities, exported/unexported types, functions, methods, and tests. The [Developer Guide](DEVELOPER_GUIDE.md) explains architecture and complete user workflows; this file is the code-navigation catalogue.

The feature-branch audit counted 81 tracked files: 58 Go files (including tests), four PNG assets, three Git LFS-managed runtime binaries (two MPV/D3D DLLs and FFmpeg), and 16 other documentation/configuration/build/license files. Build-tagged files are alternatives: only the file matching the current GOOS is compiled. The inventory includes the timeline filmstrip implementation.

# 1. Entrypoints and bootstrap

## main.go and cmd/aurora/main.go

Both have package main and define main(). main calls bootstrap.Run(); if it returns an error, log.Fatal prints it and exits with failure. These are intentionally thin entrypoints; application wiring belongs in bootstrap.

## internal/bootstrap/run.go

### Run() error

Creates the concrete application dependencies and starts the app:
1. Creates the Windows platform using winplatform.New.
2. Builds a PlayerFactory closure that resolves MyGo's resource directory and calls mpv.New when a video HWND is supplied.
3. Resolves the user's config directory.
4. Creates history and subtitle JSON repositories.
5. Resolves MyGo's packaged resources directory and creates ffmpeg.NewFilmstripGenerator(resourcesDir).
6. Injects the platform, player factory, history/subtitle repositories, and filmstrip generator into presentation.New.
7. Defers App.Close, then calls App.Run.

Returns initialization or event-loop errors. If initialization after platform creation fails, the platform is closed before returning.

# 2. Domain types

The domain package contains plain data models and no dependency on MyGo, Win32, or libmpv.

## internal/domain/media.go

### Media

Media.Path is the local media path.

### (Media).Title() string

Returns “Nothing playing” for an empty path; otherwise returns filepath.Base(Path), including the file extension.

## internal/domain/playback.go

### PlaybackState

Position and Duration are seconds, Volume is a percentage, and Paused is the current UI playback state. This is transient presentation state, not the persisted history record.

### PlaybackProgress

MediaPath identifies the local file; Position and Duration are seconds; LastUpdated is the UTC timestamp used to select the newest record. The history adapter serializes this type.

### Rect

X, Y, W, H form the neutral video-stage geometry passed from presentation to the native platform. Coordinates are DIPs.

### Point

X and Y represent a screen-space cursor point in physical pixels.

## internal/domain/filmstrip.go

### Filmstrip

Describes a disk-backed collection of preview frames for one media file. It stores the media duration, nominal sample interval, and ordered frame metadata. Paths point to compressed files on disk; the UI should decode only currently visible frames.

### FilmstripFrame

Stores the presentation timestamp, JPEG path, and encoded file size for one preview image.

## internal/domain/subtitles.go

### SubtitleStyle

FontFamily, FontSize, TextColor, OutlineColor, OutlineSize, and VerticalPosition are neutral subtitle appearance values that the MPV adapter maps to MPV properties.

### SubtitleLanguageProfile

Associates a normalized language tag with a SubtitleStyle.

### SubtitlePreferences

Version is the JSON schema version; Global is the fallback style; Languages contains per-language overrides.

### SubtitleTrack

Represents a subtitle track's ID, title, language, codec, external flag/path, and selected state. This model exists, but the current presentation does not expose a complete track-list API.

### SubtitleAssignment

MediaPath identifies the video; ExternalPath identifies an external subtitle file; TrackID and HasTrackID describe a possible embedded-track selection; Language records its language; Disabled remembers subtitle visibility; UpdatedAt timestamps the assignment. Current UI persistence writes the external path, language, and disabled state, but does not populate or restore TrackID.

### DefaultSubtitleStyle() SubtitleStyle

Returns the application defaults: Segoe UI, 38 px, white text, black outline with alpha, outline size 2, and vertical position 95.

# 3. Application contracts and rules

## internal/application/ports.go

### Player interface

The general playback contract:
- Load(path) error replaces the active media item.
- Append(path) error appends an item.
- PlayIndex(index) error plays a zero-based playlist entry.
- RemovePlaylistItem(index) error removes an item by index.
- GetDouble(name) (float64, error) reads a numeric MPV property.
- SetPaused(paused) error changes pause state.
- Seek(seconds) error seeks to an absolute time.
- SetVolume(volume) error sets volume.
- Stop() error stops/resets playback per adapter semantics.
- Close() releases the player.

The Windows MPV adapter implements this interface; presentation does not depend on libmpv types.

### FilmstripGenerator interface

`Ensure(ctx, mediaPath, duration)` returns a complete cached/generated `domain.Filmstrip`. Implementations must honor context cancellation and avoid retaining decoded images. The FFmpeg adapter implements this optional generation capability; normal playback does not depend on it.

### ProgressRepository interface

Load returns saved PlaybackProgress records; Save adds/replaces one record; Delete removes the record for a media path.

### NativePlatform interface

The platform contract includes:
- CreateVideoHost(parent): create child HWND.
- InstallVideoDoubleClick, InstallVideoMouseWheel, InstallVideoFileDrop: attach input callbacks to the child.
- LayoutVideo(parent, child, fullscreen, stage): size/position the child.
- CursorInsideWindow(hwnd): check cursor position.
- WindowDPI(hwnd): read native DPI.
- ShowCursor, ShowWindow, UpdateWindow, DestroyWindow: native window/cursor operations.
- RevealChild: show/raise child without activation.
- RemoveVideoSubclass: remove callbacks.
- LogHWND: emit handle/geometry diagnostics.
- Close: release native DLL handles.

It accepts domain.Rect rather than MyGo geometry types.

### PlayerFactory

A function type that takes a video HWND and returns a Player. This delays MPV construction until the native child window exists.

## internal/application/playback.go

### SeekTarget(position, delta, duration) float64

Adds delta to position; clamps at zero; also clamps to duration when duration is known and positive. Unknown duration has no upper clamp.

### ClampVolume(volume) float64

Clamps a volume value into 0..100.

## internal/application/playlist.go

### AddSummary

Carries accepted absolute paths in Valid plus Duplicates and Invalid counts for user feedback.

### FilterNewMedia(existing, candidates) AddSummary

Trims each candidate, creates an absolute path, stats it and rejects missing/non-regular files, resolves symlinks when possible, and checks duplicate identity against the existing queue and accepted candidates. It compares normalized path keys and os.SameFile, detecting hard links. Returns a new summary and does not mutate either input slice or the MPV playlist.

### mediaPathKey(path) string

Internal helper: trims the path, makes it absolute, resolves symlinks when possible, cleans it, and lowercases on Windows.

### RemovePlaylistEntry(queue, currentIndex, removeIndex) ([]string, int, bool)

Builds a new queue without removeIndex and computes the new current index. Removing an item before current decreases the index. Removing the current item selects the next item at that index, or the prior final item when the removed item was last. Returns new queue, new current index, and whether the removed item was current. Invalid remove indexes leave the original queue/index unchanged and report false.

## internal/application/progress.go

### ProgressKey(path) string

Creates a normalized media key by trimming, making absolute, resolving symlinks if possible, cleaning, and lowercasing on Windows.

### FindProgress(records, path) (PlaybackProgress, bool)

Finds all normalized-path matches and returns the one with the newest LastUpdated. Returns false for empty/missing paths.

### CanResumeProgress(record) bool

Rejects empty paths, NaN/infinite values, position below 10 seconds, negative duration, position past duration, and a position within 30 seconds of a known end. Duration zero is treated as unknown and allowed.

## internal/application/subtitles.go

### SubtitlePlayer interface

Optional player capability: LoadExternalSubtitle, SetSubtitleVisibility, SetSubtitleDelay, SetSubtitleStyle, and ActiveSubtitleLanguage.

### SubtitlePreferencesRepository interface

Loads and saves SubtitlePreferences.

### SubtitleAssignmentRepository interface

Loads all SubtitleAssignment records, saves one, and deletes by media path.

### NormalizeSubtitleLanguage(language) string

Trims, changes underscores to hyphens, lowercases the primary tag, uppercases two/three-character later subtags, and lowercases other subtags. Empty or malformed tags become und. It is a practical application normalizer, not a full BCP-47 parser.

### SubtitleStyleForLanguage(prefs, language) SubtitleStyle

Looks for an exact normalized language profile, then progressively tries less-specific parent tags. Returns Global when no profile matches, or DefaultSubtitleStyle if the global font family is empty.

### ValidateSubtitleStyle(style) error

Checks font family, font-size range 8..120, outline-size range 0..10, vertical position 0..100, and text/outline color format (#RRGGBB or #AARRGGBB). Returns an error before values are sent to MPV.

### validSubtitleColor(value) bool

Internal checker for trimmed #-prefixed 6- or 8-digit hexadecimal colors.

### IsSubtitleFile(path) bool

Recognizes .srt, .ass, .ssa, .vtt, .sub, and .idx. If the path already exists it must be a regular file; directories are rejected.

### MatchSubtitleFiles(videoPath, candidates) []string

Filters candidates to supported subtitle files whose basename matches the video stem exactly or has a period, underscore, or space suffix. Exact stem matches sort before suffixed matches; remaining matches use lowercase path ordering. No fuzzy or recursive search.

### DiscoverSubtitleFiles(videoPath) ([]string, error)

Reads only the video's containing directory, ignores directories, then delegates to MatchSubtitleFiles.

### SubtitleLanguageFromFilename(videoPath, subtitlePath) string

Extracts the suffix after the video basename (e.g. fa-IR from Movie.fa-IR.srt). Returns und for unrelated stems or missing/unsuitable suffixes.

# 4. Persistence adapters

## internal/adapters/history/json.go

### Constants and data types

- fileVersion: history JSON schema version 1.
- maxRecords: maximum 500 records.
- maxFileBytes: maximum 1 MiB.
- fileData: versioned JSON envelope containing Version and Records.
- JSONRepository: absolute file path and mutex for read/modify/write operations.

### NewJSONRepository(path) (*JSONRepository, error)

Rejects an empty path, resolves it to absolute form, and returns a repository. It does not create the file immediately.

### (JSONRepository).Load() ([]PlaybackProgress, error)

Locks the repository and calls loadUnlocked. Missing file returns an empty list.

### (JSONRepository).Save(record) error

Locks, normalizes path, rejects invalid numeric values, sets LastUpdated in UTC, loads current records, replaces the record with the same normalized path, and writes the result.

### (JSONRepository).Delete(mediaPath) error

Locks, normalizes the key, loads records, removes matching items, and writes the remaining list. Empty key is a no-op.

### loadUnlocked() ([]PlaybackProgress, error)

Internal reader. Checks file size, decodes JSON, checks version, discards invalid records, collapses duplicate paths by newest timestamp, sorts newest-first, and caps at 500. It does not require the referenced media files to be online.

### writeUnlocked(records) error

Sorts and trims records, enforces 500-item and 1 MiB bounds, creates the parent directory, writes a same-directory temporary file with restrictive permissions, syncs/closes it, then renames it over the destination. Removes the temporary file on cleanup.

### normalizeMediaPath(path) string

Trims, resolves absolute path and symlinks when possible, and cleans it. Empty/unresolvable paths return empty.

### pathKey(path) string

Builds the normalized key and lowercases on Windows.

### sortRecordsNewestFirst(records)

Sorts the slice in descending LastUpdated order.

## internal/adapters/subtitles/json.go

### Constants and types

- preferencesVersion and assignmentsVersion: separate schema versions, both currently 1.
- maxSubtitleFileSize: 1 MiB per file.
- preferencesEnvelope: Version plus SubtitlePreferences.
- assignmentsEnvelope: Version plus []SubtitleAssignment.
- JSONRepository: preferences path, assignments path, and mutex.

### NewJSONRepository(directory) (*JSONRepository, error)

Rejects an empty directory, resolves it to an absolute path, and sets up subtitle_preferences.json and subtitle_assignments.json paths.

### (JSONRepository).Load() (SubtitlePreferences, error)

Reads preferences. Missing file returns default preferences; existing files are bounded and version-checked. The returned preferences are tagged with the current version.

### SavePreferences(prefs) error

Sets the current version, encodes the envelope, and uses atomicWrite.

### LoadAll() ([]SubtitleAssignment, error)

Reads and validates the assignment envelope. Missing file returns an empty slice.

### SaveAssignment(record) error

Normalizes paths, requires MediaPath, fills UpdatedAt if absent, loads existing entries, replaces the same path key, sorts newest-first, caps at 500, and writes the envelope.

### Delete(mediaPath) error

Removes all assignment records for the normalized media path and writes the new envelope. Missing assignment file is already empty.

### atomicWrite(path, data) error

Rejects content above 1 MiB, creates the parent directory, writes to a same-directory temporary file, syncs and closes, then renames to the destination. Temporary file is removed after cleanup.

### normalizePath(path) string and pathKey(path) string

normalizePath trims, creates an absolute path and cleans it. Unlike history normalization, it does not call EvalSymlinks. pathKey lowercases on Windows.

# 5. FFmpeg filmstrip adapter

## internal/adapters/ffmpeg/filmstrip.go

### FilmstripGenerator and disk manifest types

The adapter owns the FFmpeg executable path, cache root, a single-generation/cache-publication lock, and a two-slot worker semaphore. The manifest records format version, source file size and modification time, media duration, sample interval, frame dimensions, and each frame's file name, timestamp, and encoded byte size.

### NewFilmstripGenerator(resourcesDir) *FilmstripGenerator

Resolves FFmpeg from the packaged resource directory, beside the application executable, development locations, or PATH. The cache root is under the OS user cache directory in `Aurora Player/filmstrip`. Missing FFmpeg is reported when filmstrip generation is requested; it does not stop normal playback.

### NewFilmstripGeneratorAt(ffmpegPath, cacheRoot) *FilmstripGenerator

Creates an adapter with explicit paths for tests and controlled use.

### (FilmstripGenerator).Ensure(ctx, mediaPath, duration)

Returns a valid cached filmstrip or generates one. This is the simple application-port method and delegates to progressive generation with no update callback.

### (FilmstripGenerator).EnsureWithProgress(...)

Compatibility wrapper that reports completed/total counts without exposing partial frame paths.

### (FilmstripGenerator).EnsureProgressive(...)

Validates the source and duration, checks the fingerprinted cache, serializes cache generation, and creates up to 48 160×90 JPEG previews. It prioritizes samples by absolute timestamp distance from the preferred playhead. Two workers may extract frames concurrently; each FFmpeg process uses one decode thread and one filter thread. Completed paths are sent to the presentation while unready frame slots retain their timeline positions.

The adapter does not publish a manifest until every frame is complete. It writes the manifest to a temporary file and renames it into place. A failed or canceled generation removes its staging directory, so an incomplete set is not treated as a valid cache.

### prioritizedSampleOrder(...) and sampleTimeDistance(...)

Sort sample indexes by their actual timestamp distance from the preferred position. Stable sorting keeps ties deterministic.

### runBoundedJobs(...)

Runs a fixed number of jobs concurrently, cancels remaining work after the first failure, and sends completion callbacks serially on the caller goroutine.

### sampleIntervalSeconds(...) and sampleFrameCount(...)

Choose a time interval and cap the filmstrip at 48 frames.

### ffmpegFrameArgs(...) and videoFilter()

Build the FFmpeg arguments and scale/pad output to 160×90. Input seeking is placed before the input file; the child is limited to one decoder and one filter thread.

### loadCachedFilmstrip(...) and cacheKey(...)

Validate the manifest and every referenced image, reject unsafe file names and unexpected image sizes, and fingerprint the source path, file size, modification time, duration, format version, and dimensions.

### trimDiskCache(...) and touchDirectory(...)

Update cache recency and enforce a best-effort 512 MiB LRU disk budget.

## internal/adapters/ffmpeg/process_windows.go

### configureFFmpegCommand(cmd)

Sets `HideWindow` and `CREATE_NO_WINDOW` on Windows child processes. This prevents background thumbnail extraction from opening a visible terminal window for each FFmpeg process.

## internal/adapters/ffmpeg/process_other.go

### configureFFmpegCommand(cmd)

A non-Windows no-op so the adapter remains buildable on other platforms without Windows process flags.

# 6. MPV adapter

## internal/adapters/mpv/player_windows.go

Selected by the windows build tag. It loads libmpv dynamically and binds its exported C functions to Go functions using purego.

### Player fields

- dll: loaded Windows DLL handle.
- ctx: native mpv_handle.
- create, setOptionString, initialize, command, setPropertyString, getProperty, terminateDestroy, errorString, getPropertyString, free: registered native function references.
- runtimeVideoOutputReinit: cached result of the MPV version capability check.
- videoOutputReleased: records whether Aurora switched vo to null; Load restores gpu.

### mpvErrorText(ptr) string

Converts an MPV-owned error-string pointer. Null pointer becomes an “unknown mpv error” message.

### (Player).readMPVVersion() string

Reads the mpv-version property and frees the MPV-allocated string. Returns empty if player/property support is unavailable.

### SupportsRuntimeVideoOutputRelease() bool

Returns the capability flag cached by New. Presentation uses it to choose between releasing only video output and destroying the whole context.

### New(hwnd, resourcesDir) (*Player, error)

Finds libmpv-2.dll in packaged resources, falling back to third_party/mpv. Sets the DLL search directory, loads the DLL, registers native functions, creates the MPV context, sets options (wid, vo=gpu, gpu-api=d3d11, hwdec=auto-safe, keep-open, and input bindings), initializes, reads mpv-version, and checks runtime VO support. On error, closes partially created resources.

### commandArgs(args...) error

Converts Go strings into null-terminated Windows byte strings, builds argv pointers, calls mpv_command, and keeps backing memory alive through the native call. Negative MPV result codes become Go errors.

### Load(path) error

Trims and validates the path, makes it absolute, checks it exists, restores vo=gpu if the renderer was released, then sends loadfile with replace mode.

### Append(path) error

Converts the path to absolute form and sends loadfile append.

### PlayIndex(index) error

Sends playlist-play-index with the zero-based index.

### RemovePlaylistItem(index) error

Rejects negative indexes, then sends playlist-remove. Upper bound checking is delegated to MPV.

### GetVideoHeight() (int, error)

Reads height, falling back to video-params/h; rejects a negative height.

### getInt64(name) (int64, error)

Internal typed MPV property read using the integer format.

### GetDouble(name) (float64, error)

Reads a named MPV property using the double format. Used for time-pos, duration, and playlist-pos.

### SetPaused(paused) error

Writes the pause property as yes/no.

### Seek(seconds) error

Clamps negative seconds to zero and sends an absolute exact seek.

### SetVolume(volume) error

Clamps 0..100 and writes MPV's volume property.

### CaptureThumbnail(path) error

Validates output path, creates the parent directory, and requests a screenshot-to-file. Presentation handles image decode and resize.

### Stop() error

Pauses and seeks to zero. It does not destroy the MPV context.

### Unload() error

Sends stop, checks runtime capability, then sets vo=null. On supported builds, MPV's option update rebuilds the video output and releases renderer resources while retaining the core. Unsupported builds return an error; presentation normally chooses full context teardown before calling this path.

### setVideoOutput(name) error

Internal helper to set MPV's vo property and translate errors.

### Close()

Calls mpv_terminate_destroy and frees the DLL handle. Clears stored handles to avoid reuse.

### LoadExternalSubtitle(path) error

Validates the file, makes it absolute, and sends sub-add in select mode.

### CycleSubtitleTrack() error

Sends cycle sid so MPV chooses the next subtitle track.

### RemoveSelectedExternalSubtitle() error

Reads selected sid and its external flag. Only sends sub-remove for an external track, not an embedded track.

### getPropertyStringValue(name) (string, error)

Reads an MPV string property and always frees the returned MPV-owned string.

### SetSubtitleVisibility(visible) error

Sets sub-visibility without removing/changing the selected subtitle track.

### DisableSubtitles() error

Convenience wrapper for SetSubtitleVisibility(false).

### SetSubtitleDelay(seconds) error

Accepts -3600..+3600 seconds and sets sub-delay.

### SetSubtitleStyle(style) error

Maps FontFamily, FontSize, TextColor, OutlineColor, OutlineSize, and VerticalPosition to MPV subtitle properties. Returns the first property error. Formats such as ASS/SSA may control some of their own appearance.

### ActiveSubtitleLanguage() (string, error)

Reads current-tracks/sub/lang. Missing language metadata is represented by an empty language; the MPV-owned string is freed.

### setSubtitleProperty(name, value) error

Internal helper that checks player initialization, writes one property, and translates negative MPV codes.

## internal/adapters/mpv/version.go

### supportsRuntimeVideoOutputReinit(version) bool

Parses major/minor from the first numeric part of the reported version. Returns true for major > 0 or MPV 0.39 and newer. Unknown/empty strings return false. This is a version gate, not a live probe of renderer behavior.

## internal/adapters/mpv/player_unsupported.go

Selected for !windows. Defines the same Player type and method set so dependent packages compile. New and all operations return “Windows only” errors (query operations also return a zero/empty value); SupportsRuntimeVideoOutputRelease returns false; Close is a no-op. This is a compile-time stub, not an alternative decoder.

# 7. Windows platform adapter

## internal/platform/windows/platform_windows.go

Selected for windows. Loads user32.dll, comctl32.dll, and shell32.dll and registers procedures via purego.

### Structures and constants

- rect mirrors RECT edges.
- monitorInfo holds monitor rectangle, work rectangle, structure size, and flags.
- gwOwner is the GetWindow owner selector.
- monitorDefaultToNearest asks Windows for the nearest monitor.

### Platform fields

Stores DLL handles, subclass callback pointers and IDs, shell drag/drop procedures, and function references for native window creation, geometry, cursor, subclass, parent/owner, monitor, and DPI APIs.

### New() (*Platform, error)

Loads the three DLLs and registers functions. If any load fails, releases those already loaded.

### CreateVideoHost(parent) (uintptr, error)

Creates a hidden STATIC child HWND titled Aurora Video under the main window. The hidden initial state preserves the application's empty-state view.

### InstallVideoDoubleClick(hwnd, callback) error

Subclasses the child window. WM_LBUTTONDBLCLK invokes callback; unrelated messages are forwarded to DefSubclassProc. Retains the callback pointer.

### InstallVideoMouseWheel(hwnd, callback) error

Installs one WM_MOUSEWHEEL subclass. Extracts signed delta from wParam, invokes callback for non-zero delta, consumes the wheel message, and forwards other messages.

### InstallVideoFileDrop(hwnd, callback) error

Enables shell drops and handles WM_DROPFILES with DragQueryFileW. Calls DragFinish before passing collected paths to callback. Disables drops again if subclass installation fails.

### LayoutVideo(parent, child, fullscreen, stage) error

Uses the parent client rectangle. Fullscreen fills that rectangle; windowed mode converts stage DIPs to physical pixels using WindowDPI/96 and rounding. Calls SetWindowPos without activation or z-order changes. Empty sizes are ignored.

### CursorInsideWindow(hwnd) (bool, domain.Point)

Gets screen cursor position and window rectangle, then checks whether the point is inside. Returns false if either query fails.

### WindowDPI(hwnd) uint32

Returns GetDpiForWindow or a 96-DPI fallback.

### ShowCursor(show), ShowWindow(hwnd, command), UpdateWindow(hwnd), DestroyWindow(hwnd)

Thin wrappers around the registered Win32 procedures.

### RevealChild(hwnd) error

Uses SetWindowPos with no-move/no-size/no-activate/show flags, then SW_SHOWNOACTIVATE and UpdateWindow. Shows/raises the child without taking focus.

### RemoveVideoSubclass(hwnd)

Removes installed subclass handlers, disables drag-and-drop, and clears callback pointers. Call before destroying the HWND.

### LogHWND(label, hwnd)

Logs HWND, parent/owner, DPI, client/window rectangles, nearest monitor/work area, and primary-monitor status.

### MakeControllerNonActivating(hwnd) error

Adds WS_EX_TOOLWINDOW and WS_EX_NOACTIVATE and applies a frame change. Source comment says it remains for diagnostics/future experiments; the current controller uses a MyGo flyout with Focusable=false.

### Close()

Frees comctl32, shell32, and user32 handles and clears them.

## internal/platform/windows/platform_unsupported.go

Selected for !windows. New and operations that require native window creation/placement return Windows-only errors. CursorInsideWindow returns false/empty point; WindowDPI returns 96; show/update/destroy methods return zero; subclass removal, diagnostics, and Close do nothing. It keeps the interface compiling without simulating a video host.

# 8. MyGo presentation package

All files below use package presentation under internal/presentation/mygo. They share App state and the MyGo UI-thread lifecycle.

## internal/presentation/mygo/app.go

### App fields

- win: main MyGo window.
- controller, resumeBanner, fullscreenHeader, volumeOSD, subtitleFlyout, fullscreenSubtitleFlyout, memoryWindow: overlay/diagnostic windows.
- anchor/expiry fields: positions and deadlines used to keep flyouts aligned and time-limited.
- wheelRemainder: partial mouse-wheel delta not yet equal to a complete detent.
- platform, videoHWND: native platform and child window handle.
- player, playerFactory, subtitle: current playback contract, lazy factory, optional subtitle capability.
- playerCommandMu: read/write lock that prevents player teardown during a background native thumbnail call.
- history, historyRecords, pendingResume: history repository, in-memory index, and pending resume choice.
- subtitlePreferences, subtitleAssignments: persistence contracts for global styles and per-video choices.
- filmstripGenerator: optional application capability used to create/reuse disk-cached timeline frames.
- filmstrip manifest/progress/token/cancel fields: UI-thread-owned frame metadata, progressive-generation state, and stale-callback protection.
- filmstripBitmaps and filmstripBitmapClock: bounded decoded-image cache and LRU clock; timelinePreviewPosition and scrubbing keep preview separate from committed playback position.
- subtitle state/maps: cached assignments, candidates, preferences/style, delay, visibility, active language.
- thumbnail maps/counters: bitmaps, capture markers/tokens, memory estimates and token sequence.
- memory fields: sampler stop channel, sample slice, export message, GC state.
- progressRequests/progressStop/progressDone: serialized progress writer channels.
- videoStageHandle/bounds/DPI flags: MyGo-to-native geometry tracking and follow-up frame state.
- path/playlist/playlistPos/pendingPlaylistRemoval: current media, Go queue, selected index, deferred removals.
- status/state/scrubbing/fullscreen/cinematicMode: current presentation state.
- pollStop/pollDone, controllerStop/controllerDone: background goroutine lifecycle.
- controllerAnchor/hasControllerAnchor: last flyout anchor.
- cursorHidden/cursorShowAdjustments: state required to balance ShowCursor.
- closeOnce: makes shutdown idempotent.

Most App state is UI-thread-owned. playerCommandMu protects the native player lifetime; it does not make every App field safe for arbitrary goroutines.

### New(...)*App

Injects platform, PlayerFactory, history repository, subtitle-preference repository, and assignment repository, plus an optional FilmstripGenerator. Initializes maps/default styles, volume=100, and startup status. Does not create a window/player.

### Run() error

Registers onReady with MyGo and starts its event loop.

### onReady()

Creates the main window, sets its icon, creates the hidden native child, installs double-click/drop/wheel callbacks, loads history and subtitle state, and registers window lifecycle handlers. Deliberately leaves libmpv unloaded.

### ensurePlayer() error

Reuses an existing player and starts polling if needed. Otherwise verifies factory and HWND, creates the player, sets initial volume, stores Player and optional SubtitlePlayer under lock, applies default subtitle style, and starts polling.

### unloadPlayerMedia() error

Locks playerCommandMu. If the player reports runtime VO release support, calls Unload. If it reports no support, closes the entire player context. If it has only Unload, calls it; otherwise falls back to Stop.

### closePlayer() and closePlayerLocked()

closePlayer obtains the exclusive player lock. closePlayerLocked clears the App references then closes the old player. The locked helper must only be called while playerCommandMu is held.

### formatTime(seconds) string

Clamps negative values to zero and formats mm:ss or h:mm:ss.

### currentTitle() string

Returns the title derived from domain.Media{Path:a.path}.

## internal/presentation/mygo/brand.go

### auroraLogoPNG

Embedded bytes from assets/logo.png.

### auroraLogoOnce and auroraLogoBitmap

sync.Once ensures the image is decoded only once and shared between UI views.

### auroraLogo() *ui.Bitmap

Decodes PNG bytes once; returns nil if decoding fails.

## internal/presentation/mygo/cursor.go

### setFullscreenCursorHidden(hidden)

Balances ShowCursor calls. Hiding decrements at most 64 times until Windows reports the pointer hidden. If it cannot, it reverses the changes and logs a warning. Restoring increments exactly the number of decrements Aurora previously performed. Does nothing if the state is already correct or platform is nil.

## internal/presentation/mygo/diagnostics.go

### logDiagnostics(label)

Logs main/controller MyGo content bounds and delegates native HWND, DPI, monitor and rectangle details to Platform.LogHWND. Used around create/move/resize/fullscreen lifecycle events.

## internal/presentation/mygo/main_view.go

### view(c)

Top-level main UI builder. Applies pending playlist removals before building the virtual list, registers shortcuts, applies the theme, synchronizes native video geometry, chooses normal or Cinematic Mode, and schedules a follow-up frame to read committed stage bounds.

### normalView(c)

Builds the title bar, inline Resume prompt, video stage, queue panel, queue rows, Add videos button, and playback deck. Queue buttons call playPlaylistIndex or queuePlaylistRemoval. Removal is deferred so the current virtual list is not mutated while it is being built.

### cinematicView(c)

Builds a dark full-window video stage and branded empty state. The floating playback controller is a separate flyout.

## internal/presentation/mygo/titlebar.go

### windowTitleBar(c)

Draws a 40-DIP custom title bar with logo, draggable area, minimize, maximize/restore, and close controls. Maximize glyph changes with MyGo window state.

## internal/presentation/mygo/theme.go

### Palette constants

auroraBackground, auroraHeader, auroraPanel, auroraPanelAlt, auroraFooter, auroraBorder, auroraBorderSoft, auroraText, auroraMuted, auroraAccent, auroraAccentHover, auroraAccentPressed, auroraAccentText, and auroraAccentSoft are shared visual colors.

### Layout constants

auroraMainPadding, auroraStageMinHeight, auroraQueueWidth, and auroraStageGap control the main layout. The current queue panel width is fixed at 320 DIPs.

### applyAuroraTheme(c)

Starts from MyGo's dark theme, applies user preferences, then sets Aurora surfaces, text, accent, focus/selection, radius, spacing, font size, and Segoe UI. High-contrast preference changes focus color.

### auroraQuietButton(c, label)

Creates a secondary Aurora button.

### auroraPrimaryButton(c, label)

Creates the gold primary action button.

### auroraAddVideosButton(c)

Creates the full-width Add videos button.

### auroraWindowButton(c, glyph, tooltip, closeButton)

Creates a title-bar button; close uses a warning hover surface and other controls use neutral hover styling.

## internal/presentation/mygo/video_layout.go

### layoutVideo()

Checks platform/window/HWND and usable geometry, then calls Platform.LayoutVideo. Logs errors.

### syncVideoHostToStage(c)

Reads committed MyGo handle bounds and native DPI, stores domain.Rect and DPI, skips unchanged geometry, and tells the native adapter to move/resize the child. Fullscreen stores the stage but relies on layoutVideo to fill the client area.

### revealVideoHost()

Applies current geometry, shows/raises child without activation, and invalidates the window.

### hideVideoHost()

Hides the native child and invalidates the window so empty-state UI can be drawn.

## internal/presentation/mygo/playback.go

### togglePlayPause()

Calls SetPaused with the inverse of current state. On success updates status/state; pausing schedules progress save and records a memory event, resuming cancels pending pause save and records a memory event.

### seekBy(seconds)

Computes an absolute target with application.SeekTarget, calls Player.Seek, updates local position, and schedules a save.

### changeVolume(amount)

Adds amount, clamps with application.ClampVolume, calls SetVolume, updates state, and shows volume OSD.

### toggleFullscreen() and exitFullscreen()

toggleFullscreen delegates to MyGo. exitFullscreen leaves fullscreen only when active.

### toggleCinematicMode()

Flips custom Cinematic Mode.

### setCinematicMode(enabled)

Checks window/state, updates mode, closes normal subtitle flyout when entering, invalidates layout, and shows/hides the overlay controller/cursor as appropriate.

### registerShortcuts(c)

Registers F/F11/Alt+Enter fullscreen; C Cinematic Mode; Esc close subtitle flyout, leave Cinema, or exit fullscreen; Space play/pause; arrows seek/volume; Alt+left/right adjust subtitle delay; Alt+Backspace reset delay; Ctrl+Shift+M opens memory diagnostics.

## internal/presentation/mygo/playlist.go

### chooseFiles()

Opens a multi-file video dialog in a goroutine; returns selected paths through Window.Update. Cancel/closed-window cases are ignored and errors update UI status.

### addFiles(paths)

Filters paths with application.FilterNewMedia, reports invalid/duplicate counts, calls ensurePlayer, then either loads the first media and appends the rest or appends everything to an existing MPV playlist. Updates the Go queue only when MPV succeeds.

### queuePlaylistRemoval(path)

Stores the media path for a later removal; does not mutate playlist immediately.

### applyPendingPlaylistRemovals()

Runs before new list construction, resolves each path against the current queue, and invokes removePlaylistIndex. Path-based resolving handles shifts caused by multiple removals.

### removePlaylistIndex(index)

Validates player/index, saves current progress if needed, calls MPV playlist-remove, and updates the Go slice with application.RemovePlaylistEntry. Evicts thumbnails only when no matching duplicate path remains. Removing a non-current item leaves playback alone. Removing current updates path/state; empty queue hides the child, stops polling and unloads MPV media. If current was the final item, it explicitly plays the new final entry; otherwise it relies on MPV's queue advancement and polling reconciliation.

### playPlaylistIndex(index)

Validates index, saves outgoing progress, reveals video, asks MPV to play the item, updates path/state, records a memory sample, offers resume, and restores subtitle state.

### next()

Moves to the next queue index if one exists.

### previous()

Restarts the current media if playback position is over three seconds; otherwise plays the previous queue item if one exists.

## internal/presentation/mygo/polling.go

### startPolling()

Idempotently starts a 250 ms polling goroutine when a player exists. Reads MPV position, duration, playlist index, and optional height. Uses Window.Update to reconcile automatic advancement, update state, capture thumbnails, schedule progress persistence, check subtitle language, and invalidate a visible controller.

### stopPolling()

Closes the stop channel, waits for the goroutine to finish, then clears its channels.

## internal/presentation/mygo/progress.go

### progressWriteRequest

Describes a queued progress operation: record to save, deletePath to remove, debounce for delayed save, or cancel for pending delayed save.

### loadProgressHistory()

Loads repository records into historyRecords keyed by application.ProgressKey and starts the writer worker.

### startProgressWorker()

Starts a single worker and one-slot request channel. Serializes Save/Delete. Keeps the latest debounced record, writes it after two seconds, and drains queued work before exit.

### sendProgressRequest(req)

Non-blockingly enqueues a request. When full, removes an older queued request and tries to enqueue the newest, coalescing updates.

### stopProgressWorker()

Signals worker shutdown, waits for it to drain queued/pending data, then clears channel references.

### progressSnapshot(now)

Returns a useful PlaybackProgress or a completed flag. Rejects no path, NaN/Inf, position below ten seconds, or negative duration. If known duration is within 30 seconds of the end, marks completed. Otherwise clamps position to duration and timestamps in UTC.

### rememberProgress(record)

Updates the in-memory record by normalized path and keeps the map at or below 500 by evicting the oldest timestamp.

### scheduleCurrentProgressSave()

Takes a snapshot; deletes history if completed; otherwise updates memory and enqueues a debounced write.

### saveCurrentProgressImmediately()

Same validation as scheduleCurrentProgressSave but requests immediate persistence.

### clearProgress(path)

Deletes the in-memory key and requests repository deletion.

### clearCurrentProgress()

Deletes current progress, clears pending resume, and closes the resume banner.

### cancelPendingProgressSave()

Enqueues a cancel operation for the writer's pending debounced save.

### offerResumeForCurrent()

Looks up saved progress, checks file existence and CanResumeProgress, deletes invalid records, otherwise pauses playback and presents the resume choice.

### chooseResume()

Seeks to saved position, unpauses, clears prompt and updates playback state/status.

### chooseStartOver()

Clears saved progress and prompt, seeks to zero, unpauses, and updates state/status.

### updateTimeline(dragging, changed)

Tracks scrubbing, seeks when slider changes, and schedules save when the drag ends.

### resumePrompt(c)

Builds inline Resume / Start over controls for normal window mode. Does nothing in fullscreen/Cinematic Mode.

### showResumeBanner()

Creates or repositions floating Smart Resume banner for fullscreen/Cinematic Mode.

### closeResumeBanner()

Closes banner window and clears reference/anchor.

### resumeBannerView(c)

Draws banner and forwards button actions to chooseResume/chooseStartOver.

## internal/presentation/mygo/fullscreen.go

### Constants

controllerWidth, controllerHeight, and controllerBottom define overlay controller dimensions and bottom placement.

### showFullscreenHeader() / hideFullscreenHeader()

Create/reposition or hide the top branding flyout.

### hideResumeBanner()

Hides the banner but does not destroy it.

### fullscreenHeaderView(c)

Draws logo/branding and the mode-appropriate exit action.

### controllerAnchorTarget() (Rectangle, bool)

Computes an anchor at the bottom of the main content area. Returns false before usable bounds exist.

### createFullscreenController()

Creates the non-focusable playback-controller flyout and records its anchor.

### updateControllerBounds()

Repositions controller after main-window movement/resizing and updates the fullscreen subtitle flyout anchor.

### updateFullscreenSubtitleFlyoutAnchor()

Converts subtitle-button bounds from controller coordinates into main-window content coordinates and repositions the flyout.

### showController() / hideController()

Show the controller plus header/resume UI, or close the fullscreen subtitle flyout and hide the group.

### startControllerMonitor()

Starts a 50 ms monitor. Uses MyGo's main thread to inspect fullscreen/Cinematic state, cursor position, overlay hover, inactivity, and volume OSD expiry. Reveals controls on activity and hides them after three seconds without controller interaction.

### stopControllerMonitor()

Signals and waits for the monitor goroutine.

## internal/presentation/mygo/fullscreen_view.go

### fullscreenControls(c)

Draws the floating playback deck: title, mode/exit actions, timeline, previous/play/next/stop, volume, and subtitle Studio button. Registers shortcuts.

## internal/presentation/mygo/subtitle_ui.go

### chooseSubtitleFile()

Opens a single-file subtitle dialog asynchronously, returns to the UI thread, loads the file, saves its per-video assignment, closes flyouts, and invalidates the window.

### applySubtitleStyle(style)

Validates style, requires SubtitlePlayer, applies settings to MPV, updates presentation state/global prefs, and saves preferences. The live player may already have changed if the later persistence write fails.

### adjustSubtitleDelay(delta) and resetSubtitleDelay()

adjustSubtitleDelay applies delta and clamps the UI control to -30..+30 seconds before calling MPV. resetSubtitleDelay sets zero. The adapter accepts a wider backend limit.

### subtitleStudioControls(c)

Builds controls for load/enable/disable/close, font family/size, outline size, text and outline colors, vertical position, sync delay, next track, remove external track, save language profile, and selecting among candidate files. Style edits copy the current style then call applySubtitleStyle.

### subtitleSectionHeading(c, label)

Draws a reusable section label with a gold dot and divider.

### saveStyleForActiveLanguage()

Normalizes active language, rejects und, updates or adds a language style profile, then saves preferences.

### loadSubtitleAssignments()

Loads assignment records and indexes by subtitlePathKey(MediaPath).

### persistSubtitleAssignment(externalPath, language, disabled)

Requires a current media path and assignment repository. Empty path/language arguments preserve prior values. Saves a record and updates in-memory map. Does not currently write embedded TrackID.

### restoreSubtitleForCurrent()

Makes subtitles visible, then either discovers matches if no saved assignment exists or validates/reloads the saved external file. One discovered match is auto-loaded; multiple matches become candidates. Restores disabled state from assignment.

### handleDroppedSubtitleFiles(paths)

Filters for supported regular subtitle files, deduplicates paths, auto-loads one match or offers multiple candidates, and requires an active media/subtitle player.

### loadExternalSubtitle(path) error

Loads through SubtitlePlayer, enables visibility, then clears subtitleDisabled.

### toggleSubtitleVisibility()

Flips visibility, saves disabled state, updates status, and invalidates the main window.

### subtitlePathKey(path) string

Returns absolute, cleaned, lowercase path key. It lowercases on all platforms, consistent with Windows target behavior.

### subtitleColorFromMPV(value) ui.Color

Converts a #RRGGBB or #AARRGGBB string to opaque MyGo UI color. Alpha is ignored for the swatch; invalid input returns white.

### closeSubtitleFlyoutWindow(flyout, anchor, open)

Shared helper that clears reference/anchor/open state, then closes the old window if still alive.

### closeSubtitleFlyout() / closeFullscreenSubtitleFlyout()

Use the shared close helper for normal and fullscreen subtitle flyouts.

### subtitleStudioFlyoutButton(c) / fullscreenSubtitleStudioFlyoutButton(c)

Configure the shared flyout helper for a main-window button or a controller button whose flyout is parented to the main window.

### subtitleStudioFlyoutButtonAt(...)

Builds button, reads committed ui.Handle bounds, converts coordinates when button and parent windows differ, creates/repositions flyout, and clears state on close.

### subtitleStudioFlyoutView(c)

Applies theme and places controls in a scroll view.

### subtitleStudioPopover(c, label, open)

Alternate PopoverBase implementation for subtitle controls. Current entrypoints use the dedicated flyout methods.

## internal/presentation/mygo/thumbnail.go

### thumbnailMaxDimension

Maximum thumbnail width or height on the longest edge: 320 pixels.

### thumbnailCapturer interface

Optional capability with CaptureThumbnail(path) error. Keeps screenshot capture out of the general Player contract.

### queueThumbnailPath(mediaPath) (string, error)

Creates a stable cache PNG path under the OS cache directory using SHA-256 of the cleaned, lowercase media path.

### captureQueueThumbnail(mediaPath)

Checks player/window/path and duplicate capture state; obtains the player; starts native screenshot capture in a goroutine under playerCommandMu's read lock; reads and resizes PNG; then returns to UI thread. UI callback checks window lifetime, capture token and playlist membership before decoding a bitmap, replacing old pixels, recording size estimates, and invalidating.

### resizeThumbnailPNG(data, maxDimension) ([]byte, int, int, error)

Decodes PNG. If already within max dimension, returns original bytes. Otherwise proportionally scales with Catmull-Rom and encodes a new PNG. Rejects invalid dimension limits and invalid/empty images.

### playlistContainsPath(mediaPath) bool

Checks the current queue with cleaned, case-insensitive path comparison.

### evictQueueThumbnail(mediaPath)

Invalidates capture token/marker, releases pixels when a bitmap exists, always deletes map entry (including nil-valued entry), and clears raster/encoded size estimates.

### queueCardTitle(path) string

Returns filename without extension, or “Untitled video” when basename is empty/dot.

### queueCardFormat(path) string

Returns uppercase extension plus “ VIDEO”, or “VIDEO” if no extension. The current queue view builds metadata inline.

## internal/presentation/mygo/filmstrip.go

### Filmstrip lifecycle and generation

- filmstripToggleButton(c) and toggleFilmstrip(): show/hide the strip and update the fullscreen controller.
- ensureFilmstripGeneration(): starts cancellable background generation, prefers the frame nearest the preview/playhead position, and sends progressive snapshots through Window.Update.
- resetFilmstripForMedia(), closeFilmstrip(), and abandonFilmstripAfterShutdown(): cancel work, invalidate callback tokens, reset media state, and release/drop bitmap cache state at the appropriate lifecycle point.
- releaseFilmstripBitmaps(): releases decoded bitmap pixels and clears the bounded cache.
- requestVisibleFilmstripBitmaps(frames) and evictFilmstripBitmaps(keep): load visible completed JPEGs and evict images outside the retained set.
- filmstripView(c): renders the timeline row, pending placeholders, progress/error state, selected border, and Previous / Next controls.
- seekToFilmstripPreview(): commits the selected preview position to the player.

### Browsing and selection helpers

- visibleFilmstripFrames(), filmstripWindowStart(...), and filmstripFramesAtStart(...): calculate the current fixed-size window.
- filmstripShiftWindowStart(...) and shiftFilmstripWindow(...): move the visible range by three frames per step.
- filmstripWheelSteps(...): handles mouse-wheel steps and accumulates precise touchpad deltas.
- filmstripFrameIndexAtRatio(...) and filmstripFrameIndexAtX(...): map pointer positions to actual tile indexes while accounting for tile gaps.
- filmstripSelectedIndex(...), filmstripFrameIndexForPosition(...), and nearestFilmstripFrame(...): derive global selection and nearest-frame behavior.
- clampPlaybackPosition(...): bound preview positions to the source duration.

The UI uses stable manifest indexes, so a pending tile does not move when its JPEG finishes. Preview/drag state does not issue MPV seeks continuously; the final action commits one seek.

## internal/presentation/mygo/volume_osd.go

### Constants

volumeWheelDetent is 120 wheel units; volumeStepPercent is 5 percent; volumeOSDDuration is 1.2 seconds; width/height are 180x66 DIPs.

### handleVolumeWheel(delta)

Accumulates wheel input until a complete detent is available, then calls changeVolume.

### volumeOSDTarget() (Rectangle, bool)

Calculates an anchor inside the video viewport or fullscreen area, with a centered fallback for narrow views.

### showVolumeOSD()

Creates/repositions the non-focusable volume flyout, shows it inactive, and sets expiry time.

### hideVolumeOSD()

Hides a visible flyout and clears expiry.

### closeVolumeOSD()

Destroys the flyout and clears its reference/expiry.

### volumeOSDView(c)

Draws volume label, clamped percentage, and progress bar.

## internal/presentation/mygo/lifecycle.go

### Close()

Uses closeOnce. Closes subtitle/resume/volume flyouts; stops memory/controller/playback monitors; destroys memory/fullscreen/controller windows; saves final progress; stops writer; restores cursor; destroys controller; closes MPV; removes Win32 subclasses; destroys the video HWND; and closes platform DLLs. Handles partial initialization.

## internal/presentation/mygo/memory_metrics_windows.go

### processMemorySnapshot

Stable result type containing WorkingSetBytes, PrivateBytes, and PeakWorkingSetBytes.

### processMemoryCountersEx

Mirror of Windows PROCESS_MEMORY_COUNTERS_EX fields, including size, page faults, working set, pool usage, page-file usage, and private usage.

### Lazy DLL/procedure variables

memoryPSAPIDLL and memoryKernelDLL are lazy handles; memoryGetProcess and memoryGetProcessMemoryInfo reference the two needed procedures.

### readProcessMemorySnapshot() (processMemorySnapshot, error)

Loads system DLLs, retrieves the current-process pseudo-handle, calls GetProcessMemoryInfo, and maps counters into processMemorySnapshot. Returns descriptive errors on load/call failure.

## internal/presentation/mygo/memory_metrics_unsupported.go

Defines the same processMemorySnapshot type for !windows. readProcessMemorySnapshot returns an explicit unsupported-platform error.

## internal/presentation/mygo/memory_monitor.go

### memorySampleLimit

Keeps 120 samples, matching a one-second sampling interval and the UI's 120-second trend label.

### memorySample

Stores timestamp/event, process metrics and error, Go heap/stack/Sys/allocation/GC/goroutine counters, thumbnail counts/estimates, playlist estimate, subtitle metadata estimate, and history estimate.

### toggleMemoryMonitor()

Shows the existing window or creates one, resets the sample session, captures initial sample, and starts sampler.

### startMemoryMonitor(window)

Starts a one-second ticker goroutine. Each tick uses Window.Update, checks window identity/lifetime, captures sample on UI thread, and invalidates.

### stopMemoryMonitor()

Closes the stop channel and clears it; ticker goroutine then exits.

### captureMemorySample(event...)

Reads runtime.MemStats and process counters, gathers Aurora metadata estimates, marks gc_observed when NumGC increased, drops oldest sample at limit, and appends new one.

### collectMemoryGC()

Captures before_gc, runs runtime.GC in a goroutine, then records after_gc through Window.Update if the window remains current/alive.

### memoryMonitorView(c)

Builds metric cards, sparklines, runtime rows, Aurora estimates, interpretation notes, and controls for baseline/GC/refresh/export.

### memoryMetricCard(c, label, value, detail)

Draws a single metric card.

### memoryPanel(c, title, children)

Draws a titled diagnostics panel and runs its child builder.

### memoryValueRow(c, label, value)

Draws one label/value pair.

### formatMemoryBytes(value) string

Formats bytes as one-decimal MiB.

### memorySparkline(samples, value) string

Maps a caller-selected sample metric to Unicode bar heights. Empty input returns an em dash; constant data returns a flat series; other data is normalized between min and max.

### exportMemorySamples()

Writes samples into a timestamped CSV under the diagnostics cache folder. Includes process/runtime/thumbnail/playlist/subtitle/history metrics, flushes/closes, deletes a failed partial file, and updates export status.

# 9. Tests

This section names every test function and the rule it protects.

## internal/adapters/history/json_test.go

- TestJSONRepositoryRoundTripAndDelete: Save/Load round-trip and Delete removal.
- TestJSONRepositoryKeepsNewest500Records: caps to 500 and stays within 1 MiB.
- TestJSONRepositoryRejectsUnsupportedVersion: rejects version 99.
- TestJSONFileUsesVersionedEnvelope: confirms saved file has current version field.

## internal/adapters/mpv/version_test.go

- TestSupportsRuntimeVideoOutputReinit: rejects 0.38, accepts tested 0.39+ versions, rejects empty/unknown values. Tests string parsing only, not native option changes.

## internal/adapters/subtitles/json_test.go

- TestJSONRepositoryPersistsPreferencesAndAssignments: preference and external assignment round-trip, then assignment delete.

## internal/application/playback_test.go

- TestSeekTarget: lower/upper clamping, relative target, unknown duration.
- TestClampVolume: below zero, zero, midrange, maximum, above maximum.

## internal/application/playlist_test.go

- TestFilterNewMedia: existing duplicates, accepted path, directory, and empty input.
- TestFilterNewMediaDetectsDuplicateWithinSelection: duplicate files in one selection.
- TestRemovePlaylistEntry: remove before/after/current/final/only item and invalid index.

## internal/application/progress_test.go

- TestCanResumeProgress: valid, too early, complete, beyond duration, unknown duration, missing path.
- TestFindProgressUsesMostRecentMatchingPath: newest LastUpdated wins for duplicate path records.

## internal/application/subtitles_test.go

- TestNormalizeSubtitleLanguage: underscore conversion and empty tag.
- TestSubtitleStyleForLanguage: regional-to-base fallback and global fallback.
- TestMatchSubtitleFilesUsesStrictVideoStem: exact video-stem match and rejects unrelated file.
- TestSubtitleLanguageFromFilename: extracts regional tag and rejects unrelated stem.
- TestIsSubtitleFile: accepted extensions, rejected formats, and directories.

## internal/presentation/mygo/memory_monitor_test.go

- TestFormatMemoryBytes: 2 MiB formats as 2.0 MiB.
- TestMemorySparkline: empty input, correct character count, non-empty output.

## internal/presentation/mygo/thumbnail_test.go

- TestPlaylistContainsPath: case-insensitive Windows path compare.
- TestEvictQueueThumbnailClearsAllTrackedState: deletes thumbnail/marker/token/estimates, including nil bitmap map entry.
- TestResizeThumbnailPNGDownscalesLargeImage: 1920x1080 becomes 320x180 valid PNG.
- TestResizeThumbnailPNGLeavesSmallImageUnchanged: small PNG bytes are unchanged.
- TestResizeThumbnailPNGRejectsInvalidInput: invalid PNG and non-positive target limit rejected.

## internal/adapters/ffmpeg/filmstrip_test.go

- TestSampleIntervalSecondsBoundsFilmstripDensity and TestSampleFrameCountNeverExceedsTarget: keep sampling interval and total frame count bounded.
- TestVideoFilterBoundsOutputDimensions: fixes output geometry at 160×90.
- TestFFmpegFrameArgsUseInputSeekBeforeSource: ensures the fast input-seek argument order.
- TestCacheKeyChangesWhenSourceFingerprintChanges and TestLoadCachedFilmstripValidatesEveryImage: check fingerprint invalidation and manifest/image integrity.
- TestPrioritizedSampleOrderStartsNearPreferredPosition and TestPrioritizedSampleOrderUsesActualTimestampDistance: verify nearest-time-first ordering.
- TestRunBoundedJobsNeverExceedsTwoWorkers: verifies the worker concurrency ceiling.

## internal/adapters/ffmpeg/process_windows_test.go

- TestConfigureFFmpegCommandHidesConsoleWindow: asserts both HideWindow and CREATE_NO_WINDOW are set for Windows FFmpeg processes.

## internal/presentation/mygo/filmstrip_test.go

- TestNearestFilmstripFrame, TestVisibleFilmstripFramesCentersAroundPreview, TestFilmstripWindowStartCentersOnPreview, and TestFilmstripFramesAtStartClampsWindow: validate the visible frame window and nearest-frame logic.
- TestFilmstripSelectedIndexUsesPersistentManifestIndex, TestFilmstripSelectionPersistsAcrossNextWindow, and related selection tests: keep highlighting tied to the global manifest index.
- TestFilmstripFrameIndexAtXUsesActualTileGaps, TestFilmstripShiftWindowStart, and TestFilmstripWheelSteps: cover pointer hit testing and browsing.
- TestProgressiveFilmstripKeepsPendingSlotsInTheirTimelinePositions: pending slots remain stable as generated paths arrive.
- TestCloseFilmstripDiscardsIncompleteProgressiveManifest: closing during generation clears partial paths and allows a clean retry.

Persistence tests use temp directories. Adapter tests use pure helper/process-configuration logic rather than an installed FFmpeg media workload. Native Win32, actual codec behavior, and playback integration still require Windows smoke testing.

# 10. Non-Go tracked files

| Path | Purpose |
|---|---|
| README.md | User-facing overview, features, requirements, and build instructions |
| ARCHITECTURE.md | Short package/layer dependency explanation |
| DESIGN_SYSTEM.md | Palette, layout measurements, visual rules, and design implementation notes |
| go.mod | Module name, Go version, MyGo tool dependency, direct/indirect modules |
| go.sum | Dependency checksums |
| mygo.json | App name, identifier, version, build output, bundled MPV DLL/FFmpeg resources, executable icon |
| makefile | build_windows convenience target |
| .gitattributes | Git LFS rules for MPV DLLs and FFmpeg executable |
| .gitignore | Excludes build/, .mygo/, and generated MyGo Windows resource file |
| LICENSE | MIT license |
| resources/embed.go | Embeds resources/icon.png as resources.IconPNG |
| resources/icon.png | Main window/package icon |
| resources/icon-dev.png | Same tracked image content as icon.png; no separate embed declaration |
| resources/aurora.png | Additional PNG; not referenced by resources/embed.go |
| internal/presentation/mygo/assets/logo.png | Embedded as visible UI logo by brand.go |
| third_party/mpv/README.md | Documents required MPV runtime files |
| third_party/mpv/libmpv-2.dll | Git LFS-managed MPV binary object |
| third_party/mpv/d3dcompiler_43.dll | Git LFS-managed D3D compiler binary object |
| scripts/setup-ffmpeg.ps1 | Downloads and SHA-256-verifies the Windows FFmpeg archive, installs ffmpeg.exe, and preserves license/readme files |
| third_party/ffmpeg/README.md | FFmpeg runtime setup, Git LFS, and licensing notes |
| third_party/ffmpeg/BUILD-INFO.txt | Source release, asset name, archive hash, and build variant provenance |
| third_party/ffmpeg/ffmpeg.exe | Git LFS-managed FFmpeg runtime used for filmstrip extraction |
| third_party/ffmpeg/licenses/ | License and readme materials from the selected FFmpeg build |
