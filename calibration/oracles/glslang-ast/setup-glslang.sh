#!/usr/bin/env bash
set -euo pipefail

# Run from the repository root. Only ignored workspace/ receives downloads and binaries.
[[ "$(uname -s)" == Darwin ]] || { echo 'This pin is the macOS universal release' >&2; exit 1; }
asset_url='https://github.com/KhronosGroup/glslang/releases/download/16.6.0/glslang-16.6.0-macos-universal-release.tar.gz'
archive_sha='4f8c05f2888a73ed0b7b274bc524af9130e09ae94b43c0f3f9ed83cfae852834'
binary_sha='18c23a68d6f4a21e822a8dcee212a2a448a66494b8f58aaf1655ef6328543d11'
archive='workspace/tmp/glslang-16.6.0-macos.tar.gz'
dest='workspace/tools/glslang-16.6.0'
mkdir -p workspace/tmp "$dest"
if [[ ! -f "$archive" ]]; then
  curl -fsSL "$asset_url" -o "${archive}.partial"
  actual="$(shasum -a 256 "${archive}.partial" | awk '{print $1}')"
  [[ "$actual" == "$archive_sha" ]] || { echo "Archive digest mismatch: $actual" >&2; exit 1; }
  mv "${archive}.partial" "$archive"
fi
actual="$(shasum -a 256 "$archive" | awk '{print $1}')"
[[ "$actual" == "$archive_sha" ]] || { echo "Archive digest mismatch: $actual" >&2; exit 1; }
tar xzf "$archive" -C "$dest"
actual="$(shasum -a 256 "$dest/bin/glslang" | awk '{print $1}')"
[[ "$actual" == "$binary_sha" ]] || { echo "Binary digest mismatch: $actual" >&2; exit 1; }
"$dest/bin/glslang" --version | grep -F 'Glslang Version: 11:16.6.0'
echo "Verified glslang release archive $archive_sha and binary $binary_sha"
