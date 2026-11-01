#!/usr/bin/env bash
# test-inputs.sh — observe which repository files the Go tests read (TQ-32,
# ADR-090), for `fathom tests select --inputs` (`task test:inputs`).
#
#   bash scripts/test-inputs.sh [--out DIR] [--per-test-min SECS] [PACKAGE...]
#
# Go's testlog is the record: `go test` passes `-test.testlogfile` to every
# test binary it can cache, and the binary logs each open, stat, getenv and
# chdir the tests make through package os; Go's own test cache keys on it.
# This script asks for that log on purpose, one per package, through
# `go test -exec` (this script again, with --exec), no -race (a read does not
# depend on the race detector) and -count=1. A package whose run took at
# least --per-test-min seconds (default 20) is then run test by test, from
# one `go test -c` binary, one process and one log per top-level test (--jobs
# at a time, default 4), so a changed file selects the tests in it that read
# the file rather than the whole package; a test that fails alone is
# recorded unobserved and its package's log stands for it.
#
# Output (default workspace/artifacts/test-inputs, replaced whole when the
# run ends, so a reader never sees half of one): meta.json (commit, time,
# root, per package its exit code and, run test by test, each test's exit
# code), pkg/<dir>/_package.log and pkg/<dir>/_test.<Name>.log. It runs
# under scratch-home.sh like `task test`, so no test writes ~/.fathom.
# The observation is one run at one commit: the push gate still runs
# everything.

set -euo pipefail

if [ "${1:-}" = "--exec" ]; then
  # go test -exec: <this> --exec <test binary> <go test's args...>, run in the
  # package directory. Keep the binary's output and exit code; record both.
  shift
  bin=$1
  shift
  here=$(pwd -P)
  rel=${here#"$TI_ROOT"}
  rel=${rel#/}
  [ -n "$rel" ] || rel=.
  dir="$TI_OUT/pkg/$rel"
  mkdir -p "$dir"
  start=$(date +%s)
  rc=0
  "$bin" -test.testlogfile="$dir/_package.log" "$@" || rc=$?
  echo "$rc" >"$dir/_exit"
  echo "$(($(date +%s) - start))" >"$dir/_secs"
  exit "$rc"
fi

if [ "${1:-}" = "--one" ]; then
  # One top-level test in its own process: <this> --one BIN PKGDIR OUTDIR NAME.
  bin=$2 pkgdir=$3 dir=$4 name=$5
  rc=0
  (cd "$pkgdir" && bash "$TI_ROOT/scripts/scratch-home.sh" "$bin" -test.run="^${name}\$" -test.count=1 \
    -test.timeout=10m -test.testlogfile="$dir/_test.${name}.log") >/dev/null 2>&1 || rc=$?
  echo "$rc" >"$dir/_rc.$name"
  exit 0
fi

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
out="$root/workspace/artifacts/test-inputs"
per_test_min=20
jobs=${TI_JOBS:-4}
while [ "$#" -gt 0 ]; do
  case $1 in
    --out) out=$2; shift 2 ;;
    --per-test-min) per_test_min=$2; shift 2 ;;
    --jobs) jobs=$2; shift 2 ;;
    --) shift; break ;;
    -*) echo "usage: test-inputs.sh [--out DIR] [--per-test-min SECS] [--jobs N] [PACKAGE...]" >&2; exit 2 ;;
    *) break ;;
  esac
done
cd "$root"
go_cmd=(bash "$root/scripts/fathom-go.sh")
if [ "$#" -gt 0 ]; then
  pkgs=("$@")
else
  # Every package of this module, as the gate's test step lists them (import
  # paths hold no spaces; bash 3.2 has no mapfile).
  # shellcheck disable=SC2207
  pkgs=($("${go_cmd[@]}" list ./... | grep -v /third_party/))
fi

mkdir -p "$(dirname "$out")"
work=$(mktemp -d "$(dirname "$out")/.test-inputs.XXXXXX")
trap 'rm -rf "$work"' EXIT
export TI_ROOT="$root" TI_OUT="$work" CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0
commit=$(git rev-parse HEAD)
at=$(date -u +%Y-%m-%dT%H:%M:%SZ)

echo "test-inputs: observing ${#pkgs[@]} package(s) at ${commit:0:12} (package logs, then test by test where a package ran ${per_test_min}s or more)"
bash "$root/scripts/scratch-home.sh" "${go_cmd[@]}" test -count=1 -p 4 \
  -exec "bash $root/scripts/test-inputs.sh --exec" "${pkgs[@]}" >"$work/run.log" 2>&1 || true
# grep -c exits 1 on no match: a passing suite must not end the run here.
nfail=$(grep -cE '^FAIL[[:space:]]' "$work/run.log" || true)
echo "test-inputs: package pass done (${nfail:-0} package(s) failed; they are recorded partial and run whole)"

# Test by test, for the packages that took long enough to be worth splitting.
while IFS= read -r secs_file; do
  dir=$(dirname "$secs_file")
  rel=${dir#"$work/pkg/"}
  [ "$rel" != "$work/pkg" ] || rel=.
  secs=$(cat "$secs_file")
  [ "$secs" -ge "$per_test_min" ] || continue
  bin="$work/bin/$(printf '%s' "$rel" | tr '/' '_').test"
  mkdir -p "$work/bin"
  if ! bash "$root/scripts/scratch-home.sh" "${go_cmd[@]}" test -c -o "$bin" "./$rel" >>"$work/run.log" 2>&1; then
    echo "test-inputs: $rel: go test -c failed; the package log stands for every test"
    continue
  fi
  [ -x "$bin" ] || continue
  # --jobs processes at once: test-shards.sh already runs cmd/fathom as
  # parallel processes of one binary, and a test that fails beside another
  # is only left unobserved (its package log stands for it).
  (cd "$root/$rel" && "$bin" -test.list '^Test' 2>/dev/null) | grep '^Test' >"$dir/_list" || true
  listed=$(wc -l <"$dir/_list" | tr -d ' ')
  xargs -P "$jobs" -I{} bash "$root/scripts/test-inputs.sh" --one "$bin" "$root/$rel" "$dir" {} <"$dir/_list" || true
  : >"$dir/_tests"
  n=0
  for rcf in "$dir"/_rc.*; do
    [ -f "$rcf" ] || continue
    printf '%s %s\n' "${rcf##*/_rc.}" "$(cat "$rcf")" >>"$dir/_tests"
    rm -f "$rcf"
    n=$((n + 1))
  done
  # A list that named nothing, or a run that left fewer results than it
  # listed, keeps the package whole: a per-test package answers only for
  # the tests it has logs for.
  if [ "$listed" = 0 ] || [ "$n" != "$listed" ]; then
    rm -f "$dir/_tests" "$dir"/_test.*.log
    echo "test-inputs: $rel: listed $listed test(s), $n result(s); kept whole"
    continue
  fi
  echo "test-inputs: $rel: $n test(s) run one by one, $jobs at a time (${secs}s as a package)"
done < <(find "$work/pkg" -name _secs | sort)

# meta.json: package paths and Go test names need no JSON escaping.
{
  printf '{"commit":"%s","at":"%s","root":"%s","packages":{' "$commit" "$at" "$root"
  first=1
  while IFS= read -r exit_file; do
    dir=$(dirname "$exit_file")
    rel=${dir#"$work/pkg/"}
    [ "$rel" != "$work/pkg" ] || rel=.
    [ "$first" = 1 ] || printf ','
    first=0
    printf '"%s":{"exit":%s' "$rel" "$(cat "$exit_file")"
    if [ -f "$dir/_tests" ]; then
      printf ',"per_test":true,"tests":{'
      tfirst=1
      while read -r name rc; do
        [ "$tfirst" = 1 ] || printf ','
        tfirst=0
        printf '"%s":%s' "$name" "$rc"
      done <"$dir/_tests"
      printf '}'
    fi
    printf '}'
  done < <(find "$work/pkg" -name _exit | sort)
  printf '}}\n'
} >"$work/meta.json"
find "$work/pkg" \( -name _exit -o -name _secs -o -name _tests -o -name _list \) -delete
rm -rf "${work:?}/bin"

rm -rf "$out.old"
[ ! -e "$out" ] || mv "$out" "$out.old"
mv "$work" "$out"
rm -rf "$out.old"
trap - EXIT
echo "test-inputs: wrote $out (commit ${commit:0:12}); fathom tests select reads it by default (--inputs names another)"
