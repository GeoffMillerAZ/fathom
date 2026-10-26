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

HLSL remains open. Its committed L3 reading has recall 3,785/3,839 and explicit
DXC scope 543/644. G-R7 replaces unavailable frozen cloud bundles with a fresh
public-source replay. The dormant
[workflow](../../calibration/oracles/dxc-ast/scope-replay/hlsl-scope-replay.yml)
and [script](../../calibration/oracles/dxc-ast/scope-replay/replay.sh) use the
unmodified driver, manifest and indexbench from exact main above. The controller
copies the workflow to a run branch. There is no active `.github/` change here.

The workflow fetches all three subjects at the committed pins and verifies the
DXC Linux release archive before extracting it. It runs the driver's three full
measurements and scoped diagnostic, then publishes only a sanitized regenerated
reading and a numeric diff. The diff includes per-subject scope and counter
changes, call recall/precision, level/grades, tool versions and timing. Fresh
Linux tenancy may change grading independently of the scope rule. The committed
baseline includes a later Apple Silicon retime, so fresh Linux timing is not a
controlled before/after performance comparison.

The controller commit, replay-script hashes and immutable action revisions are
recorded separately from the production main commit.

Raw diagnostic notes and adjudication strings are replaced with counts and
hashes. Faulting-file paths become neutral ordinal IDs, preserving fault counts
and the reading schema. Raw ASTs, oracle outputs, unmatched pairs, subject file lists and harness
logs remain private workspace data. No canonical reading is changed by this
branch. No actual Linux replay or HLSL clearance is claimed by local tests.

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

Verified: `python3 -I -B calibration/oracles/dxc-ast/scope-replay/test_replay.py -v` and `bash -n calibration/oracles/dxc-ast/scope-replay/replay.sh`.

These 19 fast tests exercise pinning after a moved branch, dirty-source refusal,
platform/ABI and tool preflights, main-checkout driver invocation, private logs,
publication redaction, fault-list cardinality and numeric diffs including missing
metrics. They verify the active workflow copy and controller commit/file hashes. They use
stubs, not native DXC or a network replay.
