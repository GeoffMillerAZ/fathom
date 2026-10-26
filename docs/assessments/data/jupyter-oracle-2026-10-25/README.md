# Notebook oracle replay

Bench-only, no notebook execution, no network after installing tools/subjects.
Do not add the modern diagnostic holdout to a canonical corpus shard or merge its
reading into `docs/bench/readings/lang/jupyter.json`.

Pins: basedpyright 1.40.2, scip-python 0.6.8, jupytext 1.19.6, nbformat 5.11.1,
universal-ctags 6.2.1. Tools Python is 3.13.5. The modern subject uses its own
Python 3.11.14 environment and declared pandas 2.0.3; NumPy 1.26.4 is a selection
within its unpinned NumPy dependency, keeping the pandas ABI compatible. The full
resolved environments are `tools-requirements.txt` and `modern-requirements.txt`.
These are local benchmark dependencies, never product dependencies.

Subjects (MIT code; fetched, not vendored; full immutable hashes in manifest.json):

- ML-For-Beginners `de2d4e122364`, all 51 nontranslation notebooks.
- PythonDataScienceHandbook `d66231454ef7`, all 136 notebooks, roundtrip only. Its declared Python 3.5 scientific environment is not replaced.
- **Separate modern diagnostic holdout:** wesm/pydata-book, third edition `a892791ce5a3`, all 14 notebooks. Its root `pyproject.toml` declares Python >=3.9 and dependencies; those declarations install in the recorded environment and numpy/pandas/scipy/sklearn/matplotlib imports pass.

Keep subjects under `workspace/calibration/`, all SDKs/config/caches under workspace
or the lane cache. Set TMPDIR/GOTMPDIR/GOCACHE/GOWORK=off per the lane brief. Also
set MPLCONFIGDIR, XDG_CACHE_HOME, XDG_CONFIG_HOME, XDG_DATA_HOME,
JUPYTER_CONFIG_DIR and IPYTHONDIR there before importing scientific packages.
Use local uv 0.9.26 (or equivalent), with UV_CACHE_DIR and UV_PYTHON_INSTALL_DIR
inside lane roots. No global installs or Docker.

```sh
# With the pinned Python distributions installed locally:
uv venv --python 3.13.5 workspace/tools/jupyter-python
uv pip install --python workspace/tools/jupyter-python/bin/python \
  -r docs/assessments/data/jupyter-oracle-2026-10-25/tools-requirements.txt
uv venv --python 3.11.14 workspace/tools/pydata-python
uv pip install --python workspace/tools/pydata-python/bin/python \
  -r docs/assessments/data/jupyter-oracle-2026-10-25/modern-requirements.txt
npm install --prefix workspace/tools/jupyter-scip --ignore-scripts --no-audit --no-fund \
  @sourcegraph/scip-python@0.6.8
nice -n 10 go build -o workspace/tools/indexbench ./cmd/indexbench
# Install/build universal-ctags 6.2.1 locally as the existing corpus recipe requires.
# Check uptime and run one job at a time; the replay refuses load above6.
nice -n 10 workspace/tools/jupyter-python/bin/python \
  docs/assessments/data/jupyter-oracle-2026-10-25/replay.py \
  --server workspace/tools/jupyter-python/bin/basedpyright-langserver \
  --basedpyright workspace/tools/jupyter-python/bin/basedpyright \
  --scip-python workspace/tools/jupyter-scip/node_modules/.bin/scip-python \
  --ml workspace/calibration/ML-For-Beginners \
  --handbook workspace/calibration/PythonDataScienceHandbook \
  --modern workspace/calibration/pydata-book \
  --modern-python workspace/tools/pydata-python/bin/python \
  --ctags workspace/tools/ctags/bin/ctags --indexbench workspace/tools/indexbench \
  --out workspace/tmp/jupyter-replay
```

Use a fresh output directory. Git checkouts must be at the above revisions; sparse
checkout may omit translations and datasets, but never a nontranslation notebook.
`manifest.json` records every attempted notebook's SHA256 and the replay source
hashes; the driver verifies that original notebooks and environment declarations
have not changed. Input notebook text sent to basedpyright is unchanged. The
position scanner alone follows the existing benchmark's cell-magic policy; opaque
magic lines and rejected cells are counted. It reuses the existing oracle's caller
and written-name projection, never fathom's extractor or guesses about unknown
native answers. The existing manifest's name normalization collapses duplicate
qualified-name pairs; the raw and scored pair populations are both recorded.

`compare_native.py` produces a **diagnostic** reading against native-answered pairs,
alongside a syntax comparison on the same complete modern subject. Unknown answers
remain in the full native receipt and in fathom's precision denominator. This does
not turn a partial native truth population into complete tier-A support or apply
an oracle-blind exclusion rule. No canonical grade is changed. The one-run timing
fields are diagnostic, not a performance-ceiling claim. No holdout enters the shard.
