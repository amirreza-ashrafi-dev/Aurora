# Aurora Player — Developer Guide

This is the onboarding guide for developers new to Aurora. It explains the application's purpose, package structure, data flow, build and test process, threading rules, and where to make safe changes. For the exhaustive declaration-by-declaration catalogue, see [Source Reference](SOURCE_REFERENCE.md). The shorter [Architecture](../ARCHITECTURE.md) and [Design System](../DESIGN_SYSTEM.md) documents remain useful companion references.

## 1. What Aurora is

Aurora Player is a Windows desktop video player written in Go. MyGo provides the desktop windows and UI, libmpv performs playback, and a Win32 adapter creates the native child window that displays video inside the MyGo main window.

The application has a local playlist, normal and fullscreen playback controls, a windowed Cinematic Mode, playback history with Smart Resume, external subtitle tools, queue thumbnails, and a memory diagnostics window.

There is no HTTP server, REST endpoint, remote service, cloud sync, or database in the current source tree. Persistent data is stored in local JSON files. “API” in this project normally means a Go interface between packages, the libmpv C API, or a Win32 function—not a web endpoint.

Playback is Windows-only. Non-Windows files are compile-time stubs so package builds and logic tests can run elsewhere; they do not provide an alternative video engine.

## 2. Technology and requirements

| Technology | Purpose |
|---|---|
| Go 1.27.3 | Main language/runtime as declared in go.mod |
| MyGo v0.9.2 | Desktop windows, UI layout, event handling, and dialogs |
| libmpv | Playback, playlist commands, media properties, screenshots, subtitles |
| purego v0.11.1 | Binding native DLL functions to Go functions |
| golang.org/x/sys/windows | Windows DLL loading, handles, strings, and system helpers |
| Win32 | Child video window, geometry, DPI, cursor, drag-and-drop, input messages |
| encoding/json | Local versioned persistence |
| golang.org/x/image/draw | Thumbnail resizing |

Aurora targets 64-bit Windows 11. The MPV adapter requires two runtime DLLs:

- third_party/mpv/libmpv-2.dll
- third_party/mpv/d3dcompiler_43.dll

Both are tracked by Git LFS. Without downloading LFS objects, a checkout can contain small pointer files rather than runnable DLLs. These files are runtime dependencies, not Go modules, so go mod download does not fetch them.

## 3. Clone, test, build

~~~powershell
git clone https://github.com/amirreza-ashrafi-dev/Aurora.git
cd Aurora
git lfs pull
go mod download
go test ./...
go tool mygo build -platform windows/amd64
~~~

The makefile also provides make build_windows, which runs the same MyGo Windows amd64 build. Output and packaged resources are configured in mygo.json.

The unit tests cover application rules, JSON persistence, MPV version parsing, thumbnail processing, and memory-formatting helpers. They do not instantiate a real Win32 video host or exercise an installed libmpv DLL. Always test actual playback on Windows with the real LFS runtime files after changing the native adapter, lifecycle, playlist, or graphics behavior.

Both main.go and cmd/aurora/main.go are small wrappers around bootstrap.Run. Keep application logic out of these entrypoints.

## 4. Project layout

~~~text
.
├── main.go
├── cmd/aurora/main.go
├── go.mod / go.sum
├── mygo.json / makefile
├── README.md / ARCHITECTURE.md / DESIGN_SYSTEM.md
├── docs/
│   ├── DEVELOPER_GUIDE.md
│   └── SOURCE_REFERENCE.md
├── internal/
│   ├── domain/
│   ├── application/
│   ├── adapters/
│   │   ├── mpv/
│   │   ├── history/
│   │   └── subtitles/
│   ├── platform/windows/
│   ├── presentation/mygo/
│   │   └── assets/
│   └── bootstrap/
├── resources/
└── third_party/mpv/
~~~

| Package | Responsibility |
|---|---|
| internal/domain | UI- and backend-neutral data types |
| internal/application | Required interfaces and reusable business rules |
| internal/adapters/mpv | libmpv playback and subtitle implementation |
| internal/adapters/history | Playback history JSON repository |
| internal/adapters/subtitles | Subtitle preferences and assignment JSON repository |
| internal/platform/windows | Native Win32 child window and input operations |
| internal/presentation/mygo | Windows, UI, event handling, and presentation state |
| internal/bootstrap | Creates real implementations and injects them |
| resources | Embedded application icon |
| third_party/mpv | Native MPV and D3D compiler runtime files |

The MyGo views are split into feature-oriented files but stay in one Go package. They share App state and the MyGo UI lifecycle. Splitting them into many packages would create artificial coupling and risk circular imports.

## 5. Architecture

The dependency direction is:

~~~mermaid
flowchart TD
    Entry["main.go / cmd/aurora"] --> Boot["internal/bootstrap"]
    Boot --> Presentation["presentation/mygo"]
    Boot --> MPV["adapters/mpv"]
    Boot --> History["adapters/history"]
    Boot --> Subs["adapters/subtitles"]
    Boot --> Win["platform/windows"]
    Presentation --> AppPorts["application contracts and rules"]
    MPV --> AppPorts
    History --> AppPorts
    Subs --> AppPorts
    Win --> AppPorts
    AppPorts --> Domain["domain data types"]
~~~

The application package defines what the presentation needs; adapters implement those contracts. The UI knows about Player, ProgressRepository, NativePlatform, and subtitle capability interfaces. It does not need to know libmpv's C signatures, JSON file layout, or Win32 function pointers.

Key contracts in internal/application/ports.go:

- Player: load/append/play/remove playlist entries, read properties, pause, seek, volume, stop, and close.
- ProgressRepository: load/save/delete playback progress.
- NativePlatform: create/layout/show/destroy the native video child and install/remove its input callbacks.
- PlayerFactory: create a Player after the native video HWND exists.
- SubtitlePlayer: optional subtitle operations kept separate from the general playback contract.
- SubtitlePreferencesRepository and SubtitleAssignmentRepository: read and write subtitle settings.

## 6. Startup and shutdown

### Startup

1. The main package calls bootstrap.Run.
2. bootstrap creates the Windows Platform.
3. It creates a lazy PlayerFactory. The factory finds the packaged MPV DLL path and calls mpv.New with a video HWND when requested.
4. It creates the history and subtitle JSON repositories under the user's configuration directory.
5. It constructs presentation.App with the platform, player factory, and repositories.
6. App.Run starts MyGo and registers onReady.
7. onReady creates the MyGo main window and hidden STATIC Win32 child window, then installs double-click, file-drop, and mouse-wheel handlers.
8. History, subtitle assignments, and subtitle preferences are loaded.
9. libmpv is deliberately not initialized yet. It is created by ensurePlayer when the first valid video is added.

~~~mermaid
sequenceDiagram
    participant Entry
    participant Bootstrap
    participant App as presentation.App
    participant Win as Windows Platform
    participant MPV as libmpv adapter
    Entry->>Bootstrap: Run()
    Bootstrap->>Win: New()
    Bootstrap->>App: New(platform, factory, repositories)
    Bootstrap->>App: Run()
    App->>App: onReady()
    App->>Win: CreateVideoHost(main HWND)
    App->>Win: Install native input handlers
    App->>App: Load local history and subtitle settings
    Note over App,MPV: MPV is still unloaded
    App->>MPV: ensurePlayer() on first valid video
    MPV-->>App: Player attached to child HWND
~~~

Lazy initialization means an empty Aurora window does not load MPV's decoder and video renderer.

### Shutdown

bootstrap defers App.Close. Close uses sync.Once and releases resources in dependency order: close flyouts, stop memory/controller/playback monitors, save final progress, stop the progress worker, restore cursor visibility, destroy overlay windows, close MPV, remove Win32 subclass callbacks, destroy the video HWND, and release platform DLL handles.

closePlayer uses playerCommandMu so it cannot unload libmpv while a background thumbnail call is still using the native context.

## 7. Data and local files

All persistent data is local. The exact root is provided by the operating system for the current user.

| Data | Relative path | Purpose |
|---|---|---|
| Playback history | Config/Aurora Player/playback_history.json | Recent position and duration by media path |
| Subtitle styles | Config/Aurora Player/subtitle_preferences.json | Global and language-specific appearance |
| Subtitle assignments | Config/Aurora Player/subtitle_assignments.json | External subtitle path/language/visibility per video |
| Queue thumbnails | Cache/Aurora Player/thumbnails/ | PNG cache keyed by a SHA-256 of the cleaned media path |
| Memory samples | Cache/Aurora Player/diagnostics/ | Timestamped CSV export |

History is limited to 500 entries and 1 MiB. Subtitle assignments are limited to 500 entries; both subtitle JSON files have a 1 MiB file limit. JSON writes use a temporary file in the destination directory and rename it after the new file is written and flushed. Media files are not copied into Aurora's config directory; the app persists their paths.

## 8. Feature workflows

### Adding videos

The Add videos button opens a MyGo multi-select dialog. Its callback returns to the UI thread with Window.Update, then calls addFiles. application.FilterNewMedia trims candidate paths, resolves absolute paths and symlinks where possible, rejects missing/non-regular files, and detects duplicate paths and hard links with os.SameFile.

If any paths are accepted, addFiles calls ensurePlayer. On an empty queue, the app reveals the native video child and loads the first item with Player.Load; remaining items are appended. For an existing queue, every accepted item is appended. The Go playlist is updated only for backend operations that succeed.

The application playlist slice and MPV's native playlist are two views of the same ordered list. A queue change must keep both indexes aligned.

### Removing a queue item

The row's close button records a path for deferred removal and requests a follow-up UI frame. The app applies pending removals at the start of the next view, before MyGo builds the virtual list. This avoids modifying the playlist slice while the list builder is still reading it.

removePlaylistIndex asks MPV to remove the item, then uses application.RemovePlaylistEntry to make a new Go slice and calculate the new selected index. Removing an item before the current one adjusts the index but leaves playback alone. Removing the current item updates the current path and state. Removing the final queued item stops polling, hides the child window, and unloads media.

Do not call removePlaylistIndex directly from a row's Clicked handler. Keep the deferred path-based flow.

### Playback polling

startPolling runs a goroutine with a 250 ms ticker. It reads MPV's time-pos, duration, playlist-pos, and video height when available. Results are applied with Window.Update. If playlist-pos changes, the app records the outgoing progress and updates its selected queue item/path to match MPV's automatic advancement.

While playing, a progress snapshot is considered every five seconds and persisted at most every 30 seconds. The same polling callback can request a thumbnail after the video has played for at least 1.5 seconds and checks the active subtitle language roughly every two seconds. Polling stops when the queue becomes empty and during shutdown. startPolling is idempotent.

### Smart Resume

History records are indexed by application.ProgressKey, which normalizes the media path and applies case-insensitive comparison on Windows. CanResumeProgress rejects missing paths, invalid numeric values, positions below ten seconds, positions beyond duration, negative durations, and positions within 30 seconds of the end. Unknown duration is allowed.

The presentation layer also checks that the media file still exists. If the saved record is usable, it pauses playback and offers Resume or Start over. In normal mode the prompt is part of the main layout; fullscreen and Cinematic Mode use a floating banner.

A single background writer serializes Save/Delete operations. Pause and seek writes are debounced by two seconds, while stop, track changes, and shutdown can request immediate persistence. The one-slot request channel coalesces updates so stale positions do not build up in a queue. Completed media has its saved record removed.

### Subtitles

Supported external formats are SRT, ASS, SSA, VTT, SUB, and IDX. Subtitle discovery searches only the video's own directory. Matching is strict: a subtitle filename must have the same video stem, optionally followed by a period, underscore, or space and a suffix. The app does not do fuzzy matching or recursive folder searches.

When a video starts, restoreSubtitleForCurrent first restores subtitle visibility. If there is no saved assignment, it discovers matching files. A single match is loaded automatically; multiple matches are presented in Subtitle Studio. If an assignment exists, the external file must still exist before it is restored.

Language tags are normalized for preference lookup. A full regional tag is tried first, followed by the base language; otherwise the global style applies. The application validates style values before passing them to MPV.

Current limit: the presentation persists external subtitle paths, language, and disabled/visible state. SubtitleAssignment also has TrackID fields for future embedded-track persistence, but the current UI does not populate or restore those fields. “Next track” cycles MPV's selected sid.

### Native video host and geometry

The Windows platform creates a Win32 STATIC child HWND under MyGo's native main HWND. MPV receives that child handle through its wid option. MyGo stage geometry is measured in DIPs. The presentation converts it to domain.Rect; the platform converts DIPs to physical pixels using window DPI / 96 and rounds before SetWindowPos.

Fullscreen uses the entire parent client rectangle. Normal mode uses the committed video-stage bounds. The conversion belongs at the presentation/platform boundary; the Windows adapter must not import MyGo UI types.

The child receives subclass callbacks for double-click, mouse wheel, and shell file drop. The Platform retains callback addresses until RemoveVideoSubclass is called. Remove them before destroying the child HWND or releasing DLL handles.

### Fullscreen and Cinematic Mode

Fullscreen is MyGo's native fullscreen state. Cinematic Mode is Aurora's custom windowed layout, which hides the normal title bar, queue, and footer while expanding the video surface.

Both share a separate floating controller so controls can appear above the native MPV child. A fullscreen header and Smart Resume banner are also separate flyouts. A controller monitor checks pointer position every 50 ms, shows overlays on movement or hover, and hides them after three seconds of inactivity. Native ShowCursor changes are balanced by recording exactly how many decrements Aurora performed.

### Thumbnails

After at least 1.5 seconds of playback, the app requests a video screenshot through the MPV adapter. The screenshot is written to a stable cache file derived from the media path, resized so the longest edge is no more than 320 pixels, and decoded into a MyGo bitmap on the UI thread.

Capture tokens prevent old asynchronous results from restoring thumbnails for removed items. A read lock keeps the player alive during the native screenshot call. When a queue item is removed and no matching path remains, the app invalidates the token, releases bitmap pixels, and deletes the tracked map entries. The PNG on disk is only a cache and can be regenerated.

### Memory diagnostics

Ctrl+Shift+M opens a memory window. It samples every second and retains 120 samples. It displays process working set/private bytes, Go runtime counters, approximate thumbnail and metadata sizes, and a trend line. It can mark a baseline, request runtime.GC, refresh, and export a CSV.

Windows metrics use GetCurrentProcess and GetProcessMemoryInfo. Non-Windows builds return an explanatory unsupported error. The thumbnail, playlist, subtitle, and history figures are estimates and overlap with Go heap values; do not add them to HeapAlloc as independent totals. Current code does not separate MPV-only or GPU-driver memory.

## 9. Threading and resource-lifecycle rules

| Background activity | Work outside UI thread | UI-thread handoff |
|---|---|---|
| Playback polling | Reads MPV properties on a timer | Window.Update applies results |
| Progress writer | Serializes repository writes/deletes | No UI mutation is needed |
| File dialogs | Waits for user selection | Window.Update applies paths |
| Thumbnail capture | Calls MPV screenshot and resizes PNG | Window.Update checks token/queue, creates bitmap |
| Controller monitor | Samples pointer and timers | RunOnMain updates MyGo windows/cursor |
| Memory sampler | Ticks every second | Window.Update records/displays sample |
| Explicit GC | Calls runtime.GC | Window.Update records after-GC sample |

Follow these rules when changing the code:

- Make MyGo UI-state changes on the MyGo main thread. Use Window.Update or RunOnMain from background callbacks.
- Stop timer goroutines before destroying the windows or native objects they touch.
- Do not terminate/unload MPV while a background native command can be in flight. playerCommandMu protects player lifetime.
- That lock does not make every App field safe for arbitrary goroutine access. Keep presentation state on the UI thread unless a field has explicit synchronization.
- Remove Win32 subclass callbacks before destroying the HWND or releasing the DLL that owns them.
- Release bitmap pixels and drop all app-owned map references when a queue thumbnail is evicted.
- Treat the Go playlist and MPV playlist as one logical state change.

## 10. Where to change things

| Task | Start here | Also inspect |
|---|---|---|
| Add a playback command | application/ports.go, adapters/mpv/player_windows.go | playback.go and polling.go |
| Add subtitle capability | application/subtitles.go | domain/subtitles.go, adapters/mpv, adapters/subtitles, subtitle_ui.go |
| Change playlist validation | application/playlist.go | presentation/playlist.go and playlist tests |
| Change queue UI | main_view.go | playlist.go, thumbnail.go, theme.go |
| Change resume rules | application/progress.go | presentation/progress.go, history/json.go and tests |
| Change history persistence | adapters/history/json.go | domain/playback.go and persistence tests |
| Change subtitle matching/language | application/subtitles.go | subtitle_ui.go and subtitle tests |
| Change subtitle JSON schema | adapters/subtitles/json.go | domain/subtitles.go and compatibility/version handling |
| Change native video host | platform/windows/platform_windows.go | application/ports.go, video_layout.go, app.go |
| Change MPV options/native calls | adapters/mpv/player_windows.go | HWND lifecycle and player teardown |
| Change fullscreen/controller | fullscreen.go and fullscreen_view.go | playback.go, cursor.go, volume_osd.go |
| Change visual tokens/buttons | theme.go and DESIGN_SYSTEM.md | main_view.go and flyout views |
| Change thumbnail cache/size | thumbnail.go | polling.go, playlist.go and thumbnail tests |
| Change memory metrics | memory_monitor.go and memory_metrics_windows.go | app.go, lifecycle.go, memory tests |
| Change packaging/resources | mygo.json, go.mod, makefile | .gitattributes and third_party/mpv |

### Add a player operation

1. Decide whether it belongs on the common Player interface or a focused capability such as SubtitlePlayer.
2. Add it to the application interface.
3. Implement it in player_windows.go.
4. Keep player_unsupported.go in sync so non-Windows packages compile.
5. Call it from presentation and report errors in status/logs.
6. Test pure rules in Go tests; test native behavior on Windows with the real DLL.

### Add a persisted field

1. Add the neutral field to the appropriate domain type.
2. Decide whether the JSON envelope version must change.
3. Update adapter load/save validation and bounds.
4. Update presentation code that reads/writes the field.
5. Add round-trip, invalid-data, missing-file, and version tests.
6. Preserve same-directory temporary-file writes and replacement; do not truncate the previous user file before the new one is complete.

### Add a UI action

Keep the view declarative. Read App state and call a focused App method from Clicked. If a dialog or background operation is needed, return to the MyGo thread before changing UI state. If a virtual list is modified, defer the mutation until the next view build.

## 11. Configuration and non-Go files

| Path | Purpose |
|---|---|
| go.mod | Module path, Go version, MyGo tool dependency, direct/indirect Go modules |
| go.sum | Dependency checksums |
| mygo.json | App name/identifier/version, build output, bundled DLL resources, executable icon |
| makefile | build_windows convenience target |
| .gitattributes | Git LFS rules for the MPV DLLs |
| .gitignore | Excludes build/, .mygo/, generated Windows resource object |
| LICENSE | MIT license |
| resources/embed.go | Embeds resources/icon.png as resources.IconPNG |
| resources/icon.png | Window icon set at startup and package icon in mygo.json |
| resources/icon-dev.png | Same tracked image content as icon.png; no separate runtime loader |
| resources/aurora.png | Additional tracked PNG; not embedded by resources/embed.go |
| internal/presentation/mygo/assets/logo.png | Embedded in brand.go for visible Aurora branding |
| third_party/mpv/README.md | Describes required native runtime files |
| third_party/mpv/*.dll | Git LFS-managed MPV and D3D compiler binaries |

icon.png, icon-dev.png, and the presentation logo currently have the same Git blob. resources/aurora.png is a separate tracked image. The Go embed declaration references icon.png, not aurora.png.

## 12. Known constraints

- The actual native video host and libmpv implementation are Windows-only.
- A normal checkout must fetch Git LFS objects before the bundled MPV DLLs can be used.
- There is no HTTP API, server, database, account system, or cloud sync.
- Playback history and subtitle settings use local JSON; schema changes need explicit compatibility handling.
- Subtitle presentation persists external paths, language, and disabled state, not embedded-track selection.
- ASS/SSA and image-based subtitle formats may not honor all appearance options.
- Memory diagnostics reports whole-process counters and Go runtime counters; it cannot identify MPV-only or GPU-driver-only memory.
- Runtime video-output release is selected using the reported mpv-version. The version parser test does not prove native reconfiguration behavior for every MPV build; the adapter has a full-context teardown fallback for older builds.
- DESIGN_SYSTEM.md includes estimates and reference measurements and marks which values have not been verified as Figma variables.

## 13. Test map

| Test file | Coverage |
|---|---|
| internal/adapters/history/json_test.go | Round-trip/delete, 500-record cap, unsupported version, versioned envelope |
| internal/adapters/mpv/version_test.go | Runtime video-output version parsing |
| internal/adapters/subtitles/json_test.go | Preference and assignment persistence/deletion |
| internal/application/playback_test.go | Seek and volume clamping |
| internal/application/playlist_test.go | File validation, duplicate detection, queue-index behavior |
| internal/application/progress_test.go | Resume eligibility and newest matching record |
| internal/application/subtitles_test.go | Language normalization/profile fallback, strict matching, language extraction, extensions |
| internal/presentation/mygo/memory_monitor_test.go | MiB formatting and sparkline output |
| internal/presentation/mygo/thumbnail_test.go | Case-insensitive path matching, eviction, PNG resizing, invalid image input |

The tests capture business rules and edge cases, but native playback, Win32 input, actual DLL loading, and GPU resource behavior still need Windows integration testing.

## 14. Further reading

- [Source Reference](SOURCE_REFERENCE.md) — file-by-file and declaration-by-declaration catalogue.
- [Architecture](../ARCHITECTURE.md) — compact dependency-boundary explanation.
- [Design System](../DESIGN_SYSTEM.md) — visual tokens, dimensions, and UI rules.
- [README](../README.md) — user-facing feature overview and build information.
