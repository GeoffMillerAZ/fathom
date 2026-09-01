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
# The vendored commit is recorded in third_party/<name>/.vendor-commit
# rather than derived from the go.mod pin. app-kit has no published version
# to derive one from (it is `v0.0.0` + replace), and a stamp needs no
# network for anyone — which is the whole point of a mirror.
#
# Usage:
#   task sync-kit                       # both mirrors
#   bash scripts/sync-vendor.sh agentic-kit
#   bash scripts/sync-vendor.sh app-kit
#
# Environment:
#   <NAME>_SIBLING  Override a sibling path (AGENTIC_KIT_SIBLING, APP_KIT_SIBLING).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

source scripts/lib/kit-pin.sh

name="${1:-}"
case "$name" in
  agentic-kit) module="github.com/geoffmilleraz/agentic-kit"; env_var="AGENTIC_KIT_SIBLING" ;;
  app-kit)     module="github.com/geoffmilleraz/app-kit";     env_var="APP_KIT_SIBLING" ;;
  *) echo "sync-vendor: usage: sync-vendor.sh agentic-kit|app-kit" >&2; exit 2 ;;
esac

SIBLING="${!env_var:-../${name}}"
DEST="third_party/${name}"

if [ ! -d "${SIBLING}/.git" ]; then
  echo "sync-vendor: ${SIBLING} is not a git checkout — clone github.com/geoffmilleraz/${name} there first" >&2
  exit 1
fi

# Prefer the commit go.mod pins when it is derivable (agentic-kit carries a
# real pseudo-version); fall back to the sibling's HEAD (app-kit is v0.0.0).
commit=""
if pin_commit=$(resolve_pin_commit "${SIBLING}" "${module}" 2>/dev/null) && [ -n "${pin_commit}" ]; then
  commit="${pin_commit}"
  echo "sync-vendor: ${name} pin $(kit_pin_version "${module}") -> commit ${commit}"
else
  commit=$(git -C "${SIBLING}" rev-parse HEAD)
  echo "sync-vendor: ${name} has no derivable pin ($(kit_pin_version "${module}")) — vendoring sibling HEAD ${commit}"
fi

if ! git -C "${SIBLING}" rev-parse --verify "${commit}^{commit}" >/dev/null 2>&1; then
  echo "sync-vendor: ${SIBLING} doesn't have commit ${commit} — fetch/pull it there first" >&2
  exit 1
fi

rm -rf "${DEST}"
mkdir -p "${DEST}"
git -C "${SIBLING}" archive "${commit}" | tar -x -C "${DEST}"

# The stamp is what vendor-check compares against; without it a mirror is
# just a pile of files nobody can prove came from anywhere.
printf '%s\n' "${commit}" > "${DEST}/.vendor-commit"

file_count=$(find "${DEST}" -type f | wc -l | tr -d ' ')
size=$(du -sh "${DEST}" | awk '{print $1}')
echo "sync-vendor: wrote ${file_count} files (${size}) to ${DEST} @ ${commit}"
