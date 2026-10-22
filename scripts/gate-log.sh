#!/usr/bin/env bash
# Local log framing only: no dispatch, transport or receipt acceptance.
# Usage: gate-log.sh <gate-task-name> <existing gate command> [args...]
set -euo pipefail
if [ "$#" -lt 2 ] || [[ ! "$1" =~ ^[A-Za-z0-9_.:-]+$ ]]; then
  echo 'usage: gate-log.sh <gate-task-name> <command> [args...]' >&2
  exit 2
fi
gate_task="$1"
shift
printf 'fathom gate: %s START %s\n' "$gate_task" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
rc=0
"$@" || rc=$?
if [ "$rc" -eq 0 ]; then
  printf '\nfathom gate: %s PASS %s\n' "$gate_task" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
else
  printf '\nfathom gate: %s FAIL exit=%s %s\n' "$gate_task" "$rc" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
fi
exit "$rc"
