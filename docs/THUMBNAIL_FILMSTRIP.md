# Timeline Thumbnail Filmstrip

This document describes the on-demand visual filmstrip used for timeline navigation. It is separate from the queue artwork thumbnails, which are screenshots captured from the active libmpv player.

## User-visible behavior

The filmstrip can be opened from the playback controls. It presents a sliding window of up to nine thumbnails from a filmstrip manifest containing up to 48 time-spaced samples. The nearest frames are shown first during generation, so the area around the current playback position becomes useful before the rest of the strip is complete.

Previous and Next buttons browse in steps of three thumbnails. Mouse-wheel browsing advances or reverses by the same step. Precise touchpad deltas are accumulated until they reach a normal wheel-step threshold. The visible window is pinned while the pointer is over the strip so the tiles do not slide underneath the pointer during hover and drag.

Hovering or dragging updates the preview position, not the committed player position. The MPV player is not repeatedly sought while the pointer moves. Releasing the drag or choosing a frame commits one precise seek; canceling a drag restores the committed playback position. The selected tile is tracked by its global manifest index, so selection survives browsing between windows.

## Generation pipeline

The application depends on the `application.FilmstripGenerator` port. The bootstrap creates `adapters/ffmpeg.FilmstripGenerator` using the packaged resources directory and injects it into the presentation. The presentation starts generation on a background goroutine only after the filmstrip is opened and a valid media duration is available.

The FFmpeg adapter:

1. Resolves the source path and verifies it is a regular file.
2. Builds a cache key from the normalized source path, file size, modification time, duration, manifest format version, and thumbnail dimensions.
3. Returns an existing valid manifest when all referenced image files pass validation.
4. On a cache miss, creates the staging cache directory and computes up to 48 sample timestamps.
5. Sorts sample indexes by the actual time distance from the current preview/playhead position.
6. Runs at most two FFmpeg child processes concurrently. Each child is restricted to one decoder thread and one filter thread.
7. Writes each completed 160×90 JPEG to its stable frame filename and reports a progressive snapshot to the UI.
8. Publishes the manifest only after every frame has succeeded.

FFmpeg uses input seeking before the input file, then decodes the short tail needed to reach the requested frame. The generated image is scaled and padded to a 160×90 canvas. Each encoded image is validated against the configured 256 KiB size ceiling; the manifest is also subject to a size limit.

### Progressive rendering

The initial callback contains the full-length frame list with timestamps but empty paths for unfinished frames. The presentation uses each manifest index as the tile identity. A completed JPEG fills its existing slot rather than causing the other tiles to move. Pending slots show a placeholder while a completed visible image can be decoded immediately.

Every callback returns to the MyGo UI thread through `Window.Update`. The callback checks the window, generation token, and media path before modifying presentation state. This prevents a late result from a previous video from replacing the filmstrip for the current video.

## Scrubbing and navigation rules

The filmstrip maintains a preview position separately from the committed `PlaybackState.Position`.

- Hovering selects a frame and updates the visual preview only.
- Dragging updates the preview without sending repeated seek commands to MPV.
- Releasing commits one exact seek.
- Canceling the drag restores the committed playback position and does not issue a seek.
- Previous / Next and mouse-wheel browsing move the visible thumbnail window; browsing itself does not seek playback.
- The highlighted selection is stored as a global manifest index rather than a local tile index.

These rules keep navigation responsive and avoid starting decoder seeks for every pointer movement.

## Persistent cache

On Windows, the filmstrip cache root is resolved beneath the user's OS cache directory:

```text
%LOCALAPPDATA%/Aurora Player/filmstrip/
└── <source-fingerprint>/
    ├── frame_000.jpg
    ├── frame_001.jpg
    ├── ...
    └── manifest.json
```

The source fingerprint includes the normalized absolute media path, file size, modification time in nanoseconds, duration, manifest version, and output width/height. A changed source fingerprint or thumbnail format yields a new cache directory.

The manifest stores the format version, source fingerprint fields, sample interval, output dimensions, and each JPEG's filename, timestamp, and encoded byte size. Cache loading rejects unknown versions, unsafe/non-basename frame names, missing or non-regular image files, and unexpected image sizes.

A manifest is written to a temporary file and renamed to `manifest.json` only after every frame has completed. A failed or canceled generation removes its incomplete staging directory. This ensures a partial set is never accepted as a complete cache on the next open.

The adapter trims the total cache to a best-effort 512 MiB least-recently-used budget. Cache eviction is best-effort and should not be treated as a hard storage quota.

## Bitmap and process lifetime

Only visible completed thumbnails are decoded. The presentation keeps at most twelve decoded UI bitmaps, even though the strip contains up to 48 compressed files. When a bitmap is evicted or the strip is closed, the UI releases its CPU pixel buffer and removes the app-owned reference.

Generation is canceled when the filmstrip closes, the media item changes, or the app shuts down. Generation tokens and media-path checks reject stale callbacks. If a user closes the filmstrip while extraction is in progress, the partial manifest is discarded because cancellation removes the staging directory; reopening starts a fresh cache check/generation rather than reusing stale file paths.

On Windows, FFmpeg children are started with both `HideWindow` and `CREATE_NO_WINDOW` so a GUI launch does not create visible console windows. On other platforms, the process configuration helper is a no-op.

## FFmpeg runtime and build

`mygo.json` packages `third_party/ffmpeg/ffmpeg.exe` as a resource. The adapter searches the packaged resource directory, beside the application executable, development `third_party` locations, and PATH. The executable is tracked with Git LFS.

A normal source checkout should run:

```powershell
git lfs install
git lfs pull
```

To install or refresh the Windows x64 build with checksum verification, run the PowerShell helper:

```powershell
.\scripts\setup-ffmpeg.ps1
# Add -Force to replace an existing executable.
```

The script downloads the selected BtbN Windows x64 build, verifies its archive against the release `checksums.sha256`, checks that `ffmpeg.exe -version` succeeds, and copies available license/readme files into the runtime directory. The build's source release and verified archive hash are recorded in `BUILD-INFO.txt`.

FFmpeg is used only for timeline thumbnails. A missing or unusable executable is surfaced when the user opens the filmstrip; normal libmpv playback remains available. The LGPL label in an archive name is not, by itself, a legal compliance determination. Review the exact build configuration and satisfy all applicable notice, source, and relinking requirements before redistributing the executable.

## Key implementation files

| File | Responsibility |
|---|---|
| `internal/domain/filmstrip.go` | Framework-neutral filmstrip and frame metadata |
| `internal/application/ports.go` | `FilmstripGenerator` application interface |
| `internal/adapters/ffmpeg/filmstrip.go` | Sampling, FFmpeg arguments, bounded jobs, cache validation, manifest publication, LRU trimming |
| `internal/adapters/ffmpeg/process_windows.go` | Hidden/no-console process creation on Windows |
| `internal/adapters/ffmpeg/process_other.go` | Non-Windows process configuration stub |
| `internal/presentation/mygo/filmstrip.go` | UI lifecycle, progressive updates, preview/seek behavior, browsing, and bitmap cache |
| `internal/bootstrap/run.go` | Creates and injects the FFmpeg adapter |
| `mygo.json`, `.gitattributes` | Packages and tracks the FFmpeg executable |
| `scripts/setup-ffmpeg.ps1` | Verifies and installs the Windows FFmpeg build |

## Tests

- `internal/adapters/ffmpeg/filmstrip_test.go` covers sample spacing/count limits, filter output, input-seek argument order, cache fingerprints and validation, timestamp priority, and the two-worker limit.
- `internal/adapters/ffmpeg/process_windows_test.go` verifies hidden/no-console Windows process flags.
- `internal/presentation/mygo/filmstrip_test.go` covers visible-window behavior, stable selection, tile hit testing, wheel browsing, progressive pending slots, and canceled-generation cleanup.
- `internal/presentation/mygo/timeline_test.go` covers preview/commit/cancel behavior and the single-seek contract.

Unit tests do not replace Windows smoke testing with real media files. Test short and long videos, different codecs/container formats, cache reuse, media switching during generation, and app shutdown while generation is active.
