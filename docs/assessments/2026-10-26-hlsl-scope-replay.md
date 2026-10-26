# HLSL explicit-scope replay and unscoped compatibility

G-R7 clears SQL and CUDA with the undeclared-scope no-op test and the
[stored-count receipt](data/hlsl-scope-replay-2026-10-26/unscoped-regrade-receipt.json).
Their native producer shapes declare no neutral `scope`, so the explicit-scope
rule cannot widen their call coverage or interpret an empty calls list as truth.
The test uses synthetic paths and producer JSON shapes. It is a contract proof,
not a fresh invocation of either native toolchain.

At exact main `6db920e72898abb20bf86c22652b401902e34598`, stored-count regrading
leaves SQL at L2 with call recall 17/17 and CUDA at `none` with recall 3,273/3,283.
Grades, metrics, all aggregate and subject counts, tenancy and required kinds are
unchanged. The replay and its test run from the repository root so current
dialect manifests resolve. Scope counts are absent for these producer contracts, rather than an
explicit zero. The receipt pins producer source hashes and the clean main binary.
PHP remains in the existing stored-count compatibility test.

HLSL remains open. The committed reading (L3) has call recall 3,785/3,839 and explicit
DXC scope 543/644. G-R7 replaces unavailable frozen cloud bundles with a fresh
public-source replay. The dormant
[workflow](../../calibration/oracles/dxc-ast/scope-replay/hlsl-scope-replay.yml)
and [script](../../calibration/oracles/dxc-ast/scope-replay/replay.sh) use the
unmodified driver, manifest and indexbench from exact main above. The controller
copies the workflow to a run branch. There is no active `.github/` change here.

The workflow fetches all three subjects at committed pins and verifies the
DXC archive before extracting it. An exact run-branch push starts the first
run; manual dispatch is optional. The main arm runs the unmodified driver's
three full measurements and scoped diagnostic. A separate exact-main control
checkout disables only #45's call guard and explicit call-file expansion, then
regrades those same oracle files and subjects. It runs no additional DXC
producer. Input identity checks refuse changes during the paired replay.

The two public artifacts contain the regenerated main reading and a diff with
the complete control reading. Recursive comparisons cover every JSON field for
committed-to-main, committed-to-control, and control-to-main. Scope matching is
compared on shared fresh truth. Runner timing remains noisy and cannot be
attributed causally to #45. Comparison with the older committed reading also
contains unrelated source and toolchain drift. The committed L3 file-edge
recall is 538/1,222 and precision is 538/597; these are distinct from call recall.

Public diagnostic notes and adjudication evidence remain available. Targeted
privacy redaction removes person-shaped metadata and absolute host paths,
without replacing whole evidence classes with uncheckable hashes. Raw ASTs,
oracle outputs, unmatched pairs, subject inventories and logs stay private.
Controller and control-patch identities remain checkable from committed files.
No canonical reading is changed here. Local tests claim neither a Linux replay
nor HLSL clearance.

## Approved runner

The owner approved ubuntu-24.04 while preserving the exact DXC pin. The original
G-R7 ubuntu-22.04 runner could not load the archive: the pinned DXC v1.9.2602 ELF
imports require GLIBC 2.38, while [ubuntu-22.04 supplies glibc 2.35](https://packages.ubuntu.com/jammy-updates/libc6).
The runtime preflight remains mandatory on ubuntu-24.04. No container, compiler
substitution or private glibc is introduced.

## Proof

Verified: `bash scripts/fathom-go.sh test -race -count=1 -run 'TestScoreToolOracleUndeclaredScopeNoOp|TestCmdLangRegradeCallScopeUnaffectedReadings' ./cmd/indexbench`.

Mutation: in `cmd/indexbench/langoracle.go`, replacing the explicit-scope family
guard with all measured files makes the no-scope test fail for empty SQL calls
and both CUDA companion shapes. Unconditionally extending call coverage makes
the nonempty SQL case fail: adding an unrelated file changes `call_fathom` from
1 to 2. Both mutants were restored. The focused restored race suite passed in
6.529 seconds. The receipt records neutral failure summaries rather than private
raw logs.

Verified commands for the corrected package are listed in
[the controller README](../../calibration/oracles/dxc-ast/scope-replay/README.md#local-proof).
They cover recursive all-field diffs, preserved public evidence and targeted
privacy redaction, control-source and same-oracle invariants, and a parsed YAML
security contract with trigger, permission, secret, action-pin and timeout
mutations. The controller still owns the actual Linux run.

## Binding review corrections

F1 starts the never-merged run branch on an exact push filter. F2 compares both
indexbench variants on the same regenerated oracle files, retaining the main
checkout and driver unchanged. F3 reports every JSON field. F4 retains public
notes and adjudication with targeted privacy redaction. F5 parses the workflow
and proves forbidden trigger, permission, secret, action and timeout mutations
fail. F6 distinguishes call recall from L3 file-edge metrics.

The control binary compiled locally from the pinned main source with only the
two committed call-scope edits. This ARM build qualifies the control preparation
path, not DXC or the Linux HLSL reading. Controller review and a real Linux run
remain required before clearing the HLSL hold.

Correction checks: 22 replay, 5 control and 3 workflow tests passed. X-R2 release
notes and full/staged merge-debris checks passed. X-R3's ADR-011 test passed in
2.609 seconds. The staged secret scan found zero findings without a baseline.
A final independent local delta review reported no remaining findings.
