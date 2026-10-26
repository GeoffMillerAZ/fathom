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

# NOTES_ONLY=1 rewrites the reading from the last runs under workspace/tmp/oracle
# (the median run and its generated notes) without measuring again.
[ "${NOTES_ONLY:-}" = 1 ] || for s in "${subjects[@]}"; do
  dir=$clones/$s
  (cd "$dir" && find . -type f \( -name '*.hlsl' -o -name '*.hlsli' -o -name '*.fx' -o -name '*.fxh' \) -not -path './.git/*' |
    sed 's|^\./||' | LC_ALL=C sort) > "$tmp/hlsl-$s.files"
  "$tools/dxc-ast" -subject "$dir" -dxc "$dxc" -files "$tmp/hlsl-$s.files" -I . -scope "$tmp/hlsl-$s.scope" \
    -refs "$tmp/hlsl-$s-refs.json" > "$tmp/hlsl-$s-dxc.json" 2> "$tmp/hlsl-$s-dxc.err"
  "$tools/clang-includes" -subject "$dir" -files "$tmp/hlsl-$s.files" -lang c++ -I . \
    > "$tmp/hlsl-$s-includes.json" 2> "$tmp/hlsl-$s-includes.err"
  # L3's truth (H5-R7): file_edge_* against the include statements plus dxc-ast's
  # same_package reference pairs, import_resolution against the statements alone
  jq -s '.[0] + {imports: (.[0].imports + .[1].imports), disclose: ([
      "file-edge truth: clang-includes'"'"' include statements plus \(.[1].imports | length) dxc-ast reference pairs (a DeclRefExpr, MemberExpr or named type in a unit DXC reads, resolved through the dump to a declaration another measured file holds; written as same_package, so they grade file_edge_* and never import_resolution)"] + .[1].disclose)}' \
    "$tmp/hlsl-$s-includes.json" "$tmp/hlsl-$s-refs.json" > "$tmp/hlsl-$s-imports.json"
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
if [ "${NOTES_ONLY:-}" != 1 ]; then
  for i in 1 2 3; do measure "$tmp/hlsl-run$i.json"; done
  measure "$tmp/hlsl-scoped.json" scoped
fi

median=$(for i in 1 2 3; do echo "$(jq .tenancy.throughput_mb_s "$tmp/hlsl-run$i.json") $i"; done | sort -n | sed -n 2p | cut -d' ' -f2)
cp "$tmp/hlsl-run$median.json" "$out"
runs=$(for i in 1 2 3; do jq .tenancy.throughput_mb_s "$tmp/hlsl-run$i.json"; done | paste -sd, - | sed 's/,/, /g')

jq --arg runs "$runs" --slurpfile scoped "$tmp/hlsl-scoped.json" '
  def pct(a; b): ((a * 1000 / b) | round / 10 | tostring) + "%";
  def ratio(a; b): if b == 0 then "\(a)/0 = n/a" else "\(a)/\(b) = \((a * 10000 / b) | round / 10000)" end;
  (.subjects | map(.tenancy.files_with_error) | add) as $err
  | (.subjects | map(.files) | add) as $files
  | ($scoped[0].subjects | map(.tenancy.files_with_error) | add) as $serr
  | ($scoped[0].subjects | map(.files) | add) as $sfiles
  | .notes += [
      ("L3 per subject (H6-R4; written by reading.sh from the counts of this run; the pooled metrics are the convention): "
        + (.subjects | map("\(.name) file_edge_precision " + ratio(.counts.file_edges_agree // 0; .counts.file_edges_fathom // 0)
          + ", file_edge_recall " + ratio(.counts.file_edges_agree // 0; .counts.file_edges_truth // 0)
          + ", import_resolution " + ratio(.counts.imports_resolved_agree // 0; .counts.imports_in_repo // 0)) | join("; "))),
      ("tenancy over every measured file (no -only; written by calibration/oracles/dxc-ast/reading.sh from this run): files_with_error "
        + (.subjects | map("\(.name) \(.tenancy.files_with_error) of \(.files)") | join(", "))
        + ", pooled \($err) of \($files) (" + pct($err; $files) + "); throughput \(.tenancy.throughput_mb_s) MB/s CPU (\(.tenancy.wall_throughput_mb_s) wall)"),
      ("throughput (design 1.9 T1, T2; reading.sh): three runs over every measured file read \($runs) MB/s CPU; the reading is the median run, \(.tenancy.throughput_mb_s) MB/s"),
      ("parse errors by dxc-ast scope (reading.sh, a run with -only the scope): \($serr) of the \($sfiles) files DXC reads hold a parse error (" + pct($serr; $sfiles) + ") and \($err - $serr) of the \($files - $sfiles) it excludes (" + pct($err - $serr; $files - $sfiles) + "); "
        + (if ($err - $serr) * $sfiles > $serr * ($files - $sfiles) then "the files DXC excludes are the more error-prone part of the corpus" + (if ($err - $serr) * 2 > $err then " and carry most of the remaining parse errors" else "" end) + ", so L1 and L2 are measured on the easier part of the corpus"
           else "the files DXC excludes are no more error-prone than those it reads" end))
    ]
  | .notes |= sort' "$out" > "$out.tmp" && mv "$out.tmp" "$out"
jq -c '{level_met, grade, tenancy: {files_with_error: .tenancy.files_with_error, throughput_mb_s: .tenancy.throughput_mb_s}}' "$out"
