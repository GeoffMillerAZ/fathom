# Stabilization handoff — 2026-09-17

Owner-directed scope change: stop the open-ended G1–G6 execution campaign and retain
demand-triggered work under [ADR-0072](../adr/0072-stabilize-and-resume-on-demand.md).
The [canonical guide](../design/stabilization-and-resumption.md) contains six resume
packets, source/spec/evidence pointers, activation triggers, authority, acceptance,
budget and stop rules. No remaining quality outcome is relabeled complete.

## Verification

- `go test -count=1 ./cmd/appkit-mcp` passed on Go1.26.3, including actual in-memory
  MCP tool/resource document parity, the new stabilization route and existing guidance
  regressions. Existing offline module/cache locations were reused; no dependency
  change, installed-server restart or live host-loading claim.
- 871 scoped document checks passed: relative links; root router/living-doc and new
  ADR budgets; ADR membership, status category and supersession targets; matching
  agent intent blocks and continuation routes; Unreleased shape and budget.
- `git diff --check` passed on the affected tracked guidance/doc/MCP paths.
- A bounded independent review found no blocking restart, authority or evidence
  contradiction. Its G4 excerpt concern was clarified: retained completion criteria
  are not current landing prerequisites.
- Ordinary Fathom re-ingestion succeeded. No semantic compilation, new layer,
  shared-daemon restart, global configuration or consumer write was performed.

The first MCP run failed because the landing introduction used a formal goal marker
without a stable goal ID. It now labels the campaign's landing boundary separately
from the retained G-numbered outcomes; the existing assertion is unchanged and passes.
An initial literal ADR-status comparison also rejected16 inherited abbreviated index
rows. Inspection confirmed matching status meaning and successor IDs; the scoped
check compares those, not verbatim explanatory prose. Historical records were not rewritten.

These are structural/protocol checks, not a full intent-doctor, whole-tree stability
audit, fresh browser/native matrix or evidence of consumer benefit. Previously tested
browser preparation and exact retained receipt identities are in the
[submission-boundary handoff](2026-09-17-theme-guide-submission-boundaries.md).
Its four browser acquisitions were deliberately not started; guard/cache/memory and
broader quality gaps remain open. The approved Fathom guide is deferred, not delivered.

## Stop and resume

The bounded documentation/MCP handoff is complete. Stop this campaign; resume only a
selected packet with a named need or explicit owner-funded maintenance budget.
The earlier approximate87% estimate is historical, not increased by this deferral.
G2/G5 remain accepted only under their fixed matrices; G1/G3/G4/G6 residuals remain.

The extensive shared dirty worktree is preserved. No mass staging, commit, push,
release, installation or deployment occurred. These local source records and retained
receipts are not a versioned distribution: exact source closure, private-source
exclusions and clean consumer delivery remain necessary before sharing a new pin.
