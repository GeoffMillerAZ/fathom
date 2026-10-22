#!/usr/bin/env bash
# Frame local gate evidence and let the receipt writer publish a successful log.
# Usage: gate-log.sh <gate-task-name> <existing gate command> [args...]
set -euo pipefail
if [ "$#" -lt 2 ] || [[ ! "$1" =~ ^[A-Za-z0-9_.:-]+$ ]]; then
  echo 'usage: gate-log.sh <gate-task-name> <command> [args...]' >&2
  exit 2
fi
gate_task="$1"
shift
# Go's automatic toolchain download is never authorized by a gate header.
export GOTOOLCHAIN=local
snapshot() {
  local tree commit clean=0 status
  tree=$(git rev-parse --verify 'HEAD^{tree}') || return 1
  commit=$(git rev-parse --verify HEAD) || return 1
  status=$(git status --porcelain=v1 --untracked-files=all --ignore-submodules=none) || return 1
  [ -n "$status" ] || clean=1
  printf 'tree=%s clean=%s commit=%s' "$tree" "$clean" "$commit"
}
start=$(snapshot)
mkdir -p workspace/logs
log=$(mktemp workspace/logs/gate.XXXXXX)
# The pipeline finishes before the writer reads the log; no asynchronous tee.
set +e
(
  printf 'fathom gate: %s START %s %s\n' "$gate_task" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$start"
  go version || exit $?
  printf 'fathom gate: go env GOOS GOARCH GOWORK CGO_ENABLED GOFLAGS\n'
  go env GOOS GOARCH GOWORK CGO_ENABLED GOFLAGS || exit $?
  rc=0
  "$@" || rc=$?
  finish=$(snapshot) || { finish='tree=unknown clean=0 commit=unknown'; rc=1; }
  if [ "$start" != "$finish" ] || [[ "$start" != *' clean=1 '* ]] || [[ "$finish" != *' clean=1 '* ]]; then
    [ "$rc" -ne 0 ] || rc=1
  fi
  if [ "$rc" -eq 0 ]; then
    printf '\nfathom gate: %s PASS %s %s\n' "$gate_task" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$finish"
  else
    printf '\nfathom gate: %s FAIL exit=%s %s %s\n' "$gate_task" "$rc" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$finish"
  fi
  exit "$rc"
) | tee "$log"
statuses=("${PIPESTATUS[@]}")
rc=${statuses[0]}
[ "${statuses[1]}" -eq 0 ] || rc=${statuses[1]}
set -e
if [ "$rc" -eq 0 ]; then
  receipt_bin=fathom
  [ ! -x bin/fathom ] || receipt_bin=bin/fathom
  "$receipt_bin" gate receipt write --command "$gate_task" --log "$log" >/dev/null
fi
exit "$rc"
