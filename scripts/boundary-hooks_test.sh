#!/usr/bin/env bash
# BE-4: execute the configured hook bodies with deterministic failures.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
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
if [ "$1" = ingest ] && [ "${STUB_INGEST_OK:-0}" = 1 ]; then
  while [ "$#" -gt 1 ]; do
    if [ "$1" = --store ]; then : > "$2"; fi
    shift
  done
  exit 0
fi
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
done
# Exercise both the bulk verify fallback and each plan's operational skip.
body pre-push verify > "$scratch/run"
STUB_INGEST_OK=1 sh "$scratch/run" </dev/null > "$scratch/out" 2>&1 || fail 'verify fallback did not fail open'
grep 're-running plan by plan: second diagnostic' "$scratch/out" >/dev/null || fail 'bulk verify swallowed stderr'
grep 'operational error.*second diagnostic' "$scratch/out" >/dev/null || fail 'per-plan verify swallowed stderr'
grep 'second diagnostic' "$scratch/out" >/dev/null || fail 'verify fallback swallowed final diagnostic'
body pre-push refactor-conserve > "$scratch/run"
rc=0
STUB_EXIT=1 sh "$scratch/run" > "$scratch/out" 2>&1 || rc=$?
[ "$rc" -eq 1 ] || fail 'conserve no longer blocks integrity losses'
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
done
grep 'verify-stderr' "$MKTEMP_LOG" >/dev/null || fail 'verify stderr path is not guarded by mktemp'
# Losing the diagnostic file must still preserve a real integrity block.
body pre-push refactor-conserve > "$scratch/run"
rc=0
TMPDIR=/dev/null STUB_EXIT=1 sh "$scratch/run" > "$scratch/out" 2>&1 || rc=$?
[ "$rc" -eq 1 ] || fail 'conserve lost integrity block when mktemp failed'
echo 'boundary-hooks test: ok'
