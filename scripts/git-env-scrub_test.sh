#!/usr/bin/env bash
# O1505: git exports GIT_DIR to a hook run from a linked worktree, so a test that runs git in a
# scratch repository under the pre-push gate writes the pushing repository's index. Both pre-push
# commands scrub the git location variables, and so does every shell test that creates a repository.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail() { echo "git-env-scrub test: $*" >&2; exit 1; }
want='unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX GIT_COMMON_DIR GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES'
n=$(grep -cF "$want" "$root/lefthook.yml" || true)
[ "$n" -ge 2 ] || fail "lefthook.yml scrubs the git variables in $n pre-push commands, want validate and verify"
for t in "$root"/scripts/*_test.sh; do
  grep -qE '(^|[^a-z-])git( -C [^ ]+)? init' "$t" || continue
  grep -qF "$want" "$t" || fail "$(basename "$t") creates a repository without scrubbing the git variables"
done
echo "git-env-scrub test: ok"
