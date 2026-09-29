#!/usr/bin/env bash
# PR-M0 part B: the ADR-038 fragment evaluators over every transcript edit fragment.
#
# Two configs, both with Enforce forced on for every guard and layer rule:
#   base  .fathom/guard.cue at f67f06a, the checkout the cells ran against
#   head  .fathom/guard.cue at fa240083, the production config arm W would use
# Inputs (environment):
#   PRM0_DIR   work directory; must hold base/ (f67f06a) and eval/ (a scratch clone at fa240083)
#   PRM0_RAW   the raw cell root
# Output: $PRM0_DIR/fragments.jsonl, $PRM0_DIR/fires.jsonl
set -euo pipefail
: "${PRM0_DIR:?}" "${PRM0_RAW:?}"
HERE="$(cd "$(dirname "$0")" && pwd)"

python3 "$HERE/extract_fragments.py" >"$PRM0_DIR/fragments.jsonl"
cp "$PRM0_DIR/base/.fathom/guard.cue" "$PRM0_DIR/guard-base.cue"
cp "$PRM0_DIR/eval/.fathom/guard.cue" "$PRM0_DIR/guard-head.cue"
cp "$HERE/zz_prm0_fragments_test.go.txt" "$PRM0_DIR/eval/internal/guardspec/zz_prm0_fragments_test.go"
trap 'rm -f "$PRM0_DIR/eval/internal/guardspec/zz_prm0_fragments_test.go"' EXIT
(cd "$PRM0_DIR/eval" &&
	PRM0_FRAGMENTS="$PRM0_DIR/fragments.jsonl" \
		PRM0_CONFIGS="base=$PRM0_DIR/guard-base.cue,head=$PRM0_DIR/guard-head.cue" \
		PRM0_OUT="$PRM0_DIR/fires.jsonl" \
		go test -count=1 -run '^TestPRM0Fragments$' ./internal/guardspec/)
