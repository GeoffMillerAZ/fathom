# Python frozen-lock retries

Measured on 2026-10-26 (UTC). This reading supersedes the live-PyPI round-1
C3 environments. `python-r2.json` records commands, exact upstream and timed
lock hashes, installed distributions, individual times, exit codes and raw-log
hashes. Pins and tracked source files are unchanged; C4 remains unrun.

Starlette uses `uv sync --frozen --no-default-groups --group dev` (that group
includes `starlette[full]`). Marshmallow, Click and attrs use `--group tests`.
The tool is uv 0.13.5. After setup, the installed Python runs pytest directly
with no dependency resolution. Every warm-up and three timed processes uses
`--network none --cpus 4`, cleared proxies, `UV_OFFLINE=1`, `PIP_NO_INDEX=1`,
and the same pinned Python 3.12.15 arm64 image as round 1. The HTTPS denial
probe exits 7. Offline subprocesses are serialized; waiting and setup are not
part of timings. Temporary files use the container's explicit paths; `/work/tmp`
is an ephemeral Linux tmpfs for Unix sockets. Native macOS/twelve cores remain
unmeasured, and comparisons are observed deltas, not causal dependency effects.

```sh
BENCH_JOB="$job" python3 calibration/bench/tools/remeasure.py "$config"
```

| Subject | Round 1 seconds | Frozen round 2 seconds | Exits | Mean delta (seconds) |
|---|---|---|---|---:|
| encode/starlette | 15.016, 14.93, 15.263 | 13.118, 16.943, 11.149 | 0,0,0 | -1.333 |
| marshmallow-code/marshmallow | 4.554, 4.518, 4.54 | 5.259, 1.323, 1.577 | 0,0,0 | -1.818 |
| pallets/click | 11.442, 11.386, 11.375 | 4.331, 18.352, 7.507 | 0,0,0 | -1.338 |
| python-attrs/attrs | 9.023, 9.021, 8.85 | 8.937, 14.241, 6.851 | 0,0,0 | +1.045 |
| Textualize/rich | 8.077, 8.073, 7.887 | 5.154, 5.306, 4.945 | 0,0,0 | -2.877 |

Rich's committed export is `calibration/bench.d/textualize-rich-requirements.txt`,
SHA-256 `623a2f8b54791115ccd1d748ce5541fe2ff3ec8e58693a4a6c00f767c6a3e991`. The source Poetry lock is unchanged. Poetry 2.2.1 with
poetry-plugin-export 1.9.0 refused the current manifest's stale content hash.
The export therefore used the matching `pyproject.toml` from lock-writing commit
`041406ca9b60af8fcc00cd7fc1b2973ab51eb851` in a scratch export directory, after verifying
that commit's lock bytes equal the current pin's lock bytes. No re-lock occurred
and the subject was not patched. The command was `poetry export --format
requirements.txt --with dev --output "$requirements"`; installation used
`pip install --require-hashes -r "$requirements"`, then `pip install --no-deps
-e .`. The manifest's `lockfile_sha256` now identifies this export, which binds
the runtime/development dependency installation. Isolated editable-build tooling
is fetched separately and is not claimed to be covered by the runtime lock.

For the other four subjects, `lockfile_sha256` identifies the exact `uv.lock`
used by the successful frozen sync. Round 1 did not use those locks and fetched
live PyPI; its archived lock hashes describe only upstream files. The corrected
manifests contain the new setup, test, maximum trial time and lock hash. Rich
and marshmallow remain dev-forced by their committed prior reads.

Click's initial frozen warm-up failed its surrogate-filename test on the shared filesystem. That invalid environment was replaced by native tmpfs for `--basetemp`; all final Click timings below use native temporary storage. No test was skipped or patched.

Click's B2 comparison initially uses the exact historical less archive, copied
by hash without fetching a live mirror. `click-snapshot-r2.md` supersedes that
package acquisition with a snapshot URL and a separate full offline retest.
