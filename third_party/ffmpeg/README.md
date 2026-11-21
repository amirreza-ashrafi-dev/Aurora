# FFmpeg runtime

Aurora uses FFmpeg only for the optional thumbnail filmstrip. Normal playback
continues to use libmpv and does not depend on this executable.

## Install on Windows

From the repository root, run this in PowerShell:

```powershell
git lfs install
.\scripts\setup-ffmpeg.ps1
```

The script downloads the Windows x64 LGPL static archive from the BtbN
FFmpeg-Builds latest release, verifies the archive against the release's
`checksums.sha256`, checks that `ffmpeg.exe -version` works, and installs the
executable in this directory. The included license/readme files are copied
to `licenses/`; the source release and verified archive hash are recorded in
`BUILD-INFO.txt`.

Use `-Force` to replace an existing executable.

## Git LFS

`third_party/ffmpeg/ffmpeg.exe` is tracked by Git LFS via the repository's
`.gitattributes`. After the setup script has installed the file, add and push
it with Git LFS enabled:

```powershell
git add third_party/ffmpeg/ffmpeg.exe third_party/ffmpeg/licenses third_party/ffmpeg/BUILD-INFO.txt
git commit -m "Add FFmpeg runtime via Git LFS"
git push
```

The executable is intentionally not stored as a regular Git blob.

## Licensing

Choose and distribute an FFmpeg build deliberately. The LGPL label in the
archive name does not by itself establish that redistribution is compliant:
obligations depend on the exact build configuration and enabled components.
Review the included license materials and satisfy all applicable notice,
source-code, and relinking requirements before distributing Aurora with this
binary.

FFmpeg project: https://ffmpeg.org/
BtbN build source: https://github.com/BtbN/FFmpeg-Builds/releases
