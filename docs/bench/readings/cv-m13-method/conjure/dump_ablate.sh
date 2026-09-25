#!/bin/bash
# usage: dump_ablate.sh <tag> <base> <head> <storeTag>
set -u
export GOCACHE=/Volumes/m2ext/tmp/cv3-build GOMODCACHE=/Volumes/m2ext/tmp/cv3-mod GOTMPDIR=/Volumes/m2ext/tmp/cv3-tmp TMPDIR=/Volumes/m2ext/tmp/cv3-tmp GOWORK=off CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0 GOFLAGS=-mod=mod
M="${CVM13_DIR:?set CVM13_DIR to a scratch directory}"
C=$M/conjure; F=$M/ctl-fathom
cd $C && git checkout -q --detach $3 || exit 1
[ -z "$(git status --porcelain)" ] || { echo dirty; exit 1; }
cd $F && CTL_HUB=conjure::internal/cli/app.app.newRoot CTL_ROOT=$C CTL_STORE=$C/workspace/tmp/$4.sqlite CTL_BASE=$2 CTL_SERVICE=conjure CTL_OUT=$M/control/$1/classes.json go test -count=1 -v -run TestCtlDumpClasses ./internal/conserve/ 2>&1 | tail -3
python3 $M/control/join_cover.py $M/control/$1 > $M/control/$1/join.out
