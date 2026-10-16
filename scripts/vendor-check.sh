#!/usr/bin/env bash
#
# vendor-check.sh — drift guard: assert each kit mirror under third_party/
# (agentic-kit, app-kit) is exactly its upstream archive at the commit its
# .vendor-commit records, the mirror's single pin (ADR-019, ADR-070). Wired
# into `task validate` / lefthook's pre-push gate so a hand edit to a mirror,
# or a sync that skipped the rewrite, can't push.
#
# A mirror is not byte-identical to upstream: since ADR-070 it is packages of
# fathom's module, its go.mod/go.sum/go.work/go.work.sum kept as
# <name>.upstream and its imports rewritten to
# github.com/geoffmilleraz/fathom/third_party/<name>/... So the check copies
# the mirror, reverses that rewrite (`cmd/kitrewrite -reverse -kit`, which
# restores the upstream form byte for byte, DS-2) and compares the copy.
#
# Compares *content* — sha256 of each file, keyed by relative path, sorted,
# hashed together — rather than raw tar bytes. `git archive` embeds the
# archived commit's timestamp in every tar entry, so the pinned-commit
# archive and the previously-synced third_party/ tree would hash differently
# byte-for-byte even when every file's contents are identical; content
# hashing sidesteps that (and any BSD-tar-vs-GNU-tar differences) and is
# deterministic regardless of tool or platform.
#
# Skips gracefully (exit 0, warning) when the sibling checkout isn't present
# or doesn't have the pinned commit — expected on a consumer machine that
# only has third_party/ and no access to the private kit repos.
#
# Usage:
#   task vendor-check
#   bash scripts/vendor-check.sh
#
# Environment:
#   AGENTIC_KIT_SIBLING   Override the sibling path (default: ../agentic-kit).
#   APP_KIT_SIBLING       Override the sibling path (default: ../app-kit).
#   KITREWRITE            A built cmd/kitrewrite (default: one built here once).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

WORK=$(mktemp -d)
trap 'rm -rf "${WORK}"' EXIT

kitrewrite() {
  if [ -z "${KITREWRITE:-}" ]; then
    KITREWRITE="${WORK}/kitrewrite"
    GOWORK=off go build -o "${KITREWRITE}" ./cmd/kitrewrite
  fi
  "${KITREWRITE}" "$@"
}

content_hash() {
  # sha256 of each file's contents, keyed by its path relative to $1, sorted
  # by path, then hashed together into one digest.
  local dir="$1"
  # .vendor-commit is fathom's own provenance stamp, written by sync-vendor
  # and never present in the upstream archive — exclude it from the compare
  # or every mirror reads as drifted the moment it is traceable.
  find "${dir}" -type f ! -name .vendor-commit -print0 \
    | LC_ALL=C sort -z \
    | while IFS= read -r -d '' f; do
        rel="${f#"${dir}"/}"
        printf '%s  %s\n' "$(shasum -a 256 "${f}" | awk '{print $1}')" "${rel}"
      done \
    | shasum -a 256 | awk '{print $1}'
}

# check_mirror <name> <sibling-dir>
# Compares third_party/<name>, reverse-rewritten, against the sibling at the
# commit the mirror records. Skips (0) when the sibling is absent — the
# expected state for a clone without private access, which is exactly who
# this mirror serves.
check_mirror() {
  local name="$1" sibling="$2"
  local dest="third_party/${name}"

  if [ ! -d "${sibling}/.git" ]; then
    echo "vendor-check: ${sibling} not present — skipping ${name} (expected without private access)"
    return 0
  fi
  if [ ! -d "${dest}" ]; then
    echo "vendor-check: ${dest} does not exist — run: task sync-kit" >&2
    return 1
  fi

  if [ -e "${dest}/go.mod" ]; then
    echo "vendor-check: ${dest} holds its own go.mod: an unrewritten copy is a nested module, not packages of this one (ADR-070) — run: task sync-kit" >&2
    return 1
  fi

  # The stamp is the pin: the source of truth for WHICH commit this mirror is.
  local commit=""
  if [ -s "${dest}/.vendor-commit" ]; then
    commit=$(tr -d '[:space:]' < "${dest}/.vendor-commit")
  else
    echo "vendor-check: ${dest}/.vendor-commit is missing — run: task sync-kit" >&2
    return 1
  fi

  if ! git -C "${sibling}" rev-parse --verify "${commit}^{commit}" >/dev/null 2>&1; then
    echo "vendor-check: ${sibling} doesn't have ${commit} — skipping ${name} (fetch/pull it to check for real)"
    return 0
  fi

  local up mirror expected actual
  up="${WORK}/upstream-${name}"
  mirror="${WORK}/mirror/${name}"
  mkdir -p "${up}" "$(dirname "${mirror}")"
  git -C "${sibling}" archive "${commit}" | tar -x -C "${up}"
  cp -R "${dest}" "${mirror}"
  if ! kitrewrite -reverse -kit "${mirror}" >/dev/null; then
    echo "vendor-check: ${dest} could not be reverse-rewritten to its upstream form — run: task sync-kit" >&2
    return 1
  fi
  expected=$(content_hash "${up}")
  actual=$(content_hash "${mirror}")

  if [ "${expected}" != "${actual}" ]; then
    echo "vendor-check: ${dest} (reverse-rewritten) does not match commit ${commit}" >&2
    echo "              expected content hash ${expected}" >&2
    echo "              got      content hash ${actual}" >&2
    echo "              run: task sync-kit" >&2
    return 1
  fi
  echo "vendor-check: ${dest} matches ${commit} (reverse-rewritten)"
  return 0
}

rc=0
check_mirror agentic-kit "${AGENTIC_KIT_SIBLING:-../agentic-kit}" || rc=1
check_mirror app-kit     "${APP_KIT_SIBLING:-../app-kit}"         || rc=1
exit "${rc}"
