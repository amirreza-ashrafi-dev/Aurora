# macOS libmpv embedding spike

## Goal

Keep libmpv as Aurora's media engine on Windows and macOS. On macOS, render through MPV's Vulkan `macvk` context (which presents through a Metal surface) and embed that rendering inside Aurora's MyGo window. OpenGL is not an acceptable production dependency.

This is a technical spike, not a commitment to ship an unsupported integration. The spike must prove the complete path before the application architecture is changed.

## Current Aurora baseline

The spike starts from `feature/thumbnail-filmstrip` without changing that branch.

- The application-facing `application.Player` interface is already independent of the concrete MPV implementation.
- The current `PlayerFactory` accepts a `uintptr` named `videoHWND`, and `NativePlatform` is Win32-specific: it creates an HWND child, handles Win32 input and lays out that child.
- `internal/adapters/mpv/player_windows.go` loads `libmpv-2.dll`, passes the HWND through MPV's `wid` option and configures `vo=gpu` with `gpu-api=d3d11`.
- `internal/bootstrap/run.go` directly constructs the Windows platform and player.
- The app currently pins MyGo `v0.9.2`.
- FFmpeg filmstrip extraction is a separate adapter and should remain separate from the playback backend.

This means macOS needs a native-view host and a Darwin MPV adapter, as well as a later platform-neutral refactor of the HWND-specific presentation port. Do not refactor the Windows implementation as part of the first renderer experiment.

## Findings that shape this spike

### MyGo can host a native macOS view

MyGo's `ui.NativeView` accepts a native module value that implements `native.View`. On macOS the view is an `NSView`; MyGo places it in the UI layout, sizes it to the element, clips it and draws subsequent UI content above it. MyGo native modules can implement the view in Swift.

References:

- MyGo v0.9.2 native-module documentation: https://github.com/egoist/mygo/blob/v0.9.2/docs/native-modules.md
- MyGo v0.9.2 native view example: https://github.com/egoist/mygo/tree/v0.9.2/examples/native-module/modules/places
- MyGo UI NativeView API: https://pkg.go.dev/github.com/egoist/mygo@v0.9.2/ui

### MPV 0.42 has a Metal-backed macOS renderer, but it is not currently an embedded-view API

MPV 0.42 documents `gpu-context=macvk` as Vulkan on macOS using a Metal surface through a translation layer. The manual labels it experimental. The release also adds a D3D11 backend to the libmpv render API, but that is not a Metal/Vulkan render API.

Important source-level details from MPV v0.42.0:

- `video/out/vulkan/context_mac.m` creates a Vulkan Metal surface using `p->vo_mac.layer`.
- `video/out/mac/metal_layer.swift` defines MPV's `CAMetalLayer` subclass.
- `video/out/mac/common.swift` creates MPV's own view and calls `initWindow`.
- `video/out/mac/window.swift` creates an MPV-owned `NSWindow` and adds MPV's view to that window.
- The documented `wid` option lists X11, Win32 and Android native handles; it does not document an `NSView` embedding target for macOS.

References:

- MPV 0.42.0 manual: https://mpv.io/manual/stable/
- MPV 0.42.0 release: https://github.com/mpv-player/mpv/releases/tag/v0.42.0
- macOS Vulkan context: https://github.com/mpv-player/mpv/blob/v0.42.0/video/out/vulkan/context_mac.m
- macOS Metal layer: https://github.com/mpv-player/mpv/blob/v0.42.0/video/out/mac/metal_layer.swift
- macOS view/window setup: https://github.com/mpv-player/mpv/tree/v0.42.0/video/out/mac

**Conclusion:** Passing MyGo's `NSView` to the existing `wid` option is not a supported solution. MPV's `macvk` renderer currently creates and manages its own window/view. The spike must prove how to reuse MPV's Metal renderer inside an Aurora-owned view.

## Proposed experiment

### POC 1 — Prove the MyGo host view

Create a minimal MyGo native module in Swift that returns a plain `NSView` suitable as an Aurora video host. Show it with `ui.NativeView` in a small test window, with Go-drawn controls above it.

Verify on an actual macOS machine:

- the view is embedded in the intended rectangle;
- resize and Retina/backing-scale changes are correct;
- clipping, hiding/showing and overlay controls work;
- the view and its native resources are released on close.

This validates the MyGo side only. It does not validate MPV embedding.

### POC 2 — Reuse MPV's existing macvk Metal renderer

Build a pinned MPV 0.42.0 macOS library with `gpu-next` and `macvk`. First verify standalone playback with an explicit no-OpenGL configuration:

```sh
mpv --vo=gpu-next --gpu-api=vulkan --gpu-context=macvk --hwdec=videotoolbox sample.mkv
```

Then investigate a small, isolated MPV patch that lets the macOS `macvk` output use a host `NSView` instead of creating its own top-level `NSWindow`. Keep MPV's own Metal layer and rendering pipeline; do not implement a second video renderer unless this experiment proves it is unavoidable.

The patch must also define how the hosted view gets its size, display, screen scale, visibility, focus and input events from the MyGo window. The MyGo application owns the top-level window and fullscreen behavior; MPV must not create a competing top-level player window in the embedded mode.

If the current macvk implementation cannot be cleanly adapted to an externally hosted view, stop and compare the cost of a maintained MPV fork against a different rendering integration before proceeding.

### POC 3 — Connect libmpv to the host

Only after POC 1 and POC 2 succeed:

- expose a narrow, versioned host-view entry point from the patched macOS libmpv build (for example, an Aurora-specific function that accepts the host `NSView` before MPV initialization);
- call it from a Darwin adapter using the same application-level playback contract as Windows;
- keep the MPV engine responsible for demuxing, decoding, audio, subtitle rendering, filters and playback state;
- keep MyGo responsible for Aurora's UI, native window and placement of the video host.

Do not pass an `NSView` to MPV's `wid` option: that option is not documented for macOS.

## Acceptance checks

The spike is successful only when all of the following are demonstrated on real Mac hardware:

1. The video is visibly embedded inside Aurora's MyGo window, not in a separate MPV window.
2. No OpenGL context, framework or renderer is used by the production path. Verify through MPV logs and runtime inspection.
3. MPV uses `gpu-next` + `macvk` and VideoToolbox hardware decoding where the test codec/device supports it.
4. Resizing, Retina scale, clipping, overlays, hide/show, app fullscreen, app activation and window close/reopen behave correctly.
5. Audio remains synchronized during playback and seeking.
6. Embedded and external subtitles work, including ASS positioning and styling.
7. The test set includes H.264, HEVC 4K, MKV with multiple audio tracks, external SRT and embedded ASS.
8. There are no avoidable GPU-to-CPU readbacks in the normal video presentation path.
9. The existing Windows build and playback path remain unchanged.

Record MPV version/build flags, macOS version, Mac model/chip, test media details, MPV logs and any rendering/decoding fallback for every test run.

## Explicit non-goals for this spike

- Do not replace libmpv with VLC.
- Do not replace the Windows D3D11 integration.
- Do not move filmstrip generation into MPV.
- Do not port all Aurora features yet.
- Do not claim macOS support based only on a successful Go cross-compile. The rendering, native view and hardware-decoding checks require a real Mac.

## Current status

- [x] Inspected Aurora's current Windows MPV adapter and application ports.
- [x] Confirmed the pinned MyGo version and reviewed its v0.9.2 native-view module example.
- [x] Reviewed MPV v0.42.0 macvk source and documentation.
- [ ] Build and run the MyGo native-view host POC on macOS.
- [ ] Verify standalone MPV 0.42 macvk + VideoToolbox on target Mac hardware.
- [ ] Implement and test the smallest MPV host-view patch.
- [ ] Demonstrate libmpv rendering inside the MyGo view with no OpenGL.
- [ ] Only then design the production Darwin platform adapter and app bootstrap.
