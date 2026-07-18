#!/usr/bin/env bash
#
# sync-kit.sh — re-archive the ../agentic-kit sibling checkout at the exact
# commit go.mod pins into third_party/agentic-kit, so a clone with no access
# to the private agentic-kit repo can still build fathom (docs/distribution.md,
# ADR-019). Idempotent — re-run any time the go.mod pin moves and commit the
# result.
#
# Refuses if the sibling checkout doesn't exist, or exists but lacks the
# pinned commit (fetch/pull it there first — this script never fetches for
# you).
#
# Usage:
#   task sync-kit
#   bash scripts/sync-kit.sh
#
# Environment:
#   AGENTIC_KIT_SIBLING   Override the sibling checkout path (default: ../agentic-kit).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

# shellcheck source=lib/kit-pin.sh
source scripts/lib/kit-pin.sh

SIBLING="${AGENTIC_KIT_SIBLING:-../agentic-kit}"
DEST="third_party/agentic-kit"

if [ ! -d "${SIBLING}/.git" ]; then
  echo "sync-kit: ${SIBLING} is not a git checkout — clone github.com/geoffmilleraz/agentic-kit there first" >&2
  exit 1
fi

pin_version=$(kit_pin_version)
if [ -z "${pin_version}" ]; then
  echo "sync-kit: go.mod has no github.com/geoffmilleraz/agentic-kit requirement" >&2
  exit 1
fi

commit=$(resolve_pin_commit "${SIBLING}") || {
  echo "sync-kit: ${SIBLING} doesn't have the commit go.mod pins (${pin_version})" >&2
  echo "          fetch/pull ${SIBLING} so it has that commit, then retry" >&2
  exit 1
}

echo "sync-kit: pin ${pin_version} -> commit ${commit}"
rm -rf "${DEST}"
mkdir -p "${DEST}"
git -C "${SIBLING}" archive "${commit}" | tar -x -C "${DEST}"

file_count=$(find "${DEST}" -type f | wc -l | tr -d ' ')
size=$(du -sh "${DEST}" | awk '{print $1}')
echo "sync-kit: wrote ${file_count} files (${size}) to ${DEST}"
echo "sync-kit: review with 'git status ${DEST}' and commit"
