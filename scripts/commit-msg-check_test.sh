#!/usr/bin/env bash
# BE-2: fixture messages share the local hook and CI checker.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
scratch=$(mktemp -d "${TMPDIR:-/tmp}/commit-msg-test.XXXXXX")
trap 'rm -rf "$scratch"' EXIT
fail() { echo "commit-msg test: $*" >&2; exit 1; }
check() {
  local want=$1 message=$2 rc=0
  printf '%s\n' "$message" > "$scratch/message"
  bash "$root/scripts/commit-msg-check.sh" "$scratch/message" > "$scratch/out" 2>&1 || rc=$?
  if [ "$want" = pass ] && [ "$rc" -ne 0 ]; then fail "rejected $message: $(cat "$scratch/out")"; fi
  if [ "$want" = fail ] && [ "$rc" -eq 0 ]; then fail "accepted $message"; fi
}
for type in bench build calibration chore ci docs feat fix perf refactor release revert style test; do
  check pass "$type(scope)!: a subject

Verified: a command"
done
check pass 'fix: a subject

Verified: a command'
check fail '# ignored comment
arbitrary subject

Verified: a command'
check pass '# ignored comment
fix: a subject

Verified: a command'
check fail 'arbitrary subject

Verified: a command'
check fail 'unknown: a subject

Verified: a command'
check fail 'fix: '
check fail 'revert: '
check fail 'fixup! '
check fail 'merge '
check fail 'fix: a subject'
check fail 'fix: a subject

Verified:'
check fail 'fix: a subject

prefix Verified: a command'
for subject in 'Merge branch topic' 'merge topic: a summary' 'merge: topic' 'Revert "fix: a subject"' 'revert: a subject' 'fixup! fix: a subject' 'squash! fix: a subject'; do
  check pass "$subject"
  check fail "$subject

Co-Authored-By: Person <person@example.com>"
done
for trailer in 'Co-Authored-By: Person <person@example.com>' 'Claude-Session: session'; do
  check fail "fix: a subject

Verified: a command
$trailer"
done
# The checker must not hide a forbidden line behind casing or indentation.
check fail 'fix: a subject

Verified: a command
 co-authored-by: Person <person@example.com>'
# A real merge may have an arbitrary edited subject.
mkdir -p "$scratch/merge-repo"
git init -q "$scratch/merge-repo"
printf 'a merge parent\n' > "$scratch/merge-repo/.git/MERGE_HEAD"
(cd "$scratch/merge-repo" && check pass 'an edited merge subject')
rm "$scratch/merge-repo/.git/MERGE_HEAD"
check pass 'fix: a subject

Verified: a command
# Co-authored-by: comment
# ------------------------ >8 ------------------------
diff --git a/message b/message
+Co-authored-by: diff content
Co-authored-by: diff context'
check fail 'fix: a subject
# Verified: comment only
# ------------------------ >8 ------------------------
Verified: diff content'
grep -F 'run: bash scripts/commit-msg-check.sh "{1}"' "$root/lefthook.yml" >/dev/null || fail 'commit-msg hook missing'
grep -F 'git cat-file -e "$BEFORE^{commit}"' "$root/.github/workflows/ci.yml" >/dev/null || fail 'unknown-before fallback missing'
# Run the exact CI range loop against deterministic git output: all messages
# must be checked, and an invalid later commit must fail the step.
mkdir -p "$scratch/repo/scripts" "$scratch/tools"
cp "$root/scripts/commit-msg-check.sh" "$scratch/repo/scripts/"
awk '
  /- name: Check pushed commit messages/ { step=1; next }
  step && /        run: \|/ { run=1; next }
  run && /^          / { print substr($0,11); next }
  run { exit }
' "$root/.github/workflows/ci.yml" > "$scratch/ci-run"
[ -s "$scratch/ci-run" ] || fail 'CI range script missing'
cat > "$scratch/tools/git" <<'STUB'
#!/bin/sh
case "$1" in
  cat-file) [ "${UNKNOWN_BEFORE:-0}" = 0 ] ;;
  rev-list)
    case "$*" in
      'rev-list --reverse base..head') printf 'one\ntwo\nmerged\n' ;;
      'rev-list --parents -n1 merged') echo 'merged parent1 parent2' ;;
      'rev-list --parents -n1 '*) echo "$4 parent" ;;
      *) exit 2 ;;
    esac ;;
  rev-parse) exit 1 ;;
  show)
    echo "$4" >> "$CHECKED_LOG"
    if [ "${BAD_ALL:-0}" = 1 ] || { [ "$4" = two ] && [ "${BAD_SECOND:-0}" = 1 ]; }; then
      printf 'fix: missing evidence\n'
    else
      printf 'fix: valid message\n\nVerified: a command\n'
    fi ;;
  *) exit 2 ;;
esac
STUB
chmod +x "$scratch/tools/git"
export PATH="$scratch/tools:$PATH" RUNNER_TEMP="$scratch" BEFORE=base AFTER=head CHECKED_LOG="$scratch/checked"
(cd "$scratch/repo" && bash "$scratch/ci-run") > "$scratch/out" 2>&1 || fail 'CI rejected valid range'
printf 'one\ntwo\n' > "$scratch/want"
cmp "$scratch/checked" "$scratch/want" || fail 'CI did not check every commit'
if (cd "$scratch/repo" && BAD_SECOND=1 bash "$scratch/ci-run") > "$scratch/out" 2>&1; then fail 'CI accepted invalid later commit'; fi
# Failures must accumulate, and an unknown BEFORE must check AFTER alone.
: > "$CHECKED_LOG"
if (cd "$scratch/repo" && BAD_ALL=1 bash "$scratch/ci-run") > "$scratch/out" 2>&1; then fail 'CI accepted bad range'; fi
cmp "$scratch/checked" "$scratch/want" || fail 'CI stopped before reporting every bad commit'
[ "$(grep -c 'body requires' "$scratch/out")" -eq 2 ] || fail 'CI did not report both invalid commits'
: > "$CHECKED_LOG"
(cd "$scratch/repo" && UNKNOWN_BEFORE=1 bash "$scratch/ci-run") > "$scratch/out" 2>&1 || fail 'CI rejected unknown BEFORE fallback'
printf 'head\n' > "$scratch/want"
cmp "$scratch/checked" "$scratch/want" || fail 'unknown BEFORE did not check AFTER alone'
echo 'commit-msg test: ok'
