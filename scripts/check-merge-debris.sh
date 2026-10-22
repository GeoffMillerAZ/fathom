#!/usr/bin/env bash
# BE-1: scan tracked files, or only staged additions/changes and their index content.
set -euo pipefail
staged=0
case "${1:-}" in
  '') ;;
  --staged) staged=1 ;;
  *) echo 'usage: check-merge-debris.sh [--staged]' >&2; exit 2 ;;
esac
scratch=$(mktemp -d "${TMPDIR:-/tmp}/merge-debris.XXXXXX")
trap 'rm -rf "$scratch"' EXIT
if [ "$staged" -eq 1 ]; then
  git diff --cached --name-only --diff-filter=ACMR -z > "$scratch/files"
else
  git ls-files -z > "$scratch/files"
fi
rc=0
while IFS= read -r -d '' path; do
  case "/$path" in */testdata/*|/third_party/*) continue ;; esac
  case "$path" in
    *-e|*-E|*.orig|*.rej) printf '%s:1: backup or reject file\n' "$path"; rc=1 ;;
  esac
  if [ "$staged" -eq 1 ]; then
    git show ":$path" > "$scratch/content"
    content="$scratch/content"
  else
    # Deleted files and symlinks cannot contain worktree conflict lines.
    [ -f "$path" ] && [ ! -L "$path" ] || continue
    content="$path"
  fi
  code=0
  LC_ALL=C grep -nIE '^(<{7}|>{7}|[|]{7})( |$)' -- "$content" > "$scratch/matches" || code=$?
  if [ "$code" -eq 0 ]; then
    while IFS= read -r match; do printf '%s:%s\n' "$path" "$match"; done < "$scratch/matches"
    rc=1
  elif [ "$code" -ne 1 ]; then
    exit "$code"
  fi
done < "$scratch/files"
exit "$rc"
