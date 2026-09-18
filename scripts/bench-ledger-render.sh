#!/usr/bin/env bash
#
# scripts/bench-ledger-render.sh — re-render docs/bench/README.md's results
# table from the committed JSONL ledger. Free: it reads the ledger, it never
# runs the bench (that is `task bench-home`, which spends real tokens).
#
# The rendered table lives between the two marker comments in the README so
# the prose around it survives every re-render.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=scripts/lib/build-cache.sh
source "${ROOT}/scripts/lib/build-cache.sh"
fathom_cache_configure || exit $?
LEDGER="${1:-$ROOT/docs/bench/ledger.jsonl}"
README="$ROOT/docs/bench/README.md"
BEGIN="<!-- BEGIN ledger table (task bench-ledger) -->"
END="<!-- END ledger table -->"

if [ ! -f "$README" ]; then
  echo "bench-ledger-render: $README not found" >&2
  exit 2
fi

TABLE="$(GOWORK=off CGO_ENABLED=1 go run "$ROOT/cmd/benchledgerrender" -ledger "$LEDGER")" || {
  echo "bench-ledger-render: rendering failed" >&2
  exit 1
}

BEGIN="$BEGIN" END="$END" TABLE="$TABLE" python3 - "$README" <<'PY'
import os, sys
path = sys.argv[1]
begin, end, table = os.environ["BEGIN"], os.environ["END"], os.environ["TABLE"]
text = open(path).read()
i, j = text.find(begin), text.find(end)
if i < 0 or j < 0 or j < i:
    sys.exit(f"bench-ledger-render: markers not found in {path}")
open(path, "w").write(text[: i + len(begin)] + "\n\n" + table.rstrip() + "\n\n" + text[j:])
print(f"bench-ledger-render: rewrote the results table in {path}")
PY
