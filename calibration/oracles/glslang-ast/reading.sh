#!/usr/bin/env bash
# Reproduce a full-corpus GLSL reading. Run from the repository root with a
# CPU lease; on the origin host use the external nice-10/load<=9 gate.
# Usage: calibration/oracles/glslang-ast/reading.sh <reading.json>
set -euo pipefail
out=${1:?usage: reading.sh <reading.json>}
[[ $# == 1 ]] || { echo 'usage: reading.sh <reading.json>' >&2; exit 2; }
[[ -f go.mod && -d calibration/oracles/glslang-ast ]] || { echo 'run from repository root' >&2; exit 2; }
catalog_mode=${GLSL_CATALOG_MODE:-full-registry}
case "$catalog_mode" in baseline-core|core-plus-types|full-registry) ;; *) echo 'invalid GLSL_CATALOG_MODE' >&2; exit 2;; esac
if [[ ${GLSL_RUNS_ROOT+x} ]]; then
  runs=$GLSL_RUNS_ROOT
  [[ -n "$runs" ]] || { echo 'invalid GLSL_RUNS_ROOT: must be under workspace/' >&2; exit 2; }
else
  runs=workspace/oracles/glslang-ast/catalog-modes/$catalog_mode
fi
case "$runs" in
  workspace/*) ;;
  *) echo 'invalid GLSL_RUNS_ROOT: must be under workspace/' >&2; exit 2;;
esac
case "/$runs/" in
  */../*) echo 'invalid GLSL_RUNS_ROOT: traversal is not allowed' >&2; exit 2;;
esac
if ! python3 -I -B - "$runs" <<'PY'
import pathlib
import sys

checkout = pathlib.Path.cwd().resolve(strict=True)
workspace = (checkout / "workspace").resolve(strict=True)
candidate = (checkout / sys.argv[1]).resolve(strict=False)
try:
    relative = candidate.relative_to(workspace)
except ValueError:
    raise SystemExit(1)
if not relative.parts:
    raise SystemExit(1)
PY
then
  echo 'invalid GLSL_RUNS_ROOT: resolved path must be below workspace/' >&2; exit 2
fi
if [[ "$catalog_mode" != full-registry && "$out" != workspace/* ]]; then
  echo 'non-final catalog mode readings must stay under workspace/' >&2; exit 2
fi

oracle_dir=calibration/oracles/glslang-ast
clones=workspace/calibration/glsl
compiler=workspace/tools/glslang-16.6.0/bin/glslang
ctags=workspace/tools/ctags-6.2.1/ctags
indexbench=workspace/tools/indexbench-glsl-reading
subjects=(vulkan spirv-cross vulkan-samples)
extension_index=$oracle_dir/registered-extensions.json
extension_review=$oracle_dir/registered-extensions-review.json
source_commit=$(python3 -I -B "$oracle_dir/reading_support.py" source --override "${FATHOM_COMMIT:-}")
mkdir -p workspace/tmp/oracle workspace/tools
export TMPDIR="${TMPDIR:-$PWD/workspace/tmp}"
scratch=$(mktemp -d "workspace/tmp/oracle/glsl-$catalog_mode.XXXXXXXX")
python3 -I -B "$oracle_dir/subject-recipes/generate.py"
for s in "${subjects[@]}"; do
  mkdir -p "$runs/$s"
  python3 -I -B "$oracle_dir/glslang_ast.py" \
    --subject-kind "$s" --compiler "$compiler" --builtin-index "$oracle_dir/glsl460-functions.json" \
    --catalog-mode "$catalog_mode" --extension-index "$extension_index" --extension-review "$extension_review" \
    "$clones/$s" "$runs/$s/oracle.json" "$runs/$s/receipt.json"
done

verify=(python3 -I -B "$oracle_dir/reading_support.py" verify --clones "$clones" --runs "$runs" \
  --catalog-mode "$catalog_mode" --extension-index "$extension_index" --extension-review "$extension_review")
"${verify[@]}" > "$scratch/oracle-coverage.json"

for s in "${subjects[@]}"; do
  python3 -I -B calibration/oracles/ctags-c-glsl/ctags_c_glsl.py --ctags "$ctags" \
    "$clones/$s" "$scratch/$s-defs.json"
done

go build -o "$indexbench" ./cmd/indexbench
python3 -I -B "$oracle_dir/reading_support.py" source --expected "$source_commit" >/dev/null
bench_note=()
if [[ -n "${GLSL_BENCH_CONTEXT:-}" ]]; then bench_note=(-note "$GLSL_BENCH_CONTEXT"); fi
for i in 1 2 3; do
  reading=$scratch/run$i.json
  for s in "${subjects[@]}"; do
    case "$s" in
      vulkan) subject_name='SaschaWillems/Vulkan' ;;
      spirv-cross) subject_name='KhronosGroup/SPIRV-Cross' ;;
      vulkan-samples) subject_name='KhronosGroup/Vulkan-Samples' ;;
    esac
    "$indexbench" lang-oracle -dialect glsl -subject "$clones/$s" -subject-name "$subject_name" \
      -commit "$source_commit" -ctags "$ctags" \
      -tool-oracle "$scratch/$s-defs.json" -tool-oracle "$runs/$s/oracle.json" \
      -unmatched "$scratch/$s-run$i-unmatched.json" ${bench_note[@]+"${bench_note[@]}"} -out "$reading"
  done
done

python3 -I -B "$oracle_dir/reading_support.py" source --expected "$source_commit" >/dev/null

annotate=(python3 -I -B "$oracle_dir/reading_support.py" annotate --clones "$clones" --runs "$runs" \
  --scratch "$scratch" --out "$out" --source-commit "$source_commit" \
  --catalog-mode "$catalog_mode" --extension-index "$extension_index" --extension-review "$extension_review")
"${annotate[@]}"
echo "scratch=$scratch source_commit=$source_commit reading=$out evidence=$scratch/evidence.json" >&2
