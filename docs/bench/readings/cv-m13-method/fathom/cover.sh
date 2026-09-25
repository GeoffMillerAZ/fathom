#!/usr/bin/env bash
# cover.sh <clone-dir> <tag> — execution truth at the same head:
# go test -coverpkg=./... -coverprofile, then go tool cover -func.
set -eu
export GOCACHE=/Volumes/m2ext/tmp/cv3-build GOMODCACHE=/Volumes/m2ext/tmp/cv3-mod GOTMPDIR=/Volumes/m2ext/tmp/cv3-tmp TMPDIR=/Volumes/m2ext/tmp/cv3-tmp GOWORK=off CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0 GOFLAGS=-mod=mod
M="${CVM13_DIR:?set CVM13_DIR to a scratch directory}"
clone=$1; tag=$2
mkdir -p $M/cover $M/fathom
cd "$clone"
echo "head $(git rev-parse HEAD)"
t0=$(date +%s)
go test -json -coverpkg=./... -coverprofile="$M/cover/$tag.coverprofile" ./... > "$M/cover/$tag-cover-run.jsonl" 2> "$M/cover/$tag-cover-run.stderr" || echo "go test rc=$?"
t1=$(date +%s)
go tool cover -func="$M/cover/$tag.coverprofile" > "$M/fathom/$tag-cover-func.txt"
echo "cover: secs=$((t1-t0)) funcs=$(wc -l < $M/fathom/$tag-cover-func.txt)"
echo "post-run porcelain:"; git status --porcelain | head
