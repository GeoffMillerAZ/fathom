#!/usr/bin/env bash
# prepush-refs_test.sh — the pre-push gate skips validate and verify when no
# commit is new to the remote, and keeps the full gate for everything else:
# a tag-only push of a commit the remote holds and a delete skip; a tag plus a
# new commit, a branch push, a tag on a commit the remote lacks, an unreadable
# line and a terminal stdin do not. Run by `task scripts-check`. Works in a
# scratch repo with a bare remote, never the real checkout.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"

fail() {
  echo "prepush-refs test: $*" >&2
  exit 1
}

scratch="$(mktemp -d "${TMPDIR:-/tmp}/fathom-prepush-refs-test.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.com GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.com
export PREPUSH_SKIP_MARKER="${scratch}/marker"

git init -q --bare "${scratch}/remote.git"
git init -q -b main "${scratch}/work"
cd "${scratch}/work"
git remote add origin "${scratch}/remote.git"
git commit -q --allow-empty -m one
pushed=$(git rev-parse HEAD)
git push -q origin main # origin/main now holds `pushed`
git commit -q --allow-empty -m two
fresh=$(git rev-parse HEAD) # a commit the remote does not hold
git tag -a -m old v-old "$pushed" # annotated tag on a pushed commit
git tag v-new "$fresh"
zero=0000000000000000000000000000000000000000
oldtag=$(git rev-parse v-old)

decide() { printf '%s\n' "$@" | bash "${SCRIPT_DIR}/prepush-refs.sh" decide; }
want_skip() {
  local what="$1"
  shift
  decide "$@" >"${scratch}/out" || fail "$what: want skip, got the full gate"
  grep -q "skipping validate and verify" "${scratch}/out" || fail "$what: no skip line: $(cat "${scratch}/out")"
  bash "${SCRIPT_DIR}/prepush-refs.sh" skipped || fail "$what: the marker does not say skip"
}
want_full() {
  local what="$1"
  shift
  if decide "$@" >"${scratch}/out"; then fail "$what: want the full gate, got skip"; fi
  if bash "${SCRIPT_DIR}/prepush-refs.sh" skipped; then fail "$what: a marker survived a full-gate verdict"; fi
}

want_skip "tag-only push (annotated tag, commit on the remote)" "refs/tags/v-old $oldtag refs/tags/v-old $zero"
want_skip "lightweight tag on a pushed commit" "refs/tags/v-light $pushed refs/tags/v-light $zero"
want_skip "delete-only push" "(delete) $zero refs/heads/gone $pushed"
want_skip "two tags, one delete" "refs/tags/v-old $oldtag refs/tags/v-old $zero" "refs/tags/v-light $pushed refs/tags/v-light $zero" "(delete) $zero refs/tags/old $pushed"

want_full "tag plus a new commit" "refs/tags/v-old $oldtag refs/tags/v-old $zero" "refs/heads/main $fresh refs/heads/main $pushed"
want_full "branch push" "refs/heads/main $fresh refs/heads/main $pushed"
want_full "new branch" "refs/heads/feat $fresh refs/heads/feat $zero"
want_full "tag on a commit the remote lacks" "refs/tags/v-new $fresh refs/tags/v-new $zero"
want_full "unresolvable sha" "refs/tags/v-x ffffffffffffffffffffffffffffffffffffffff refs/tags/v-x $zero"
want_full "no ref lines" ""

# A stale skip marker never carries over: a skip verdict, then a full one.
want_skip "again" "refs/tags/v-old $oldtag refs/tags/v-old $zero"
want_full "after a skip" "refs/heads/main $fresh refs/heads/main $pushed"

# The marker names HEAD: a marker for another HEAD does not skip.
want_skip "before moving HEAD" "refs/tags/v-old $oldtag refs/tags/v-old $zero"
git commit -q --allow-empty -m three
if bash "${SCRIPT_DIR}/prepush-refs.sh" skipped; then fail "a marker for an older HEAD skipped the gate"; fi

# A terminal stdin is the full gate (no ref lines to read).
if command -v script >/dev/null 2>&1 && [ "$(uname)" = Darwin ]; then
  if script -q /dev/null bash "${SCRIPT_DIR}/prepush-refs.sh" decide >/dev/null 2>&1; then fail "terminal stdin skipped the gate"; fi
fi

# End to end under lefthook, when it is installed: the tag-only push runs no
# validate (the stub would write a file), and a push with a new commit does.
if command -v lefthook >/dev/null 2>&1; then
  e2e="${scratch}/e2e"
  git clone -q "${scratch}/remote.git" "$e2e" 2>/dev/null
  cd "$e2e"
  git checkout -q main
  mkdir -p scripts workspace/tmp
  cp "${SCRIPT_DIR}/prepush-refs.sh" scripts/
  cat >lefthook.yml <<'YML'
pre-push:
  commands:
    validate:
      run: |
        if bash scripts/prepush-refs.sh skipped; then exit 0; fi
        echo ran >>workspace/tmp/validate.ran
    verify:
      priority: 1
      use_stdin: true
      run: |
        refs=""
        if [ ! -t 0 ]; then refs=$(cat); fi
        if [ -n "$refs" ] && printf '%s\n' "$refs" | bash scripts/prepush-refs.sh decide; then exit 0; fi
        echo ran >>workspace/tmp/verify.ran
YML
  unset PREPUSH_SKIP_MARKER
  lefthook install >/dev/null 2>&1 || fail "lefthook install failed"
  git tag v-tag-only
  perl -e 'alarm 60; exec @ARGV' git push -q origin v-tag-only >/dev/null 2>&1 || fail "tag-only push failed or hung"
  [ ! -e workspace/tmp/validate.ran ] && [ ! -e workspace/tmp/verify.ran ] || fail "a tag-only push ran the gate"
  echo four >four.txt
  git add four.txt
  git commit -q -m four # a file in the commit: lefthook skips a push with no files
  perl -e 'alarm 60; exec @ARGV' git push -q origin main >/dev/null 2>&1 || fail "branch push failed or hung"
  [ -e workspace/tmp/validate.ran ] && [ -e workspace/tmp/verify.ran ] || fail "a push with a new commit skipped the gate"
fi

echo "prepush-refs test: ok"
