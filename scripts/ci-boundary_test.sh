#!/usr/bin/env bash
# BE-7: main runs remain independent Linux witnesses.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
grep -F 'cancel-in-progress: ${{ github.ref != '\''refs/heads/main'\'' }}' "$root/.github/workflows/ci.yml" >/dev/null || {
  echo 'ci-boundary test: main runs can still cancel each other' >&2
  exit 1
}
grep -F "group: ci-"'${{ github.ref == '\''refs/heads/main'\'' && github.sha || github.ref }}' "$root/.github/workflows/ci.yml" >/dev/null || {
  echo 'ci-boundary test: pending main runs share a concurrency group' >&2
  exit 1
}
# Commit policy is an independent job, so an invalid message cannot prevent validation.
awk '
  /^  commit-messages:/ { messages=1 }
  /^  validate:/ { validate=1 }
  /^    needs:/ { exit 1 }
  END { if (!messages || !validate) exit 1 }
' "$root/.github/workflows/ci.yml" || { echo 'ci-boundary test: message checks are not independent' >&2; exit 1; }
echo 'ci-boundary test: ok'
