#!/usr/bin/env bash
# readingset.sh <clone-dir> <base> <tag> — scratch-store ingest (+ the
# commit-bound envelope), fathom conserve --json, and the throwaway
# every-function dump, all with the d41cdf14 fathom.
set -eu
export GOCACHE=/Volumes/m2ext/tmp/cv3-build GOMODCACHE=/Volumes/m2ext/tmp/cv3-mod GOTMPDIR=/Volumes/m2ext/tmp/cv3-tmp TMPDIR=/Volumes/m2ext/tmp/cv3-tmp GOWORK=off CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0 GOFLAGS=-mod=mod
M="${CVM13_DIR:?set CVM13_DIR to a scratch directory}"
F=$M/bin/fathom
clone=$1; base=$2; tag=$3
st=$M/stores/$tag.sqlite
out=$M/fathom
mkdir -p $M/stores $out
cd "$clone"
echo "porcelain before ingest:"; git status --porcelain | head
rm -f "$st"
$F ingest --service fathom --path . --store "$st"
if [ "${EXPLICIT_TESTS:-0}" = 1 ]; then
  $F tests ingest --service fathom --store "$st" --manifest . --results workspace/artifacts/test-results
fi
$F conserve --since "$base" --json --service fathom --store "$st" > "$out/$tag-conserve.json" || echo "conserve rc=$?"
cd $M/tool
CVM13_ROOT="$clone" CVM13_BASE="$base" CVM13_STORE="$st" CVM13_SERVICE=fathom CVM13_OUT="$out/$tag-classes.json" \
  go test -count=1 -run TestCVM13Dump -v ./internal/conserve 2>&1 | tail -8
