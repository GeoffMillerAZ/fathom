#!/usr/bin/env bash
# Only the current push's complete local SHA set can stand in for validate.
set -euo pipefail
script_dir=$(cd "$(dirname "$0")" && pwd -P)
scratch=$(mktemp -d "${TMPDIR:-/tmp}/prepush-receipts.XXXXXX")
trap 'rm -rf "$scratch"' EXIT
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null
export GIT_AUTHOR_NAME=Fixture GIT_AUTHOR_EMAIL=fixture@example.test GIT_COMMITTER_NAME=Fixture GIT_COMMITTER_EMAIL=fixture@example.test
export PREPUSH_SKIP_MARKER="$scratch/marker"
git init -q "$scratch/repo"
cd "$scratch/repo"
git -c core.hooksPath=/dev/null commit -qm first --allow-empty
one=$(git rev-parse HEAD)
git -c core.hooksPath=/dev/null commit -qm second --allow-empty
two=$(git rev-parse HEAD)
zero=0000000000000000000000000000000000000000
printf 'refs/heads/one %s refs/heads/one %s\nrefs/heads/two %s refs/heads/two %s\n' "$one" "$zero" "$two" "$zero" | bash "$script_dir/prepush-refs.sh" decide origin && exit 1
actual=$(bash "$script_dir/prepush-refs.sh" receipt-trees)
expected=$(printf '%s\n%s\n' "$one" "$two" | sort -u)
[ "$actual" = "$expected" ] || { echo "lost pushed SHA: $actual" >&2; exit 1; }
# No stale previous push list after malformed/missing input.
printf 'refs/heads/bad garbage refs/heads/bad %s\n' "$zero" | bash "$script_dir/prepush-refs.sh" decide origin && exit 1
if bash "$script_dir/prepush-refs.sh" receipt-trees; then echo 'malformed push reused old SHAs' >&2; exit 1; fi
printf '\n' | bash "$script_dir/prepush-refs.sh" decide origin && exit 1
if bash "$script_dir/prepush-refs.sh" receipt-trees; then echo 'empty push reused old SHAs' >&2; exit 1; fi
echo 'prepush receipt SHA handoff: PASS'
