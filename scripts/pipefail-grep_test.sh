#!/usr/bin/env bash
# pipefail-grep_test.sh — a value piped into `grep -q` under pipefail fails the
# pipeline when grep exits on its first match while the writer still has bytes
# to write (SIGPIPE, status 141). bash's printf and echo flush a value larger
# than the stdio buffer in several writes, so a self-check output of ~1.6 KiB
# failed `task scripts-check` at the gate's nice 10 on a loaded machine
# (tenancy-stress_test.sh, 2026-09-28; the same pipeline failed 12 of 3200 runs
# at nice 10 beside 24 busy loops and 0 of 9600 at normal priority). The fix is
# a here-string, `grep -q pattern <<<"$value"`, which has no writer to signal.
#
# This test fails a scripts/*_test.sh that sets pipefail and pipes a printf or
# echo into `grep -q`. It reads the scripts/ tests only: the vendored
# third_party/agentic-kit copies pipe values under 512 bytes (a single write).
# Run by `task scripts-check`.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"

self="$(basename "${BASH_SOURCE[0]}")"
bad=0
for f in "${SCRIPT_DIR}"/*_test.sh; do
  [ "$(basename "$f")" = "$self" ] && continue
  grep -q 'pipefail' "$f" || continue
  hits="$(grep -nE '^[^#]*(printf|echo)[^|]*\|[[:space:]]*grep[[:space:]]+-[A-Za-z]*q' "$f" || true)"
  if [ -n "$hits" ]; then
    echo "pipefail-grep test: $(basename "$f") pipes a printf/echo into grep -q under pipefail (SIGPIPE flake; use grep -q ... <<<\"\$value\"):" >&2
    printf '%s\n' "$hits" >&2
    bad=1
  fi
done
[ "$bad" -eq 0 ] || exit 1

echo "pipefail-grep test: ok (no scripts/*_test.sh pipes a printf or echo into grep -q under pipefail)"
