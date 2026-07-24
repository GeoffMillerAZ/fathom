# Wave 5 — Trust bugs + the tokenomics second swing

Owner directive (2026-07-23): fix the bench's trust findings AND take one
more falsifiable swing at pulled-tool tokenomics/discovery — not a retreat
from the home-turf refutation, a causal-fix attempt with a pre-registered
end state either way.

Design (normative):
- **T5-1 O13 — citation drift.** The sweep verifies anchor TEXT, not just
  the line hash at the cited offset: on drift, attempt re-anchor (search
  the corpus for the rule's stored anchor text; single unambiguous match →
  re-cite with an event recording the move, like ADR-017 remap), else flip
  stale. Fixture: upstream-insertion shift (the exact O13 reproduction
  shape). Then the operational pass: a fleet re-sweep flags/heals existing
  wrong citations (recorded counts in this row).
- **T5-2 O14 — eviction visibility.** The obligations serving path reports
  `displaced: N` when the cap drops served-eligible claims, and the
  response labels which layer each served rule came from. Scope-mapping
  noise (mis-scoped rules) is a compile-prompt fix — FILED as the next
  flag-day's change (prompt identity bump), not silently patched.
- **T5-3 refutation propagation.** Every doc asserting the pulled-tool/
  home-turf story gets the verdict with the dated pointer (the H3-1 row's
  owed slice). Sweep: design docs, use-cases, adoption guides, vision.
- **T5-4 read_source.** A line-ranged, size-capped, read-only raw-text tool
  on the agent surface. Kills the T13 class (601k tokens of contortion for
  want of `sed -n`). Doctrine-4 arithmetic stated honestly: the program's
  second net-new tool, justified by measured evidence; the orientation
  quadrant's law-8 clock remains the offset candidate.
- **T5-5 line-granular doc anchors.** DocChunk carries line offsets so
  doc-cited answers can pin lines (kills the T08 class; feeds O13
  precision).
- **T5-6 one-shot completeness.** symbol_context returns the definition's
  source snippet inline (capped) — define-and-show answers in ONE turn.
  Turns are the tokenomics lever; every saved turn ≈ the session baseline.
- **T5-7 forced-channel measurement.** The load-bearing claim gets the
  measure-or-retract discipline: verify_plan/review calls log their verdict
  outcome (pass/fail/catch counts) and `instrument --adoption` reports
  gate catches honestly (replacing the current "not instrumented" marker
  for exactly that field).
- **T5-8 the second swing, pre-registered.** After T5-4/5/6 merge: re-run
  BOTH benches (lookup + home corpora, unchanged criteria, turns reported
  alongside). Outcomes recorded in this row either way. KILL CRITERION,
  BINDING: if the home bench still fails its criterion, the parity
  positioning is PERMANENT — pulled-tool investment drops to maintenance,
  the docs keep the refutation, and no third swing is attempted.

## Tasks

- [ ] Sweep verifies anchor text with single-match re-anchor + event trail; upstream-shift fixture proves heal and stale paths {id: T5-1, symbol: SweepClaims, package: pkg/client}
- [ ] Fleet re-sweep run post-merge: wrong-citation counts found/healed/staled recorded here {id: T5-1b, symbol: SweepClaims, package: pkg/client, skip_test: true, skip: "operational row; evidence = recorded counts + O13 status update"}
- [ ] Obligations serving reports displaced count + per-rule layer labels; eviction test {id: T5-2, symbol: ObligationsFor, package: internal/semantic}
- [ ] Scope-mapping prompt fix filed for the next flag-day (prompt identity bump, NOT compiled now); O14 updated {id: T5-2b, symbol: ObligationsFor, package: internal/semantic, skip_test: true, skip: "filing row — the fix is recompile-gated by design; anchor shared with T5-2"}
- [x] Refutation propagated to every doc asserting the pulled-tool story; sweep list recorded {id: T5-3, symbol: cmdVerify, package: cmd/fathom, skip_test: true, skip: "docs row anchored to the gate at the story's center; critic pass enforces the sweep"}
  - **Sweep (2026-07-24 home-bench H3-1 refutation pointer added):** grepped every doc for pulled-tool/home-turf/×10/context-bench claim language, then read the candidate list plus every match. Full sweep table (file → status):
    - `docs/design/coding-agent-excellence.md` → **edited** (§3 lead-in, the "governed capabilities...have no grep equivalent and were not measured by that bench" sentence — appended the dated H3-1 verdict)
    - `docs/design/product-vision.md` → **edited** (§3 use-case table, "Agentic coding companion" row — appended the 2026-07-24 update)
    - `docs/use-cases/coding-agents.md` → **edited** (§5 "The cost story, honestly" — appended the home-turf follow-up paragraph after the B1 cost story)
    - `docs/vision/04-coding-agents.md` → **edited** (§1 world-class metric table, Context-bench row — appended the H3-1 follow-up)
    - `docs/vision/11-dial-to-eleven.md` → **edited** (Phase B, B1 row — appended the second-swing outcome)
    - `docs/use-cases/getting-started.md` → **clean** (its "~77% token reduction" claim is `query_symbols` vs unbounded symbol dumps, not a grep-comparison — not the story under refutation)
    - `docs/adoption/claude-code.md`, `docs/adoption/codex.md`, `docs/adoption/spec-driven-development.md` → **clean** (no match)
    - `README.md` → **clean** (no match; the tokenomics-explainer link is unretouched)
    - `dist/agents/skills/fathom-onboard/SKILL.md`, `fathom-verify/SKILL.md`, `fathom-plan/SKILL.md`, `fathom-feedback/SKILL.md` → **clean** (no match; feedback's SKILL.md mentions falling back to grep mechanically, not a value claim)
    - `docs/design/semantic-layers.md` → **clean** (false-positive grep match: a plan filename, `pulled-tools-excellence.md`, not a claim)
    - `docs/vision/README.md`, `docs/vision/01-thesis.md`, `docs/vision/07-questions-and-risks.md` → **checked, left as history** (2026-07-14 "FOR REVIEW, nothing binding" excellence-review directory; already correctly hedged "plausible but unproven" / "testable in a month" pre-registration language predating both benches — the B1-4 precedent's own discipline: describing a past gap/criterion stays history, only live status cells get the pointer)
    - `docs/dogfood-observations.md` (O12) → **checked, left as history** (dated incident-log entry, not a live assertion)
    - `docs/tokenomics-explainer.html`, `docs/graphify-comparison.html`, `docs/assessment-visual.html` → **checked, left as-is** (pre-rename visualizations, self-labeled OBSERVED/SPECULATIVE already, not on the candidate list, predate the ×10 claim's own introduction)
    - Pointer text used everywhere (paraphrased to fit each doc's voice): *"2026-07-24 home-bench refutation (docs/plans/home-bench.md H3-1): equal success 12/13 vs 12/13; the surviving honest pitch is equal answers, cheaper on 8/13 (outlier-driven aggregate premium), 2–3× faster, governance unmeasured by that bench."*
- [ ] read_source: line-ranged, size-capped, read-only; on the agent surface; refusal tests for range/size abuse {id: T5-4, symbol: registerReadSource, package: internal/mcp}
- [ ] DocChunk line offsets end-to-end (extractor → store → serving); T08-class fixture {id: T5-5, symbol: registerReadSource, package: internal/mcp, skip_test: true, skip: "anchor shared with T5-4 until landing — update to the real extractor symbol at landing; its own tests are the coverage"}
- [ ] symbol_context inline definition snippet (capped); one-turn define-and-show test {id: T5-6, symbol: registerSymbolContext, package: internal/mcp}
- [x] verify_plan/review verdict outcomes logged; instrument --adoption reports gate catches (marker replaced with real counts); tests {id: T5-7, symbol: cmdInstrument, package: cmd/fathom}
  - **(a) Logging.** `internal/mcp/calllog.go`: `callLogEntry` gains `VerdictOutcome map[string]int json:"verdict_outcome,omitempty"`; `verdictOutcomeFrom` mirrors `claimsAttachedFrom`'s two-shape extraction (wrapped top-level envelope + raw batch sub-call map), wired into both `dispatch.go`'s `loggedToolsCall` and `extra_tools.go`'s batch `logSub`. `verify_plan` (`internal/mcp/verify_plan_tool.go`) sets `verdict_outcome: {claimed, verified, failing}` over exactly the Claimed-and-not-Skipped population `internal/verify.Gate` itself gates on (nil/omitted when nothing claimed). `review`'s forced channel — C3-1's `bin/fathom review` (lefthook's pre-push step), which never touches MCP dispatch — gets its own writer: `cmd/fathom/review.go`'s `appendReviewCallLogEntry` appends a call-log-shaped JSONL row (`tool:"review"`, `surface:"cli"`) with `verdict_outcome` keyed by CHECKED dimension name only (unchecked dimensions omitted, never a misleading 0); new `--call-log` flag defaults through the same `resolveCallLogPath()` config resolution `instrument --adoption` uses.
  - **(b) Reporting.** `cmd/fathom/instrument_adoption.go`: `adoptionCallEntry` gains the mirrored `VerdictOutcome` field; `gateCatchStats` sums verify_plan's `failing` + review's per-dimension counts into `VerifyGateCatchCount` (real, non-nil once ≥1 row is instrumented), while rows with `VerdictOutcome == nil` are counted separately into the new `VerifyGatePredatesInstrumentation` field — the "honest split," never folded in as a fabricated zero. `VerifyGateInstrumentedCalls` + an informative `VerifyGateNote` complete the report; the old always-nil "not instrumented" marker only fires when the window truly contains zero instrumented rows.
  - **Tests** (all green, `-race`): `internal/mcp/verify_plan_tool_test.go` `TestVerifyPlan_VerdictOutcomeReachesCallLog` (top_level / via_batch / no_claimed_tasks_omits_field — the D1-2 `TestBrief_ClaimsAttachedReachesCallLog` precedent shape, generalized from scalar to map); `cmd/fathom/review_test.go` `TestReview_AppendsCallLogVerdictOutcome`, `TestReview_CallLogDefaultsFromConfig`; `cmd/fathom/instrument_adoption_test.go` `TestBuildAdoptionReport_GateCatchesRealCounts`, `TestBuildAdoptionReport_GateCatchesHonestSplit`, `TestCmdInstrumentAdoption_MixedEraGateCatches` (the fixture-log-with-mixed-era-rows proof), plus the existing `TestBuildAdoptionReport_Golden` updated for the new predates-instrumentation fields. `go test -race ./internal/mcp/... ./cmd/fathom/...` green; `task validate` green repo-wide.
- [ ] (post-merge) Both benches re-run, unchanged criteria, turns reported; outcome + the binding consequence recorded {id: T5-8, symbol: cmdVerify, package: cmd/fathom, skip_test: true, skip: "measurement row; evidence = committed artifacts + this row's verdict"}
