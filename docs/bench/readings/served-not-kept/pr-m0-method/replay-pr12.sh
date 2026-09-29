#!/usr/bin/env bash
# PR-12 addendum to PR-M0 part A: the pre-push verdict of every i28 cell.
#
# For each i28 attempt-1/agent.diff under $PRM0_RAW/guards-fathom-r{1,2}: reset
# a scratch clone to f67f06a, apply the diff (minus the instrument's own
# .claude/scenariobench-mcp.json, strip_instrument.py), commit, then run the
# pre-push step's own command, `fathom conserve --since f67f06a
# --strict-plane integrity`, and keep its exit code and pin lines. No model is
# called anywhere.
#
# The pin check is storeless (PinLosses reads blobs through git only), so the
# store is a fresh empty file per cell rather than replay.sh's ingest: the
# ingest feeds the other planes' advisory report, never the exit code of
# --strict-plane integrity. Set PRM0_INGEST=1 to ingest each cell first anyway.
#
# Inputs (environment):
#   PRM0_DIR       work directory; must hold base/ (a clone at f67f06a)
#   PRM0_RAW       the raw cell root (holds guards-fathom-r1/ and guards-fathom-r2/)
#   PRM0_FATHOM    the fathom binary under test
#   PRM0_PRE_CELL  optional shell command run before every cell
#   PRM0_INGEST    optional; 1 ingests each cell into its store before conserve
# Output: $PRM0_DIR/pr12/<run>/<cell>/{applied.diff,pins.txt,exit} and a TSV
# on stdout: run, cell, exit, then the summary line.
set -uo pipefail
: "${PRM0_DIR:?}" "${PRM0_RAW:?}" "${PRM0_FATHOM:?}"
BASE_SHA=f67f06adfea927de6296c1ccd27520c6a4d7090b
HERE="$(cd "$(dirname "$0")" && pwd)"
WT="$PRM0_DIR/wt/pr12"
[ -d "$WT/.git" ] || git clone -q "$PRM0_DIR/base" "$WT" || exit 2

for diff in $(find "$PRM0_RAW"/guards-fathom-r1 "$PRM0_RAW"/guards-fathom-r2 -path '*i28*' -name agent.diff | sort); do
	[ -n "${PRM0_PRE_CELL:-}" ] && { bash -c "$PRM0_PRE_CELL" >/dev/null 2>&1 || true; }
	rel="${diff#"$PRM0_RAW"/}"
	key="${rel%/attempt-*/agent.diff}"
	out="$PRM0_DIR/pr12/$key"
	mkdir -p "$out"
	git -C "$WT" checkout -q -f -B replay "$BASE_SHA" || exit 2
	git -C "$WT" clean -q -fdx || exit 2
	python3 "$HERE/strip_instrument.py" <"$diff" >"$out/applied.diff" || exit 2
	if [ -s "$out/applied.diff" ]; then
		git -C "$WT" apply --whitespace=nowarn "$out/applied.diff" || { echo "apply-failed $key"; continue; }
	fi
	git -C "$WT" add -A
	git -C "$WT" -c user.name=replay -c user.email=replay@example.invalid \
		commit -q --no-verify --allow-empty -m "replay $key" || exit 2
	db="$out/store.sqlite"
	rm -f "$db" "$db-wal" "$db-shm"
	if [ "${PRM0_INGEST:-}" = 1 ]; then
		(cd "$WT" && "$PRM0_FATHOM" ingest --store "$db" --service prm0 --path . >"$out/ingest.log" 2>&1) || exit 2
	fi
	(cd "$WT" && "$PRM0_FATHOM" conserve --since "$BASE_SHA" --service prm0 --store "$db" \
		--strict-plane integrity >"$out/conserve.txt" 2>"$out/conserve.err")
	rc=$?
	echo "$rc" >"$out/exit"
	grep -E '^(pin |integrity pins:)' "$out/conserve.txt" >"$out/pins.txt"
	printf '%s\t%s\t%s\t%s\n' "${key%%/*}" "${key#*/}" "$rc" "$(grep '^integrity pins:' "$out/pins.txt")"
done
