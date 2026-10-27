#!/usr/bin/env bash
# temp-path-canary.sh CMD [ARGS...] -- run CMD with TMPDIR set to a symlink
# whose name holds both "_" and "." (fathom_tmp.d.XXXXXX -> a real sibling
# directory), so every test that assumes something about the temp path runs
# under it: a symlinked prefix (macOS /var/folders, issue 118) and a name
# with "_" and "." (CI's /home/runner/work/_temp). GOTMPDIR is set to the
# same symlink, since a Go test's t.TempDir reads GOTMPDIR before TMPDIR: a
# caller's own GOTMPDIR would otherwise bypass the canary (issue 120). The
# base is the caller's TMPDIR (else /tmp); both entries are removed
# afterwards and CMD's exit status passes through. Never inside the module
# tree.

set -euo pipefail

if [ "$#" -eq 0 ]; then
  echo "usage: temp-path-canary.sh CMD [ARGS...]" >&2
  exit 2
fi

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
base="${TMPDIR:-/tmp}"
base="${base%/}"
[ -n "$base" ] || base=/tmp
case "$(cd "$base" && pwd -P)/" in
  "$root"/*) base=/tmp ;;
esac

real="$(mktemp -d "$base/fathom_tmp.d-real.XXXXXX")"
link="$base/fathom_tmp.d.$(basename "$real" | sed 's/.*\.//')"
trap 'rm -rf "$link" "$real"' EXIT
ln -s "$real" "$link"

TMPDIR="$link" GOTMPDIR="$link" "$@"
