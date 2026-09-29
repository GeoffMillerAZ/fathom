#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd -P)"
# shellcheck source=scripts/lib/build-cache.sh
source "${SCRIPT_DIR}/lib/build-cache.sh"

fail() {
  echo "build-cache test: $*" >&2
  exit 1
}

assert_eq() {
  local want="$1" got="$2" label="$3"
  [ "$got" = "$want" ] || fail "${label}: got '$got', want '$want'"
}

scratch="$(mktemp -d "${TMPDIR:-/tmp}/fathom-build-cache-test.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT

# With no FATHOM_CACHE_ROOT, fathom must preserve the caller's ordinary Go
# environment. Other machines should not gain a new cache tree merely by
# cloning the repository.
(
  unset FATHOM_CACHE_ROOT FATHOM_CACHE_MAX_MIB FATHOM_CACHE_CONFIGURED
  export GOCACHE="${scratch}/caller-go-cache"
  fathom_cache_configure
  assert_eq "${scratch}/caller-go-cache" "$GOCACHE" "unset root preserves GOCACHE"
)

# A configured removable/external root must already exist. This prevents a
# missing mount from being recreated as an ordinary directory on the internal
# disk and silently defeating the operator's placement decision.
if (
  export FATHOM_CACHE_ROOT="${scratch}/missing"
  unset FATHOM_CACHE_CONFIGURED
  fathom_cache_configure
) 2>/dev/null; then
  fail "missing configured root was accepted"
fi
[ ! -e "${scratch}/missing" ] || fail "missing configured root was created"

if (
  export FATHOM_CACHE_ROOT="relative/cache"
  unset FATHOM_CACHE_CONFIGURED
  fathom_cache_configure
) 2>/dev/null; then
  fail "relative configured root was accepted"
fi

root="${scratch}/cache root"
mkdir -p "$root"
(
  export FATHOM_CACHE_ROOT="$root" FATHOM_CACHE_MAX_MIB=0
  unset FATHOM_CACHE_CONFIGURED
  fathom_cache_configure
  assert_eq "$root/build" "$GOCACHE" "GOCACHE"
  assert_eq "$root/mod" "$GOMODCACHE" "GOMODCACHE"
  assert_eq "$root/tmp" "$GOTMPDIR" "GOTMPDIR"
  assert_eq "$root/tmp" "$TMPDIR" "TMPDIR"
  assert_eq "$root/uv" "$UV_CACHE_DIR" "UV_CACHE_DIR"
  for dir in "$GOCACHE" "$GOMODCACHE" "$GOTMPDIR" "$UV_CACHE_DIR"; do
    [ -d "$dir" ] || fail "cache subdirectory was not created: $dir"
  done
)

# EW-4 (O124): the size guard trims by age and never runs `go clean` on a
# root another build may hold. Go stamps a cache entry's mtime when a build
# uses it, so an entry used inside the last day may be in a live build's hands
# and is never removed; entries unused for longer go oldest first until the
# cache fits FATHOM_CACHE_MAX_MIB. A fake `go` logs any invocation, so the
# guard calling `go clean` at all fails the test.
fake_bin="${scratch}/fake-bin"
mkdir -p "$fake_bin"
cat >"${fake_bin}/go" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$FAKE_GO_LOG"
EOF
chmod +x "${fake_bin}/go"

# cache_entry FILE MIB STAMP: a MIB-sized cache entry (two hex dirs deep, as
# Go lays them out) whose mtime is STAMP (touch -t; "" leaves it fresh).
cache_entry() {
  local file="$1" mib="$2" stamp="$3"
  mkdir -p "$(dirname "$file")"
  dd if=/dev/zero of="$file" bs=1048576 count="$mib" 2>/dev/null
  if [ -n "$stamp" ]; then touch -t "$stamp" "$file"; fi
}

# guard_run ROOT MAX_MIB ERRFILE: one guarded configure with the fake go
# first on PATH.
guard_run() {
  (
    export PATH="${fake_bin}:$PATH"
    export FAKE_GO_LOG="$go_log"
    export FATHOM_CACHE_ROOT="$1" FATHOM_CACHE_MAX_MIB="$2"
    unset FATHOM_CACHE_CONFIGURED
    fathom_cache_configure 2>"$3"
  )
}

go_log="${scratch}/go.log"
: >"$go_log"
guard_root="${scratch}/guard-root"
mkdir -p "$guard_root/build" "$guard_root/mod"
cache_entry "$guard_root/build/aa/old-1-d" 2 200001010000
cache_entry "$guard_root/build/bb/old-2-a" 2 200001020000
cache_entry "$guard_root/build/cc/fresh-d" 2 ""
cache_entry "$guard_root/mod/keep-me" 2 200001010000
guard_run "$guard_root" 3 "${scratch}/guard.err"
[ ! -s "$go_log" ] || fail "the over-limit guard ran go: $(cat "$go_log")"
[ ! -e "$guard_root/build/aa/old-1-d" ] || fail "an entry unused for years survived the trim"
[ ! -e "$guard_root/build/bb/old-2-a" ] || fail "a second stale entry survived the trim"
[ -e "$guard_root/build/cc/fresh-d" ] || fail "an entry used today was trimmed"
[ -e "$guard_root/mod/keep-me" ] || fail "the module cache was trimmed"
grep -Fq 'trimming' "${scratch}/guard.err" || fail "the trim said nothing: $(cat "${scratch}/guard.err")"

# Entries used within the last day are never removed even when the cache is
# still over the limit: the guard says so and the build goes on.
over_root="${scratch}/over-root"
mkdir -p "$over_root/build"
cache_entry "$over_root/build/aa/fresh-1-d" 2 ""
cache_entry "$over_root/build/bb/fresh-2-d" 2 ""
guard_run "$over_root" 1 "${scratch}/over.err"
{ [ -e "$over_root/build/aa/fresh-1-d" ] && [ -e "$over_root/build/bb/fresh-2-d" ]; } \
  || fail "fresh entries were trimmed to meet the limit"
grep -Fq 'still above' "${scratch}/over.err" || fail "an unmet limit was not reported: $(cat "${scratch}/over.err")"
[ ! -s "$go_log" ] || fail "the still-over guard ran go: $(cat "$go_log")"

# A build running while the guard trims keeps every entry it reads: the
# builder below reads a fresh entry in a loop for the whole trim and must
# never see it missing (the O124 race, where a clean deleted the cache under
# a build that had already started).
race_root="${scratch}/race-root"
mkdir -p "$race_root/build"
cache_entry "$race_root/build/aa/stale-d" 3 200001010000
cache_entry "$race_root/build/bb/in-use-d" 2 ""
builder_stop="${scratch}/builder.stop"
builder_fail="${scratch}/builder.fail"
: >"$builder_fail"
(
  while [ ! -e "$builder_stop" ]; do
    cat "$race_root/build/bb/in-use-d" >/dev/null 2>&1 || echo "missing" >>"$builder_fail"
  done
) &
builder_pid=$!
for _ in 1 2 3; do
  guard_run "$race_root" 1 "${scratch}/race.err"
done
: >"$builder_stop"
wait "$builder_pid"
[ ! -s "$builder_fail" ] || fail "a concurrent build lost the entry it was reading"
[ ! -e "$race_root/build/aa/stale-d" ] || fail "the stale entry survived while the builder ran"

# Under the limit the guard removes nothing, however old.
under_root="${scratch}/under-root"
mkdir -p "$under_root/build"
cache_entry "$under_root/build/aa/old-d" 1 200001010000
guard_run "$under_root" 64 "${scratch}/under.err"
[ -e "$under_root/build/aa/old-d" ] || fail "a cache under the limit was trimmed"

: >"$go_log"
(
  export PATH="${fake_bin}:$PATH"
  export FAKE_GO_LOG="$go_log"
  export FATHOM_CACHE_ROOT="$root" FATHOM_CACHE_MAX_MIB=0
  unset FATHOM_CACHE_CONFIGURED
  fathom_cache_configure
)
[ ! -s "$go_log" ] || fail "disabled size guard invoked go"

# The whole-cache clean is what raced a starting build, and no branch of the
# guard may spell it (EW-4's lexical oracle reads this file for it).
if grep -Fq 'clean -cache' "${SCRIPT_DIR}/lib/build-cache.sh"; then
  fail "scripts/lib/build-cache.sh names a whole-cache clean"
fi

grep -Fq 'GO: bash scripts/fathom-go.sh' "${REPO_ROOT}/Taskfile.yml" \
  || fail "Taskfile GO commands do not use the cache wrapper"
if grep -Fq '/Volumes/' "${REPO_ROOT}/Taskfile.yml"; then
  fail "Taskfile contains a machine-specific volume path"
fi
for file in \
  module-cold-run.sh ci.sh clean-clone-check.sh stranger-check.sh \
  bench-ledger-render.sh ui-css.sh context-bench.sh; do
  grep -Fq 'fathom_cache_configure' "${SCRIPT_DIR}/${file}" \
    || fail "${file} bypasses the build-cache policy"
done

echo "build-cache test: PASS"
