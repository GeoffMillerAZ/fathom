# Phase K — agentic-kit uplift (vision 11 §3.K)

Truth and discipline, not bulk adoption. Boundary fact: fathom imports 13
distinct kit packages across 7 of 27 foundations categories.

Design (normative):
- **K1 vector donor deletion (H4 Stage 4).** fathom's `internal/vector`
  (185 LOC) duplicates code the kit already generalized FROM fathom
  (foundations/embedding/vector-store). Execute the kit handoff plan's own
  acceptance checks: zero cosineSimilarity/NewMemoryVectorStore definitions
  in fathom by grep, import proof, tests green on the kit implementation.
- **K2 file asks, don't build.** Two dated, D8-disciplined parked rows in
  agentic-kit/docs/plans/: (a) the evidence-agnostic proof-gate kernel of
  internal/verify (Task/Criterion/Disposition + Gate aggregation, no
  Evidence impl); (b) the eval-harness kernel. Each with trigger: a second
  named consumer. Neither is built now.
- **K3 instrument existence checks into kit telemetry.** A reusable
  assertion (traffic expected on channel X since t0; zero → loud) promoted
  from doctrine law 7; A1/B2's check script consumes it.
- **K4 non-adoption ADR paragraphs.** Why fathom runs its own MCP server
  (not kits/mcp-server) and its own eval harness (not eval-runner) — one
  honest paragraph each in fathom's ADR.md.

## Tasks

- [x] internal/vector deleted; consumers import the kit's vector-store; H4 Stage 4 acceptance checks pass; kit handoff plan checkboxes closed (fathom cherry-pick of 3b8fb8f; kit 519bbe6) — superseded 2026-09-19: ADR-041 decision 2 deleted the vector path outright (`WithVector` and the hybrid-rank branch are gone), so the row anchors to the decision that retired it {id: K1-1, oracle: lexical, path: ADR.md, pattern: "`WithVector` option and the hybrid-rank vector branch are deleted"}
- [x] Two parked ask rows filed in agentic-kit/docs/plans with named-consumer triggers (FA-11 proof-gate kernel, FA-12 eval-harness kernel; kit 75e3c1b); kit gate green {id: K2-1, symbol: VerifyPlan, package: internal/verify, skip_test: true, skip: "cross-repo docs filing anchored to the package whose kernel is being offered; nothing is built"}
- [x] Kit telemetry existence-check assertion (heartbeat.CheckExistence, kit eb24494) + fathom's check script consuming it (scripts/check-surface-traffic.sh, python>=3.11 guard; surface_traffic emission landed with A1-1) {id: K3-1, symbol: registerServerStats, package: internal/mcp, skip_test: true, skip: "kit-side code carries kit-side tests; fathom-side is the thin consumer script over A1-1's tested block"}
- [x] ADR paragraphs recording the two deliberate non-adoptions (ADR-022/ADR-023, renumbered + set_profile reference corrected at merge) {id: K4-1, symbol: registerServerStats, package: internal/mcp, skip_test: true, skip: "docs-only; anchored to the served component whose choice it documents"}
