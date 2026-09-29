#!/usr/bin/env bash
# calibration/clone_test.sh — hermetic test of clone.sh's shard path and
# clean.sh (LB-F21): a local tagged repo, one shard with a subpath, no network.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
command -v yq >/dev/null || { echo "clone_test: yq not installed — skipping"; exit 0; }
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

src="$tmp/src"; mkdir -p "$src/keep" "$src/drop"
echo 'a' >"$src/keep/a.txt"; echo 'b' >"$src/drop/b.txt"
git -C "$src" init -q
git -C "$src" -c user.email=t@example.invalid -c user.name=t add -A
git -C "$src" -c user.email=t@example.invalid -c user.name=t commit -qm init
git -C "$src" tag v1
sha=$(git -C "$src" rev-parse HEAD)

mkdir -p "$tmp/shards"
cat >"$tmp/shards/toy.yaml" <<YAML
id: toy
language: Toy
repos:
  - name: example/src
    url: file://$src
    ref: v1
    sha: $sha
    licence: MIT
    subpath: keep
    idioms: [one]
YAML
cat >"$tmp/shards/Bad_Id.yaml" <<YAML
id: Bad_Id
language: Bad
repos: []
YAML

export CALIBRATION_MANIFEST="$tmp/none.yaml" CALIBRATION_SHARDS="$tmp/shards" CALIBRATION_DEST="$tmp/dest"
if bash "$HERE/clone.sh" toy >"$tmp/out" 2>&1; then
  echo "FAIL: clone.sh passed despite a malformed shard id"; cat "$tmp/out"; exit 1
fi
grep -q "Bad_Id" "$tmp/out" || { echo "FAIL: malformed shard not named"; cat "$tmp/out"; exit 1; }
rm "$tmp/shards/Bad_Id.yaml"

bash "$HERE/clone.sh" toy >"$tmp/out" 2>&1 || { echo "FAIL: clone.sh"; cat "$tmp/out"; exit 1; }
[ -f "$tmp/dest/toy/src/keep/a.txt" ] || { echo "FAIL: subpath file missing"; cat "$tmp/out"; exit 1; }
[ ! -e "$tmp/dest/toy/src/drop/b.txt" ] || { echo "FAIL: sparse checkout kept a file outside subpath"; exit 1; }
bash "$HERE/clone.sh" toy >"$tmp/out2" 2>&1
grep -q "^skip" "$tmp/out2" || { echo "FAIL: rerun did not skip"; cat "$tmp/out2"; exit 1; }

# A shard with `sparse` patterns (a tool format's manifests, at any depth in a
# large tree): only the matching files land on disk.
src2="$tmp/src2"; mkdir -p "$src2/a/b" "$src2/big"
echo 'module root' >"$src2/go.mod"; echo 'module nested' >"$src2/a/b/go.mod"
echo 'x' >"$src2/a/b/code.go"; echo 'y' >"$src2/big/blob.bin"; echo 's' >"$src2/go.sum"
git -C "$src2" init -q
git -C "$src2" -c user.email=t@example.invalid -c user.name=t add -A
git -C "$src2" -c user.email=t@example.invalid -c user.name=t commit -qm init
git -C "$src2" tag v1
sha2=$(git -C "$src2" rev-parse HEAD)
cat >"$tmp/shards/mods.yaml" <<YAML
id: mods
language: Mods
repos:
  - name: example/src2
    url: file://$src2
    ref: v1
    sha: $sha2
    licence: MIT
    sparse: ["**/go.mod", "**/go.sum"]
    idioms: [one]
YAML
bash "$HERE/clone.sh" mods >"$tmp/out" 2>&1 || { echo "FAIL: clone.sh with sparse"; cat "$tmp/out"; exit 1; }
[ -f "$tmp/dest/mods/src2/go.mod" ] && [ -f "$tmp/dest/mods/src2/a/b/go.mod" ] && [ -f "$tmp/dest/mods/src2/go.sum" ] \
  || { echo "FAIL: a sparse pattern's files missing"; cat "$tmp/out"; exit 1; }
[ ! -e "$tmp/dest/mods/src2/a/b/code.go" ] && [ ! -e "$tmp/dest/mods/src2/big/blob.bin" ] \
  || { echo "FAIL: sparse checkout kept a file outside its patterns"; exit 1; }
bash "$HERE/clean.sh" mods >/dev/null
rm "$tmp/shards/mods.yaml"

# A filter that names neither the shard id nor its language clones nothing.
rm -rf "$tmp/dest"
bash "$HERE/clone.sh" other >/dev/null
[ ! -e "$tmp/dest/toy" ] || { echo "FAIL: filter ignored"; exit 1; }

bash "$HERE/clone.sh" toy >/dev/null
bash "$HERE/clean.sh" toy >/dev/null
[ ! -e "$tmp/dest/toy" ] || { echo "FAIL: clean.sh left the clone"; exit 1; }
if bash "$HERE/clean.sh" "../x" 2>/dev/null; then echo "FAIL: clean.sh accepted a path"; exit 1; fi
if bash "$HERE/clean.sh" 2>/dev/null; then echo "FAIL: clean.sh accepted no id"; exit 1; fi
echo "clone_test: ok"
