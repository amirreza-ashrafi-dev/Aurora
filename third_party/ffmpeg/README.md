# FFmpeg runtime

Aurora uses FFmpeg only for the optional thumbnail filmstrip. Normal playback
continues to use libmpv and does not depend on this executable.

Place a Windows x64 FFmpeg build here as `ffmpeg.exe`, or make `ffmpeg.exe`
available on `PATH` during development. The packaged application looks for
this resource at runtime.

Choose and distribute an FFmpeg build deliberately. FFmpeg's licensing
obligations depend on the exact build configuration and enabled components.
For redistribution, use an appropriately licensed build and ship its required
license notices and corresponding source obligations. Aurora does not commit
the FFmpeg executable to this repository.
