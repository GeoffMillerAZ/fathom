#!/usr/bin/env bash
# run.sh — scip-dotnet over a C# calibration subject (tier A for
# def_mapping/typed_recall; language-breadth §1.3, ADR-048 amendment D2).
#
#   run.sh <tools-dir> <subject-dir> <sln-dir> <out.scip> [global.json-sdk-override]
#
# scip-dotnet 0.2.14 reads only .sln/.csproj (both subjects ship .slnx), so a
# throwaway scipall.sln listing every tracked .csproj is created in <sln-dir>,
# restored, indexed, and removed with `git clean -fdx` (which also drops the
# restore's obj/ trees) — the checkout is left as cloned.
set -euo pipefail
T=$1 subj=$2 slndir=$3 out=$4 sdk=${5:-}
export DOTNET_ROOT=$T/dotnet DOTNET_CLI_TELEMETRY_OPTOUT=1 DOTNET_NOLOGO=1 DOTNET_ROLL_FORWARD=Major \
  NUGET_PACKAGES=$T/nuget DOTNET_CLI_HOME=$T/dotnet-home PATH=$T/dotnet:$PATH
cd "$subj/$slndir"
[ -n "$sdk" ] && echo "{\"sdk\":{\"version\":\"$sdk\"}}" > global.json
dotnet new sln -n scipall --format sln >/dev/null
# shellcheck disable=SC2046
dotnet sln scipall.sln add $(git ls-files '*.csproj' | grep -vi cake) >/dev/null
nice -n 10 perl -e 'alarm 1200; exec @ARGV' dotnet restore scipall.sln || true
rc=0
nice -n 10 perl -e 'alarm 1200; exec @ARGV' "$T/scip-dotnet/scip-dotnet" index scipall.sln --working-directory "$subj" --output "$out" || rc=$?
cd "$subj" && git checkout -q -- . && git clean -fdxq
exit $rc
