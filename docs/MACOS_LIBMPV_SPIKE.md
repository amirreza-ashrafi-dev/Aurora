# Reuse MPV's macvk Metal renderer inside Aurora

## Decision

Keep **libmpv** as Aurora's media engine. On macOS, reuse MPV 0.42.0's existing gpu-next + macvk renderer (Vulkan presented through Apple's Metal surface). Do not replace MPV's renderer with a new Swift/Metal renderer, do not use OpenGL, and do not introduce VLC.

The implementation uses one small Aurora-maintained patch to make MPV's existing macOS video view a child of a Go-created, MyGo-hosted NSView. Aurora's application code remains Go-only. It does not implement a second renderer.

## What is implemented in this branch

- third_party/mpv/patches/0001-macos-macvk-embed-nsview.patch: patch against MPV v0.42.0.
- scripts/build-mpv-macos.sh: Apple Silicon build script for patched libmpv, with Vulkan/macvk/VideoToolbox enabled and OpenGL build features disabled.
- internal/platform/macos/videohost: Go-only NSView host created through the Objective-C runtime with the existing purego dependency. Aurora has no Swift module or Swift source.
- internal/adapters/mpv/player_darwin.go: Darwin libmpv adapter. It passes the host NSView pointer through the macOS-specific wid contract and configures gpu-next, Vulkan, macvk and VideoToolbox.
- internal/platform/macos/platform_darwin.go and internal/bootstrap/run_darwin.go: macOS composition root.
- internal/application/ports.go and the MyGo presentation: platform-native video-host abstraction; Windows retains its existing HWND path, while macOS mounts the native view in the video stage.
- internal/adapters/mpv/player_unsupported.go: now excludes both Windows and macOS so the Darwin adapter is selected on macOS.

**Important:** this is source implementation, not a verified macOS build. Aurora's host and integration code are Go-only; the Swift files changed by the MPV patch are part of MPV's existing macOS Cocoa/macvk renderer. This environment does not have macOS, Xcode, an Apple Silicon GPU, the Vulkan SDK or MoltenVK, so the MPV patch and real Metal surface have not been compiled or run here.

## How the embedding works

MPV 0.42.0's existing macOS Vulkan context already does this:

1. It asks its macOS helper for a CAMetalLayer.
2. It passes that layer to vkCreateMetalSurfaceEXT.
3. Vulkan/MoltenVK presents the existing MPV GPU-rendered frames through the Metal surface.

The Aurora patch changes only the host ownership and view sizing:

1. Aurora creates a layer-backed NSView directly from Go with purego's Objective-C runtime calls. This happens in MyGo's application-ready callback on the AppKit main thread.
2. Aurora keeps that native view mounted even before the first file is selected. This ensures the host already belongs to Aurora's window when libmpv initializes.
3. Aurora passes the view pointer to the Darwin libmpv adapter.
4. In the patched macOS macvk context, MPV interprets wid as that host NSView pointer.
5. MPV creates its existing input View, attaches its existing MetalLayer to it, and adds that view as a child of Aurora's host view.
6. The same MetalLayer is still passed to vkCreateMetalSurfaceEXT. MPV's existing Vulkan surface, swapchain, GPU shaders, subtitle compositing and presentation pipeline remain responsible for the video.

MPV does not create a second top-level NSWindow in embedded mode. Aurora/MyGo owns the window, layout and fullscreen lifecycle. When no host pointer is supplied, MPV's original standalone macvk window behavior remains.

## MPV patch details

The patch changes these MPV v0.42.0 files:

- video/out/vulkan/context_mac.m: reads the macOS embedded host pointer from WinID, passes it to MacCommon, and sizes the Vulkan swapchain from the hosted view's backing-pixel bounds.
- video/out/mac_common.swift: supports host-view mode, attaches MPV's existing view/layer, tracks host resize/screen/scale/occlusion changes, and removes observers on teardown.
- video/out/mac/common.swift: uses the containing host window for display, scale, window-id and backing-size queries. Fullscreen size changes are left to Aurora when embedded.
- video/out/mac/view.swift: avoids setting OpenGL-specific view flags for macvk, forwards host lifecycle/resize notifications, uses the actual containing window for cursor hit-testing, and does not register MPV's drag/drop targets in embedded mode.

The macOS-specific meaning of wid is an Aurora-patched MPV contract. It does not change wid behavior on Windows, X11 or Android.

## Build the patched MPV library on a Mac

The first target is Apple Silicon arm64.

Prerequisites:
- Xcode Command Line Tools and a compatible Swift toolchain (required by MPV's existing macOS renderer sources; Aurora itself does not build a Swift module).
- Meson, Ninja, Git and pkg-config.
- Vulkan SDK / MoltenVK, shaderc and libplacebo development packages visible to pkg-config.

From the Aurora repository root on an Apple Silicon Mac:

    bash scripts/build-mpv-macos.sh

The script checks out the MPV v0.42.0 tag, applies the checked-in patch and configures a libmpv-only build with the macvk Vulkan context and VideoToolbox/libplacebo decoding. It disables MPV's OpenGL build features for this library. The library is installed under third_party/mpv/macos.

The app adapter looks for libmpv.2.dylib / libmpv.dylib in MyGo resources, the app's Frameworks directory, or the build output path. For a local development build, AURORA_LIBMPV_PATH can point directly to the built dylib.

The script does **not** yet copy all transitive dylib dependencies into the final .app, sign the bundle, or notarize a release. Those are separate release-engineering tasks after the renderer works on a real Mac.

## Windows remains unchanged

- Windows continues using the existing HWND, wid, vo=gpu, gpu-api=d3d11, bundled libmpv-2.dll and existing Win32 platform adapter.
- macOS uses a MyGo-hosted NSView, patched wid embedding, vo=gpu-next, gpu-api=vulkan, gpu-context=macvk and VideoToolbox.
- Both builds continue using libmpv as the playback engine.
- FFmpeg filmstrip generation remains a separate adapter.

## Verification checklist

<badge color="warning">Mac build and runtime verification pending</badge>

| Item | Status |
|---|---|
| Inspect Aurora's Windows/libmpv integration | Done |
| Inspect MPV 0.42.0 macvk and Metal source | Done |
| Add MPV macOS NSView embedding patch | Patch generated from and hunk-validated against upstream v0.42.0; not compiled |
| Add Go/purego NSView host | Source added; no Aurora Swift module or Swift build step |
| Add Darwin libmpv adapter and bootstrap | Source added; macOS CI compile check pending |
| Keep host view mounted before first playback | Implemented in presentation source |
| Build patched libmpv with OpenGL disabled | Pending real Mac |
| Show video inside Aurora's window using existing macvk MetalLayer | Pending real Mac |
| Verify VideoToolbox hardware decoding and 4K | Pending real Mac |
| Verify audio sync, subtitles, ASS styling and seeking | Pending real Mac |
| Verify Retina scaling, resize, hide/show and fullscreen | Pending real Mac |
| Verify macOS video-area double-click / wheel routing | Not yet implemented |
| Bundle dependencies, sign and notarize the app | Later release work |
| Confirm Windows build and playback remain unchanged | Must run CI after commit |

## Acceptance criteria

The macOS renderer work is complete only when all of the following are demonstrated on real Apple Silicon hardware:

1. No separate MPV window appears; video is embedded in Aurora's MyGo video stage.
2. MPV logs confirm gpu-next + macvk; no OpenGL context or output is created.
3. Vulkan creates its Metal surface using MPV's existing MetalLayer.
4. VideoToolbox hardware decoding is used when supported by the device and codec.
5. The hosted view handles resize, Retina scale, display changes, occlusion and Aurora fullscreen transitions.
6. MyGo controls and overlays remain visible above the video.
7. Audio remains synchronized; seeking and media switching work.
8. Embedded and external subtitles, including ASS styling and positioning, retain MPV behavior.
9. H.264, 4K HEVC, MKV with multiple audio tracks, external SRT and embedded ASS all pass.
10. Closing/reopening the host does not leak or invalidate MPV/Metal/Vulkan resources.
11. The Windows playback path remains unchanged.

## Sources reviewed

- MPV 0.42.0 release: https://github.com/mpv-player/mpv/releases/tag/v0.42.0
- MPV macOS Vulkan context: https://github.com/mpv-player/mpv/blob/v0.42.0/video/out/vulkan/context_mac.m
- MPV macOS Metal layer: https://github.com/mpv-player/mpv/blob/v0.42.0/video/out/mac/metal_layer.swift
- MPV macOS view/window setup: https://github.com/mpv-player/mpv/tree/v0.42.0/video/out/mac
- MyGo 0.9.2 native view embedding API: https://github.com/egoist/mygo/blob/v0.9.2/ui/webview.go
- purego 0.11.1 Objective-C runtime API: https://github.com/ebitengine/purego/tree/v0.11.1/objc
