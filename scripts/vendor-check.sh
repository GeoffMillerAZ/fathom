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
#   AGENTIC_KIT_SIBLING   Override the sibling path (default: ../agentic-kit).
#   APP_KIT_SIBLING       Override the sibling path (default: ../app-kit).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

# shellcheck source=lib/kit-pin.sh
source scripts/lib/kit-pin.sh

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

# check_mirror <name> <module-path> <sibling-dir>
# Compares third_party/<name> against the sibling at the commit the mirror
# records. Skips (0) when the sibling is absent — the expected state for a
# clone without private access, which is exactly who this mirror serves.
check_mirror() {
  local name="$1" module="$2" sibling="$3"
  local dest="third_party/${name}"

  if [ ! -d "${sibling}/.git" ]; then
    echo "vendor-check: ${sibling} not present — skipping ${name} (expected without private access)"
    return 0
  fi
  if [ ! -d "${dest}" ]; then
    echo "vendor-check: ${dest} does not exist — run: task sync-kit" >&2
    return 1
  fi

  # The stamp is the source of truth for WHICH commit this mirror is.
  local commit=""
  if [ -s "${dest}/.vendor-commit" ]; then
    commit=$(tr -d '[:space:]' < "${dest}/.vendor-commit")
  else
    echo "vendor-check: ${dest}/.vendor-commit is missing — run: task sync-kit" >&2
    return 1
  fi

  # When go.mod carries a derivable pin, it must AGREE with the stamp;
  # a mirror that is internally consistent but pinned elsewhere is the
  # drift this check exists to catch.
  local pin_version pin_commit
  pin_version=$(kit_pin_version "${module}")
  if pin_commit=$(resolve_pin_commit "${sibling}" "${module}" 2>/dev/null) && [ -n "${pin_commit}" ]; then
    if [ "${pin_commit}" != "${commit}" ]; then
      echo "vendor-check: ${name} go.mod pins ${pin_version} (${pin_commit}) but ${dest} was vendored from ${commit} — run: task sync-kit" >&2
      return 1
    fi
  fi

  if ! git -C "${sibling}" rev-parse --verify "${commit}^{commit}" >/dev/null 2>&1; then
    echo "vendor-check: ${sibling} doesn't have ${commit} — skipping ${name} (fetch/pull it to check for real)"
    return 0
  fi

  local tmp expected actual
  tmp=$(mktemp -d)
  git -C "${sibling}" archive "${commit}" | tar -x -C "${tmp}"
  expected=$(content_hash "${tmp}")
  actual=$(content_hash "${dest}")
  rm -rf "${tmp}"

  if [ "${expected}" != "${actual}" ]; then
    echo "vendor-check: ${dest} does not match commit ${commit}" >&2
    echo "              expected content hash ${expected}" >&2
    echo "              got      content hash ${actual}" >&2
    echo "              run: task sync-kit" >&2
    return 1
  fi
  echo "vendor-check: ${dest} matches ${commit}${pin_version:+ (pin ${pin_version})}"
  return 0
}

rc=0
check_mirror agentic-kit github.com/geoffmilleraz/agentic-kit "${AGENTIC_KIT_SIBLING:-../agentic-kit}" || rc=1
check_mirror app-kit     github.com/geoffmilleraz/app-kit     "${APP_KIT_SIBLING:-../app-kit}"         || rc=1
exit "${rc}"
