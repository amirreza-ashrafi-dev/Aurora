# Architecture

## Dependency direction

```text
cmd/aurora ─┐
main.go ─────┴─> internal/bootstrap
                         │
                         ├──> adapters/mpv ───────┐
                         ├──> platform/windows ───┤ concrete adapters
                         └──> presentation/mygo ──┤
                                                  ▼
                                    application ports + domain types
```

The direction is enforced by Go package boundaries:

- `domain` has no framework, operating-system, or playback-backend dependency.
- `application` defines the `Player`, `NativePlatform`, and `PlayerFactory` contracts, plus reusable playlist-validation and playback-clamping logic.
- `adapters/mpv` implements playback using `purego` and the bundled libmpv DLL.
- `platform/windows` implements the native child-window and cursor operations using Win32.
- `presentation/mygo` owns MyGo windows, UI composition, event handling, and view state. It depends on interfaces from `application`, not concrete MPV or Win32 types.
- `bootstrap` creates the concrete adapters and injects them into the presentation layer.

## Important boundary: video geometry

MyGo's stage bounds are read in the presentation layer and converted to `domain.Rect`. The Windows adapter receives only that neutral DIP rectangle and performs DPI conversion to physical pixels. It no longer imports `github.com/egoist/mygo/ui`.

## Why the presentation package remains cohesive

The MyGo view functions, fullscreen flyout, UI-thread callbacks, and window lifecycle share the same mutable presentation state and must run through MyGo's UI context. They are separated into focused files but remain in one package to avoid artificial package boundaries and circular imports. Playback validation and backend/platform operations have separate packages and interfaces.
