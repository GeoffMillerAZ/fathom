#!/usr/bin/env bash
# calibration/clone.sh — shallow-clone the calibration corpus into workspace/ (gitignored).
#
# Reads calibration/corpus.yaml and clones each repo at its pinned tag/SHA into
# workspace/calibration/<language>/<repo>. Clones are --depth 1 single-branch, so
# the whole corpus stays small and reproducible. Re-running skips repos already
# present (idempotent).
#
# Usage:
#   calibration/clone.sh                 # clone the whole corpus
#   calibration/clone.sh Python Helm     # clone only the named language(s)/tool(s)
#
# Requires: git, yq.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MANIFEST="$ROOT/calibration/corpus.yaml"
DEST="$ROOT/workspace/calibration"

command -v yq >/dev/null || { echo "error: yq is required" >&2; exit 1; }
[ -f "$MANIFEST" ] || { echo "error: manifest not found: $MANIFEST" >&2; exit 1; }

# Optional language filter (case-insensitive substring match on the language name).
FILTERS=("$@")
matches_filter() {
  [ ${#FILTERS[@]} -eq 0 ] && return 0
  local lang_lc; lang_lc="$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')"
  local f
  for f in "${FILTERS[@]}"; do
    case "$lang_lc" in *"$(printf '%s' "$f" | tr '[:upper:]' '[:lower:]')"*) return 0;; esac
  done
  return 1
}

mkdir -p "$DEST"
n_lang=$(yq '.languages | length' "$MANIFEST")
total_ok=0; total_fail=0; total_skip=0

for i in $(seq 0 $((n_lang - 1))); do
  lang=$(yq -r ".languages[$i].language" "$MANIFEST")
  matches_filter "$lang" || continue
  slug=$(printf '%s' "$lang" | tr '[:upper:] /' '[:lower:]--' | tr -cd 'a-z0-9-')
  n_repo=$(yq ".languages[$i].repos | length" "$MANIFEST")
  for j in $(seq 0 $((n_repo - 1))); do
    name=$(yq -r ".languages[$i].repos[$j].name" "$MANIFEST")
    url=$(yq  -r ".languages[$i].repos[$j].url"  "$MANIFEST")
    ref=$(yq  -r ".languages[$i].repos[$j].ref"  "$MANIFEST")
    sha=$(yq  -r ".languages[$i].repos[$j].sha"  "$MANIFEST")
    # Sanitize the repo name into a safe dir: drop any "org/" prefix and " (note)"
    # suffix, lowercase, keep only [a-z0-9._-].
    base="${name##*/}"; base="${base%% (*}"
    safe="$(printf '%s' "$base" | tr '[:upper:] ' '[:lower:]-' | tr -cd 'a-z0-9._-')"
    [ -n "$safe" ] || safe="repo$j"
    target="$DEST/$slug/$safe"
    if [ -d "$target/.git" ]; then
      echo "skip   $lang/$name (already cloned)"; total_skip=$((total_skip+1)); continue
    fi
    echo "clone  $lang/$name  <-  $url @ $ref"
    rm -rf "$target"; mkdir -p "$target"
    if git clone --quiet --depth 1 --branch "$ref" --single-branch "$url" "$target" 2>/dev/null; then
      got=$(git -C "$target" rev-parse HEAD)
      if [ "$sha" != "null" ] && [ -n "$sha" ] && [ "$got" != "$sha" ]; then
        echo "  warn: HEAD $got != pinned $sha (tag may have moved)"
      fi
      total_ok=$((total_ok+1))
    else
      echo "  FAIL: clone failed for $url @ $ref"; rm -rf "$target"; total_fail=$((total_fail+1))
    fi
  done
done

echo
echo "done: $total_ok cloned, $total_skip skipped, $total_fail failed -> $DEST"
