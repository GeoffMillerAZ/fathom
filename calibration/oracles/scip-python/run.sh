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
#
# A src-layout subject (flask: src/flask/__init__.py) has its source root off that import
# path too, so `import flask` in its tests resolved to nothing and every call on a value
# built from it went untyped (O450: 598 of flask's 1,355 fathom-only call pairs). The
# source roots are put on PYTHONPATH for the index run only: src/ when it holds a package,
# and each root pyproject.toml names (setuptools package-dir, poetry packages `from`,
# hatch/pdm/flit-style `src`); no subject is named here (ADR-011, O485).
set -euo pipefail
bin=$1 subj=$2 out=$3 venv=$4; shift 4
[ "${1:-}" = "--" ] && shift
here=$(cd "$(dirname "$0")" && pwd)
name=$(basename "$subj")
uv venv -q --seed --python 3.13 "$venv"
VIRTUAL_ENV=$venv uv pip install -q "$@"
VIRTUAL_ENV=$venv uv pip uninstall -q "$name" 2>/dev/null || true
cd "$subj"
roots=$("$venv/bin/python" "$here/srcroots.py" "$subj")
[ -n "$roots" ] && echo "run.sh: source roots on the import path: $roots" >&2
env PATH="$venv/bin:$PATH" VIRTUAL_ENV="$venv" PYTHONPATH="${roots}${roots:+${PYTHONPATH:+:}}${PYTHONPATH:-}" "$bin" index --cwd "$subj" --project-name "$name" --output "$out"
git status --short | grep . && { echo "run.sh: the subject checkout changed" >&2; exit 1; } || true
