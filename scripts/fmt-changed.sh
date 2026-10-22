#!/usr/bin/env bash
# BE-10: inspect tracked Go changes since this branch forked from origin/main.
set -euo pipefail
base=$(git merge-base origin/main HEAD 2>/dev/null) || {
  echo 'fmt:changed: cannot resolve merge-base with origin/main; fetch the missing ref origin/main' >&2
  exit 1
}
scratch=$(mktemp -d "${TMPDIR:-/tmp}/fmt-changed.XXXXXX")
trap 'rm -rf "$scratch"' EXIT
git diff --name-only --diff-filter=ACMR -z "$base" -- '*.go' > "$scratch/files"
files=()
while IFS= read -r -d '' path; do
  # Match Taskfile.yml GOFILES: fixture/reference/vendor and scratch trees.
  case "/$path" in
    /.claude/*|/tmp/*|/ref/*|/workspace/*|/third_party/*|*/_reference/*|*/testdata/*) continue ;;
  esac
  [ -f "$path" ] || continue
  files+=("$path")
done < "$scratch/files"
[ "${#files[@]}" -gt 0 ] || exit 0
rc=0
bad=$(command gofmt -l "${files[@]}" 2>&1) || rc=1
# gofmt names parse errors on stderr; keep them alongside formatting offenders.
if [ -n "$bad" ]; then printf '%s\n' "$bad"; rc=1; fi
exit "$rc"
