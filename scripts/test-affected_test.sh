#!/usr/bin/env bash
# TQ-32, TQ-35, TQ-37 (ADR-090): test-inputs.sh's observation on a real
# module, test-affected.sh's plan and fail-open paths, and test-boundary.sh's
# score, against a stub fathom in a scratch repository.
set -euo pipefail
# A git hook exports GIT_DIR to its children: scrub it so this test's scratch
# repository never writes the pushing repository's index (O48, O1505).
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX GIT_COMMON_DIR GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
scratch=$(mktemp -d "${TMPDIR:-/tmp}/test-affected-test.XXXXXX")
scratch=$(cd "$scratch" && pwd -P)
trap 'rm -rf "$scratch"' EXIT
fail() { echo "test-affected test: $*" >&2; exit 1; }

repo="$scratch/repo"
mkdir -p "$repo/scripts/lib" "$scratch/bin"
cp "$root/scripts/test-affected.sh" "$root/scripts/test-boundary.sh" "$root/scripts/test-inputs.sh" \
  "$root/scripts/scratch-home.sh" "$root/scripts/fathom-go.sh" "$repo/scripts/"
cp "$root/scripts/lib/build-cache.sh" "$repo/scripts/lib/"
printf 'module example.com/m\n\ngo 1.22\n' >"$repo/go.mod"
git -C "$repo" init -q
git -C "$repo" -c user.name=t -c user.email=t@example.com add -A
git -C "$repo" -c user.name=t -c user.email=t@example.com commit -qm init
git -C "$repo" update-ref refs/remotes/origin/main HEAD

# The stub fathom answers `tests select` with $SELECT_JSON (or prints junk
# for SELECT_JUNK, or fails for SELECT_FAIL); the stub task echoes its task.
cat >"$scratch/bin/fathom" <<'STUB'
#!/bin/sh
[ -n "${SELECT_FAIL:-}" ] && { echo "boom" >&2; exit 1; }
[ -n "${SELECT_JUNK:-}" ] && { echo "not json"; exit 0; }
cat "$SELECT_JSON"
STUB
cat >"$scratch/bin/task" <<'STUB'
#!/bin/sh
echo "task $*"
STUB
chmod +x "$scratch/bin/fathom" "$scratch/bin/task"
export PATH="$scratch/bin:$PATH" FATHOM_BIN="$scratch/bin/fathom" TASK_BIN="$scratch/bin/task"

cat >"$scratch/select.json" <<'JSON'
{"invocations":[
 {"package":"cmd/fathom","tests":["TestA","TestB"],"command":"go test ./cmd/fathom -run '^(TestA|TestB)$'"},
 {"package":"internal/lint/tenancy","tests":[],"whole":true,"command":"go test ./internal/lint/tenancy"}],
 "inputs":{"missing":false,"commit":"abc"},"index":{"commit":"def"}}
JSON
sed 's/"missing":false/"missing":true/' "$scratch/select.json" >"$scratch/missing.json"
export SELECT_JSON="$scratch/select.json"
falls_back() { grep -qE "(^|/)task test$" <<<"$1"; }

# 1. The plan: one whole-package run, one narrowed run with the union of names.
echo 'doc' >"$repo/notes.md"
out=$(cd "$repo" && bash scripts/test-affected.sh --dry-run 2>&1)
grep -q "go test -race ./internal/lint/tenancy" <<<"$out" || fail "no whole-package run: $out"
grep -qF "go test -race -run '^(TestA|TestB)\$' ./cmd/fathom" <<<"$out" || fail "no narrowed run: $out"

# 2. Fail open: a select that fails, or answers what the runner cannot read,
# runs the whole suite.
out=$(cd "$repo" && SELECT_FAIL=1 bash scripts/test-affected.sh --dry-run 2>&1)
falls_back "$out" || fail "a failed select did not fall back to task test: $out"
out=$(cd "$repo" && SELECT_JUNK=1 bash scripts/test-affected.sh --dry-run 2>&1)
falls_back "$out" || fail "an unreadable selection did not fall back to task test: $out"

# 3. Fail open: no observation runs the whole suite, for a Go edit too (a
# test may walk the source tree).
out=$(cd "$repo" && SELECT_JSON="$scratch/missing.json" bash scripts/test-affected.sh --dry-run 2>&1)
falls_back "$out" || fail "a non-Go change with no observation did not fall back: $out"
rm "$repo/notes.md"
echo 'package m' >"$repo/m.go"
out=$(cd "$repo" && SELECT_JSON="$scratch/missing.json" bash scripts/test-affected.sh --dry-run 2>&1)
falls_back "$out" || fail "a Go change with no observation did not fall back: $out"

# 4. A changed shell script also runs the script tests.
echo 'true' >"$repo/scripts/new.sh"
out=$(cd "$repo" && bash scripts/test-affected.sh --dry-run 2>&1)
grep -qE "(^|/)task scripts-check$" <<<"$out" || fail "a changed script did not run task scripts-check: $out"

# 5. The boundary score: two of three failures selected, the third named; the
# row lands in the checkout's ledger and says the index is not HEAD's.
mkdir -p "$repo/workspace/artifacts/test-inputs"
echo '{"commit":"abc","packages":{}}' >"$repo/workspace/artifacts/test-inputs/meta.json"
cat >"$scratch/pkgs.log" <<'LOG'
--- FAIL: TestTenancyRules (0.10s)
    tenancy_test.go:9: bare goroutine
FAIL
FAIL	example.com/m/internal/lint/tenancy	0.2s
--- FAIL: TestZ (0.01s)
    --- FAIL: TestZ/sub (0.00s)
FAIL
FAIL	example.com/m/internal/x	0.1s
LOG
cat >"$scratch/shards.log" <<'LOG'
--- FAIL: TestA (1.00s)
FAIL	example.com/m/cmd/fathom	12.0s	shard 1/4 (10 tests, exit 1)
LOG
out=$(cd "$repo" && bash scripts/test-boundary.sh "$scratch/pkgs.log" "$scratch/shards.log")
case $out in "tests select boundary: 2 of 3 failing test(s) selected (missed: internal/x.TestZ); the index holds def, not HEAD") ;; *) fail "score line: $out" ;; esac
grep -q '"missed": \["internal/x.TestZ"\]' "$repo/workspace/artifacts/tests-select/boundary.jsonl" || fail "no ledger row"

# 6. Advisory and honest: a select that fails, no observation, or no change
# is named, not scored, and exits 0.
out=$(cd "$repo" && SELECT_FAIL=1 bash scripts/test-boundary.sh "$scratch/pkgs.log") || fail "the boundary score failed the gate"
grep -q "not scored" <<<"$out" || fail "a failed select was not named: $out"
mv "$repo/workspace/artifacts/test-inputs" "$scratch/held-inputs"
out=$(cd "$repo" && bash scripts/test-boundary.sh "$scratch/pkgs.log") || fail "the boundary score failed the gate"
grep -q "no test-input observation" <<<"$out" || fail "no observation was scored: $out"
mv "$scratch/held-inputs" "$repo/workspace/artifacts/test-inputs"
git -C "$repo" add -A
git -C "$repo" -c user.name=t -c user.email=t@example.com commit -qm more
git -C "$repo" update-ref refs/remotes/origin/main HEAD
out=$(cd "$repo" && bash scripts/test-boundary.sh "$scratch/pkgs.log") || fail "the boundary score failed the gate"
grep -q "nothing changed" <<<"$out" || fail "an empty diff was scored: $out"

# 7. test-inputs.sh on a real module whose suite passes (TQ-32): the package
# log and each test's own log, the read attributed to the test that made it.
mkdir -p "$repo/p"
echo 'data' >"$repo/data.txt"
printf 'package p\n' >"$repo/p/p.go"
cat >"$repo/p/p_test.go" <<'GO'
package p

import (
	"os"
	"testing"
)

func TestReadsData(t *testing.T) {
	if _, err := os.ReadFile("../data.txt"); err != nil {
		t.Fatal(err)
	}
}

func TestNothing(t *testing.T) {}
GO
(cd "$repo" && bash scripts/test-inputs.sh --out "$scratch/obs" --per-test-min 0 ./p) >"$scratch/obs.log" 2>&1 ||
  fail "test-inputs.sh failed on a passing suite: $(cat "$scratch/obs.log")"
grep -q '"p":{"exit":0,"per_test":true,"tests":{"TestNothing":0,"TestReadsData":0}}' "$scratch/obs/meta.json" ||
  fail "meta.json: $(cat "$scratch/obs/meta.json")"
grep -q 'data.txt' "$scratch/obs/pkg/p/_package.log" || fail "the package log does not name data.txt"
grep -q 'data.txt' "$scratch/obs/pkg/p/_test.TestReadsData.log" || fail "TestReadsData's log does not name data.txt"
if grep -q 'data.txt' "$scratch/obs/pkg/p/_test.TestNothing.log"; then fail "TestNothing's log names a file it never read"; fi

# 8. test-inputs.sh --exec keeps the binary's output and exit code.
mkdir -p "$repo/internal/p" "$scratch/ti"
cat >"$scratch/bin/fakebin" <<'STUB'
#!/bin/sh
echo "args: $*"
exit 3
STUB
chmod +x "$scratch/bin/fakebin"
rc=0
out=$(cd "$repo/internal/p" && TI_ROOT="$repo" TI_OUT="$scratch/ti" bash "$repo/scripts/test-inputs.sh" --exec "$scratch/bin/fakebin" -test.v=true) || rc=$?
[ "$rc" = 3 ] || fail "the binary's exit code was not kept: $rc"
grep -qF -- "-test.testlogfile=$scratch/ti/pkg/internal/p/_package.log -test.v=true" <<<"$out" || fail "no testlog flag before go test's own args: $out"
[ "$(cat "$scratch/ti/pkg/internal/p/_exit")" = 3 ] || fail "the package's exit code was not recorded"

echo "test-affected test: ok"
