#!/usr/bin/env bash
# run.sh — scip-go over a Go calibration subject (tier A; language-breadth §1.3).
# Bench-only: scip-go is an instrument, never a product dependency.
#
#   run.sh <scip-go> <subject-dir> <out.scip>
#
# The module's dependencies download into GOMODCACHE (keep it outside the subject);
# every package of the module, tests included, is indexed.
set -euo pipefail
bin=$1 subj=$2 out=$3
cd "$subj"
"$bin" index --output "$out" ./...
git status --short | grep . && { echo "run.sh: the subject checkout changed" >&2; exit 1; } || true
