#!/usr/bin/env bash
# attest.sh <clone-dir> — the Taskfile tests:attest recipe, verbatim in shape:
# go test -json ./... into workspace/artifacts/test-results, then test-kit's
# surveyor go-test-json adapter bound to the clone's HEAD commit.
set -eu
export GOCACHE=/Volumes/m2ext/tmp/cv3-build GOMODCACHE=/Volumes/m2ext/tmp/cv3-mod GOTMPDIR=/Volumes/m2ext/tmp/cv3-tmp TMPDIR=/Volumes/m2ext/tmp/cv3-tmp GOWORK=off CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0 GOFLAGS=-mod=mod
M="${CVM13_DIR:?set CVM13_DIR to a scratch directory}"
cd "$1"
out=workspace/artifacts/test-results
mkdir -p "$out"
commit=$(git rev-parse HEAD)
dirty=false
[ -z "$(git status --porcelain)" ] || dirty=true
started=$(date -u +%Y-%m-%dT%H:%M:%SZ)
dirtyarg=""
[ "$dirty" = true ] && dirtyarg="--dirty"
t0=$(date +%s)
go test -json ./... > "$out/go-test.jsonl" 2> "$out/go-test.stderr" || true
t1=$(date +%s)
"$M/bin/surveyor" ingest --format go-test-json \
  --component fathom --suite unit --class unit \
  --started-at "$started" --commit "$commit" $dirtyarg \
  < "$out/go-test.jsonl" > "$out/envelope.json"
echo "attest: $1 commit=$commit dirty=$dirty secs=$((t1-t0))"
echo "post-run porcelain:"; git status --porcelain | head
