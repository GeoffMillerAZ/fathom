# HLSL scope replay controller

This directory is copied to a controller branch to replay the committed HLSL
reading at `6db920e72898abb20bf86c22652b401902e34598`. The replay uses that
checkout's unmodified `calibration/oracles/dxc-ast/reading.sh`, baseline reading,
corpus manifest, and `cmd/indexbench`. It fetches the three public subjects with
the manifest's pins and sparse/subpath rules.

Copy the workflow to its only supported active path without changing it:

```sh
install -D -m 0644 \
  calibration/oracles/dxc-ast/scope-replay/hlsl-scope-replay.yml \
  .github/workflows/hlsl-scope-replay.yml
cmp -s \
  calibration/oracles/dxc-ast/scope-replay/hlsl-scope-replay.yml \
  .github/workflows/hlsl-scope-replay.yml
```

In GitHub Actions, the replay verifies `GITHUB_WORKFLOW_REF` names that path and
that the active file is byte-identical to the committed controller copy. The
public diff records its path and SHA-256. A standalone Linux invocation records
`manual-no-active-workflow` with no active path or hash; it does not claim that
an active workflow was verified.

The only publishable files are:

- `workspace/artifacts/hlsl-scope-replay/regenerated-reading.json`
- `workspace/artifacts/hlsl-scope-replay/reading-diff.json`

The regenerated reading retains its schema, numeric evidence, tool versions and
source pins. Publication removes all raw diagnostic notes and per-subject
`adjudicated` entries. Each private `faulting_files` path becomes a generated
`redacted-fault-NNNNNN` ordinal placeholder so the schema invariant
`faults == len(faulting_files)` remains true. These placeholders contain no
source-derived names, paths, or hashes. Aggregate scope and three-run timing
receipts include separate redaction counts and SHA-256 hashes. The diff carries aggregate
scope by subject, the actual current level and grades, call precision/recall,
pooled and per-subject counter deltas, tool-version changes, and fresh timing.
Fresh tenancy participates in grading and may change the current level or grade;
the diff keeps timing separate from scope and counter changes. Raw AST,
oracle, unmatched, subject file lists, logs, manifests and private receipts stay
under `workspace/tmp/` and are never uploaded.

## Approved runner

The exact DXC archive is correctly pinned at SHA-256
`a1d3e3b5e1c5685b3eb27d5e8890e41d87df45def05112a2d6f1a63a931f7d60`,
but its ELF imports require `GLIBC_2.38` (`__isoc23_strtoul`,
`__isoc23_strtoll`, `__isoc23_strtol`, and `fmod`) and `GLIBC_2.36`
(`arc4random`). The approved Ubuntu 24.04 runner supplies
[glibc 2.39](https://packages.ubuntu.com/noble/libc6), satisfying the archive's
2.38 requirement. The script still inspects the downloaded binaries and compares
their highest GLIBC requirement with the actual runner before executing DXC. No
private glibc or container workaround is implemented.

Before creating replay data, the script refuses tracked, untracked, or ignored
content outside `workspace/` in both the exact source and controller checkouts.
It verifies `GITHUB_SHA` against the controller commit, checks each controller
file against that commit, and records the commit plus controller file SHA-256
hashes in the public diff. Publication also requires the regenerated reading's
7-to-40-character hexadecimal `fathom_commit` to prefix-match the exact source
commit. It also verifies
the runner provides mikefarah `yq` v4 and `jq` 1.6 or newer, and keeps the
committed reading driver's complete stdout/stderr in a private workspace log.
Failures expose only a generic stage error in the Actions job log.

The replay prefers `clang-18` when the runner exposes it, otherwise it uses the
runner's explicit `clang` and records the exact version. The public diff shows
the committed baseline version beside the replay version so a tool change is
never silent.

Run the fast local proof with:

```sh
python3 -I -B calibration/oracles/dxc-ast/scope-replay/test_replay.py -v
bash -n calibration/oracles/dxc-ast/scope-replay/replay.sh
```
