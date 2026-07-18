# Specs — minimal-theatre, docs-as-spec

The **design docs are the spec.** They live in
`tmp/design_inspiration/files/` (reconstructed from the original scans). This
file adds only the thin layer the docs don't give us: a crisp, *testable*
acceptance contract per component being implemented now, so each one has a
clear "done" bar that maps to tests in `TRACKING.md`.

Rules:
- Behaviour is driven by the linked design doc; this file lists the checks.
- A component is `[x]` in TRACKING only when its acceptance checks have tests and the gate (`task validate`) is green.
- No org-specific names or catalogs — configurable only.

Verdicts (2026-07 honesty pass):
- **REAL** — code satisfies the spec; enforcing tests named inline.
- **PARKED** — intent lives in docs/roadmap.md; original spec text below is informational only.

---

## contracts  ← `foundations-overview.md`, `09-cue-explainer.md`
<!-- VERDICT: REAL. Enforced by foundations/contracts/contracts_test.go:
     TestEventShape_JSONRoundTrip, TestNodeType_JSONRoundTrip,
     TestAnswerArchetype_JSONRoundTrip, TestKnowledgeCardArchetype_*,
     TestEventShape_Validate_invalid, and cue vet in the gate. -->
Canonical CUE-first types other packages import without taking Go deps.

**API:** `EventShape`, `NodeType`, `AnswerArchetype`, `KnowledgeCardArchetype` Go types mirroring `schema.cue`.

**Acceptance:**
- [x] `schema.cue` compiles and `cue vet` passes with `testdata/*.cue` examples.
- [x] Each Go type round-trips JSON and matches the CUE field set.
- [x] An invalid event shape (missing required field / bad enum) is rejected by CUE.
- [x] `Validate()` on each type enforces the same constraints CUE encodes.

## llmclient  ← `14-llm-client-ladder.md`
<!-- VERDICT: REAL. Enforced by foundations/llmclient/llmclient_test.go
     (TestLadder_ClimbAdvancesOnEscalate, TestWithRetry_*) and
     foundations/llmclient/mock.go used throughout all kit tests. -->
Single LLM interface + the ladder escalation pattern. Behavioural; no `schema.cue`.

**API:** `LLMClient{Complete,Stream}`, `Ladder{Climb(Signal) (Rung,bool)}`, `Rung`, `RetryConfig`, `WithRetry(LLMClient, RetryConfig)`.

**Acceptance:**
- [x] Ladder starts at rung 0 and `Climb` advances only on an escalation signal; stops at the top rung.
- [x] `WithRetry` retries retryable errors (rate-limit) with backoff+jitter and fails fast on non-retryable (invalid request).
- [x] Attempt count is surfaced to the caller.
- [x] A deterministic `mock` client supports tests without network.

## review/correction-ledger  ← `15-correction-ledger-fsm.md`
<!-- VERDICT: PARKED. The implemented package is foundations/review/claim-lifecycle,
     whose FSM (proposed→supported|refuted|uncertain|recanted) does not match the
     spec's states (open→reviewing→corrected|dismissed|deferred). The spec captured
     a design that evolved before the package was written. Intent parked to
     docs/roadmap.md under "correction-ledger FSM alignment". -->
Append-only finding ledger with an enforced FSM.

**Parked intent:** A finding-oriented FSM (open→reviewing→corrected|dismissed|deferred)
and a `Replay`-based projection may be added once there is a consumer that needs
it (the claim-lifecycle package covers the current consumer's needs).

## security/injection-detector  ← `foundations-overview.md` (security), `05-anti-patterns.md`
<!-- VERDICT: REAL. Enforced by foundations/security/injection-detector/injectiondetector_test.go,
     recall_test.go, and fuzz_test.go. Schema validated by cue vet gate. -->
Detects prompt-injection / instruction-override attempts in untrusted content.

**API:** `Detector.Scan(content) Result{Flagged bool, Matches []Match, Severity}`; rule set configurable via `schema.cue`.

**Acceptance:**
- [x] Known injection patterns ("ignore previous instructions", tool-exfiltration, role override) are flagged.
- [x] Benign content is not flagged (low false-positive on a fixture corpus).
- [x] Rules/severity are data (`schema.cue` + testdata), not hard-coded org specifics.
- [x] Severity ranking is deterministic and tested.

## evidence/evidence-graph  ← `16-evidence-graph-and-federation.md`
<!-- VERDICT: REAL. Enforced by foundations/evidence/evidence-graph/evidencegraph_test.go:
     TestAddAndLink, TestNeighborsExcludesExpired, TestSweepEvictsExpiredSpans,
     TestContradictionsDetected. Schema.cue + testdata present; cue vet in gate. -->
In-memory evidence graph with TTL and contradiction detection.

**API:** `Graph.Add(span)`, `Graph.Neighbors(id)`, `Graph.Contradictions()`, TTL expiry sweep.

**Acceptance:**
- [x] Adding spans + relations builds a queryable graph; `Neighbors` returns linked evidence.
- [x] Expired (TTL) spans are swept and no longer returned.
- [x] Two spans asserting incompatible values on the same entity are reported as a contradiction.
- [x] `schema.cue` validates span/edge shapes; `cue vet` green.
