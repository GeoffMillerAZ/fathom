#!/usr/bin/env bash
#
# push-check_test.sh — SU-26 (docs/plans/substrate.md section F, ADR-041
# decision 10, invariant I40): proves push-check.sh actually discriminates
# a lefthook pre-push hook from every other state a clone's hooks
# directory can be in, over disposable temp git repos — never the real
# checkout. Wired into `task scripts-check` the same way build-cache_test.sh
# is.
#
# Usage:
#   bash scripts/push-check_test.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PUSH_CHECK="${SCRIPT_DIR}/push-check.sh"

fail() {
  echo "push-check test: $*" >&2
  exit 1
}

scratch="$(mktemp -d "${TMPDIR:-/tmp}/fathom-push-check-test.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT

new_repo() {
  local dir="$1"
  mkdir -p "$dir"
  git -C "$dir" init -q
  # a fresh `git init` writes only *.sample hooks — remove them so each
  # case below starts from a genuinely empty hooks directory, not
  # whatever the local git version ships as samples.
  rm -f "$dir"/.git/hooks/*.sample
}

run_check() {
  local dir="$1"
  (cd "$dir" && bash "$PUSH_CHECK")
}

# Case 1: no pre-push hook at all — refused, exit 3.
repo1="${scratch}/missing-hook"
new_repo "$repo1"
set +e
out1="$(run_check "$repo1" 2>&1)"
rc1=$?
set -e
[ "$rc1" -eq 3 ] || fail "missing hook: exit $rc1, want 3 (output: $out1)"
echo "$out1" | grep -q "refused" || fail "missing hook: output missing 'refused': $out1"

# Case 2: a hook file present but not lefthook's (a generic sample-style
# hook, the exact shape a `git init` ships and the exact shape INC-20's
# wiped clone was left with once something else repopulated it) — refused,
# exit 3.
repo2="${scratch}/sample-hook"
new_repo "$repo2"
cat >"$repo2/.git/hooks/pre-push" <<'EOF'
#!/bin/sh
# Called by "git push" after it has checked the remote status, but before
# anything has been pushed. An ordinary git sample-style hook — the exact
# shape a fresh `git init` ships, deliberately naming no hook manager.
exit 0
EOF
chmod +x "$repo2/.git/hooks/pre-push"
set +e
out2="$(run_check "$repo2" 2>&1)"
rc2=$?
set -e
[ "$rc2" -eq 3 ] || fail "sample-only hook: exit $rc2, want 3 (output: $out2)"
echo "$out2" | grep -q "refused" || fail "sample-only hook: output missing 'refused': $out2"

# Case 3: lefthook's own pre-push hook (the real shape `lefthook install`
# writes — a shebang plus an invocation naming lefthook) — ok, exit 0.
repo3="${scratch}/lefthook-hook"
new_repo "$repo3"
cat >"$repo3/.git/hooks/pre-push" <<'EOF'
#!/bin/sh
# lefthook
call_lefthook()
{
  if command -v lefthook >/dev/null 2>&1; then
    lefthook "$@"
  fi
}
call_lefthook run pre-push "$@"
EOF
chmod +x "$repo3/.git/hooks/pre-push"
set +e
out3="$(run_check "$repo3" 2>&1)"
rc3=$?
set -e
[ "$rc3" -eq 0 ] || fail "lefthook hook: exit $rc3, want 0 (output: $out3)"
echo "$out3" | grep -q "push-check: ok (lefthook pre-push present)" || fail "lefthook hook: unexpected output: $out3"

# Case 4: core.hooksPath points somewhere else entirely, and lefthook's
# hook lives THERE instead of .git/hooks — the INC-20 "unless hooksPath
# points elsewhere, then check there" variant — ok, exit 0.
repo4="${scratch}/hookspath-variant"
new_repo "$repo4"
mkdir -p "$repo4/custom-hooks"
cat >"$repo4/custom-hooks/pre-push" <<'EOF'
#!/bin/sh
# lefthook
call_lefthook()
{
  if command -v lefthook >/dev/null 2>&1; then
    lefthook "$@"
  fi
}
call_lefthook run pre-push "$@"
EOF
chmod +x "$repo4/custom-hooks/pre-push"
git -C "$repo4" config core.hooksPath custom-hooks
# .git/hooks/pre-push must NOT exist here — this case is only meaningful
# if push-check.sh is actually following core.hooksPath, not the default.
[ ! -e "$repo4/.git/hooks/pre-push" ] || fail "hooksPath variant: setup bug — a default-path hook also exists"
set +e
out4="$(run_check "$repo4" 2>&1)"
rc4=$?
set -e
[ "$rc4" -eq 0 ] || fail "hooksPath variant: exit $rc4, want 0 (output: $out4)"
echo "$out4" | grep -q "push-check: ok (lefthook pre-push present)" || fail "hooksPath variant: unexpected output: $out4"

echo "push-check test: PASS"
