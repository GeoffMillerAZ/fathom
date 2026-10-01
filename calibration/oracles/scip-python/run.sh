#!/usr/bin/env bash
# run.sh — scip-python over a Python calibration subject, in a virtual environment
# holding the subject's dependencies (tier A; language-breadth §1.3). Bench-only:
# node, Python and the environment are instruments, never product dependencies.
#
#   run.sh <scip-python> <subject-dir> <out.scip> <venv-dir> -- <uv pip install args>...
#
# scip-python resolves a name defined outside the subject only through the Python
# environment it finds on PATH (pip list): without the subject's dependencies
# installed it names no symbol for a call on a vendor-typed receiver (starlette's
# TestClient.get, pydantic's BaseModel methods), so those calls are absent from
# its call truth (O263). The subject package itself is never installed into the
# environment: its files are the project scip-python indexes.
set -euo pipefail
bin=$1 subj=$2 out=$3 venv=$4; shift 4
[ "${1:-}" = "--" ] && shift
name=$(basename "$subj")
uv venv -q --seed --python 3.13 "$venv"
VIRTUAL_ENV=$venv uv pip install -q "$@"
VIRTUAL_ENV=$venv uv pip uninstall -q "$name" 2>/dev/null || true
cd "$subj"
env PATH="$venv/bin:$PATH" VIRTUAL_ENV="$venv" "$bin" index --cwd "$subj" --project-name "$name" --output "$out"
git status --short | grep . && { echo "run.sh: the subject checkout changed" >&2; exit 1; } || true
