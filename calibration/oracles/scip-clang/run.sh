#!/usr/bin/env bash
# run.sh — scip-clang 0.4.0 over a C or C++ calibration subject (tier A for
# def_mapping/typed_recall/calls; language-breadth §1.3, ADR-048 amendment D2).
#
#   run.sh <scip-clang> <subject-dir> <cmake-build-dir> <out.scip> <tier> [jobs]
#
# <tier> is the index's tier, required (round 4 C4-R2): A when <cmake-build-dir>'s
# database is the subject's own build's, untranslated; B for a translation (the CUDA
# driver's clang database derived from nvcc's). Pass the same value to
# `indexbench lang-oracle -scip-tier`, which refuses a scip-clang index without one.
# <cmake-build-dir> holds the compile_commands.json that
#   cmake -S <subject-dir> -B <cmake-build-dir> -DCMAKE_EXPORT_COMPILE_COMMANDS=ON
# wrote. Keep it at <subject-dir>/build, in a scratch copy of the subject (never in
# the shared checkout): that is where fathom's include-path resolver reads a
# database (internal/fusion/compdb.go: the repo root, build/, builddir/, _build/,
# cmake-build-*/, out/build/*/), so the harness's file edges and imports see the
# same flags the index was built from. scip-clang resolves paths against its
# working directory, so it runs from <subject-dir>.
# The release is pinned: scip-clang 0.4.0 (sourcegraph/scip-clang v0.4.0,
# scip-clang-arm64-darwin, sha256 in README.md).
set -euo pipefail
bin=$1 subj=$2 build=$3 out=$4 tier=${5:-} jobs=${6:-4}
case "$tier" in A|B|C|D) ;; *) echo "run.sh: <tier> must be A, B, C or D (the index's tier, lang-oracle -scip-tier), have '$tier'" >&2; exit 2 ;; esac
ver=$("$bin" --version | head -1)
[ "$ver" = "scip-clang 0.4.0" ] || { echo "run.sh: want scip-clang 0.4.0, have '$ver'" >&2; exit 2; }
[ -f "$build/compile_commands.json" ] || { echo "run.sh: no $build/compile_commands.json" >&2; exit 2; }
[ "$(cd "$build" && pwd -P)" = "$(cd "$subj" && pwd -P)/build" ] || echo "run.sh: note: the database is not at $subj/build, where fathom's resolver reads it; the index and the resolver will not share flags" >&2
cd "$subj"
nice -n 10 perl -e 'alarm 1800; exec @ARGV' "$bin" --compdb-path "$build/compile_commands.json" \
  --index-output-path "$out" --jobs "$jobs" --no-progress-report --log-level warning
echo "run.sh: $out is a tier-$tier index: read it with indexbench lang-oracle -scip $out -scip-tier $tier" >&2
