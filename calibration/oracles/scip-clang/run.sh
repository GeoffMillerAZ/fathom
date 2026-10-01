#!/usr/bin/env bash
# run.sh — scip-clang 0.4.0 over a C or C++ calibration subject (tier A for
# def_mapping/typed_recall/calls; language-breadth §1.3, ADR-048 amendment D2).
#
#   run.sh <scip-clang> <subject-dir> <cmake-build-dir> <out.scip> [jobs]
#
# <cmake-build-dir> holds the compile_commands.json that
#   cmake -S <subject-dir> -B <cmake-build-dir> -DCMAKE_EXPORT_COMPILE_COMMANDS=ON
# wrote, outside the subject tree (the checkout is never written to). scip-clang
# resolves paths against its working directory, so it runs from <subject-dir>.
# The release is pinned: scip-clang 0.4.0 (sourcegraph/scip-clang v0.4.0,
# scip-clang-arm64-darwin, sha256 in README.md).
set -euo pipefail
bin=$1 subj=$2 build=$3 out=$4 jobs=${5:-4}
ver=$("$bin" --version | head -1)
[ "$ver" = "scip-clang 0.4.0" ] || { echo "run.sh: want scip-clang 0.4.0, have '$ver'" >&2; exit 2; }
[ -f "$build/compile_commands.json" ] || { echo "run.sh: no $build/compile_commands.json" >&2; exit 2; }
cd "$subj"
nice -n 10 perl -e 'alarm 1800; exec @ARGV' "$bin" --compdb-path "$build/compile_commands.json" \
  --index-output-path "$out" --jobs "$jobs" --no-progress-report --log-level warning
