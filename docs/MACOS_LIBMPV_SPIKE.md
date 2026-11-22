# Reuse MPV's macvk Metal renderer inside Aurora

## Decision

Keep **libmpv** as Aurora's media engine. On macOS, reuse MPV 0.42's existing `gpu-next` + `macvk` rendering path (Vulkan presented through Apple's Metal surface). Do not replace MPV's renderer with a new Swift/Metal renderer, do not use OpenGL, and do not introduce VLC.

This work has one implementation target: **make MPV's existing macvk output attach to a native view supplied by Aurora/MyGo instead of creating its own top-level MPV window.**

## Why a source change is required

The current MPV 0.42 `macvk` implementation creates its own `MacCommon`, `MetalLayer`, MPV `View` and MPV `Window`:

- `video/out/vulkan/context_mac.m` constructs `MacCommon` and gives Vulkan the `CAMetalLayer` from it.
- `video/out/mac_common.swift` creates the `MetalLayer`, creates the view and creates the MPV-owned window in `config()`.
- `video/out/mac/view.swift` is MPV's native view and handles MPV input events.
- `video/out/mac/metal_layer.swift` is the existing `CAMetalLayer` subclass, including its color/HDR handling.
- MPV's `wid` option is an `int64_t` option, but upstream only documents native window embedding for X11, Win32 and Android. It does not currently define an NSView embedding contract on macOS.

Therefore, simply passing a MyGo NSView through `wid` will not work with an unmodified MPV build.

## The implementation to make

### 1. Add a macOS host-view mode to MPV's existing macvk output

Maintain a small Aurora patch/fork against a pinned MPV 0.42.0 source revision.

Use MPV's existing `wid` value as the macOS host `NSView` pointer in this explicit embedded mode. Document that macOS-specific meaning and validate the pointer before use. Do not change the meaning of `wid` on Windows, X11 or Android.

Inside `MacCommon`:

1. Keep creating MPV's existing `MetalLayer`. Do not replace it.
2. Keep creating MPV's existing `View` so MPV's existing input/event handling remains available.
3. When a host NSView is supplied, attach that MPV view as a child of the host view, sized to the host bounds with autoresizing/layout updates.
4. Do **not** create an MPV-owned `NSWindow`, do not activate a second application window, and do not apply MPV window/fullscreen/title-bar behavior in embedded mode.
5. When no host view is supplied, preserve MPV's existing standalone macOS behavior unchanged.

Inside `context_mac.m`, continue passing the same MPV-owned `MetalLayer` to `VkMetalSurfaceCreateInfoEXT.pLayer`. Keep `vkCreateMetalSurfaceEXT`, Vulkan swapchain, MPV's GPU rendering pipeline and frame presentation path unchanged.

Adapt only the host-dependent parts of the existing `MacCommon`/macOS view integration:

- Resize the Vulkan surface from the hosted view's backing-pixel bounds.
- Determine visibility from the hosted view and its containing window.
- Read backing scale, display and color profile from the containing Aurora window/screen.
- React to host-window resize, backing-scale, screen/color-profile, occlusion and layout changes.
- Let Aurora own fullscreen and top-level window lifecycle; MPV's embedded mode must never create or resize a separate top-level window.
- Preserve correct teardown: stop rendering before the host view is released, remove MPV's child view, release the Metal layer and Vulkan resources, and never close Aurora's window.
- Keep MPV input event handling on its child view where appropriate; Aurora remains responsible for app-level shortcuts and controls.

The host view must be supplied before MPV initializes the video output. The embedding contract must be explicit and versioned/documented in the Aurora-patched MPV build.

### 2. Host that MPV view from MyGo

Use a MyGo native module implemented in Swift to create a plain container `NSView`. Return it through MyGo's `native.View` interface and place it with `ui.NativeView` in Aurora's video rectangle.

Pass the native view pointer to the patched libmpv instance through the macOS embedded `wid` contract before `mpv_initialize`.

MyGo remains responsible for the Aurora window and layout. MPV remains responsible for playback and for presenting its own existing Metal layer inside the hosted child view. Aurora's Go controls can continue to be layered above the native view by MyGo.

### 3. Keep the Windows path unchanged

- Windows continues using the existing `HWND`, `wid`, `vo=gpu`, `gpu-api=d3d11` implementation.
- macOS uses the hosted NSView mode and `vo=gpu-next`, `gpu-api=vulkan`, `gpu-context=macvk`.
- Both platforms continue using the same libmpv playback engine and Aurora application-level player contract.
- FFmpeg filmstrip generation remains its own adapter.

The current Aurora application port is still Win32-shaped (`uintptr` HWND methods and a `videoHWND` factory parameter). Generalize that port only as needed to carry a native video-host handle and platform operations; keep the existing Windows implementation behavior unchanged.

## Required build configuration

Build and bundle a pinned MPV 0.42.0 macOS libmpv with:

- `vo=gpu-next`
- Vulkan enabled
- `macvk` enabled
- MoltenVK / Vulkan Metal-surface support
- VideoToolbox hardware decoding enabled
- OpenGL excluded from Aurora's macOS runtime rendering path

Start with Apple Silicon arm64. Consider universal packaging only after the arm64 build and embedding path are stable.

## Acceptance criteria

This implementation is complete only when the **existing macvk renderer** is demonstrably rendering inside Aurora's MyGo video rectangle:

1. No separate MPV window appears; the video is a child view inside Aurora.
2. MPV logs confirm `gpu-next` + `macvk`; no OpenGL context or output is created.
3. Vulkan creates its Metal surface using MPV's existing `MetalLayer`.
4. Hardware decoding uses VideoToolbox where supported by the Mac and codec.
5. The hosted view handles resize, Retina scale, display changes, occlusion and Aurora fullscreen transitions.
6. MyGo controls and overlays remain visible above the video.
7. Playback audio stays synchronized; seeking and media switching remain stable.
8. Embedded/external subtitles, including ASS styling and positioning, retain MPV behavior.
9. Test H.264, 4K HEVC, MKV with multiple audio tracks, external SRT and embedded ASS.
10. Closing/reopening the host does not leak or invalidate MPV/Metal/Vulkan resources.
11. Windows playback and packaging remain unchanged.

## Sources reviewed

- MPV 0.42.0 release: https://github.com/mpv-player/mpv/releases/tag/v0.42.0
- MPV macOS Vulkan context: https://github.com/mpv-player/mpv/blob/v0.42.0/video/out/vulkan/context_mac.m
- MPV macOS Metal layer: https://github.com/mpv-player/mpv/blob/v0.42.0/video/out/mac/metal_layer.swift
- MPV macOS view/window setup: https://github.com/mpv-player/mpv/blob/v0.42.0/video/out/mac_common.swift
- MyGo 0.9.2 native modules: https://github.com/egoist/mygo/blob/v0.9.2/docs/native-modules.md
- MyGo 0.9.2 native view example: https://github.com/egoist/mygo/tree/v0.9.2/examples/native-module/modules/places

## Status

- [x] Inspected Aurora's current Win32/libmpv integration.
- [x] Inspected MPV 0.42.0's existing macvk, Metal layer and macOS view/window code.
- [x] Confirmed MyGo 0.9.2 can host a Swift-backed NSView.
- [ ] Implement MPV's embedded-host mode while reusing the existing macvk Metal layer and Vulkan surface.
- [ ] Build the patched macOS libmpv and integrate its native host handle into Aurora.
- [ ] Verify on a real Apple Silicon Mac against the acceptance criteria above.
