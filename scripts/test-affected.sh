#!/usr/bin/env bash
# test-affected.sh — run the tests a diff can reach (TQ-35, ADR-090;
# `task test:affected`): what the call graph reaches, what Go's testlog saw
# the tests read (`task test:inputs`) and the new tests in changed test
# files, as `fathom tests select --safe` names them, through the same
# isolation as `task t` (scripts/test-targeted.sh: -race, CGO_ENABLED=1, a
# scratch FATHOM_HOME).
#
#   bash scripts/test-affected.sh [--since REF] [--inputs DIR] [--store DB] [--dry-run]
#
# REF defaults to the merge-base of HEAD and origin/main, so every local
# commit, the working tree's changes and untracked files count. Whole
# packages run in one go test, narrowed ones in a second with the union of
# their names (a test of the same name in another narrowed package runs too:
# extra, never missing). In a linked worktree the observation defaults to the
# main checkout's, since a worktree has none of its own. --store names the
# index (a worker clone's scratch ingest); the default is the daemon's.
#
# Fails open, toward running more: no fathom on PATH, a select that fails or
# answers what this script cannot read, or no observation to say which tests
# read the changed files (a Go edit too: a test may walk the source tree)
# runs `task test`. A changed shell script also runs `task scripts-check`,
# whose tests are not Go. Advisory either way: the pre-push gate runs the
# whole suite, and scores this selection when it fails
# (scripts/test-boundary.sh).

set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$root"
since="" inputs="" store="" dry=0
while [ "$#" -gt 0 ]; do
  case $1 in
    --since) since=$2; shift 2 ;;
    --inputs) inputs=$2; shift 2 ;;
    --store) store=$2; shift 2 ;;
    --dry-run) dry=1; shift ;;
    *) echo "usage: test-affected.sh [--since REF] [--inputs DIR] [--store DB] [--dry-run]" >&2; exit 2 ;;
  esac
done
fathom_bin=${FATHOM_BIN:-fathom}
task_bin=${TASK_BIN:-task}

full() {
  echo "test:affected: $1 — running the whole suite (task test)" >&2
  [ "$dry" = 1 ] && { echo "$task_bin test"; exit 0; }
  exec "$task_bin" test
}

if [ -z "$since" ]; then
  since=$(git merge-base HEAD origin/main 2>/dev/null || git rev-parse HEAD)
fi
if [ -z "$inputs" ]; then
  inputs="$root/workspace/artifacts/test-inputs"
  main=$(git worktree list --porcelain 2>/dev/null | sed -n '1s/^worktree //p')
  if [ ! -f "$inputs/meta.json" ] && [ -n "$main" ] && [ -f "$main/workspace/artifacts/test-inputs/meta.json" ]; then
    inputs="$main/workspace/artifacts/test-inputs"
  fi
fi
command -v "$fathom_bin" >/dev/null 2>&1 || full "no fathom on PATH"

mkdir -p workspace/tmp
sel=$(mktemp workspace/tmp/test-affected.XXXXXX)
trap 'rm -f "$sel" "$sel.err" "$sel.plan"' EXIT
store_args=()
[ -z "$store" ] || store_args=(--store "$store")
if ! "$fathom_bin" tests select --since "$since" --safe --json --service "${FATHOM_SERVICE:-fathom}" --inputs "$inputs" ${store_args[@]+"${store_args[@]}"} >"$sel" 2>"$sel.err"; then
  sed -n '1,5p' "$sel.err" >&2
  full "fathom tests select failed"
fi
grep -E '^(note|warning): tests select: (the index holds|no observed|observed inputs|the observed)' "$sel.err" >&2 || true

# The plan: line 1 the whole packages, line 2 the narrowed ones, line 3 the
# union of their test names, line 4 "missing" when no observation loaded,
# line 5 the count of named tests.
python3 -I - "$sel" >"$sel.plan" <<'PY' || full "the selection could not be read"
import json, sys
d = json.load(open(sys.argv[1]))
whole, narrow, names = [], [], set()
for inv in d.get("invocations") or []:
    pkg = "./" + (inv.get("package") or ".")
    if inv.get("whole"):
        whole.append(pkg)
    elif inv.get("tests"):
        narrow.append(pkg)
        names.update(inv["tests"])
narrow = [p for p in narrow if p not in whole]
inputs = d.get("inputs") or {}
print(" ".join(sorted(set(whole))))
print(" ".join(sorted(set(narrow))))
print("|".join(sorted(names)))
print("missing" if inputs.get("missing") or inputs.get("error") else "loaded")
print(len(names))
PY
whole=$(sed -n 1p "$sel.plan")
narrow=$(sed -n 2p "$sel.plan")
names=$(sed -n 3p "$sel.plan")
observed=$(sed -n 4p "$sel.plan")
count=$(sed -n 5p "$sel.plan")

changed=$( { git diff --name-only --no-renames "$since"; git ls-files --others --exclude-standard; } | sort -u)
if [ -z "$changed" ]; then
  echo "test:affected: nothing changed since ${since:0:12}"
  exit 0
fi
if [ "$observed" = missing ]; then
  full "no test-input observation says which tests read the changed files ('task test:inputs' writes one)"
fi
scripts_changed=0
grep -qE '^scripts/.*\.sh$' <<<"$changed" && scripts_changed=1
if [ -z "$whole$narrow" ] && [ "$scripts_changed" = 0 ]; then
  echo "test:affected: no test reaches or reads the $(printf '%s\n' "$changed" | wc -l | tr -d ' ') changed file(s) since ${since:0:12}; the pre-push gate runs everything"
  exit 0
fi

rc=0
if [ "$scripts_changed" = 1 ]; then
  if [ "$dry" = 1 ]; then echo "$task_bin scripts-check"; else "$task_bin" scripts-check || rc=$?; fi
fi
nwhole=$(printf '%s' "$whole" | wc -w | tr -d ' ')
nnarrow=$(printf '%s' "$narrow" | wc -w | tr -d ' ')
echo "test:affected: since ${since:0:12}: $nwhole whole package(s), $count test(s) in $nnarrow package(s); the pre-push gate still runs everything"
if [ -n "$whole" ]; then
  # shellcheck disable=SC2086
  if [ "$dry" = 1 ]; then echo "go test -race $whole"; else bash scripts/test-targeted.sh $whole || rc=$?; fi
fi
if [ -n "$narrow" ]; then
  # shellcheck disable=SC2086
  if [ "$dry" = 1 ]; then echo "go test -race -run '^($names)\$' $narrow"; else FATHOM_TEST_RUN="^($names)\$" bash scripts/test-targeted.sh $narrow || rc=$?; fi
fi
exit "$rc"
