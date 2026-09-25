# Explicit first-adoption draft

## Prospective scope and acceptance

Owner-resumed adoption maintenance, baseline `ddb0201`. Existing `pins` requires
a valid manifest and cannot remove first-time JSON/hash boilerplate. Add the
CLI-only [ADR-0078](../adr/0078-draft-explicit-adoption-selections.md) path. No
consumer write, installed-binary change, global configuration or runtime claim.

Acceptance fixed before execution:

- An explicit selection yields schema-valid deterministic v2 JSON. Reordered
  equivalent flags produce identical bytes; input options/files remain unchanged.
- Selected metadata uses canonical current pins; optional discovery revision
  mismatch, incompatible fit, duplicates and invalid UTF-8 refuse before output.
- Artifact reads reuse the confined reader and its item/per-file/aggregate limits.
  Missing/unsafe/symlink/special files and cancellation do not yield a selection.
- Optional module/agent requirements remain explicit declarations. Absent related
  files may still permit drafting but must fail the subsequent selected check.
- A synthetic operator saves the draft, creates contract/router files, and obtains
  a static pass; artifact drift/deletion fails, restoration passes. Existing
  inspect/pins/check report bytes match baseline for the same complete fixture.
- Actual CLI processes have truthful 0/1/2 behavior and machine-only stdout;
  validation/read failures have no JSON, writer errors return2. No terminal UI changed.
- Focused regression/race/vet/build checks, MCP guidance/protocol tests and existing
  intent/index gates pass. No browser, host-registration or real-consumer proof inferred.

Evidence root: `/Volumes/m2ext/appkit-evidence/appkit-adoption-draft.MrK8rvoY`.
Independent baseline/candidate sources retain Git archive plus exact changed files
before execution, selected Go binary/version, commands and output. Tests run in the
private source copies, not consumer roots. Actual-process fixture:
[`fixtures/adoption-draft/probe.mjs`](fixtures/adoption-draft/probe.mjs).
Bound this packet to 45 minutes from 10:03 UTC, with at most one separately retained
verification repair before handing off remaining failures. No acceptance weakening.

## Results

The first frozen candidate ran 768 test/subtest events. The sole failed test was
`TestDecisionsIndexCoversEveryADR`: the new D78 prose/index existed but the compact
DECISIONS table still ended at D77, so MCP guidance omitted the new range. Add the
missing D78 row; retain the failed candidate and unchanged test. All draft and old
adoption tests passed in that run. No acceptance expectation was weakened.

The corrected `final` private source run passes all 768 test/subtest events across
`internal/adoption`, `cmd/appkit` and `cmd/appkit-mcp` with race detection, plus
scoped vet and an offline CGO-disabled CLI build. The source overlay did not drift.
Go 1.26.3, the baseline archive, changed files, command/environment and logs are
retained. No new-API pre-implementation failing-test run is claimed.

`process-probe` passes 21 actual baseline/candidate CLI invocations using a private
synthetic consumer. It proves the baseline has no draft command; candidate output
is deterministic, matches the artifact SHA-256 and preserves a malformed existing
manifest without reading it. Explicit fixture saving alone does not pass: missing
contract/routers fail. After fixture setup, old inspect/pins/check JSON bytes are
identical between baseline and candidate. Artifact drift and deletion fail; restoring
the original bytes passes. Stale discovery, unknown IDs, unsafe paths and duplicates
refuse with exit2/no JSON. A module-only draft names its exact version and empty
arrays without creating or reading go.mod. Every owned process exits normally.

Additional package tests cover input immutability, invalid UTF-8, module/agent-only
selections, cancellation, symlink/directory/oversize rejection, item limits, the
shared 64 MiB read budget, help boundaries and broken/short stdout writers.
Independent bounded review found no remaining correctness issue; its initially
reported empty-selection concern was retracted after checking the existing Decode
guard, which is also exercised by a refusal test.

Tested candidate binary SHA-256:
`72df33717f4ad4523a139ce745a297de67b9496054c06c7fe09fd7863883843c`.
This is source-snapshot/CLI proof, not an installed release, consumer upgrade,
host-registration, runtime, browser or complete adoption/G4 acceptance claim.
No dependencies, consumer files, installed servers or global configuration changed.
