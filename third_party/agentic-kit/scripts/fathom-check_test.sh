#!/usr/bin/env bash
set -euo pipefail

unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_COMMON_DIR GIT_PREFIX GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES FATHOM_CHECK_KEEP FATHOM_TEST_GOMAXPROCS

proposal=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
fixture=$(mktemp -d "${TMPDIR:-/tmp}/agentic-kit-fathom-check-test.XXXXXX")
trap 'rm -rf -- "$fixture"' EXIT
mkdir -p "$fixture/kit" "$fixture/fathom/scripts" "$fixture/bin" "$fixture/tmp"

git -C "$fixture/kit" init -q
git -C "$fixture/kit" -c user.name=Test -c user.email=test@example.invalid commit -q --allow-empty -m initial
printf 'PRIVATE\n' > "$fixture/kit/private.dat"
printf 'private.dat\n' > "$fixture/kit/.gitignore"
git -C "$fixture/kit" add .gitignore
git -C "$fixture/kit" -c user.name=Test -c user.email=test@example.invalid commit -q -m ignore

cat > "$fixture/fathom/scripts/sync-vendor.sh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
[ -d "$AGENTIC_KIT_SIBLING/.git" ]
[ ! -e "$AGENTIC_KIT_SIBLING/private.dat" ]
[ "$GOWORK" = off ]
mkdir -p third_party/agentic-kit
printf '%s\n' "$2" > third_party/agentic-kit/.vendor-commit
SH
cat > "$fixture/fathom/scripts/fathom-go.sh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
printf 'all-package-build:%s:%s\n' "$*" "${GOMAXPROCS:-unset}" >> "$FATHOM_CHECK_TASK_LOG"
[ "$#" -eq 2 ] && [ "$1" = build ] && [ "$2" = './...' ]
[ "${FATHOM_CHECK_FAIL_AT:-}" != all-package-build ]
SH
git -C "$fixture/fathom" init -q
git -C "$fixture/fathom" add scripts
git -C "$fixture/fathom" -c user.name=Test -c user.email=test@example.invalid commit -q -m initial

cat > "$fixture/bin/task" <<'SH'
#!/usr/bin/env bash
printf '%s:%s\n' "$1" "${GOMAXPROCS:-unset}" >> "$FATHOM_CHECK_TASK_LOG"
[ "${FATHOM_CHECK_FAIL_AT:-}" != "$1" ]
SH
chmod +x "$fixture/bin/task"
export PATH="$fixture/bin:$PATH" TMPDIR="$fixture/tmp"
export FATHOM_CHECK_TASK_LOG="$fixture/tasks.log"
export GOMAXPROCS=4

(cd "$fixture/kit" && bash "$proposal/fathom-check.sh") > "$fixture/pass.log"
cmp -s "$fixture/tasks.log" <(printf 'vendor-check:4\nbuild:4\nall-package-build:build ./...:4\ntest:2\nperfgate:4\n')
[ -z "$(find "$fixture/tmp" -mindepth 1 -print -quit)" ]
[ -z "$(git -C "$fixture/fathom" status --porcelain)" ]
[ -z "$(git -C "$fixture/kit" status --porcelain)" ]

: > "$fixture/tasks.log"
if (cd "$fixture/kit" && FATHOM_CHECK_FAIL_AT='test' bash "$proposal/fathom-check.sh") > "$fixture/fail.log" 2>&1; then
  echo 'expected task test failure' >&2
  exit 1
fi
cmp -s "$fixture/tasks.log" <(printf 'vendor-check:4\nbuild:4\nall-package-build:build ./...:4\ntest:2\n')
[ -z "$(find "$fixture/tmp" -mindepth 1 -print -quit)" ]

: > "$fixture/tasks.log"
if (cd "$fixture/kit" && FATHOM_CHECK_FAIL_AT=all-package-build bash "$proposal/fathom-check.sh") > "$fixture/build-fail.log" 2>&1; then
  echo 'expected all-package build failure' >&2
  exit 1
fi
cmp -s "$fixture/tasks.log" <(printf 'vendor-check:4\nbuild:4\nall-package-build:build ./...:4\n')
[ -z "$(find "$fixture/tmp" -mindepth 1 -print -quit)" ]

: > "$fixture/tasks.log"
(cd "$fixture/kit" && FATHOM_TEST_GOMAXPROCS=3 bash "$proposal/fathom-check.sh") > "$fixture/override.log"
cmp -s "$fixture/tasks.log" <(printf 'vendor-check:4\nbuild:4\nall-package-build:build ./...:4\ntest:3\nperfgate:4\n')
[ -z "$(find "$fixture/tmp" -mindepth 1 -print -quit)" ]

: > "$fixture/tasks.log"
if (cd "$fixture/kit" && FATHOM_TEST_GOMAXPROCS=0 bash "$proposal/fathom-check.sh") > "$fixture/invalid.log" 2>&1; then
  echo 'expected invalid test capacity refusal' >&2
  exit 1
fi
[ ! -s "$fixture/tasks.log" ]
[ -z "$(find "$fixture/tmp" -mindepth 1 -print -quit)" ]

: > "$fixture/kit/untracked"
if (cd "$fixture/kit" && bash "$proposal/fathom-check.sh") > "$fixture/dirty.log" 2>&1; then
  echo 'expected dirty kit refusal' >&2
  exit 1
fi
grep -q 'kit_dirty=true' "$fixture/dirty.log"
[ -z "$(find "$fixture/tmp" -mindepth 1 -print -quit)" ]
echo 'lifecycle: PASS (snapshots, ignored data, all-package build, task order, phase CPU isolation, capacity override/refusal, failure propagation, dirty refusal, cleanup, source preservation)'
