#!/usr/bin/env bash
#
# testkit-schema-check.sh — drift guard: assert schema/testkit/{manifest,
# result}.cue matches the exact commit schema/testkit/.vendor-commit pins
# from platform-engineering/kits/test-kit's own schema/ directory (vision
# 33 / ADR-043 stage 0, TC-1). Modelled on scripts/vendor-check.sh, wired
# into `task validate` beside vendor-check — but simpler: test-kit is not a
# Go dependency of fathom (no go.mod pin to cross-check against), and only
# two files are mirrored, not a whole tree.
#
# The mirrored package line is left as `package testkit`, unchanged from
# the source: cue/load resolves a package by the DIRECTORY it lives in
# (schema/testkit/, under fathom's own root cue.mod module), never by a
# global package registry, so a same-named "testkit" package coexisting
# with test-kit's own upstream "testkit" package in a DIFFERENT repo's
# module is never a collision fathom's own `cue vet`/`cue/load` calls can
# see.
#
# Skips gracefully (exit 0, warning) when the test-kit sibling isn't
# present or doesn't have the pinned commit — expected on a machine without
# access to the private platform-engineering monorepo.
#
# Usage:
#   task testkit-schema-check
#   bash scripts/testkit-schema-check.sh
#
# Environment:
#   TEST_KIT_SIBLING   Override the sibling path
#                       (default: ../platform-engineering/kits/test-kit).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

DEST="schema/testkit"
SIBLING="${TEST_KIT_SIBLING:-../platform-engineering/kits/test-kit}"
FILES=(manifest.cue result.cue)

if [ ! -d "${SIBLING}" ] || ! git -C "${SIBLING}" rev-parse --git-dir >/dev/null 2>&1; then
  # test-kit lives NESTED inside the platform-engineering monorepo
  # (kits/test-kit), unlike agentic-kit/app-kit's own standalone sibling
  # repos — `git -C rev-parse --git-dir` finds the monorepo's .git no
  # matter how deep SIBLING sits under it, where a bare `-d .git` check
  # would not.
  echo "testkit-schema-check: ${SIBLING} not present — skipping (expected without private access)"
  exit 0
fi
if [ ! -d "${DEST}" ]; then
  echo "testkit-schema-check: ${DEST} does not exist" >&2
  exit 1
fi

commit=""
if [ -s "${DEST}/.vendor-commit" ]; then
  commit=$(tr -d '[:space:]' < "${DEST}/.vendor-commit")
else
  echo "testkit-schema-check: ${DEST}/.vendor-commit is missing" >&2
  exit 1
fi

if ! git -C "${SIBLING}" rev-parse --verify "${commit}^{commit}" >/dev/null 2>&1; then
  echo "testkit-schema-check: ${SIBLING} doesn't have ${commit} — skipping (fetch/pull it to check for real)"
  exit 0
fi

# test-kit is nested inside the platform-engineering monorepo
# (kits/test-kit) — git paths at $commit are relative to the MONOREPO
# root, not to SIBLING, so the subdirectory prefix has to be recovered
# and prepended before every `git show <commit>:<path>`.
prefix="$(git -C "${SIBLING}" rev-parse --show-prefix)"

rc=0
for f in "${FILES[@]}"; do
  if [ ! -f "${DEST}/${f}" ]; then
    echo "testkit-schema-check: ${DEST}/${f} does not exist" >&2
    rc=1
    continue
  fi
  expected=$(git -C "${SIBLING}" show "${commit}:${prefix}schema/${f}" 2>/dev/null | shasum -a 256 | awk '{print $1}')
  actual=$(shasum -a 256 "${DEST}/${f}" | awk '{print $1}')
  if [ -z "${expected}" ]; then
    echo "testkit-schema-check: ${SIBLING} has no schema/${f} at ${commit}" >&2
    rc=1
    continue
  fi
  if [ "${expected}" != "${actual}" ]; then
    echo "testkit-schema-check: ${DEST}/${f} does not match commit ${commit}" >&2
    echo "                      expected content hash ${expected}" >&2
    echo "                      got      content hash ${actual}" >&2
    rc=1
    continue
  fi
  echo "testkit-schema-check: ${DEST}/${f} matches ${commit}"
done

exit "${rc}"
