#!/usr/bin/env bash
# calibration/clean.sh — delete one queue item's calibration clones
# (docs/design/language-breadth.md §13.4 step 25, LB-F21):
#   calibration/clean.sh <id> [<id>...]   # removes workspace/calibration/<id>/
# Ids must be [a-z0-9-]+; nothing outside workspace/calibration is touched.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEST="${CALIBRATION_DEST:-$ROOT/workspace/calibration}"

[ $# -gt 0 ] || { echo "usage: calibration/clean.sh <id> [<id>...]" >&2; exit 2; }
for id in "$@"; do
  if ! printf '%s' "$id" | grep -Eq '^[a-z0-9-]+$'; then
    echo "error: id '$id' must match [a-z0-9-]+" >&2; exit 2
  fi
done
for id in "$@"; do
  if [ -d "$DEST/$id" ]; then
    size=$(du -sh "$DEST/$id" 2>/dev/null | cut -f1)
    rm -rf "${DEST:?}/$id"
    echo "removed $DEST/$id ($size)"
  else
    echo "absent  $DEST/$id"
  fi
done
