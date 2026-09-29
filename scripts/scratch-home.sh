#!/usr/bin/env bash
# scratch-home.sh CMD [ARGS...] — PE-28 (docs/plans/progress-and-economics.md,
# observation O121): run CMD with FATHOM_HOME pointing at a fresh directory
# under workspace/tmp/, removed when CMD ends, so a gate or scratch run that
# reaches a per-user ledger, log or daemon record writes there and never to
# the operator's real ~/.fathom. A FATHOM_HOME already in the environment
# wins (a worker that named its own keeps it, and is not deleted). The
# command's exit status passes through.

set -euo pipefail

if [ "$#" -eq 0 ]; then
  echo "usage: scratch-home.sh CMD [ARGS...]" >&2
  exit 2
fi

if [ -z "${FATHOM_HOME:-}" ]; then
  root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
  mkdir -p "${root}/workspace/tmp"
  FATHOM_HOME="$(mktemp -d "${root}/workspace/tmp/fathom-home.XXXXXX")"
  export FATHOM_HOME
  trap 'rm -rf "$FATHOM_HOME"' EXIT
fi

"$@"
