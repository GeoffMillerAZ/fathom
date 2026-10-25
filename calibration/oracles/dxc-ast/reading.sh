#!/usr/bin/env bash
# reading.sh — the HLSL reading's driver (LB-Q-hlsl): runs dxc-ast and
# clang-includes over each calibration subject, then indexbench lang-oracle
# three times over every measured file (no -only: tenancy, throughput and
# L3 cover all of them, while dxc-ast's `scope` limits L1 and L2 to the files
# DXC reads), keeps the median-throughput run as the reading and appends
# notes generated from the runs' own tenancy output, never typed by hand.
#
#   calibration/oracles/dxc-ast/reading.sh <reading.json>
#
# Run from the repository root after `task calibrate-clone -- hlsl`, with DXC
# unpacked under workspace/tools/dxc (README.md). Scratch output goes to
# workspace/tmp/oracle/. Bench-only.
set -euo pipefail
out=${1:?usage: reading.sh <reading.json>}
root=$(pwd)
clones=${HLSL_CLONES:-workspace/calibration/hlsl}
tools=workspace/tools
tmp=workspace/tmp/oracle
dxc=$root/$tools/dxc/bin/dxc
subjects=(directx-graphics-samples kajiya directxtk12)
mkdir -p "$tmp" "$tools"
(cd calibration/oracles/dxc-ast && go build -o "$root/$tools/dxc-ast" .)
(cd calibration/oracles/clang-includes && go build -o "$root/$tools/clang-includes" .)

for s in "${subjects[@]}"; do
  dir=$clones/$s
  (cd "$dir" && find . -type f \( -name '*.hlsl' -o -name '*.hlsli' -o -name '*.fx' -o -name '*.fxh' \) -not -path './.git/*' |
    sed 's|^\./||' | LC_ALL=C sort) > "$tmp/hlsl-$s.files"
  "$tools/dxc-ast" -subject "$dir" -dxc "$dxc" -files "$tmp/hlsl-$s.files" -I . -scope "$tmp/hlsl-$s.scope" \
    > "$tmp/hlsl-$s-dxc.json" 2> "$tmp/hlsl-$s-dxc.err"
  "$tools/clang-includes" -subject "$dir" -files "$tmp/hlsl-$s.files" -lang c++ -I . \
    > "$tmp/hlsl-$s-imports.json" 2> "$tmp/hlsl-$s-includes.err"
  echo "$s: $(tail -1 "$tmp/hlsl-$s-dxc.err") | $(tail -1 "$tmp/hlsl-$s-includes.err")"
done

measure() { # $1 reading, $2 "scoped" to pass dxc-ast's scope as -only
  rm -f "$1"
  for s in "${subjects[@]}"; do
    only=()
    if [ "${2:-}" = scoped ]; then only=(-only "$(paste -sd, "$tmp/hlsl-$s.scope")"); fi
    go run ./cmd/indexbench lang-oracle -dialect hlsl -subject "$clones/$s" \
      -tool-oracle "$tmp/hlsl-$s-dxc.json" -tool-oracle "$tmp/hlsl-$s-imports.json" \
      -unmatched "$tmp/hlsl-$s-unmatched.json" "${only[@]}" -out "$1" >&2
  done
}
for i in 1 2 3; do measure "$tmp/hlsl-run$i.json"; done
measure "$tmp/hlsl-scoped.json" scoped

median=$(for i in 1 2 3; do echo "$(jq .tenancy.throughput_mb_s "$tmp/hlsl-run$i.json") $i"; done | sort -n | sed -n 2p | cut -d' ' -f2)
cp "$tmp/hlsl-run$median.json" "$out"
runs=$(for i in 1 2 3; do jq .tenancy.throughput_mb_s "$tmp/hlsl-run$i.json"; done | paste -sd, - | sed 's/,/, /g')

jq --arg runs "$runs" --slurpfile scoped "$tmp/hlsl-scoped.json" '
  def pct(a; b): ((a * 1000 / b) | round / 10 | tostring) + "%";
  (.subjects | map(.tenancy.files_with_error) | add) as $err
  | (.subjects | map(.files) | add) as $files
  | ($scoped[0].subjects | map(.tenancy.files_with_error) | add) as $serr
  | ($scoped[0].subjects | map(.files) | add) as $sfiles
  | .notes += [
      ("tenancy over every measured file (no -only; written by calibration/oracles/dxc-ast/reading.sh from this run): files_with_error "
        + (.subjects | map("\(.name) \(.tenancy.files_with_error) of \(.files)") | join(", "))
        + ", pooled \($err) of \($files) (" + pct($err; $files) + "); throughput \(.tenancy.throughput_mb_s) MB/s CPU (\(.tenancy.wall_throughput_mb_s) wall)"),
      ("throughput (design 1.9 T1, T2; reading.sh): three runs over every measured file read \($runs) MB/s CPU; the reading is the median run, \(.tenancy.throughput_mb_s) MB/s"),
      ("parse errors by dxc-ast scope (reading.sh, a run with -only the scope): \($serr) of the \($sfiles) files DXC reads hold a parse error and \($err - $serr) of the \($files - $sfiles) it excludes; "
        + (if ($err - $serr) * 2 > $err then "the files DXC excludes carry most of the remaining parse errors, so L1 and L2 are measured on the easier part of the corpus"
           else "the files DXC excludes carry \($err - $serr) of the \($err) remaining parse errors" end))
    ]
  | .notes |= sort' "$out" > "$out.tmp" && mv "$out.tmp" "$out"
jq -c '{level_met, grade, tenancy: {files_with_error: .tenancy.files_with_error, throughput_mb_s: .tenancy.throughput_mb_s}}' "$out"
