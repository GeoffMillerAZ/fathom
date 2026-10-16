#!/usr/bin/env bash
#
# sync-vendor.sh — re-archive a private sibling checkout into third_party/,
# so a clone with no access to that private repo can still build fathom
# (docs/distribution.md, ADR-019; extended to app-kit 2026-09-01).
#
# Why this exists at all: fathom requires TWO private modules, agentic-kit
# and app-kit. Before 2026-09-01 only agentic-kit was mirrored and app-kit
# was a bare `replace => ../app-kit`, so a fresh clone failed to build with
# "replacement directory ../app-kit does not exist" — ADR-019's guarantee
# was half-kept, and nobody noticed because every machine that built fathom
# happened to have both siblings.
#
# The vendored commit is recorded in third_party/<name>/.vendor-commit, the
# mirror's single pin (ADR-070): go.mod no longer names the kits at all.
#
# Since ADR-070 fathom is one self-contained module: a mirror is a plain
# directory of it, not a nested module. After archiving, the copy is
# rewritten by cmd/kitrewrite -kit: its go.mod/go.sum/go.work/go.work.sum
# move to <name>.upstream (content kept) and every import of the kit's
# upstream path becomes github.com/geoffmilleraz/fathom/third_party/<name>/...
# vendor-check reverses exactly that before comparing with the pinned commit.
# app-kit's copy is then reduced to fathom's build closure, the paths
# scripts/kit-closure.sh computes into third_party/app-kit.keep (DS-4).
#
# Usage:
#   task sync-kit                              # both mirrors, each at its sibling's HEAD
#   bash scripts/sync-vendor.sh agentic-kit [COMMIT]
#   bash scripts/sync-vendor.sh app-kit [COMMIT]
#
# Environment:
#   <NAME>_SIBLING  Override a sibling path (AGENTIC_KIT_SIBLING, APP_KIT_SIBLING).
#   KITREWRITE      A built cmd/kitrewrite (default: go run ./cmd/kitrewrite).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

name="${1:-}"
case "$name" in
  agentic-kit) env_var="AGENTIC_KIT_SIBLING" ;;
  app-kit)     env_var="APP_KIT_SIBLING" ;;
  *) echo "sync-vendor: usage: sync-vendor.sh agentic-kit|app-kit [COMMIT]" >&2; exit 2 ;;
esac

SIBLING="${!env_var:-../${name}}"
DEST="third_party/${name}"

if [ ! -d "${SIBLING}/.git" ]; then
  echo "sync-vendor: ${SIBLING} is not a git checkout — clone github.com/geoffmilleraz/${name} there first" >&2
  exit 1
fi

# The pin: an explicit commit, else the sibling's HEAD.
commit="${2:-$(git -C "${SIBLING}" rev-parse HEAD)}"
if ! commit=$(git -C "${SIBLING}" rev-parse --verify "${commit}^{commit}" 2>/dev/null); then
  echo "sync-vendor: ${SIBLING} doesn't have commit ${2:-HEAD} — fetch/pull it there first" >&2
  exit 1
fi
echo "sync-vendor: ${name} -> commit ${commit}"

kitrewrite() {
  if [ -n "${KITREWRITE:-}" ]; then
    "${KITREWRITE}" "$@"
  else
    GOWORK=off go run ./cmd/kitrewrite "$@"
  fi
}

# Archive and rewrite in a scratch copy first, so a failed rewrite never
# leaves a half-converted mirror in place.
mkdir -p workspace/tmp
stage=$(mktemp -d workspace/tmp/sync-vendor.XXXXXX)
trap 'rm -rf "${stage}"' EXIT
mkdir -p "${stage}/${name}"
git -C "${SIBLING}" archive "${commit}" | tar -x -C "${stage}/${name}"
kitrewrite -kit "${stage}/${name}"

rm -rf "${DEST}"
mv "${stage}/${name}" "${DEST}"

# The stamp is what vendor-check compares against; without it a mirror is
# just a pile of files nobody can prove came from anywhere.
printf '%s\n' "${commit}" > "${DEST}/.vendor-commit"

# app-kit's copy keeps only fathom's build closure (DS-4): the closure is
# computed against the full rewritten copy just placed (so a package fathom
# newly reaches is found), written to third_party/app-kit.keep, and every
# other file is removed. The module zip ships what the mirror holds.
if [ "${name}" = "app-kit" ]; then
  bash "${SCRIPT_DIR}/kit-closure.sh" --write
  bash "${SCRIPT_DIR}/kit-closure.sh" --prune
fi

file_count=$(find "${DEST}" -type f | wc -l | tr -d ' ')
size=$(du -sh "${DEST}" | awk '{print $1}')
echo "sync-vendor: wrote ${file_count} files (${size}) to ${DEST} @ ${commit}"
