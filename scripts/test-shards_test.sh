#!/usr/bin/env bash
# test-shards_test.sh — scripts/test-shards.sh, the cmd/fathom shard runner
# behind `task test`. Proves: the same list and weights give byte-identical
# shards (whatever order the list arrives in); the shards cover the list
# exactly; the largest shard weighs at most total/N plus the largest single
# weight; a test missing from the weights file gets the median weight; a
# failing shard fails the script and its output is shown; a shard pattern
# that selects other than its planned names fails the script; and the
# weights refresh keeps top-level tests only. Run by `task scripts-check`.
# A fake test binary stands in for the race build: nothing here compiles Go.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
SHARDS="${SCRIPT_DIR}/test-shards.sh"

fail() {
  echo "test-shards test: $*" >&2
  exit 1
}

[ -f "$SHARDS" ] || fail "scripts/test-shards.sh is missing"

scratch="$(mktemp -d "${TMPDIR:-/tmp}/fathom-test-shards-test.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT
export LC_ALL=C

# Eleven weights (an odd count, so the median is one entry): in ms, floored
# at 1, sorted 1 10 500 1000 2000 3250 3250 4000 7000 12500 99000 — the
# median is 3250. TestGone and TestOld are in the file but not in the list.
weights="${scratch}/weights.tsv"
cat >"$weights" <<'W'
# test-shards weights (fixture)
TestA	12.5
TestB	7
TestC	3.25
TestD	3.25
TestE	1
TestF	0.5
TestG	0.01
TestH	0
TestI	2
TestGone	99
TestOld	4
W
printf '%s\n' TestE TestNew1 TestA TestH TestC TestB TestNew2 TestG TestD TestF TestI >"${scratch}/list1"
printf '%s\n' TestI TestF TestD TestG TestNew2 TestB TestC TestH TestA TestNew1 TestE >"${scratch}/list2"
sort "${scratch}/list1" >"${scratch}/list.sorted"

for n in 1 2 3 4 20; do
  bash "$SHARDS" plan "${scratch}/list1" "$weights" "$n" >"${scratch}/plan1.$n" || fail "plan N=$n failed"
  bash "$SHARDS" plan "${scratch}/list2" "$weights" "$n" >"${scratch}/plan2.$n" || fail "plan N=$n (reordered list) failed"
  bash "$SHARDS" plan "${scratch}/list1" "$weights" "$n" >"${scratch}/plan3.$n"

  # 1. Deterministic: byte-identical across runs and list orders.
  cmp -s "${scratch}/plan1.$n" "${scratch}/plan3.$n" || fail "N=$n: two plans of the same input differ"
  cmp -s "${scratch}/plan1.$n" "${scratch}/plan2.$n" || fail "N=$n: the plan depends on the list's order:
$(diff "${scratch}/plan1.$n" "${scratch}/plan2.$n")"

  # 2. Exact cover: every listed name once, nothing else.
  cut -f3 "${scratch}/plan1.$n" | sort >"${scratch}/covered.$n"
  cmp -s "${scratch}/covered.$n" "${scratch}/list.sorted" || fail "N=$n: the shards do not cover the list exactly:
$(diff "${scratch}/list.sorted" "${scratch}/covered.$n")"
  awk -F'\t' -v n="$n" '$1 < 1 || $1 > n { bad = 1 } END { exit bad }' "${scratch}/plan1.$n" || fail "N=$n: a shard index is out of 1..$n"

  # 3. Balance: largest shard <= total/N + largest single weight.
  awk -F'\t' -v n="$n" '
    { load[$1] += $2; total += $2; if ($2 > max) max = $2 }
    END {
      top = 0; for (s in load) if (load[s] > top) top = load[s]
      if (top * n > total + n * max) { printf "largest shard %d ms > total %d/%d + max %d\n", top, total, n, max; exit 1 }
    }' "${scratch}/plan1.$n" || fail "N=$n: unbalanced"
done

# 4. Unknown tests weigh the median (3250 ms); known ones their own.
for t in TestNew1 TestNew2; do
  w=$(awk -F'\t' -v t="$t" '$3 == t { print $2 }' "${scratch}/plan1.4")
  [ "$w" = 3250 ] || fail "$t (not in the weights file) weighs ${w:-nothing}, want the median 3250"
done
[ "$(awk -F'\t' '$3 == "TestA" { print $2 }' "${scratch}/plan1.4")" = 12500 ] || fail "TestA must weigh its measured 12500 ms"
[ "$(awk -F'\t' '$3 == "TestH" { print $2 }' "${scratch}/plan1.4")" = 1 ] || fail "TestH (0 s) must weigh the 1 ms floor"

# The run: a fake test binary. -test.list prints the names its pattern
# matches (minus FAKE_HIDE, to simulate a pattern that loses a name);
# -test.run records each name it runs and fails FAKE_FAIL.
fake="${scratch}/fake.test"
cat >"$fake" <<'FAKE'
#!/usr/bin/env bash
names="TestA TestB TestC TestD TestE TestF TestG TestH TestI TestNew1 TestNew2 BenchmarkZ"
list="" run="" bad=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    -test.list) list="$2"; shift 2 ;;
    -test.list=*) list="${1#*=}"; shift ;;
    -test.run) run="$2"; shift 2 ;;
    -test.run=*) run="${1#*=}"; shift ;;
    *) echo "$1" >>"${FAKE_ARGS}"; shift ;;
  esac
done
echo "$PWD" >>"${FAKE_CWD}"
if [ -n "$list" ]; then
  for n in $names; do
    if [[ "$n" =~ $list ]] && { [ "$list" = '.*' ] || [ "$n" != "${FAKE_HIDE:-}" ]; }; then echo "$n"; fi
  done
  exit 0
fi
for n in $names; do
  case "$n" in Benchmark*) continue ;; esac
  if [[ "$n" =~ $run ]]; then
    echo "$n" >>"${FAKE_RAN}"
    if [ "$n" = "${FAKE_FAIL:-}" ]; then
      echo "--- FAIL: $n (0.00s)"
      echo "    fake_test.go:3: boom MARKER-$n"
      bad=1
    fi
  fi
done
if [ "$bad" = 1 ]; then echo FAIL; exit 1; fi
echo PASS
FAKE
chmod +x "$fake"
export FAKE_ARGS="${scratch}/args" FAKE_RAN="${scratch}/ran" FAKE_CWD="${scratch}/cwd"
export FATHOM_TEST_SHARDS_BIN="$fake" FATHOM_TEST_SHARDS_WEIGHTS="$weights" FATHOM_TEST_SHARDS=3

# A clean run: exit 0, every test ran exactly once, from the package
# directory, with go test's own flags, and each shard's wall time printed.
: >"$FAKE_ARGS"; : >"$FAKE_RAN"; : >"$FAKE_CWD"
bash "$SHARDS" >"${scratch}/ok.out" 2>&1 || fail "a passing run failed:
$(cat "${scratch}/ok.out")"
sort "$FAKE_RAN" | cmp -s - "${scratch}/list.sorted" || fail "the shards did not run each test exactly once:
$(sort "$FAKE_RAN" | diff "${scratch}/list.sorted" -)"
grep -qx -- '-test.paniconexit0' "$FAKE_ARGS" || fail "a shard ran without -test.paniconexit0"
grep -qE -- '^-test\.timeout=[0-9]+m[0-9]+s$' "$FAKE_ARGS" || fail "a shard ran without an explicit -test.timeout"
[ "$(sort -u "$FAKE_CWD")" = "$(cd "${SCRIPT_DIR}/../cmd/fathom" && pwd -P)" ] || fail "the binary did not run from cmd/fathom: $(sort -u "$FAKE_CWD")"
[ "$(grep -cE $'^ok  \t[^\t]+/cmd/fathom\t[0-9]+\\.[0-9]+s\tshard [1-3]/3' "${scratch}/ok.out")" = 3 ] || fail "want one ok line with a wall time per shard:
$(cat "${scratch}/ok.out")"

# 5. A failing shard fails the script and shows that shard's output.
: >"$FAKE_RAN"
set +e
FAKE_FAIL=TestC bash "$SHARDS" >"${scratch}/fail.out" 2>&1
rc=$?
set -e
[ "$rc" -ne 0 ] || fail "a failing shard must fail the script"
grep -q -- '--- FAIL: TestC (' "${scratch}/fail.out" || fail "the failing shard's output is not shown:
$(cat "${scratch}/fail.out")"
grep -q 'MARKER-TestC' "${scratch}/fail.out" || fail "the failing test's message is not shown"
grep -qE $'^FAIL\t[^\t]+/cmd/fathom\t' "${scratch}/fail.out" || fail "no go-test FAIL line for the package"
sort "$FAKE_RAN" | cmp -s - "${scratch}/list.sorted" || fail "a failing shard must not stop the other shards"

# 6. A shard whose pattern selects other than its planned names fails the
# run before any test runs (a test dropped or run twice).
: >"$FAKE_RAN"
set +e
FAKE_HIDE=TestB bash "$SHARDS" >"${scratch}/drop.out" 2>&1
rc=$?
set -e
[ "$rc" -ne 0 ] || fail "a shard pattern that drops TestB must fail the run"
grep -q 'TestB' "${scratch}/drop.out" || fail "the dropped test is not named:
$(cat "${scratch}/drop.out")"
[ ! -s "$FAKE_RAN" ] || fail "no test may run when the shard selection is wrong"

# 7. The weights refresh keeps top-level tests' Elapsed, sorted by name.
json="${scratch}/run.json"
cat >"$json" <<'J'
{"Time":"t","Action":"start","Package":"example.com/p"}
{"Time":"t","Action":"run","Package":"example.com/p","Test":"TestZ"}
{"Time":"t","Action":"pass","Package":"example.com/p","Test":"TestZ/sub","Elapsed":0.5}
{"Time":"t","Action":"pass","Package":"example.com/p","Test":"TestZ","Elapsed":1.25}
{"Time":"t","Action":"fail","Package":"example.com/p","Test":"TestY","Elapsed":0}
{"Time":"t","Action":"skip","Package":"example.com/p","Test":"TestX","Elapsed":0.01}
{"Time":"t","Action":"output","Package":"example.com/p","Test":"TestW","Output":"\"Elapsed\":9\n"}
{"Time":"t","Action":"pass","Package":"example.com/p","Elapsed":1.3}
J
bash "$SHARDS" weights "$json" "${scratch}/refreshed.tsv" || fail "weights refresh failed"
got="$(grep -v '^#' "${scratch}/refreshed.tsv" | tr '\t\n' ' |')"
[ "$got" = "TestX 0.01|TestY 0|TestZ 1.25|" ] || fail "refreshed weights: got '$got'"

echo "test-shards test: ok"
