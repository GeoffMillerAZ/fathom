#!/usr/bin/env bash
# BE-4/BE-5: execute the configured hook bodies with deterministic fixtures.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
real_git=$(command -v git)
shell=$(command -v sh)
scratch=$(mktemp -d "${TMPDIR:-/tmp}/boundary-hooks-test.XXXXXX")
trap 'rm -rf "$scratch"' EXIT
fail() { echo "boundary-hooks test: $*" >&2; exit 1; }
body() {
  awk -v hook="$1" -v step="$2" '
    /^[a-z-]+:$/ { active=($0 == hook ":") }
    active && $0 == "    " step ":" { found=1; next }
    found && /^      run: \|/ { run=1; next }
    run && /^        / { print substr($0,9); next }
    run { exit }
  ' "$root/lefthook.yml"
}
mkdir -p "$scratch/repo/bin" "$scratch/repo/workspace/tmp" "$scratch/repo/docs/plans" "$scratch/repo/scripts" "$scratch/tools"
printf '%s\n' '[ ] a plan' > "$scratch/repo/docs/plans/example.md"
cat > "$scratch/repo/bin/fathom" <<'STUB'
#!/bin/sh
if [ -n "${FATHOM_ARGS_LOG:-}" ]; then printf '%s\n' "$*" >> "$FATHOM_ARGS_LOG"; fi
if [ -n "${COMMAND_ENV_LOG:-}" ]; then
  printf '%s\n' "$1 CGO_ENABLED=${CGO_ENABLED-unset}" >> "$COMMAND_ENV_LOG"
fi
if [ "$1" = ingest ] && [ "${STUB_INGEST_OK:-0}" = 1 ]; then
  while [ "$#" -gt 1 ]; do
    if [ "$1" = --store ]; then : > "$2"; fi
    shift
  done
  exit 0
fi
if [ "${STUB_SUCCESS:-0}" = 1 ]; then echo "$1 ok"; exit 0; fi
printf 'first diagnostic\nsecond diagnostic\n\n' >&2
exit "${STUB_EXIT:-2}"
STUB
cat > "$scratch/tools/git" <<'STUB'
#!/bin/sh
case "$*" in
  *diff-tree*) echo example.go ;;
  *merge-base*) echo base ;;
  *rev-parse*) echo origin/main ;;
esac
STUB
cat > "$scratch/tools/go" <<'STUB'
#!/bin/sh
printf 'first diagnostic\nsecond diagnostic\n\n' >&2
exit 2
STUB
cat > "$scratch/repo/scripts/gate-lease.sh" <<'STUB'
#!/bin/sh
exec "$@"
STUB
# Keep the ingest test independent of the optional system RSS reader.
cat > "$scratch/tools/uname" <<'STUB'
#!/bin/sh
echo fixture
STUB
chmod +x "$scratch/repo/bin/fathom" "$scratch/tools/"*
export PATH="$scratch/tools:$PATH"
cd "$scratch/repo"
for pair in 'pre-push review' 'pre-push refactor-conserve' 'pre-push verify' 'post-commit rebuild-binary' 'post-commit fathom-refresh'; do
  read -r hook step <<< "$pair"
  body "$hook" "$step" > "$scratch/run"
  rc=0
  sh "$scratch/run" </dev/null > "$scratch/out" 2>&1 || rc=$?
  [ "$rc" -eq 0 ] || fail "$step did not fail open"
  grep 'first diagnostic' "$scratch/out" >/dev/null || fail "$step swallowed first stderr line"
  if grep 'second diagnostic' "$scratch/out" >/dev/null; then fail "$step dumped extra stderr"; fi
  echo "boundary-hooks test: $step first stderr line: ok"
done
# Exercise both the bulk verify fallback and each plan's operational skip.
body pre-push verify > "$scratch/run"
STUB_INGEST_OK=1 sh "$scratch/run" </dev/null > "$scratch/out" 2>&1 || fail 'verify fallback did not fail open'
grep 're-running plan by plan: second diagnostic' "$scratch/out" >/dev/null || fail 'bulk verify swallowed stderr'
grep 'operational error.*second diagnostic' "$scratch/out" >/dev/null || fail 'per-plan verify swallowed stderr'
grep 'second diagnostic' "$scratch/out" >/dev/null || fail 'verify fallback swallowed final diagnostic'
echo 'boundary-hooks test: verify fallback final diagnostic: ok'
body pre-push refactor-conserve > "$scratch/run"
rc=0
STUB_EXIT=1 sh "$scratch/run" > "$scratch/out" 2>&1 || rc=$?
[ "$rc" -eq 1 ] || fail 'conserve no longer blocks integrity losses'
echo 'boundary-hooks test: conserve integrity block: ok'
# An unwritable TMPDIR makes mktemp fail. Model that failure even for
# workspace-relative templates, without changing permissions on a shared tree.
cat > "$scratch/tools/mktemp" <<'STUB'
#!/bin/sh
printf '%s\n' "$*" >> "$MKTEMP_LOG"
exec /usr/bin/mktemp "$TMPDIR/hook-stderr.XXXXXX"
STUB
chmod +x "$scratch/tools/mktemp"
export MKTEMP_LOG="$scratch/mktemp-log"
for step in review refactor-conserve verify; do
  body pre-push "$step" > "$scratch/run"
  rc=0
  TMPDIR=/dev/null STUB_INGEST_OK=1 sh "$scratch/run" </dev/null > "$scratch/out" 2>&1 || rc=$?
  [ "$rc" -eq 0 ] || fail "$step treated mktemp failure as integrity loss"
  echo "boundary-hooks test: $step mktemp failure: ok"
done
grep 'verify-stderr' "$MKTEMP_LOG" >/dev/null || fail 'verify stderr path is not guarded by mktemp'
# Losing the diagnostic file must still preserve a real integrity block.
body pre-push refactor-conserve > "$scratch/run"
rc=0
TMPDIR=/dev/null STUB_EXIT=1 sh "$scratch/run" > "$scratch/out" 2>&1 || rc=$?
[ "$rc" -eq 1 ] || fail 'conserve lost integrity block when mktemp failed'
echo 'boundary-hooks test: conserve integrity block without stderr file: ok'
# New cases run independently so the red run names every missing behavior.
rm "$scratch/tools/mktemp"
cp bin/fathom "$scratch/fathom-stub"
failures=0
check_case() {
  local name=$1
  shift
  if ( "$@" ) > "$scratch/case-$name.log" 2>&1; then
    echo "boundary-hooks test: $name: ok"
  else
    cat "$scratch/case-$name.log" >&2
    echo "boundary-hooks test: $name: FAILED" >&2
    failures=$((failures + 1))
  fi
}
upstream_case() {
  local kind=$1 repo="$scratch/upstream-$1" rc=0
  export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null
  export GIT_AUTHOR_NAME=Fixture GIT_AUTHOR_EMAIL=fixture@example.invalid GIT_COMMITTER_NAME=Fixture GIT_COMMITTER_EMAIL=fixture@example.invalid
  mkdir -p "$repo/bin" "$repo/workspace/tmp"
  "$real_git" -c init.defaultBranch=main init -q "$repo" || fail 'git init failed'
  cd "$repo"
  "$real_git" -c user.name=Fixture -c user.email=fixture@example.invalid -c commit.gpgsign=false -c core.hooksPath=/dev/null commit --allow-empty -qm fixture || fail 'fixture commit failed'
  "$real_git" remote add origin . || fail 'fixture remote failed'
  "$real_git" config branch.main.remote origin || fail 'fixture remote config failed'
  "$real_git" config branch.main.merge refs/heads/gone || fail 'fixture merge config failed'
  case "$kind" in
    missing)
      "$real_git" rev-parse --abbrev-ref --symbolic-full-name '@{u}' > "$scratch/upstream-stdout" 2> "$scratch/upstream-stderr" || rc=$?
      [ "$rc" -eq 128 ] || fail 'missing upstream did not exit 128'
      [ "$(cat "$scratch/upstream-stdout")" = '@{u}' ] || fail 'missing upstream did not print literal @{u}'
      ;;
    present)
      "$real_git" update-ref refs/remotes/origin/gone HEAD || fail 'upstream ref setup failed'
      ;;
  esac
  mkdir -p "$repo/tools"
  ln -s "$real_git" "$repo/tools/git"
  cp "$scratch/fathom-stub" bin/fathom
  body pre-push refactor-conserve > "$scratch/run"
  PATH="$repo/tools:$PATH" STUB_SUCCESS=1 FATHOM_ARGS_LOG="$repo/args" "$shell" "$scratch/run" > "$repo/out" 2>&1 || fail 'conserve upstream case blocked'
  if [ "$kind" = missing ]; then
    grep -F -- '--since origin/main ' "$repo/args" >/dev/null || fail 'missing upstream was passed to conserve instead of origin/main'
    grep -F "upstream unavailable — using origin/main: $(sed -n '1p' "$scratch/upstream-stderr")" "$repo/out" >/dev/null || fail 'missing upstream warning lost first stderr line'
    [ "$(wc -l < "$repo/out")" -eq 2 ] || fail 'missing upstream dumped extra stderr'
  else
    grep -F -- '--since origin/gone ' "$repo/args" >/dev/null || fail 'real upstream was not preserved'
    [ "$(cat "$repo/out")" = 'conserve ok' ] || fail 'real upstream changed success output'
  fi
}
check_case conserve-missing-upstream upstream_case missing
check_case conserve-real-upstream upstream_case present
# Also reject a literal @{u} even if a git wrapper incorrectly exits zero.
cat > "$scratch/tools/git" <<'STUB'
#!/bin/sh
case "$*" in
  *rev-parse*) echo '@{u}' ;;
  *merge-base*) echo base ;;
esac
STUB
literal_upstream_case() {
  body pre-push refactor-conserve > "$scratch/run"
  STUB_SUCCESS=1 FATHOM_ARGS_LOG="$scratch/literal-args" "$shell" "$scratch/run" > "$scratch/out" 2>&1 || fail 'literal upstream case blocked'
  grep -F -- '--since origin/main ' "$scratch/literal-args" >/dev/null || fail 'literal @{u} was not rejected'
  grep -F 'upstream unavailable — using origin/main:' "$scratch/out" >/dev/null || fail 'literal upstream fallback was silent'
}
check_case conserve-literal-upstream literal_upstream_case
cat > "$scratch/tools/git" <<'STUB'
#!/bin/sh
case "$*" in
  *rev-parse*) echo origin/main ;;
  *merge-base*) echo base ;;
esac
STUB
cat > "$scratch/tools/go" <<'STUB'
#!/bin/sh
printf '%s\n' "GOWORK=${GOWORK:-} $*" >> "$BUILD_LOG"
if [ -n "${BUILD_ENV_LOG:-}" ]; then
  printf '%s\n' "GOWORK=${GOWORK:-} GOTOOLCHAIN=${GOTOOLCHAIN:-} CGO_ENABLED=${CGO_ENABLED:-}" >> "$BUILD_ENV_LOG"
fi
if [ "${BUILD_LEASE_CHECK:-0}" = 1 ]; then
  perl -MFcntl=:flock -e 'open(my $fh, ">>", $ARGV[0]) or die $!; exit(flock($fh, LOCK_EX | LOCK_NB) ? 1 : 0)' "$FATHOM_GATE_LOCK" || {
    echo 'build did not hold the gate lease' >&2
    exit 2
  }
fi
if [ "${BUILD_FAIL:-0}" = 1 ]; then
  printf '# pkg\ngo: downloading fixture\nfirst build diagnostic\nsecond build diagnostic\n' >&2
  exit 2
fi
if [ "${BUILD_HEADERS_ONLY:-0}" = 1 ]; then
  printf '# pkg\ngo: downloading fixture\n' >&2
  exit 2
fi
mkdir -p bin
cp "$FATHOM_STUB" bin/fathom
STUB
build_case() {
  local step=$1 mode=$2 repo="$scratch/build-$1-$2" rc=0
  mkdir -p "$repo/docs/plans" "$repo/scripts/lib"
  cp scripts/gate-lease.sh "$repo/scripts/"
  cp "$root/scripts/fathom-go.sh" "$repo/scripts/"
  cp "$root/scripts/lib/build-cache.sh" "$repo/scripts/lib/"
  cp docs/plans/example.md "$repo/docs/plans/"
  cd "$repo"
  if [ "$mode" = present ]; then
    mkdir -p bin
    cp "$scratch/fathom-stub" bin/fathom
  fi
  body pre-push "$step" > "$scratch/run"
  : > build-log
  : > args
  local build_fail=0
  [ "$mode" != failure ] || build_fail=1
  BUILD_LOG="$repo/build-log" FATHOM_STUB="$scratch/fathom-stub" BUILD_FAIL="$build_fail" GOWORK=fixture.work STUB_SUCCESS=1 STUB_INGEST_OK=1 FATHOM_ARGS_LOG="$repo/args" "$shell" "$scratch/run" </dev/null > out 2>&1 || rc=$?
  if [ "$mode" = present ]; then
    [ ! -s build-log ] || fail "$step rebuilt a present binary"
  else
    [ "$(cat build-log)" = 'GOWORK=off build -o bin/fathom ./cmd/fathom' ] || fail "$step did not build missing binary with GOWORK=off"
    grep -F "fathom ${step/refactor-/}: bin/fathom missing" out >/dev/null || fail "$step did not announce build"
  fi
  if [ "$mode" = failure ]; then
    [ ! -s args ] || fail "$step ran after build failed"
    if [ "$step" = verify ]; then
      [ "$rc" -eq 1 ] || fail 'verify allowed a failed build'
      grep -F 'fathom verify: build failed: first build diagnostic' out >/dev/null || fail 'verify swallowed build error'
    else
      [ "$rc" -eq 0 ] || fail "$step blocked on build failure"
      grep -F "fathom ${step/refactor-/}: build failed -- skipped (fail-open): first build diagnostic" out >/dev/null || fail "$step build skip was silent"
    fi
    if grep -F 'second build diagnostic' out >/dev/null; then fail "$step dumped extra build stderr"; fi
    [ "$(wc -l < out)" -eq 2 ] || fail "$step build failure was not two named lines"
  else
    [ "$rc" -eq 0 ] || fail "$step failed with a working binary"
    grep -E "^${step/refactor-/} " args >/dev/null || fail "$step did not continue with the binary"
    if [ "$step" = verify ]; then
      grep '^ingest ' args >/dev/null || fail 'verify skipped ingest after build'
    fi
    if [ "$mode" = present ]; then
      [ "$(cat out)" = "${step/refactor-/} ok" ] || fail "$step changed success output with binary present"
    fi
  fi
}
for step in verify review refactor-conserve; do
  for mode in success failure present; do
    check_case "$step-build-$mode" build_case "$step" "$mode"
  done
done
# Run the shipped build/cache and lease wrappers with stubbed Go and nice.
# The lock probe checks ownership without relying on elapsed time or sleeps.
cat > "$scratch/tools/nice" <<'STUB'
#!/bin/sh
printf '%s\n' "$*" >> "$BUILD_NICE_LOG"
[ "$1" = -n ] && [ "$2" = 10 ] || exit 2
shift 2
exec "$@"
STUB
chmod +x "$scratch/tools/nice"
# Hold the real lease until the hook reports contention. Polling is only a
# hang guard; no assertion depends on elapsed time or scheduler speed.
cat > "$scratch/tools/held-lease" <<'STUB'
#!/usr/bin/env perl
use strict;
use warnings;
use Fcntl qw(:flock);
open(my $lock, '>>', $ENV{FATHOM_GATE_LOCK}) or die $!;
flock($lock, LOCK_EX) or die $!;
my $pid = fork();
defined($pid) or die $!;
if (!$pid) { close($lock); exec @ARGV; die $!; }
my $waiting = 0;
for (1 .. 600) {
  for my $path (glob('workspace/tmp/*-stderr.*')) {
    open(my $err, '<', $path) or next;
    while (<$err>) { $waiting = 1 if /^gate-lease: waiting for gate lease held by /; }
    close($err);
  }
  last if $waiting;
  select(undef, undef, undef, 0.05);
}
close($lock);
waitpid($pid, 0);
my $rc = $?;
die "hook never reported lease contention\n" unless $waiting;
exit(($rc & 127) ? 128 + ($rc & 127) : $rc >> 8);
STUB
chmod +x "$scratch/tools/held-lease"
build_contract_case() {
  local step=$1 mode=$2 repo="$scratch/contract-$1-$2" rc=0 os=Linux cgo=1
  local -a cgo_env=(env CGO_ENABLED="$cgo") runner=("$shell")
  local cache_root= cache_configured=1 build_fail=0
  mkdir -p "$repo/scripts/lib" "$repo/tools" "$repo/workspace/tmp"
  cp "$root/scripts/"{gate-lease,fathom-go,prepush-refs}.sh "$repo/scripts/"
  cp "$root/scripts/lib/build-cache.sh" "$repo/scripts/lib/"
  cat > "$repo/tools/uname" <<'STUB'
#!/bin/sh
echo "$BUILD_OS"
STUB
  chmod +x "$repo/tools/uname"
  cd "$repo"
  case "$mode" in
    env-darwin-cgo) os=Darwin ;;
    env-darwin-pure) os=Darwin; cgo=0; cgo_env=(env CGO_ENABLED=0) ;;
    env-cgo-unset)
      # Non-Darwin without invoking the host-specific RSS timer.
      os=fixture
      cgo_env=(env -u CGO_ENABLED)
      mkdir -p docs/plans
      printf '%s\n' '[ ] a plan' > docs/plans/example.md
      ;;
    diagnostic-busy-cache)
      runner=("$scratch/tools/held-lease" "$shell")
      cache_root="$repo/cache"
      cache_configured=0
      build_fail=1
      mkdir -p "$cache_root"
      # A failed cache measurement emits the shipped wrapper's warning.
      printf '#!/bin/sh\n: > cache-probed\nexit 1\n' > tools/du
      chmod +x tools/du
      ;;
    tag-*)
      export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null
      export GIT_AUTHOR_NAME=Fixture GIT_AUTHOR_EMAIL=fixture@example.invalid GIT_COMMITTER_NAME=Fixture GIT_COMMITTER_EMAIL=fixture@example.invalid
      "$real_git" -c init.defaultBranch=main init -q . || fail 'tag fixture init failed'
      "$real_git" -c commit.gpgsign=false -c core.hooksPath=/dev/null commit --allow-empty -qm fixture || fail 'tag fixture commit failed'
      "$real_git" remote add origin . || fail 'tag fixture remote failed'
      "$real_git" config branch.main.remote origin || fail 'tag fixture remote config failed'
      "$real_git" config branch.main.merge refs/heads/main || fail 'tag fixture merge config failed'
      "$real_git" update-ref refs/remotes/origin/main HEAD || fail 'tag fixture remote ref failed'
      "$real_git" tag fixture-tag || fail 'tag fixture tag failed'
      ln -s "$real_git" tools/git
      local sha
      sha=$("$real_git" rev-parse HEAD)
      printf 'refs/tags/fixture-tag %s refs/tags/fixture-tag %040d\n' "$sha" 0 > refs
      PATH="$repo/tools:$PATH" bash scripts/prepush-refs.sh decide origin < refs > /dev/null || fail 'tag fixture should skip'
      if [ "$mode" = tag-present ]; then
        mkdir -p bin
        cp "$scratch/fathom-stub" bin/fathom
      fi
      ;;
  esac
  : > build-log
  : > build-env
  : > nice-log
  : > args
  : > command-env
  : > refs-empty
  local headers_only=0 lease_check=0 input=refs-empty
  [ "$mode" != diagnostic-fallback ] || headers_only=1
  [ "$mode" != lease ] || lease_check=1
  case "$mode" in tag-*) input=refs ;; esac
  body pre-push "$step" | sed 's/{1}/origin/g' > "$scratch/run"
  LC_ALL=C PATH="$repo/tools:$PATH" BUILD_OS="$os" GOWORK=fixture.work GOTOOLCHAIN=auto \
    BUILD_LOG="$repo/build-log" BUILD_ENV_LOG="$repo/build-env" BUILD_NICE_LOG="$repo/nice-log" \
    BUILD_HEADERS_ONLY="$headers_only" BUILD_LEASE_CHECK="$lease_check" BUILD_FAIL="$build_fail" \
    FATHOM_GATE_LOCK="$repo/workspace/tmp/gate.lock" FATHOM_GATE_NICE=on \
    FATHOM_GO_BINARY=go FATHOM_CACHE_ROOT="$cache_root" FATHOM_CACHE_CONFIGURED="$cache_configured" \
    FATHOM_STUB="$scratch/fathom-stub" STUB_SUCCESS=1 STUB_INGEST_OK=1 FATHOM_ARGS_LOG="$repo/args" \
    COMMAND_ENV_LOG="$repo/command-env" "${cgo_env[@]}" "${runner[@]}" "$scratch/run" < "$input" > out 2>&1 || rc=$?
  case "$mode" in
    env-cgo-unset)
      [ "$rc" -eq 0 ] && [ -x bin/fathom ] || fail "$step did not build successfully"
      grep ' CGO_ENABLED=0$' build-env >/dev/null || fail "$step did not scope pure-Go settings to the build"
      grep -Fx "${step/refactor-/} CGO_ENABLED=unset" command-env >/dev/null || fail "$step leaked build CGO_ENABLED into the later command: $(cat command-env)"
      if [ "$step" = verify ]; then
        grep -Fx 'ingest CGO_ENABLED=unset' command-env >/dev/null || fail 'verify leaked build CGO_ENABLED into ingest'
      fi
      ;;
    env-*)
      [ "$rc" -eq 0 ] && [ -x bin/fathom ] || fail "$step did not build successfully"
      [ "$os" = Darwin ] || cgo=0
      if grep 'GOTOOLCHAIN=' "$scratch/run" >/dev/null; then fail "$step sets GOTOOLCHAIN instead of inheriting it"; fi
      [ "$(cat build-env)" = "GOWORK=off GOTOOLCHAIN=auto CGO_ENABLED=$cgo" ] || fail "$step changed inherited build environment: $(cat build-env)"
      if [ "$step" != verify ]; then
        grep -Fx "${step/refactor-/} CGO_ENABLED=${cgo_env[1]#CGO_ENABLED=}" command-env >/dev/null || fail "$step changed the later command's CGO_ENABLED: $(cat command-env)"
      fi
      ;;
    lease)
      [ "$rc" -eq 0 ] && [ -x bin/fathom ] || fail "$step built outside gate lease"
      [ "$(cat nice-log)" = '-n 10 bash scripts/fathom-go.sh build -o bin/fathom ./cmd/fathom' ] || fail "$step build did not run niced through the build wrapper"
      ;;
    diagnostic-fallback)
      if [ "$step" = verify ]; then
        [ "$rc" -eq 1 ] || fail 'verify allowed failed header-only build'
      else
        [ "$rc" -eq 0 ] || fail "$step blocked on failed header-only build"
      fi
      grep -F 'build failed' out | grep -F ': # pkg' >/dev/null || fail "$step lost fallback first line: $(cat out)"
      [ "$(wc -l < out)" -eq 2 ] || fail "$step dumped header-only build stderr"
      [ ! -s args ] || fail "$step ran after failed header-only build"
      ;;
    diagnostic-busy-cache)
      [ -f cache-probed ] || fail "$step did not exercise the cache warning"
      if [ "$step" = verify ]; then
        [ "$rc" -eq 1 ] || fail 'verify allowed failed build after lease contention'
      else
        [ "$rc" -eq 0 ] || fail "$step blocked on failed build after lease contention"
      fi
      grep -F 'build failed' out | grep -F ': first build diagnostic' >/dev/null || fail "$step hid the real build error behind lease/cache diagnostics: $(cat out)"
      [ "$(wc -l < out)" -eq 2 ] || fail "$step dumped lease/cache or extra build diagnostics"
      [ ! -s args ] || fail "$step ran after failed build"
      ;;
    tag-*)
      [ "$rc" -eq 0 ] || fail "$step blocked tag-only push"
      [ ! -s build-log ] && [ ! -s nice-log ] || fail "$step built on tag-only push"
      if [ "$step" = verify ]; then
        [ ! -s args ] || fail 'verify ran on tag-only push'
        grep -F 'no commit is new to the remote' out >/dev/null || fail 'verify tag-only skip was silent'
      elif [ "$mode" = tag-missing ]; then
        [ ! -s args ] || fail "$step ran without binary on tag-only push"
        [ "$(cat out)" = "fathom ${step/refactor-/}: skipped: no commit is new to the remote" ] || fail "$step tag-only skip was not named"
      else
        [ "$(cat out)" = "${step/refactor-/} ok" ] || fail "$step changed tag-only output with binary present"
        grep -E "^${step/refactor-/} " args >/dev/null || fail "$step skipped existing binary on tag-only push"
      fi
      ;;
  esac
}
for step in verify review refactor-conserve; do
  for mode in env-linux env-darwin-cgo env-darwin-pure env-cgo-unset lease diagnostic-fallback diagnostic-busy-cache tag-missing tag-present; do
    check_case "$step-$mode" build_contract_case "$step" "$mode"
  done
done
mkdir "$scratch/no-cue"
cue_missing_case() {
  local ci=$1 rc=0
  body pre-commit cue-vet > "$scratch/run"
  CI="$ci" PATH="$scratch/no-cue" "$shell" "$scratch/run" > "$scratch/out" 2>&1 || rc=$?
  if [ "$ci" = true ]; then
    [ "$rc" -eq 1 ] || fail 'cue missing in CI did not fail'
    [ "$(cat "$scratch/out")" = 'cue-vet: cue is missing (required in CI)' ] || fail 'CI cue error did not name missing tool in one line'
  else
    [ "$rc" -eq 0 ] || fail 'cue missing locally did not fail open'
    [ "$(cat "$scratch/out")" = 'cue-vet: skipped -- cue is missing' ] || fail 'local cue skip was silent'
  fi
}
check_case cue-missing-ci cue_missing_case true
check_case cue-missing-local cue_missing_case ''
check_case cue-missing-ci-false cue_missing_case false
cue_present_case() {
  local ci=$1 repo="$scratch/cue-present-$1"
  mkdir -p "$repo/schema"
  : > "$repo/schema/schema.cue"
  cd "$repo"
  body pre-commit cue-vet | sed 's|{staged_files}|schema/schema.cue|g' > "$scratch/run"
  CI="$ci" CUE_LOG="$repo/cue-log" "$shell" "$scratch/run" > out 2>&1 || fail 'cue present failed'
  [ "$(cat cue-log)" = 'vet ./...' ] || fail 'cue vet was not called'
  [ ! -s out ] || fail 'cue present success output changed'
}
cat > "$scratch/tools/cue" <<'STUB'
#!/bin/sh
printf '%s\n' "$*" >> "$CUE_LOG"
STUB
chmod +x "$scratch/tools/cue"
check_case cue-present-ci cue_present_case true
check_case cue-present-local cue_present_case false
[ "$failures" -eq 0 ] || fail "$failures new cases failed"
echo 'boundary-hooks test: ok'
