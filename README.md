# Aurora Player 0.1.0

Aurora Player is a Windows 11 native media player built with Go, MyGo's native UI, and an embedded libmpv video surface.

## Project structure

```text
.
├── main.go                         # MyGo native-app entry point
├── cmd/aurora/main.go              # conventional Go entry point
├── internal/
│   ├── bootstrap/                  # composition root; creates concrete adapters
│   ├── domain/                     # media and playback value types
│   ├── application/                # ports and UI-independent use-case helpers
│   ├── adapters/mpv/               # libmpv implementation of application.Player
│   ├── platform/windows/           # Win32 child-window, cursor and DPI adapter
│   └── presentation/mygo/          # MyGo window, views, controls and lifecycle
├── resources/
├── third_party/mpv/                # bundled runtime DLLs go here
├── go.mod
└── mygo.json
```

## Requirements

- Windows 11 x64
- Go 1.27.3 (as specified by `go.mod`)
- MyGo v0.9.2 (pinned by `go.mod`)
- The MPV runtime DLLs used by the original project

Place these files in `third_party/mpv/` before building:

- `libmpv-2.dll`
- `d3dcompiler_43.dll`

The `mygo.json` resources list is already configured to package these two files. The DLLs were not included in the source provided in chat, so they are not present in this ZIP. Use the same copies from your existing project; do not download unrelated or mismatched builds.

## Build

From the project root, in a Windows x64 terminal:

```powershell
go mod tidy
go tool mygo build -platform windows/amd64
```

For a direct Go build of the conventional command package:

```powershell
go build -o build\AuroraPlayer.exe .\cmd\aurora
```

The MyGo build is the recommended packaged-app workflow because it reads `mygo.json` and copies the configured resources.

## Architecture notes

- `internal/application` owns the ports consumed by the presentation layer. The MPV and Win32 adapters satisfy those interfaces without importing MyGo.
- `internal/domain` contains neutral playback and geometry types. The Windows adapter receives `domain.Rect`; it does not import `github.com/egoist/mygo/ui`.
- `internal/presentation/mygo` contains the existing MyGo UI and window/event coordination. It talks to MPV and Win32 through application interfaces.
- `internal/bootstrap` is the only place that wires concrete adapters to the presentation controller.
- The native video host and MPV adapter are Windows-only. Small unsupported-platform files allow the project packages to be type-checked on other operating systems, but they do not provide cross-platform playback.

## Verification status

The source was formatted with `gofmt`. A full Windows build and playback test have not been run in this environment: the available Go toolchain here is older than the project's declared Go 1.27.3, and the two MPV runtime DLLs were not supplied with the pasted source. Run the build command above on your Windows development machine after placing the original DLLs in `third_party/mpv/`.
