# Aurora Player

<p align="center">
  <strong>A simple, beautiful media player for Windows.</strong>
</p>

<p align="center">
  <img src="./resources/icon.png" alt="Aurora Player screenshot placeholder" width="800">
</p>

## About Aurora Player

**Aurora Player** is a modern desktop media player designed for Windows 11. It aims to combine a clean, elegant interface with reliable media playback and a comfortable user experience.

Built with **Go**, **MyGo**, and the powerful **libmpv** playback engine, Aurora Player brings together native Windows integration and a modern graphical interface.

The goal is simple: make watching videos feel effortless.

## Current Features

- Video playback powered by libmpv
- Playlist and playback controls, including removing individual queue items
- Fullscreen playback with an auto-hiding floating controller
- **Cinematic Mode** for distraction-free windowed or fullscreen playback
- **Smart Resume** offers to continue a video from its last saved position or start over
- External subtitle loading, strict filename matching, per-video external subtitle restoration, drag-and-drop, custom text/outline colors, and language-specific appearance profiles
- Keyboard shortcuts for playback, seeking, volume, fullscreen, and Cinematic Mode
- On-demand visual timeline filmstrip with persistent FFmpeg-generated thumbnails and bounded in-memory previews

### Cinematic Mode

Press **C** or use the Cinematic Mode button to hide the normal player interface and expand the video to the available window area. Move the pointer to reveal the floating playback controller; it automatically hides after a short period of inactivity. Press **C** or **Esc** to leave Cinematic Mode.

### Subtitle Studio (in progress)

Load external SRT, ASS/SSA, WebVTT, or SUB subtitle files while a video is playing. The initial Studio controls font family, size, text color, outline thickness, vertical position, and subtitle delay. Appearance defaults are stored locally; language-specific profiles can be saved from the Studio when the active track exposes language metadata. Embedded track selections are not yet persisted between sessions.

### Smart Resume

When you open a video with saved progress, Aurora offers **Resume** or **Start over**. Progress is stored locally in a versioned JSON file under your Windows user configuration directory. The history is limited to 500 videos and 1 MiB; the least recently watched entries are removed when a limit is reached. Automatic queue playback always starts at the beginning and does not interrupt playback with a resume prompt.

## Open videos from Windows File Explorer

Aurora registers itself for its supported video file types for the current Windows user when it starts. This makes Aurora available in the Windows **Open with** menu and the **Default apps** settings. Registration does not change your existing defaults.

To make Aurora your default video player, right-click a video and choose **Open with**, or open **Settings → Apps → Default apps** and choose Aurora for the file types you want. After that, double-clicking an associated video opens it directly in Aurora. Aurora starts playback automatically and adds any video paths passed to it to the play queue.

The supported file types are MP4, MKV, AVI, MOV, WMV, WebM, M4V, MPG, MPEG, TS, M2TS, FLV, OGV, and 3GP. If you move Aurora to a different folder, start it once from its new location so Windows can refresh the registered executable path.

## Technology

| Technology | Purpose |
|---|---|
| Go | Main programming language |
| MyGo | Desktop application framework |
| libmpv | Media playback engine |
| FFmpeg | Background thumbnail extraction for the timeline filmstrip |
| purego | Calling native Windows APIs |
| Win32 API | Windows video hosting and integration |

## Requirements

- Windows 11 (64-bit)
- Go 1.27.3 or a compatible version
- MyGo v0.9.2
- libmpv runtime files
- Git LFS for the large Windows FFmpeg runtime file

## Building from Source

1. Clone the repository with Git LFS installed and initialized.
2. Make sure the required Go dependencies are installed.
3. Check that the MPV runtime files are available in `third_party/mpv`.
4. Open PowerShell in the project directory and run the FFmpeg setup script:

```powershell
git lfs install
.\scripts\setup-ffmpeg.ps1
```

The script downloads the current Windows x64 LGPL static build from BtbN FFmpeg Builds, verifies its SHA-256 checksum against the release checksum file, checks that `ffmpeg.exe` runs, and copies the build's license/readme files into `third_party/ffmpeg/licenses`. Use `-Force` to replace an existing executable.

5. Build Aurora:

```powershell
go tool mygo build -platform windows/amd64
```

The build output and packaged runtime resources are configured in `mygo.json`. FFmpeg is used only for filmstrip thumbnails; normal playback remains powered by libmpv.

### Updating the FFmpeg binary in Git LFS

The executable is tracked by Git LFS through `.gitattributes`, not as a regular Git blob. After running the setup script, review the license files and then stage and push the resource:

```powershell
git add third_party/ffmpeg/ffmpeg.exe third_party/ffmpeg/licenses third_party/ffmpeg/BUILD-INFO.txt
git commit -m "Add FFmpeg runtime via Git LFS"
git push
```

The FFmpeg build's LGPL label does not by itself establish that redistribution is compliant. Review the exact build configuration and include all notices and source materials required by its license.

## Project Status

Aurora Player is currently in early development.

The current focus is on:

- Reliable playback
- A polished and consistent user interface
- Clean project architecture
- Better Windows integration
- Stability and performance

A media library and more advanced features may be considered in future versions. The priority is to make the core player solid before expanding its functionality.

## Roadmap

- [ ] Improve playback stability
- [ ] Refine the user interface
- [ ] Improve application architecture
- [x] Add Cinematic Mode
- [ ] Complete Subtitle Studio (per-video subtitle matching and language profile editor)
- [ ] Add more playback options
- [x] Add visual timeline thumbnail filmstrip
- [ ] Improve accessibility and usability
- [ ] Explore support for additional platforms

## Screenshots

More screenshots will be added as the project develops.

## License

Aurora Player is licensed under the MIT License.

You are free to use, copy, modify, merge, publish, distribute, sublicense, and sell copies of the software, subject to the terms of the MIT License.

---

<p align="center">
  <strong>Aurora Player</strong><br>
  <em>A clearer, calmer way to enjoy your media.</em>
</p>
