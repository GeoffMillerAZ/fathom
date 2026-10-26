#!/usr/bin/env bash
# Check committed agentic-kit HEAD against Fathom's committed HEAD and its
# consumer-owned gates. The source checkouts are never changed.
set -euo pipefail

# Hooks can export Git paths; every git -C below must address its own clone.
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_COMMON_DIR GIT_PREFIX GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES

kit_root=$(git rev-parse --show-toplevel)
kit_root=$(cd "$kit_root" && pwd -P)
consumer_root=
for sibling in fathom sextant; do
  candidate="$(dirname "$kit_root")/$sibling"
  if [ -d "$candidate/.git" ] || [ -f "$candidate/.git" ]; then
    consumer_root=$(cd "$candidate" && pwd -P)
    break
  fi
done
if [ -z "$consumer_root" ]; then
  echo 'fathom-check: fathom/sextant not present — skipping (consumer untested)'
  exit 0
fi

kit_sha=$(git -C "$kit_root" rev-parse --verify 'HEAD^{commit}')
consumer_sha=$(git -C "$consumer_root" rev-parse --verify 'HEAD^{commit}')
kit_dirty=false
if [ -n "$(git -C "$kit_root" status --porcelain --untracked-files=normal)" ]; then
  kit_dirty=true
fi
consumer_dirty=false
if [ -n "$(git -C "$consumer_root" status --porcelain --untracked-files=normal)" ]; then
  consumer_dirty=true
fi
original_pin=$(cat "$consumer_root/third_party/agentic-kit/.vendor-commit" 2>/dev/null || echo missing)
echo "fathom-check: kit=$kit_sha kit_dirty=$kit_dirty consumer=$consumer_sha consumer_dirty=$consumer_dirty original_pin=$original_pin"
if [ "$kit_dirty" = true ]; then
  echo 'fathom-check: agentic-kit working tree is dirty — commit first; the mirror must match a commit' >&2
  exit 1
fi

test_procs=${FATHOM_TEST_GOMAXPROCS:-2}
if [[ ! "$test_procs" =~ ^[1-9][0-9]*$ ]]; then
  echo 'fathom-check: FATHOM_TEST_GOMAXPROCS must be a positive integer' >&2
  exit 2
fi
echo "fathom-check: race_gomaxprocs=$test_procs performance_gomaxprocs=${GOMAXPROCS:-default}"

tmp_base=$(cd "${TMPDIR:-/tmp}" && pwd -P)
case "$tmp_base/" in
  "$kit_root/"*|"$consumer_root/"*)
    echo 'fathom-check: TMPDIR must be outside both repositories' >&2
    exit 2
    ;;
esac
scratch=$(mktemp -d "$tmp_base/agentic-kit-fathom-check.XXXXXX")
cleanup() {
  if [ "${FATHOM_CHECK_KEEP:-0}" = 1 ]; then
    echo "fathom-check: kept scratch=$scratch"
  else
    rm -rf -- "$scratch"
  fi
}
trap cleanup EXIT

# Local clones materialize only committed trees. --no-hardlinks keeps the
# scratch object stores independent of the live repositories.
git clone --quiet --no-hardlinks --no-checkout -- "$kit_root" "$scratch/agentic-kit"
git -C "$scratch/agentic-kit" checkout --quiet --detach "$kit_sha"
git clone --quiet --no-hardlinks --no-checkout -- "$consumer_root" "$scratch/fathom"
git -C "$scratch/fathom" checkout --quiet --detach "$consumer_sha"

cd "$scratch/fathom"
unset KITREWRITE
export AGENTIC_KIT_SIBLING="$scratch/agentic-kit" GOWORK=off
bash scripts/sync-vendor.sh agentic-kit "$kit_sha"
result_pin=$(cat third_party/agentic-kit/.vendor-commit)
if [ "$result_pin" != "$kit_sha" ]; then
  echo "fathom-check: mirror pin mismatch: got $result_pin, expected $kit_sha" >&2
  exit 1
fi
echo "fathom-check: resulting_pin=$result_pin"

task vendor-check
task build
bash scripts/fathom-go.sh build ./...
# The owned suite runs four command shards alongside four package workers.
# Bound correctness-test contention without throttling CPU-ratio measurements.
GOMAXPROCS="$test_procs" task test
task perfgate
echo "fathom-check: PASS kit=$kit_sha consumer=$consumer_sha resulting_pin=$result_pin"
