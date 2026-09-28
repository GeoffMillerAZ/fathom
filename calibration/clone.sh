#!/usr/bin/env bash
# calibration/clone.sh — shallow-clone the calibration corpus into workspace/ (gitignored).
#
# Reads calibration/corpus.yaml and every calibration/corpus.d/<id>.yaml shard
# (docs/design/language-breadth.md §13.3, LB-F21) and clones each repo at its
# pinned tag/SHA:
#   corpus.yaml entries  -> workspace/calibration/<language slug>/<repo>
#   corpus.d/<id>.yaml   -> workspace/calibration/<id>/<repo>
# Clones are --depth 1 single-branch. A repo with a `subpath` is cloned
# blob-less and sparse, so only that subtree lands on disk. Re-running skips
# repos already present (idempotent).
#
# A shard is one queue item's subjects:
#   id: swift                      # the queue id; [a-z0-9-]+; equals the file name
#   language: Swift
#   repos:
#     - name: Alamofire/Alamofire
#       url: https://github.com/Alamofire/Alamofire
#       ref: 5.10.2                # a release tag
#       sha: <commit from git ls-remote>
#       licence: MIT               # OSI-approved; read, never vendored
#       subpath: Source            # optional: large repos
#       idioms: [...]              # 3-6 idioms the subject exercises
#
# Usage:
#   calibration/clone.sh                 # clone the whole corpus and every shard
#   calibration/clone.sh Python swift    # a filter: a shard id (exact) or a
#                                        # language name (case-insensitive substring)
#
# Env (tests): CALIBRATION_MANIFEST, CALIBRATION_SHARDS, CALIBRATION_DEST.
# Requires: git, yq (mikefarah v4).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MANIFEST="${CALIBRATION_MANIFEST:-$ROOT/calibration/corpus.yaml}"
SHARDS="${CALIBRATION_SHARDS:-$ROOT/calibration/corpus.d}"
DEST="${CALIBRATION_DEST:-$ROOT/workspace/calibration}"

command -v yq >/dev/null || { echo "error: yq is required" >&2; exit 1; }

FILTERS=("$@")
lc() { printf '%s' "$1" | tr '[:upper:]' '[:lower:]'; }
# matches_filter <language> [shard id]
matches_filter() {
  [ ${#FILTERS[@]} -eq 0 ] && return 0
  local lang_lc f; lang_lc="$(lc "$1")"
  for f in "${FILTERS[@]}"; do
    [ -n "${2:-}" ] && [ "$f" = "$2" ] && return 0
    case "$lang_lc" in *"$(lc "$f")"*) return 0;; esac
  done
  return 1
}

total_ok=0; total_fail=0; total_skip=0; shard_errors=0

# clone_repo <slug> <index> <name> <url> <ref> <sha> <subpath>
clone_repo() {
  local slug="$1" j="$2" name="$3" url="$4" ref="$5" sha="$6" subpath="$7"
  # Sanitize the repo name into a safe dir: drop any "org/" prefix and " (note)"
  # suffix, lowercase, keep only [a-z0-9._-].
  local base="${name##*/}"; base="${base%% (*}"
  local safe; safe="$(printf '%s' "$base" | tr '[:upper:] ' '[:lower:]-' | tr -cd 'a-z0-9._-')"
  [ -n "$safe" ] || safe="repo$j"
  local target="$DEST/$slug/$safe"
  if [ -d "$target/.git" ]; then
    echo "skip   $slug/$name (already cloned)"; total_skip=$((total_skip+1)); return 0
  fi
  echo "clone  $slug/$name  <-  $url @ $ref${subpath:+ (subpath $subpath)}"
  rm -rf "$target"; mkdir -p "$target"
  local ok=1
  if [ -n "$subpath" ]; then
    git clone --quiet --depth 1 --branch "$ref" --single-branch --filter=blob:none --sparse "$url" "$target" 2>/dev/null \
      && git -C "$target" sparse-checkout set -- "$subpath" 2>/dev/null || ok=0
  else
    git clone --quiet --depth 1 --branch "$ref" --single-branch "$url" "$target" 2>/dev/null || ok=0
  fi
  if [ "$ok" = 1 ]; then
    local got; got=$(git -C "$target" rev-parse HEAD)
    if [ "$sha" != "null" ] && [ -n "$sha" ] && [ "$got" != "$sha" ]; then
      echo "  warn: HEAD $got != pinned $sha (tag may have moved)"
    fi
    total_ok=$((total_ok+1))
  else
    echo "  FAIL: clone failed for $url @ $ref"; rm -rf "$target"; total_fail=$((total_fail+1))
  fi
}

# clone_list <file> <yq path to the language entry, "" for a shard root> <slug>
clone_list() {
  local file="$1" at="$2" slug="$3" n j
  n=$(yq "$at.repos | length" "$file")
  for j in $(seq 0 $((n - 1))); do
    clone_repo "$slug" "$j" \
      "$(yq -r "$at.repos[$j].name" "$file")" \
      "$(yq -r "$at.repos[$j].url" "$file")" \
      "$(yq -r "$at.repos[$j].ref" "$file")" \
      "$(yq -r "$at.repos[$j].sha" "$file")" \
      "$(yq -r "$at.repos[$j].subpath // \"\"" "$file")"
  done
}

mkdir -p "$DEST"

if [ -f "$MANIFEST" ]; then
  n_lang=$(yq '.languages | length' "$MANIFEST")
  for i in $(seq 0 $((n_lang - 1))); do
    lang=$(yq -r ".languages[$i].language" "$MANIFEST")
    matches_filter "$lang" || continue
    slug=$(printf '%s' "$lang" | tr '[:upper:] /' '[:lower:]--' | tr -cd 'a-z0-9-')
    clone_list "$MANIFEST" ".languages[$i]" "$slug"
  done
fi

if [ -d "$SHARDS" ]; then
  for shard in "$SHARDS"/*.yaml; do
    [ -e "$shard" ] || continue
    id=$(yq -r '.id' "$shard")
    want="$(basename "$shard" .yaml)"
    if ! printf '%s' "$id" | grep -Eq '^[a-z0-9-]+$' || [ "$id" != "$want" ]; then
      echo "  FAIL: shard $shard: id '$id' must be [a-z0-9-]+ and equal the file name"; shard_errors=$((shard_errors+1)); continue
    fi
    matches_filter "$(yq -r '.language' "$shard")" "$id" || continue
    clone_list "$shard" "" "$id"
  done
fi

echo
echo "done: $total_ok cloned, $total_skip skipped, $total_fail failed -> $DEST"
# A failed clone is reported (as before); a malformed shard fails the run.
[ "$shard_errors" -eq 0 ]
