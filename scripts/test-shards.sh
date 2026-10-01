#!/usr/bin/env bash
# test-shards.sh — run cmd/fathom's race tests as N parallel shards of ONE
# test binary (`task test`; docs/usage.md "The gate"). cmd/fathom is ~1,900
# serial tests (no t.Parallel) and ~357 s alone, the gate's critical path;
# every other package stays on the plain `go test -race -p 4` in `task test`.
#
#   bash scripts/test-shards.sh                       build, list, plan, check, run
#   bash scripts/test-shards.sh plan LIST WEIGHTS N   print the partition, one
#                                                     "shard<TAB>ms<TAB>name" line per test
#   bash scripts/test-shards.sh weights JSON [OUT]    rewrite the weights file from a
#                                                     `go test -json ./cmd/fathom` run
#
# The run: `go test -race -c` once (through scripts/fathom-go.sh, so
# FATHOM_CACHE_ROOT applies) into workspace/tmp/, `-test.list '.*'` for the
# names (Benchmark* dropped: -test.run never runs one), then a deterministic
# partition: greedy longest-first by the measured seconds in the weights file
# (a test missing from it weighs the median; 0 s floors at 1 ms; ties go to
# the name, then the lowest shard), so the largest shard weighs at most
# total/N plus the largest single test. Before any test runs, every shard's
# `-test.list '^(A|B|…)$'` must select exactly its planned names and the
# shards together exactly the list: a test dropped or run twice fails the
# run. Each shard then runs from cmd/fathom (go test's cwd) as
# `<bin> -test.paniconexit0 -test.timeout=<T> -test.run=^(A|B|…)$`, the flags
# go test itself passes. Each shard prints a go-test "ok"/"FAIL" line with its
# wall time; a failing shard's whole output is shown above its FAIL line; any
# failure exits 1. Each shard is its own process under testisolate.Main (own
# HOME, state dir, call log, daemon URL, temp caches).
#
# Env: FATHOM_TEST_SHARDS (N, default 4); FATHOM_TEST_SHARDS_WEIGHTS (default
# scripts/test-shards.weights.tsv); FATHOM_TEST_SHARDS_TIMEOUT (per shard,
# default 10m0s, go test's own default); FATHOM_TEST_SHARDS_BIN (a prebuilt
# test binary — skips the build; scripts/test-shards_test.sh passes a fake).
# Bash 3.2-compatible (macOS /bin/bash); perl (already the gate lease's
# dependency) reads the clock.

set -euo pipefail
export LC_ALL=C

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd -P)"
PKG_DIR="cmd/fathom"
DEFAULT_WEIGHTS="${SCRIPT_DIR}/test-shards.weights.tsv"

die() {
  echo "test-shards: $*" >&2
  exit 1
}

now() { perl -MTime::HiRes=time -e 'printf "%.3f\n", time'; }

# plan LIST WEIGHTS N — the partition, sorted by shard then name.
plan() {
  local list="$1" weights="$2" n="$3" median
  case "$n" in '' | *[!0-9]* | 0) die "plan: N must be a positive integer, got '$n'" ;; esac
  [ -r "$list" ] || die "plan: cannot read the list $list"
  [ -r "$weights" ] || die "plan: cannot read the weights $weights"
  # Lower median of the file's weights, in ms (1 ms when the file is empty).
  median=$(awk -F'\t' '!/^#/ && NF >= 2 { ms = int($2 * 1000 + 0.5); if (ms < 1) ms = 1; print ms }' "$weights" |
    sort -n | awk '{ a[NR] = $1 } END { print (NR ? a[int((NR + 1) / 2)] : 1) }')
  awk -F'\t' -v median="$median" '
    NR == FNR { if ($0 !~ /^#/ && NF >= 2) { ms = int($2 * 1000 + 0.5); if (ms < 1) ms = 1; w[$1] = ms }; next }
    $1 == "" || $1 ~ /^Benchmark/ { next }
    { print (($1 in w) ? w[$1] : median) "\t" $1 }
  ' "$weights" "$list" |
    sort -t "$(printf '\t')" -k1,1nr -k2,2 |
    awk -F'\t' -v n="$n" '{
      best = 1
      for (s = 2; s <= n; s++) if (load[s] < load[best]) best = s
      load[best] += $1
      print best "\t" $1 "\t" $2
    }' |
    sort -t "$(printf '\t')" -k1,1n -k3,3
}

# weights JSON [OUT] — top-level tests' Elapsed (pass, fail or skip), by name.
weights() {
  local json="$1" out="${2:-${FATHOM_TEST_SHARDS_WEIGHTS:-$DEFAULT_WEIGHTS}}" tmp
  [ -r "$json" ] || die "weights: cannot read $json"
  tmp="$(mktemp "${out}.XXXXXX")"
  {
    echo "# scripts/test-shards.weights.tsv — cmd/fathom's measured seconds per top-level test"
    echo "# (go test -json Elapsed), read by scripts/test-shards.sh to balance its shards; a test"
    echo "# missing here weighs the median. Refresh with \`task test:weights\`."
    awk '
      /^\{"Time":"[^"]*","Action":"(pass|fail|skip)","Package":"[^"]*","Test":"[^"\/]+","Elapsed":/ {
        t = $0; sub(/^.*"Test":"/, "", t); sub(/".*$/, "", t)
        e = $0; sub(/^.*"Elapsed":/, "", e); sub(/[,}].*$/, "", e)
        s = sprintf("%.3f", e); sub(/0+$/, "", s); sub(/\.$/, "", s)
        w[t] = s
      }
      END { for (t in w) printf "%s\t%s\n", t, w[t] }
    ' "$json" | sort -t "$(printf '\t')" -k1,1
  } >"$tmp"
  [ "$(grep -vc '^#' "$tmp")" -gt 0 ] || {
    rm -f "$tmp"
    die "weights: no top-level test results in $json (want a go test -json run)"
  }
  mv "$tmp" "$out"
  echo "test-shards: wrote $(grep -vc '^#' "$out") weights to $out"
}

case "${1:-run}" in
  plan)
    [ "$#" -eq 4 ] || die "usage: test-shards.sh plan LIST WEIGHTS N"
    plan "$2" "$3" "$4"
    exit 0
    ;;
  weights)
    [ "$#" -ge 2 ] || die "usage: test-shards.sh weights JSON [OUT]"
    weights "$2" "${3:-}"
    exit 0
    ;;
  run) ;;
  *) die "usage: test-shards.sh [run | plan LIST WEIGHTS N | weights JSON [OUT]]" ;;
esac

n="${FATHOM_TEST_SHARDS:-4}"
case "$n" in '' | *[!0-9]* | 0) die "FATHOM_TEST_SHARDS must be a positive integer, got '$n'" ;; esac
weights_file="${FATHOM_TEST_SHARDS_WEIGHTS:-$DEFAULT_WEIGHTS}"
timeout="${FATHOM_TEST_SHARDS_TIMEOUT:-10m0s}"
module="$(awk '$1 == "module" { print $2; exit }' "${ROOT}/go.mod")"
[ -n "$module" ] || die "no module line in ${ROOT}/go.mod"
import="${module}/${PKG_DIR}"

mkdir -p "${ROOT}/workspace/tmp"
scratch="$(mktemp -d "${ROOT}/workspace/tmp/test-shards.XXXXXX")"
pids=""
cleanup() { rm -rf "$scratch"; }
interrupt() {
  for p in $pids; do
    pkill -TERM -P "$p" 2>/dev/null || true
    kill -TERM "$p" 2>/dev/null || true
  done
  cleanup
  exit 130
}
trap cleanup EXIT
trap interrupt INT TERM

t_start=$(now)
bin="${FATHOM_TEST_SHARDS_BIN:-}"
if [ -z "$bin" ]; then
  bin="${scratch}/fathom.test"
  if ! (cd "$ROOT" && bash "${SCRIPT_DIR}/fathom-go.sh" test -race -c -o "$bin" "./${PKG_DIR}") >"${scratch}/build.log" 2>&1; then
    cat "${scratch}/build.log"
    printf 'FAIL\t%s [build failed]\n' "$import"
    exit 1
  fi
fi
t_built=$(now)

if ! (cd "${ROOT}/${PKG_DIR}" && "$bin" -test.list '.*') >"${scratch}/list.raw" 2>"${scratch}/list.err"; then
  cat "${scratch}/list.raw" "${scratch}/list.err"
  printf 'FAIL\t%s [setup failed: -test.list]\n' "$import"
  exit 1
fi
grep -v '^Benchmark' "${scratch}/list.raw" | grep -v '^$' >"${scratch}/list" || true
sort "${scratch}/list" >"${scratch}/list.sorted"
dups="$(uniq -d "${scratch}/list.sorted")"
[ -z "$dups" ] || die "the test list names a test twice: ${dups}"
total=$(wc -l <"${scratch}/list.sorted" | tr -d ' ')
if [ "$total" -eq 0 ]; then
  printf 'ok  \t%s\t0.000s [no tests to run]\n' "$import"
  exit 0
fi

plan "${scratch}/list" "$weights_file" "$n" >"${scratch}/plan"
cut -f3 "${scratch}/plan" | sort >"${scratch}/planned"
if ! cmp -s "${scratch}/planned" "${scratch}/list.sorted"; then
  echo "test-shards: the plan does not cover the test list exactly (< listed only, > planned only):" >&2
  diff "${scratch}/list.sorted" "${scratch}/planned" >&2 || true
  exit 1
fi

# Each shard's pattern must select exactly its planned names.
shards=""
: >"${scratch}/selected"
i=1
while [ "$i" -le "$n" ]; do
  awk -F'\t' -v s="$i" '$1 == s { print $3 }' "${scratch}/plan" >"${scratch}/names.$i"
  if [ -s "${scratch}/names.$i" ]; then
    printf '^(%s)$' "$(paste -sd'|' "${scratch}/names.$i")" >"${scratch}/pattern.$i"
    (cd "${ROOT}/${PKG_DIR}" && "$bin" -test.list "$(cat "${scratch}/pattern.$i")") 2>/dev/null |
      grep -v '^Benchmark' | grep -v '^$' | sort >"${scratch}/selected.$i" || true
    sort "${scratch}/names.$i" >"${scratch}/names.$i.sorted"
    if ! cmp -s "${scratch}/selected.$i" "${scratch}/names.$i.sorted"; then
      echo "test-shards: shard $i/$n's pattern does not select exactly its planned tests (< planned only, > selected only):" >&2
      diff "${scratch}/names.$i.sorted" "${scratch}/selected.$i" >&2 || true
      exit 1
    fi
    cat "${scratch}/selected.$i" >>"${scratch}/selected"
    shards="$shards $i"
  fi
  i=$((i + 1))
done
sort "${scratch}/selected" >"${scratch}/selected.sorted"
if ! cmp -s "${scratch}/selected.sorted" "${scratch}/list.sorted"; then
  echo "test-shards: the shards together select other than the test list — a test dropped or run twice (< listed, > selected):" >&2
  diff "${scratch}/list.sorted" "${scratch}/selected.sorted" >&2 || true
  exit 1
fi
t_listed=$(now)

for i in $shards; do
  (
    cd "${ROOT}/${PKG_DIR}"
    t0=$(now)
    set +e
    "$bin" -test.paniconexit0 "-test.timeout=${timeout}" "-test.run=$(cat "${scratch}/pattern.$i")" >"${scratch}/shard.$i.log" 2>&1
    rc=$?
    set -e
    echo "$rc $t0 $(now)" >"${scratch}/shard.$i.rc"
  ) &
  pids="$pids $!"
done
for p in $pids; do wait "$p" || true; done
t_end=$(now)

failed=0
walls=""
for i in $shards; do
  read -r rc t0 t1 <"${scratch}/shard.$i.rc" || { rc=1 t0=0 t1=0; }
  wall=$(awk -v a="$t0" -v b="$t1" 'BEGIN { printf "%.3f", b - a }')
  count=$(wc -l <"${scratch}/names.$i" | tr -d ' ')
  planned=$(awk -F'\t' -v s="$i" '$1 == s { ms += $2 } END { printf "%.1f", ms / 1000 }' "${scratch}/plan")
  walls="${walls} ${i}:${wall}s"
  if [ "$rc" -eq 0 ]; then
    printf 'ok  \t%s\t%ss\tshard %d/%d (%d tests, planned %ss)\n' "$import" "$wall" "$i" "$n" "$count" "$planned"
  else
    failed=1
    cat "${scratch}/shard.$i.log"
    printf 'FAIL\t%s\t%ss\tshard %d/%d (%d tests, exit %d)\n' "$import" "$wall" "$i" "$n" "$count" "$rc"
  fi
done
awk -v s="$t_start" -v b="$t_built" -v l="$t_listed" -v e="$t_end" -v n="$n" -v w="$walls" -v total="$total" 'BEGIN {
  printf "test-shards: %d tests in %d shards, total %.1fs (build %.1fs, list+check %.1fs, shards %.1fs; per shard%s)\n", total, n, e - s, b - s, l - b, e - l, w
}'
exit "$failed"
