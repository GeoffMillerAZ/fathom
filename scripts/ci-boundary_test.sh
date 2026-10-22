#!/usr/bin/env bash
# BE-7: main runs remain independent Linux witnesses.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
grep -F 'cancel-in-progress: ${{ github.ref != '\''refs/heads/main'\'' }}' "$root/.github/workflows/ci.yml" >/dev/null || {
  echo 'ci-boundary test: main runs can still cancel each other' >&2
  exit 1
}
echo 'ci-boundary test: ok'
