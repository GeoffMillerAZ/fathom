#!/usr/bin/env bash
# BE-10: inspect current content of tracked Go changes against origin/main.
set -euo pipefail
scratch=$(mktemp -d "${TMPDIR:-/tmp}/fmt-changed.XXXXXX")
trap 'rm -rf "$scratch"' EXIT
git diff --name-only --diff-filter=ACMR -z origin/main -- '*.go' > "$scratch/files"
rc=0
while IFS= read -r -d '' path; do
  [ -f "$path" ] || continue
  bad=$(command gofmt -l "$path")
  if [ -n "$bad" ]; then printf '%s\n' "$bad"; rc=1; fi
done < "$scratch/files"
exit "$rc"
