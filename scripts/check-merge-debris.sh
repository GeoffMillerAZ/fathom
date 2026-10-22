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
paths=()
while IFS= read -r -d '' path; do
  case "/$path" in */testdata/*|/third_party/*) continue ;; esac
  case "$path" in
    *-e|*-E|*.orig|*.rej) printf '%s:1: backup or reject file\n' "$path"; rc=1 ;;
  esac
  if [ "$staged" -eq 1 ]; then paths+=(":(literal)$path"); fi
done < "$scratch/files"
mode=()
if [ "$staged" -eq 1 ]; then
  [ "${#paths[@]}" -gt 0 ] || exit "$rc"
  mode=(--cached)
fi
code=0
LC_ALL=C git grep --no-color -nIE ${mode[@]+"${mode[@]}"} '^(<{7}|>{7}|[|]{7})( |$)' -- ${paths[@]+"${paths[@]}"} ':!third_party/*' ':!*testdata/*' || code=$?
if [ "$code" -eq 0 ]; then
  rc=1
elif [ "$code" -ne 1 ]; then
  exit "$code"
fi
exit "$rc"
