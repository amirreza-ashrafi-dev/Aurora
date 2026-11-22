#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TAG="v0.42.0"
SOURCE_DIR="${MPV_SOURCE_DIR:-"$ROOT/.build/mpv-$TAG"}"
BUILD_DIR="${MPV_BUILD_DIR:-"$ROOT/.build/mpv-$TAG-macos-arm64"}"
PREFIX="${MPV_PREFIX:-"$ROOT/third_party/mpv/macos"}"
PATCH="$ROOT/third_party/mpv/patches/0001-macos-macvk-embed-nsview.patch"

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

[[ "$(uname -s)" == "Darwin" ]] || fail "run this script on macOS"
[[ "$(uname -m)" == "arm64" ]] || fail "the first target is Apple Silicon arm64"
for tool in git meson ninja pkg-config; do
  command -v "$tool" >/dev/null 2>&1 || fail "missing build tool: $tool"
done
[[ -f "$PATCH" ]] || fail "missing MPV patch: $PATCH"

# gpu-next requires libplacebo; macvk needs Vulkan/MoltenVK and shaderc.
for package in libplacebo vulkan shaderc; do
  pkg-config --exists "$package" || fail "pkg-config cannot find '$package'; configure Vulkan SDK/MoltenVK, libplacebo and shaderc first"
done

if [[ ! -d "$SOURCE_DIR/.git" ]]; then
  mkdir -p "$(dirname "$SOURCE_DIR")"
  git clone --depth 1 --branch "$TAG" https://github.com/mpv-player/mpv.git "$SOURCE_DIR"
fi

git -C "$SOURCE_DIR" fetch --depth 1 origin "refs/tags/$TAG:refs/tags/$TAG"
git -C "$SOURCE_DIR" checkout --detach "$TAG"
if git -C "$SOURCE_DIR" apply --reverse --check "$PATCH" >/dev/null 2>&1; then
  echo "MPV embedding patch is already applied."
else
  git -C "$SOURCE_DIR" apply --check "$PATCH"
  git -C "$SOURCE_DIR" apply "$PATCH"
fi

MESON_ARGS=(
  --prefix "$PREFIX"
  --buildtype release
  -Dlibmpv=true
  -Dcplayer=false
  -Dcocoa=enabled
  -Dswift-build=enabled
  -Dvulkan=enabled
  -Dshaderc=enabled
  -Dvideotoolbox-pl=enabled
  -Dvideotoolbox-gl=disabled
  -Dmacos-cocoa-cb=disabled
  -Dgl=disabled
  -Dgl-cocoa=disabled
  -Dplain-gl=disabled
)

if [[ -d "$BUILD_DIR" ]]; then
  meson setup --reconfigure "$BUILD_DIR" "$SOURCE_DIR" "${MESON_ARGS[@]}"
else
  meson setup "$BUILD_DIR" "$SOURCE_DIR" "${MESON_ARGS[@]}"
fi

ninja -C "$BUILD_DIR"
ninja -C "$BUILD_DIR" install

LIBMPV="$(find "$PREFIX" -type f -name 'libmpv*.dylib' -print -quit)"
[[ -n "$LIBMPV" ]] || fail "MPV built, but no libmpv dylib was found under $PREFIX"

printf '\nBuilt MPV %s with Aurora macvk embedding patch.\n' "$TAG"
printf 'Library: %s\n' "$LIBMPV"
printf 'Renderer: gpu-next + Vulkan + macvk (MoltenVK)\n'
printf 'Hardware decoding: VideoToolbox/libplacebo\n'
printf 'OpenGL build features: disabled\n'
printf '\nThe final .app still needs its dylib dependencies bundled, signed and notarized.\n'
