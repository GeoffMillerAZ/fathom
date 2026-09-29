#!/usr/bin/env bash
# PR-M0 part A: CV-M1's method over every guards cell diff on disk.
#
# For each attempt-N/agent.diff under $PRM0_RAW/guards-fathom-r{1,2}: reset a
# scratch clone to f67f06a, apply the diff (minus the instrument's own
# .claude/scenariobench-mcp.json), commit, scratch-ingest into a fresh store,
# then run `fathom conserve --since f67f06a --json`. No model is called anywhere.
#
# Each of $PRM0_JOBS workers keeps one clone path for all its cells: the ingest's
# typed pass compiles through the Go build cache, whose keys include the source
# directory, so a fixed path turns every unchanged package into a cache hit.
#
# Inputs (environment):
#   PRM0_DIR     work directory; must hold base/ (a clone at f67f06a)
#   PRM0_RAW     the raw cell root (holds guards-fathom-r1/ and guards-fathom-r2/)
#   PRM0_FATHOM  the fathom binary (the reading used one built at fa240083)
#   PRM0_JOBS    workers (default 3)
#   PRM0_PRE_CELL  optional shell command run before every try of a cell (a failed try is
#                retried twice: a clean ingest and a non-empty report are the success test)
# Output: $PRM0_DIR/out/<run>/<cell>/<attempt>/{applied.diff,conserve.json,conserve.err,changed.txt,status}
set -uo pipefail
: "${PRM0_DIR:?}" "${PRM0_RAW:?}" "${PRM0_FATHOM:?}"
JOBS="${PRM0_JOBS:-3}"
BASE_SHA=f67f06adfea927de6296c1ccd27520c6a4d7090b
HERE="$(cd "$(dirname "$0")" && pwd)"

pre_cell() { # optional per-try hook, e.g. clearing a host's stale file-access verdict before a retry
	[ -n "${PRM0_PRE_CELL:-}" ] && { bash -c "$PRM0_PRE_CELL" >/dev/null 2>&1 || true; }
	return 0
}

attempt() { # slot diff out: one try; 0 on a clean ingest and a conserve report
	local wt="$PRM0_DIR/wt/slot-$1" diff="$2" out="$3" key="$4"
	local db="$PRM0_DIR/db/slot-$1.sqlite"
	[ -d "$wt/.git" ] || git clone -q "$PRM0_DIR/base" "$wt" || return 1
	git -C "$wt" checkout -q -f -B replay "$BASE_SHA" 2>/dev/null || return 1
	git -C "$wt" clean -q -fdx || return 1
	rm -f "$db" "$db-wal" "$db-shm"
	python3 "$HERE/strip_instrument.py" <"$diff" >"$out/applied.diff" || return 1
	if [ -s "$out/applied.diff" ] && ! git -C "$wt" apply --whitespace=nowarn "$out/applied.diff" 2>"$out/apply.err"; then
		echo "apply-failed" >"$out/status"; return 0
	fi
	git -C "$wt" add -A || return 1
	git -C "$wt" -c user.name=replay -c user.email=replay@example.invalid \
		commit -q --no-verify --allow-empty -m "replay $key" || return 1
	(cd "$wt" && "$PRM0_FATHOM" ingest --store "$db" --service prm0 --path . >"$out/ingest.log" 2>&1) || return 1
	grep -q "ingest degraded\|operation not permitted" "$out/ingest.log" && return 1
	(cd "$wt" && "$PRM0_FATHOM" conserve --since "$BASE_SHA" --service prm0 --store "$db" --json \
		>"$out/conserve.json" 2>"$out/conserve.err") || true
	[ -s "$out/conserve.json" ] || return 1
	git -C "$wt" diff --name-only "$BASE_SHA" HEAD >"$out/changed.txt" || return 1
	echo "ok" >"$out/status"
}

one() { # slot diff
	local diff="$2"
	local rel="${diff#"$PRM0_RAW"/}"
	local key="${rel%/agent.diff}"
	local out="$PRM0_DIR/out/$key"
	if [ -s "$out/conserve.json" ] && [ "$(cat "$out/status" 2>/dev/null)" = ok ]; then echo "skip $key"; return 0; fi
	local n
	for n in 1 2 3; do
		pre_cell
		mkdir -p "$out" "$PRM0_DIR/db" 2>/dev/null
		rm -f "$out/conserve.json" "$out/status"
		if attempt "$1" "$diff" "$out" "$key"; then echo "done $key (try $n)"; return 0; fi
		echo "retry $key (try $n failed)"
	done
	echo "gave-up" >"$out/status" 2>/dev/null
	echo "gave-up $key"
}

mapfile -t DIFFS < <(find "$PRM0_RAW"/guards-fathom-r1 "$PRM0_RAW"/guards-fathom-r2 -name agent.diff | sort)
for ((s = 0; s < JOBS; s++)); do
	(for ((i = s; i < ${#DIFFS[@]}; i += JOBS)); do one "$s" "${DIFFS[$i]}"; done) &
done
wait
