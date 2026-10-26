# OpenCL C: pinned upstream compiler qualification

The official LLVM 21.1.8 macOS ARM64 toolchain successfully replays the existing
OpenCL SDK measurement pipeline. This qualifies one bench instrument. It claims
no OpenCL support level and does not change the frozen grammar population.
The language still must pass the whole-pool parser gate and independent oracle
coverage before registration or a reading.

## Source identity

[The archive plan](../../calibration/oracles/opencl-c/llvm-plan.json) pins three
platform archives. Only macOS ARM64 was downloaded and run. Its 1,503,047,352
bytes matched SHA-256
`b95bdd32a33a81ee4d40363aaeb26728a26783fcef26a4d80f65457433ea4669`
before extraction and again before recording the compiler/header identities.
The [selected profile](../../calibration/oracles/opencl-c/llvm-21.1.8-macos-arm64.json)
binds the extracted archive member, compiler bytes and both resource headers.
The historical Apple profile is unchanged and remains the default.

## Actual replay

| Fact | Count |
|---|---:|
| SDK compilation units | 10 |
| Errors / skips | 0 / 0 |
| Compiler function ranges | 23 |
| Resolved written call sites | 147 |
| Declaration-shaped sites | 26 |
| Active-line roots / ranges / lines | 10 / 89 / 546 |

The [receipt](data/opencl-c-implementation-2026-10-26/llvm-sdk.json) records
source, resource and database stability and the compiler pins checked before and
after every consumer. This is a reconstructed CL3.0/SPIR64 context, tier B.
It is not the subject's runtime configuration union. Path-bound run hashes
identify private runs, not cross-host reproducibility. The counts reproduce the earlier SDK facts in an actual replay; parity was not
inferred from compiler version labels. Raw output hashes identify each run;
the upstream LLVM and historical Apple output hashes differ.

## Proof and privacy

[The committed replay recipe](../../calibration/oracles/opencl-c/README.md#explicit-upstream-llvm-replay)
contains the archive check and exact SDK command. Run the focused tests with:

```sh
workspace/tools/python/bin/python -I -B calibration/oracles/opencl-c/test_compdb.py
workspace/tools/python/bin/python -I -B calibration/oracles/opencl-c/test_replay_sdk.py
```

The initial API tests failed before implementation; all 24 tests now pass.
Review added two regression tests: public receipts omit fetch URLs that may carry
credentials, and portable profiles require and receipt a canonical relative
compiler archive member. Those tests failed before their fixes and pass after.
The bounded delta review reported no remaining findings. Raw ASTs, subject
inventories and tool logs stay in ignored workspace.

This implements the existing compiler-reuse path under ADR-048. No new design
decision, plan checkbox, canonical corpus or generated tracker is changed.

## Whole-pool context work still required

[The pinned host-source audit](../../calibration/oracles/opencl-c/contexts-r1.json)
records each primary project's program assembly and compiler-option sources.
The SDK baseline does not stand in for the other seven projects. OpenCV and SHOC
use source-specific option variants; ArrayFire prepends generated source;
PyOpenCL's tracked `.cl` files are consumer headers; clBLAS contains host-string
templates. Those facts affect oracle coverage, not membership in the frozen pool.
The exact variant manifest must be fixed from host source before measuring
compiler coverage. This audit implements no transform and claims no additional
compiler run. General grammar repair remains the next implementation step.
