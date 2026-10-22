#!/usr/bin/env bash
# BE-1: check tracked content and names, including the staged snapshot.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
check="$root/scripts/check-merge-debris.sh"
scratch=$(mktemp -d "${TMPDIR:-/tmp}/merge-debris-test.XXXXXX")
trap 'rm -rf "$scratch"' EXIT
fail() { echo "merge-debris test: $*" >&2; exit 1; }
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null
git init -q "$scratch/repo"
cd "$scratch/repo"
printf 'clean\n' > 'clean file'
git add -- 'clean file'
bash "$check" || fail 'clean tree rejected'
for marker in '<<<<<<< topic' '>>>>>>> topic' '||||||| base' '<<<<<<<' '>>>>>>>' '|||||||'; do
  printf 'clean\n%s\n' "$marker" > conflict
  git add conflict
  if bash "$check" > "$scratch/out"; then fail "accepted $marker"; fi
  grep -q '^conflict:2:' "$scratch/out" || fail 'missing file:line'
done
rm conflict
git rm -q --cached conflict
for name in 'backup-e' 'backup-E' 'backup.orig' 'backup.rej'; do
  : > "$name"
  git add -- "$name"
  if bash "$check" > "$scratch/out"; then fail "accepted $name"; fi
  grep -F "$name:1:" "$scratch/out" >/dev/null || fail 'missing backup location'
  git rm -q --cached -- "$name"
  rm "$name"
done
mkdir -p testdata nested/testdata third_party
printf '<<<<<<< ignored\n' > testdata/conflict
printf '>>>>>>> ignored\n' > nested/testdata/conflict
: > third_party/backup.orig
printf '<<<<<<<< not a marker\n|||||||| not a marker\n>>>>>>>suffix\n    <<<<<<< indented\n' > clean
git add -- testdata nested third_party clean
bash "$check" || fail 'excluded or non-marker content rejected'
printf '<<<<<<< staged\n' > 'clean file'
git add -- 'clean file'
printf 'clean\n' > 'clean file'
if bash "$check" --staged > "$scratch/out"; then fail 'missed staged conflict'; fi
grep -q '^clean file:1:' "$scratch/out" || fail 'staged location missing'
bash "$check" || fail 'whole-tree check read index instead of worktree'
printf 'clean\n' > 'clean file'
git add -- 'clean file'
printf '>>>>>>> unstaged\n' > 'clean file'
bash "$check" --staged || fail 'staged check read unstaged content'
: > untracked.rej
bash "$check" --staged || fail 'checked untracked backup'
grep -F 'run: bash scripts/check-merge-debris.sh --staged' "$root/lefthook.yml" >/dev/null || fail 'pre-commit does not check staged snapshot'
grep -E 'deps: \[.*merge-debris-check' "$root/Taskfile.yml" >/dev/null || fail 'validate does not scan the tree'
echo 'merge-debris test: ok' 
