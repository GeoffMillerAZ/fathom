#!/usr/bin/env bash
# BE-10: task arguments/env/output, and a read-only changed-file format gate.
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
scratch=$(mktemp -d "${TMPDIR:-/tmp}/targeted-tasks-test.XXXXXX")
trap 'rm -rf "$scratch"' EXIT
fail() { echo "targeted-tasks test: $*" >&2; exit 1; }
mkdir -p "$scratch/repo/scripts/lib" "$scratch/tools"
cp "$root/Taskfile.yml" "$scratch/repo/Taskfile.yml"
cp "$root/scripts/scratch-home.sh" "$root/scripts/fathom-go.sh" "$scratch/repo/scripts/"
cp "$root/scripts/lib/build-cache.sh" "$scratch/repo/scripts/lib/"
for script in test-targeted.sh fmt-changed.sh; do
  [ ! -f "$root/scripts/$script" ] || cp "$root/scripts/$script" "$scratch/repo/scripts/"
done
cat > "$scratch/tools/go" <<'STUB'
#!/bin/sh
printf '%s\n' "$@" > "$ARGS_LOG"
printf '%s\n' "$CGO_ENABLED" "$MACOSX_DEPLOYMENT_TARGET" "$FATHOM_HOME" > "$ENV_LOG"
printf 'noise from a successful test\nok\tfixture/one\t0.1s\n?\tfixture/empty\t[no test files]\n'
if [ "${TEST_EXIT:-0}" != 0 ]; then
  printf '%s\n' '--- FAIL: TestExample (0.00s)' '    example_test.go:3: useful failure' FAIL 'FAIL fixture/two 0.1s'
fi
exit "${TEST_EXIT:-0}"
STUB
chmod +x "$scratch/tools/go"
export FATHOM_GO_BINARY="$scratch/tools/go" ARGS_LOG="$scratch/args" ENV_LOG="$scratch/env"
cd "$scratch/repo"
# task --list is also the discoverability contract.
task --list > "$scratch/list"
grep -E '^\* t:' "$scratch/list" >/dev/null || fail 't not listed'
grep -E '^\* fmt:changed:' "$scratch/list" >/dev/null || fail 'fmt:changed not listed'
task t 'RUN=TestOne|TestTwo' -- ./fixture/one ./fixture/two > "$scratch/out" 2>&1 || fail 't did not run'
printf '%s\n' test -race -count=1 -p 4 -run 'TestOne|TestTwo' ./fixture/one ./fixture/two > "$scratch/want"
cmp "$scratch/args" "$scratch/want" || fail 'wrong test arguments'
[ "$(sed -n '1p' "$scratch/env")" = 1 ] || fail 'race cgo missing'
[ "$(sed -n '2p' "$scratch/env")" = 14.0 ] || fail 'deployment target missing'
grep '/workspace/tmp/fathom-home\.' "$scratch/env" >/dev/null || fail 'scratch home missing'
grep '^ok' "$scratch/out" >/dev/null || fail 'ok line missing'
if grep -E 'noise|no test files' "$scratch/out" >/dev/null; then fail 'successful noise printed'; fi
rc=0
TEST_EXIT=1 task t -- ./fixture/two > "$scratch/out" 2>&1 || rc=$?
[ "$rc" -ne 0 ] || fail 't hid test failure'
grep -F 'example_test.go:3: useful failure' "$scratch/out" >/dev/null || fail 'failure block lost'
if task t > "$scratch/out" 2>&1; then fail 't without packages ran the whole module'; fi
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null
git init -q .
# Build fixture history with commit-tree; never run commit hooks.
printf 'package fixture\n' > unchanged.go
git add .
tree=$(git write-tree)
export GIT_AUTHOR_NAME=Fixture GIT_AUTHOR_EMAIL=fixture@example.com GIT_COMMITTER_NAME=Fixture GIT_COMMITTER_EMAIL=fixture@example.com
base=$(printf 'fixture base\n' | git commit-tree "$tree")
git update-ref HEAD "$base"
# A main-only parse error must not count as a change on this branch.
printf 'not go source\n' > upstream.go
git add upstream.go
upstream_tree=$(git write-tree)
upstream=$(printf 'fixture upstream\n' | git commit-tree "$upstream_tree" -p "$base")
git update-ref refs/remotes/origin/main "$upstream"
git reset -q --hard "$base"
printf 'package fixture\nfunc F( ){ }\n' > 'changed file.go'
git add 'changed file.go'
cp 'changed file.go' "$scratch/before"
if task fmt:changed > "$scratch/out" 2>&1; then fail 'fmt:changed accepted unformatted staged file'; fi
grep -F 'changed file.go' "$scratch/out" >/dev/null || fail 'format offender missing'
cmp 'changed file.go' "$scratch/before" || fail 'format check rewrote source'
printf 'package fixture\n\nfunc F() {}\n' > 'changed file.go'
task fmt:changed > "$scratch/out" 2>&1 || fail 'fmt:changed rejected clean worktree'
printf 'package fixture\nfunc F( ){ }\n' > unchanged.go
# Existing tracked changes also count; deleted files must not reach gofmt.
rm 'changed file.go'
if task fmt:changed > "$scratch/out" 2>&1; then fail 'fmt:changed missed unstaged file'; fi
grep -F 'unchanged.go' "$scratch/out" >/dev/null || fail 'unstaged offender missing'
# Match the GOFILES exclusions, including ignored parse errors.
printf 'package fixture\n' > unchanged.go
mkdir -p third_party nested/_reference nested/testdata .claude tmp ref workspace
for path in third_party/ignored.go nested/_reference/ignored.go nested/testdata/ignored.go .claude/ignored.go tmp/ignored.go ref/ignored.go workspace/ignored.go; do
  printf 'not go source\n' > "$path"
  git add -f "$path"
done
task fmt:changed > "$scratch/out" 2>&1 || fail 'fmt:changed inspected excluded or main-only files'
# One invocation must report both a parse error and another format offender.
printf 'not go source\n' > 'broken file.go'
printf 'package fixture\nfunc F( ){ }\n' > 'another file.go'
git add 'broken file.go' 'another file.go'
real_gofmt=$(command -v gofmt)
export REAL_GOFMT="$real_gofmt" FORMAT_LOG="$scratch/format-log"
cat > "$scratch/tools/gofmt" <<'STUB'
#!/bin/sh
echo call >> "$FORMAT_LOG"
exec "$REAL_GOFMT" "$@"
STUB
chmod +x "$scratch/tools/gofmt"
export PATH="$scratch/tools:$PATH"
if task fmt:changed > "$scratch/out" 2>&1; then fail 'fmt:changed accepted parse error'; fi
grep -F 'broken file.go' "$scratch/out" >/dev/null || fail 'parse offender missing'
grep -F 'another file.go' "$scratch/out" >/dev/null || fail 'other format offender missing'
[ "$(wc -l < "$FORMAT_LOG" | tr -d ' ')" -eq 1 ] || fail 'fmt:changed invokes gofmt more than once'
git update-ref -d refs/remotes/origin/main
if task fmt:changed > "$scratch/out" 2>&1; then fail 'fmt:changed accepted missing base'; fi
grep -F 'origin/main' "$scratch/out" >/dev/null || fail 'missing ref not named'
echo 'targeted-tasks test: ok'
