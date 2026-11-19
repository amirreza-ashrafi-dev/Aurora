# Architecture

## Dependency direction

```text
cmd/aurora ─┐
main.go ─────┴─> internal/bootstrap
                         │
                         ├──> adapters/mpv ───────┐
                         ├──> adapters/history ───┤ concrete adapters
                         ├──> platform/windows ───┤
                         └──> presentation/mygo ──┤
                                                  ▼
                                    application ports + domain types
```

The direction is enforced by Go package boundaries:

- `domain` has no framework, operating-system, or playback-backend dependency. It owns neutral playback and progress data types.
- `application` defines the `Player`, `ProgressRepository`, `NativePlatform`, and `PlayerFactory` contracts, plus reusable playlist-validation, playback-clamping, and resume-validation logic.
- `adapters/mpv` implements playback using `purego` and the bundled libmpv DLL.
- `adapters/history` implements `ProgressRepository` with a versioned JSON file in the user's configuration directory. It owns atomic file replacement and history-size limits.
- `platform/windows` implements the native child-window and cursor operations using Win32.
- `presentation/mygo` owns MyGo windows, UI composition, event handling, and view state. It depends on interfaces from `application`, not concrete MPV, persistence, or Win32 types.
- `bootstrap` creates the concrete adapters and injects them into the presentation layer.

## Smart Resume

The presentation layer samples the current MPV position and sends progress snapshots through a serialized background writer. The JSON adapter enforces a maximum of 500 records and 1 MiB, evicting the least recently watched entries when necessary. Resume validation and path matching live in `application`; the presentation layer owns the Resume / Start over prompt and decides when to offer it. Automatic queue advancement does not interrupt playback with a resume prompt.

## Important boundary: video geometry

MyGo's stage bounds are read in the presentation layer and converted to `domain.Rect`. The Windows adapter receives only that neutral DIP rectangle and performs DPI conversion to physical pixels. It no longer imports `github.com/egoist/mygo/ui`.

## Why the presentation package remains cohesive

The MyGo view functions, fullscreen flyout, UI-thread callbacks, and window lifecycle share the same mutable presentation state and must run through MyGo's UI context. They are separated into focused files but remain in one package to avoid artificial package boundaries and circular imports. Playback validation, persistence, and backend/platform operations have separate packages and interfaces.
