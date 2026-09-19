#!/usr/bin/env bash
#
# push-check.sh — INC-20 (docs/intent/incidents.md), ADR-041 decision 10,
# invariant I40 (docs/intent/invariants.md): refuse to push from a clone
# whose pre-push hook is not lefthook's. Two real pushes reached origin/main
# ungated on 2026-09-19 after a scratch push clone's `.git/hooks/pre-push`
# (and `.git/HEAD`) were wiped by an unrelated process, so `task validate`
# never ran before either push — both push logs were two lines long, the
# tell of a gate that never fired. This script is the gate that cannot be
# skipped by a wiped hooks directory: it is not itself the pre-push hook,
# it is what `task push` runs BEFORE handing off to `git push`.
#
# Resolves the hook path through `git rev-parse --git-path hooks/pre-push`
# rather than hardcoding ".git/hooks/pre-push" directly: that honors
# `core.hooksPath` when set (INC-20's own "unless hooksPath points
# elsewhere, then check there") and resolves correctly from a linked
# worktree too (Gate hooks and worktrees: pushes sometimes happen from a
# detached worktree in this project), which a literal ".git/hooks" path
# would not.
#
# Usage:
#   task push            (runs this, then `git push origin HEAD:main`)
#   bash scripts/push-check.sh
#
# Exit codes:
#   0   lefthook's pre-push hook is present — safe to push.
#   3   refused: the hook is missing, or present but not lefthook's.
#   2   usage/operational error (not a git repository, git not on PATH).

set -euo pipefail

if ! command -v git >/dev/null 2>&1; then
  echo "push-check: refused — git not found on PATH" >&2
  exit 2
fi

# Deliberately no `cd` to this script's own directory: `git push` runs from
# wherever the OPERATOR currently is, so the repo/worktree/hooksPath this
# check must gate is $PWD's, not wherever scripts/push-check.sh happens to
# live on disk (the two are the same in the ordinary `task push` case, but
# never assumed).
if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "push-check: refused — $(pwd) is not a git repository" >&2
  exit 2
fi

# git-path is core.hooksPath-aware and worktree-aware; it names where THIS
# checkout's pre-push hook actually lives, never assumed — and, per its own
# docs, relative to $PWD (never necessarily the repo's top level), so it is
# resolved from here, not from `git rev-parse --show-toplevel`.
hook_rel="$(git rev-parse --git-path hooks/pre-push)"
if [[ "${hook_rel}" = /* ]]; then
  hook_path="${hook_rel}"
else
  hook_dir="$(cd "$(dirname "${hook_rel}")" && pwd)"
  hook_path="${hook_dir}/$(basename "${hook_rel}")"
fi

if [ ! -e "${hook_path}" ]; then
  echo "push-check: refused — no pre-push hook at ${hook_path} (run: lefthook install)" >&2
  exit 3
fi
if [ ! -s "${hook_path}" ]; then
  echo "push-check: refused — ${hook_path} is empty, not a lefthook hook (run: lefthook install)" >&2
  exit 3
fi
if ! grep -qi "lefthook" "${hook_path}"; then
  echo "push-check: refused — ${hook_path} exists but is not lefthook's pre-push hook (run: lefthook install)" >&2
  exit 3
fi

echo "push-check: ok (lefthook pre-push present)"
