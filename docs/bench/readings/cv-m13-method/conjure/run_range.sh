#!/bin/bash
# usage: run_range.sh <tag> <base> <head>
set -u
export GOCACHE=/Volumes/m2ext/tmp/cv3-build GOMODCACHE=/Volumes/m2ext/tmp/cv3-mod GOTMPDIR=/Volumes/m2ext/tmp/cv3-tmp TMPDIR=/Volumes/m2ext/tmp/cv3-tmp GOWORK=off CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0
# conjure commits vendor/ and (at older heads) replace => ../app-kit: -mod=mod cannot resolve it; vendor mode builds the committed tree.
export GOFLAGS=-mod=vendor
tag=$1; base=$2; head=$3
M="${CVM13_DIR:?set CVM13_DIR to a scratch directory}"
B=$M/bin; C=$M/conjure; CT=$M/control/$tag; F=$M/ctl-fathom
mkdir -p $CT
cd $C || exit 1
git checkout -q --detach $head || exit 1
git status --porcelain > $CT/git-status-before.txt
commit=$(git rev-parse HEAD); echo "$base..$commit" > $CT/range.txt
out=$C/workspace/artifacts/test-results-$tag; rm -rf $out; mkdir -p $out
store=$C/workspace/tmp/c-$tag.sqlite; rm -f $store
started=$(date -u +%Y-%m-%dT%H:%M:%SZ); echo $started > $CT/started.txt
dirtyarg=""; [ -z "$(git status --porcelain)" ] || dirtyarg="--dirty"
go test -json ./... > $out/go-test.jsonl; echo "go test exit $?" > $CT/go-test-exit.txt
$B/surveyor ingest --format go-test-json --component conjure --suite unit --class unit --started-at "$started" --commit "$commit" $dirtyarg < $out/go-test.jsonl > $out/envelope.json
echo "surveyor exit $?" >> $CT/go-test-exit.txt
cp $out/go-test.jsonl $out/envelope.json $CT/
$B/fathom ingest --json --service conjure --path $C --store $store > $CT/ingest.json 2> $CT/ingest.stderr; echo "ingest exit $?" >> $CT/go-test-exit.txt
$B/fathom tests ingest --service conjure --manifest $C --results $out --store $store > $CT/tests-ingest.out 2>&1; echo "tests ingest exit $?" >> $CT/go-test-exit.txt
$B/fathom conserve --since $base --json --service conjure --store $store --residue-max 1000 > $CT/conserve.json 2> $CT/conserve.stderr; echo "conserve exit $?" >> $CT/go-test-exit.txt
go test -coverpkg=./... -coverprofile=$CT/cover.out ./... > $CT/cover-run.txt 2>&1; echo "cover test exit $?" >> $CT/go-test-exit.txt
go tool cover -func=$CT/cover.out > $CT/cover-func.txt
git status --porcelain > $CT/git-status-after.txt
{ echo "## typed_edge_status"; sqlite3 -header $store "select * from typed_edge_status"; echo "## typed_edges by kind"; sqlite3 -header $store "select kind,count(*) from typed_edges group by kind"; echo "## test_runs"; sqlite3 -header $store "select service_id,run_digest,verdict,totals,commit_sha,dirty,race from test_runs"; echo "## test_cases join"; sqlite3 -header $store "select count(*) total, sum(test_symbol_key!='') joined from test_cases"; echo "## unjoined top-level cases"; sqlite3 $store "select case_id from test_cases where test_symbol_key='' and name not like '%/%'"; } > $CT/store-summary.txt
cd $F && GOFLAGS=-mod=mod CTL_ROOT=$C CTL_STORE=$store CTL_BASE=$base CTL_SERVICE=conjure CTL_OUT=$CT/classes.json go test -count=1 -run TestCtlDumpClasses ./internal/conserve/ > $CT/dump.out 2>&1; echo "dump exit $?" >> $CT/go-test-exit.txt
python3 $M/control/join_cover.py $CT > $CT/join.out
cat $CT/go-test-exit.txt
