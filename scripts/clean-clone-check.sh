#!/usr/bin/env bash
#
# clean-clone-check.sh — prove fathom still builds the way an ADOPTER gets it:
# from the committed tree alone, with no sibling checkouts and no go.work.
#
# This exists because that guarantee silently broke and nobody noticed for
# months (2026-09-01): go.work was committed pointing at ../agentic-kit and
# ../app-kit, app-kit had no in-tree mirror at all, and go.sum was missing
# entries the workspace happened to supply. Every machine that built fathom
# had both siblings, so the failure was invisible to everyone who could have
# seen it. The one person who could not — a new adopter — was the one who
# would hit it first.
#
# Run: task clean-clone-check
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

fail=0

# 1. go.work must not be tracked.
if git ls-files --error-unmatch go.work >/dev/null 2>&1; then
  echo "clean-clone-check: FAIL go.work is tracked — a clone without ../agentic-kit and ../app-kit cannot build" >&2
  fail=1
fi

# 2. Every replace must point in-tree.
while IFS= read -r line; do
  target=$(awk '{print $NF}' <<<"$line")
  case "$target" in
    ./*|third_party/*) ;;
    *) echo "clean-clone-check: FAIL replace points outside the tree: ${line}" >&2; fail=1 ;;
  esac
done < <(grep -E '^replace ' go.mod || true)

# 3. Every mirror carries the commit it was cut from.
for name in agentic-kit app-kit; do
  if [ ! -s "third_party/${name}/.vendor-commit" ]; then
    echo "clean-clone-check: FAIL third_party/${name}/.vendor-commit is missing — the mirror cannot be traced to a source commit" >&2
    fail=1
  fi
done

# 4. The real proof: build from a pristine copy of the COMMITTED tree, with
#    no siblings reachable and the workspace off.
# Tracked paths with WORKING-TREE content: a gate must check what would be
# committed, not what was committed last time — otherwise it always lags the
# fix by one commit and passes on a tree that is already broken.
tmp="$(mktemp -d "${TMPDIR:-/tmp}/fathom-clean-clone.XXXXXX")"
trap 'rm -rf "${tmp}"' EXIT
# Deleted-but-still-tracked paths would abort rsync; the gate is about what
# builds, not about staging hygiene, so skip anything not on disk.
git ls-files | while IFS= read -r f; do [ -e "$f" ] && printf '%s\n' "$f"; done > "${tmp}/.tracked"
rsync -a --files-from="${tmp}/.tracked" . "${tmp}/"
rm -f "${tmp}/.tracked"
if [ -e "${tmp}/go.work" ]; then
  echo "clean-clone-check: FAIL go.work is present among tracked files" >&2
  fail=1
fi
if ! (cd "${tmp}" && GOWORK=off go build ./... >"${tmp}/build.log" 2>&1); then
  echo "clean-clone-check: FAIL the tracked tree does not build standalone:" >&2
  head -20 "${tmp}/build.log" >&2
  fail=1
fi

if [ "${fail}" -ne 0 ]; then
  exit 1
fi
echo "clean-clone-check: the tracked tree builds standalone (no siblings, no go.work)"
