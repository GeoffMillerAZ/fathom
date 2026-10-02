#!/usr/bin/env bash
# run.sh — a tier-B call-graph oracle for JavaScript: @persper/js-callgraph (an
# acorn/babel-parser, field-based approximate call graph, EPL-2.0), run unmodified
# and offline over every .js/.cjs/.mjs file of a calibration subject, converted
# to the neutral tool-oracle shape (`indexbench lang-oracle -tool-oracle`; LB-F204).
# Bench-only: node is an instrument, never a product dependency; the tool is never
# copied into the tree.
#
#   run.sh <tools-dir> <subject-dir> <out.json> [<subject-relative-dir-to-skip>...]
#
# <tools-dir>/jscg holds `npm install --prefix <tools-dir>/jscg @persper/js-callgraph@1.3.2`.
# node_modules, dist, build and coverage are always skipped.
set -euo pipefail
tools=$1 subj=$(cd "$2" && pwd) out=$3; shift 3
bin=$tools/jscg/node_modules/.bin/js-callgraph
here=$(cd "$(dirname "$0")" && pwd)
cg=$(mktemp "${TMPDIR:-/tmp}/jscg.XXXXXX")
trap 'rm -f "$cg" "$cg.files"' EXIT
skip=(-name node_modules -o -name dist -o -name build -o -name coverage -o -name .git)
for d in "$@"; do skip+=(-o -path "$subj/$d"); done
find "$subj" \( "${skip[@]}" \) -prune -o -type f \( -name '*.js' -o -name '*.cjs' -o -name '*.mjs' \) -print | LC_ALL=C sort > "$cg.files"
version=$(node -p "require('$tools/jscg/node_modules/@persper/js-callgraph/package.json').version")
# one process: the tool takes the file list as arguments
xargs nice -n 10 node --max-old-space-size=8192 "$bin" --cg --output "$cg.json" < "$cg.files" > /dev/null
node "$here/convert.js" "$cg.json" "$subj" "$version" > "$out"
rm -f "$cg.json"
