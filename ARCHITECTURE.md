# Architecture

For the complete newcomer walkthrough and the file-by-file reference, see the [Developer Guide](docs/DEVELOPER_GUIDE.md) and [Source Reference](docs/SOURCE_REFERENCE.md).


## Dependency direction

```text
cmd/aurora ─┐
main.go ─────┴─> internal/bootstrap
                         │
                         ├──> adapters/mpv ───────┐
                         ├──> adapters/ffmpeg ────┤
                         ├──> adapters/history ───┤ concrete adapters
                         ├──> platform/windows ───┤
                         └──> presentation/mygo ──┤
                                                  ▼
                                    application ports + domain types
```

The direction is enforced by Go package boundaries:

- `domain` has no framework, operating-system, or playback-backend dependency. It owns neutral playback and progress data types.
- `application` defines the `Player`, `FilmstripGenerator`, `ProgressRepository`, `NativePlatform`, and `PlayerFactory` contracts, plus reusable playlist-validation, playback-clamping, and resume-validation logic.
- `adapters/mpv` implements playback using `purego` and the bundled libmpv DLL.
- `adapters/ffmpeg` generates optional timeline thumbnails in cancellable child processes and stores compressed JPEG frames under the user's cache directory. It is independent of normal playback.
- `adapters/history` implements `ProgressRepository` with a versioned JSON file in the user's configuration directory. It owns atomic file replacement and history-size limits.
- `platform/windows` implements the native child-window and cursor operations using Win32.
- `presentation/mygo` owns MyGo windows, UI composition, event handling, and view state. It depends on interfaces from `application`, not concrete MPV, persistence, or Win32 types.
- `bootstrap` creates the concrete adapters and injects them into the presentation layer.

## Smart Resume

The presentation layer samples the current MPV position and sends progress snapshots through a serialized background writer. The JSON adapter enforces a maximum of 500 records and 1 MiB, evicting the least recently watched entries when necessary. Resume validation and path matching live in `application`; the presentation layer owns the Resume / Start over prompt and decides when to offer it. Automatic queue advancement does not interrupt playback with a resume prompt.

## Subtitle Studio

Subtitle behavior is exposed through the focused `application.SubtitlePlayer` capability instead of expanding the general playback port. Subtitle preferences and per-video assignment models live in `domain`; application logic owns language normalization, style validation, and strict filename matching. `adapters/mpv` maps the capability to libmpv subtitle commands and properties. `adapters/subtitles` stores versioned preferences and assignment JSON separately with bounded, atomic writes. The MyGo presentation owns the Studio controls and synchronization shortcuts.

## Visual timeline filmstrip

The filmstrip is an optional capability exposed through `application.FilmstripGenerator`. The bootstrap resolves the packaged resources directory, creates the FFmpeg adapter, and injects it into the MyGo presentation. The UI does not depend directly on the FFmpeg implementation.

When the filmstrip is opened, the adapter validates the media file and checks its fingerprinted disk cache. On a cache miss, it extracts up to 48 160×90 JPEG frames using at most two FFmpeg processes at a time. Each child is limited to one decoder and one filter thread. Samples nearest the current playhead are processed first.

The presentation receives a full set of stable timeline slots and updates each slot when its image is ready. It decodes only visible thumbnails and retains no more than twelve UI bitmaps. Previous / Next buttons and mouse-wheel navigation browse the strip without seeking the player.

Preview and playback positions are separate. Dragging updates only the preview; releasing commits one precise MPV seek, while cancellation restores the committed position. Generation is cancellable when the media changes, the strip closes, or the application shuts down. A generation token and media-path check reject stale UI callbacks.

The adapter writes the cache manifest only after every frame succeeds, using a temporary file followed by an atomic rename. Failed or canceled generations remove the incomplete cache directory. The disk cache has a best-effort 512 MiB LRU budget. On Windows, FFmpeg child processes use hidden/no-console creation flags so no terminal windows appear during extraction.

FFmpeg is needed only for the filmstrip. If it is unavailable, the filmstrip reports an extraction error; libmpv playback remains available.

## Important boundary: video geometry

MyGo's stage bounds are read in the presentation layer and converted to `domain.Rect`. The Windows adapter receives only that neutral DIP rectangle and performs DPI conversion to physical pixels. It no longer imports `github.com/egoist/mygo/ui`.

## Why the presentation package remains cohesive

The MyGo view functions, fullscreen flyout, UI-thread callbacks, and window lifecycle share the same mutable presentation state and must run through MyGo's UI context. They are separated into focused files but remain in one package to avoid artificial package boundaries and circular imports. Playback validation, persistence, and backend/platform operations have separate packages and interfaces.
