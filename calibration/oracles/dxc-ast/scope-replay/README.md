# HLSL scope replay controller

Replay exact main `6db920e72898abb20bf86c22652b401902e34598` on the owner-approved
ubuntu-24.04 x64 runner. The dormant workflow stays under `calibration/`; this
branch changes no active `.github/` workflow or canonical language reading.

## Controller run

Create the exact run branch `gpu-sci/hlsl-scope-replay-run-r1` from the reviewed
head. Copy the workflow without editing it, commit the active copy, then push
that run branch. Its push event starts the first run, including when the
workflow has never existed on the default branch. Manual dispatch is optional.

```sh
mkdir -p .github/workflows
install -m 0644 \
  calibration/oracles/dxc-ast/scope-replay/hlsl-scope-replay.yml \
  .github/workflows/hlsl-scope-replay.yml
cmp -s \
  calibration/oracles/dxc-ast/scope-replay/hlsl-scope-replay.yml \
  .github/workflows/hlsl-scope-replay.yml
```

The script verifies the controller commit and committed controller files, the
active workflow path and byte equality, and a clean exact-main checkout outside
`workspace/`. The workflow has read-only contents permission, immutable action
revisions, no secrets or pull-request triggers, and a 120-minute job timeout.

## Paired measurement

The exact-main checkout runs its unmodified `reading.sh`: regenerate DXC and
include truth once, run three whole-population measurements and one scoped
diagnostic, and assemble the median reading and generated notes.

A separate private checkout at that same main commit applies only
[scope-control.patch](scope-control.patch). This restores the pre-#45 call guard
and named-call-file coverage. Definitions and other scope handling stay as they
were at main. The control indexbench regrades the same oracle files and subject
checkouts with the same arguments, for three whole-population readings and one
scoped diagnostic. `NOTES_ONLY=1` uses the unchanged driver to assemble its
reading. The control does not rerun DXC. Input identity checks refuse changes to
the shared oracle JSON and scope files.

The two uploaded files are:

- `workspace/artifacts/hlsl-scope-replay/regenerated-reading.json`
- `workspace/artifacts/hlsl-scope-replay/reading-diff.json`

The diff includes the complete control reading and recursive field-by-field
comparisons of committed-to-main, committed-to-control, and control-to-main.
These include tenancy, families, kinds, tiers, target/disposition, notes, and
subject details. Main versus control isolates the changed scope rule on shared
truth. Timing remains a fresh measurement in each arm and is not a controlled
performance effect. Comparison with the committed reading also includes other
source, toolchain and runner drift.

Public notes, adjudication entries and fault evidence are retained. Publication
redacts person-shaped metadata and absolute host paths, with redaction counts.
It does not substitute hashes for removed public evidence. Raw ASTs, oracle
outputs, unmatched pairs, subject file lists and logs remain private workspace
data. Only the two named JSON artifacts are uploaded.

## Pinned runtime

DXC v1.9.2602 uses `linux_dxc_2026_02_20.x86_64.tar.gz`, SHA-256
`a1d3e3b5e1c5685b3eb27d5e8890e41d87df45def05112a2d6f1a63a931f7d60`.
Its GLIBC_2.38 imports exclude ubuntu-22.04. The owner approved ubuntu-24.04
without changing the compiler pin. The script checks the archive before
extraction and compares actual ELF requirements with the runner before DXC
executes. It records the selected Clang version and verifies mikefarah yq v4
and jq 1.6 or newer. There is no container or private glibc.

## Local proof

Install the pinned YAML test dependency only in a workspace Python environment:

```sh
uv pip install --python workspace/tools/python/bin/python \
  -r calibration/oracles/dxc-ast/scope-replay/requirements-tests.txt
workspace/tools/python/bin/python -I -B calibration/oracles/dxc-ast/scope-replay/test_replay.py -v
workspace/tools/python/bin/python -I -B calibration/oracles/dxc-ast/scope-replay/test_control.py -v
workspace/tools/python/bin/python -I -B calibration/oracles/dxc-ast/scope-replay/test_workflow.py -v
bash -n calibration/oracles/dxc-ast/scope-replay/replay.sh
```

These are local contract tests. They do not clear HLSL's replay hold or claim a
Linux DXC result. The controller runs the reviewed workflow.
