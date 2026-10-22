#!/usr/bin/env bash
# BE-10: run selected packages with the same isolation as task test.
set -euo pipefail
[ "$#" -gt 0 ] || { echo 'usage: task t [RUN=pattern] -- PACKAGE [PACKAGE...]' >&2; exit 2; }
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
mkdir -p "$root/workspace/tmp"
log=$(mktemp "$root/workspace/tmp/targeted-test.XXXXXX")
trap 'rm -f "$log"' EXIT
args=(test -race -count=1 -p 4)
if [ -n "${FATHOM_TEST_RUN:-}" ]; then args+=(-run "$FATHOM_TEST_RUN"); fi
rc=0
CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0 bash "$root/scripts/scratch-home.sh" \
  bash "$root/scripts/fathom-go.sh" "${args[@]}" "$@" > "$log" 2>&1 || rc=$?
# Default go test output already contains complete failure blocks. Buffer each
# package until its verdict so successful chatter and no-test lines stay quiet.
awk -v rc="$rc" '
  /^ok[ \t]/ { print; block=""; next }
  /^\?[ \t]/ { block=""; next }
  /^FAIL([ \t]|$)/ { printf "%s", block; print; block=""; next }
  { block=block $0 ORS }
  END { if (rc != 0) printf "%s", block }
' "$log"
exit "$rc"
