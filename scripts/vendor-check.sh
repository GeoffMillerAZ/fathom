#!/usr/bin/env bash
#
# vendor-check.sh — drift guard: assert third_party/agentic-kit matches the
# exact commit go.mod pins for github.com/geoffmilleraz/agentic-kit. Wired
# into `task validate` / lefthook's pre-push gate so a kit pin bump without a
# `task sync-kit` can't push (ADR-019, docs/distribution.md).
#
# Compares *content* — sha256 of each file, keyed by relative path, sorted,
# hashed together — rather than raw tar bytes. `git archive` embeds the
# archived commit's timestamp in every tar entry, so the pinned-commit
# archive and the previously-synced third_party/ tree would hash differently
# byte-for-byte even when every file's contents are identical; content
# hashing sidesteps that (and any BSD-tar-vs-GNU-tar differences) and is
# deterministic regardless of tool or platform.
#
# Skips gracefully (exit 0, warning) when ../agentic-kit isn't present or
# doesn't have the pinned commit — expected on a consumer machine that only
# has third_party/agentic-kit and no access to the private kit repo.
#
# Usage:
#   task vendor-check
#   bash scripts/vendor-check.sh
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

content_hash() {
  # sha256 of each file's contents, keyed by its path relative to $1, sorted
  # by path, then hashed together into one digest.
  local dir="$1"
  find "${dir}" -type f -print0 \
    | LC_ALL=C sort -z \
    | while IFS= read -r -d '' f; do
        rel="${f#"${dir}"/}"
        printf '%s  %s\n' "$(shasum -a 256 "${f}" | awk '{print $1}')" "${rel}"
      done \
    | shasum -a 256 | awk '{print $1}'
}

if [ ! -d "${SIBLING}/.git" ]; then
  echo "vendor-check: ${SIBLING} not present — skipping (expected without kit access)"
  exit 0
fi

pin_version=$(kit_pin_version)
commit=$(resolve_pin_commit "${SIBLING}") || {
  echo "vendor-check: ${SIBLING} doesn't have the commit go.mod pins (${pin_version}) — skipping"
  echo "              fetch/pull ${SIBLING} to check for real"
  exit 0
}

if [ ! -d "${DEST}" ]; then
  echo "vendor-check: ${DEST} does not exist — run: task sync-kit" >&2
  exit 1
fi

tmp=$(mktemp -d)
trap 'rm -rf "${tmp}"' EXIT
git -C "${SIBLING}" archive "${commit}" | tar -x -C "${tmp}"

expected=$(content_hash "${tmp}")
actual=$(content_hash "${DEST}")

if [ "${expected}" != "${actual}" ]; then
  echo "vendor-check: ${DEST} does not match pinned commit ${commit} (pin ${pin_version})" >&2
  echo "              expected content hash ${expected}" >&2
  echo "              got      content hash ${actual}" >&2
  echo "              run: task sync-kit" >&2
  exit 1
fi

echo "vendor-check: ${DEST} matches pinned commit ${commit} (pin ${pin_version})"
