#!/usr/bin/env bash
# test-boundary.sh — score the local selection against a failing gate (TQ-37,
# ADR-090). `task test` runs it when the suite fails: the full run is the
# oracle, so each failure it finds says whether `task test:affected` would
# have found it first.
#
#   bash scripts/test-boundary.sh LOG...
#
# LOG is go test output (`--- FAIL: TestX` lines, then the package's `FAIL
# <import path>` line; scripts/test-shards.sh prints the same). It scores the
# selection test-affected.sh would have run: the change range is the
# merge-base of HEAD and origin/main, the observation the same one the
# runner reads (this checkout's, else the main checkout's of a linked
# worktree), and the index the scratch store the pre-push verify built
# before this step (workspace/tmp/gate-verify.sqlite) when there is one,
# else the daemon's; the row records whether that index holds HEAD. With no
# observation the runner would have run the whole suite, and with no change
# there is nothing to select, so neither is scored. Prints one line, `tests
# select boundary: K of M failing test(s) selected` with the missed ones
# named, and appends the row to workspace/artifacts/tests-select/boundary.jsonl
# in the main checkout (a throwaway gate clone's row is printed only).
# Advisory: it always exits 0 and never changes the gate's verdict; a step
# it cannot take is named.

set -uo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$root" || exit 0
fathom_bin=${FATHOM_BIN:-bin/fathom}
[ -x "$fathom_bin" ] || fathom_bin=fathom
[ "$#" -gt 0 ] || { echo "tests select boundary: no log named; not scored"; exit 0; }
command -v "$fathom_bin" >/dev/null 2>&1 || [ -x "$fathom_bin" ] || { echo "tests select boundary: no fathom binary; not scored"; exit 0; }

since=$(git merge-base HEAD origin/main 2>/dev/null) || { echo "tests select boundary: no merge-base with origin/main; not scored"; exit 0; }
if [ -z "$( { git diff --name-only "$since"; git ls-files --others --exclude-standard; } 2>/dev/null)" ]; then
  echo "tests select boundary: nothing changed since ${since:0:12}; not scored"
  exit 0
fi
inputs="$root/workspace/artifacts/test-inputs"
main=$(git worktree list --porcelain 2>/dev/null | sed -n '1s/^worktree //p')
if [ ! -f "$inputs/meta.json" ] && [ -n "$main" ] && [ -f "$main/workspace/artifacts/test-inputs/meta.json" ]; then
  inputs="$main/workspace/artifacts/test-inputs"
fi
if [ ! -f "$inputs/meta.json" ]; then
  echo "tests select boundary: no test-input observation, so task test:affected would have run the whole suite; not scored"
  exit 0
fi
ledger_root="$root"
common=$(git rev-parse --path-format=absolute --git-common-dir 2>/dev/null) && [ "$(basename "$common")" = .git ] && ledger_root=$(dirname "$common")
store_args=()
[ -f workspace/tmp/gate-verify.sqlite ] && store_args=(--store workspace/tmp/gate-verify.sqlite)
mkdir -p workspace/tmp
sel=$(mktemp workspace/tmp/test-boundary.XXXXXX)
trap 'rm -f "$sel" "$sel.err"' EXIT
if ! "$fathom_bin" tests select --since "$since" --safe --json --record=false --service "${FATHOM_SERVICE:-fathom}" --inputs "$inputs" ${store_args[@]+"${store_args[@]}"} >"$sel" 2>"$sel.err"; then
  echo "tests select boundary: fathom tests select failed ($(sed -n 1p "$sel.err")); not scored"
  exit 0
fi
module=$(sed -n 's/^module //p' go.mod | head -1)
python3 -I - "$sel" "$module" "$since" "$(git rev-parse HEAD)" "$ledger_root" "$@" <<'PY' || echo "tests select boundary: could not score the logs; not scored"
import json, os, re, sys, datetime
sel_path, module, since, head, ledger_root, logs = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4], sys.argv[5], sys.argv[6:]
d = json.load(open(sel_path))
whole, named = set(), set()
for inv in d.get("invocations") or []:
    pkg = inv.get("package") or "."
    if inv.get("whole"):
        whole.add(pkg)
    for t in inv.get("tests") or []:
        named.add((pkg, t))
failing = []
fail_line = re.compile(r"^\s*--- FAIL: (Test[^\s/]*)\s")
pkg_line = re.compile(r"^FAIL\s+(\S+)")
for log in logs:
    pending = []
    try:
        lines = open(log, errors="replace").read().splitlines()
    except OSError:
        continue
    for line in lines:
        m = fail_line.match(line)
        if m and not line.startswith("    "):
            pending.append(m.group(1))
            continue
        m = pkg_line.match(line)
        if m and pending:
            imp = m.group(1)
            rel = imp[len(module) + 1:] if imp.startswith(module + "/") else ("." if imp == module else imp)
            failing += [(rel, t) for t in pending]
            pending = []
failing = sorted(set(failing))
selected = [f for f in failing if f[0] in whole or f in named]
missed = [f for f in failing if f not in selected]
idx = d.get("index") or {}
row = {
    "at": datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    "head": head, "since": since, "index_commit": idx.get("commit", ""),
    "index_holds_head": idx.get("commit", "") == head,
    "inputs_commit": (d.get("inputs") or {}).get("commit", ""),
    "failing": ["%s.%s" % f for f in failing],
    "selected": ["%s.%s" % f for f in selected],
    "missed": ["%s.%s" % f for f in missed],
}
if not failing:
    print("tests select boundary: no failing test named in the log (a build or setup failure); not scored")
    sys.exit(0)
ledger = os.path.join(ledger_root, "workspace", "artifacts", "tests-select", "boundary.jsonl")
os.makedirs(os.path.dirname(ledger), exist_ok=True)
with open(ledger, "a") as f:
    f.write(json.dumps(row, sort_keys=True) + "\n")
line = "tests select boundary: %d of %d failing test(s) selected" % (len(selected), len(failing))
if missed:
    line += " (missed: %s)" % ", ".join(row["missed"][:8])
if not row["index_holds_head"]:
    line += "; the index holds %s, not HEAD" % (row["index_commit"][:12] or "an unknown commit")
print(line)
PY
exit 0
