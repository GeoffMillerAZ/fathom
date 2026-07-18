# Throughline v0.2 adoption — friction log

Recorded during the phase-by-phase adoption (2026-06-10, Throughline commits
`9a19919`…phase-6). Per the brief: where a kit API didn't fit cleanly we did
NOT silently work around it — each item below is a candidate kit improvement,
ordered by how much consumer code it would delete.

## 1. No deterministic-token path in `llmegress.GateFromVault` (Phase 2)

`GateFromVault` mints a fresh random token per match through the
`tokenization.Vault` interface. Two consequences for an evidence product:

- The same email address appearing in two different calls (or twice in one
  prompt across separate rule matches) gets two different tokens, so the
  model cannot correlate "tok_pii_aaa… mentioned Monday" with the same
  person on Tuesday. Throughline's doc 21 §3 ("stable references") wants
  value-stable tokens per namespace.
- Cross-call prompt reproducibility (PromptHash over the gated text) is
  unattainable when the substitution itself is random.

The vault already supports HMAC-deterministic minting
(`Tokenizer.MintDeterministic`, keyed). **Ask:** a `GateFromVault` option to
prefer deterministic minting (per-namespace HMAC), e.g.
`WithDeterministicTokens()`. Until then Throughline accepts per-call token
instability on the LLM path.

## 2. `GateFromVault` cannot block a class outright (Phase 2)

The old Throughline gate had `denyClasses` — privileged content classes were
refused egress entirely, not tokenized. The kit gate can only substitute.
Throughline's deepgram transcriber kept the legacy `governance.EgressGate`
partly for this blocking semantic. **Ask:** a `VaultRule.Action` (tokenize |
block) or a parallel `DenyRule{Pattern, Class}` that fails the Transformer —
fail-closed refusal is already the middleware's error contract.

## 3. No budget middleware in `llmclient/instrument` (Phase 4)

`WithCost` exists; nothing consults `governance/budget-guard` on the call
path. Every consumer will hand-roll the same ~40-line wrapper (Throughline:
`adapters/llmclient.budgetClient` — pre-flight estimate, `Charge`, typed
refusal error, Complete+Stream). **Ask:**
`llminstrument.WithBudget(inner, guard, table, estimator)` returning a typed
`BudgetExceededError`, composed outermost in the documented stack
(budget → cost → telemetry → egress → retry).

## 4. `budget-guard.Charge` commits the estimate with no settlement (Phase 4)

`Charge(estUSD)` is pre-flight by design (fail-closed before spend), but the
committed amount is the ESTIMATE: actual usage comes back later and there is
no `Adjust(delta)`/`Settle(actualUSD)`, so the USD dimension drifts by the
estimate error all day (conservative estimates over-throttle; optimistic
ones under-cap). **Ask:** a two-phase `Charge → Settle` or a signed
`Adjust(deltaUSD)` so the day window converges on provider ground truth.

## 5. Per-call nonce control in `untrusted-content-wrapper` (Phase 5)

`randHex` is injected at `NewWrapper` construction, but Throughline needs a
*content-derived* nonce (same context → same fences → reproducible
PromptHash, while still preventing close-fence embedding — embedding changes
the hash, no fixpoint). That forces constructing a throwaway `Wrapper` per
wrap just to close over the content hash. **Ask:** `Wrapper.WrapWithNonce
(content, nonce string)` (validated by the existing `checkNonce`), or
document the per-wrap-Wrapper pattern as intended.

## 6. `prompttemplate.Render` params are `map[string]any` only (Phase 3, minor)

Mapping a struct (`chat.CaseContext`) into the params map by hand creates a
silent drift risk between template keys and struct fields (missingkey=error
catches absent keys but not orphaned struct fields). **Ask (nice-to-have):**
a `ParamsFromStruct(v any)` helper honouring `json:` tags, sharing the
canonical-JSON hashing path.

## Non-friction notes (things that fit exactly)

- `evidence-citation` Parse/Validate dropped in with zero adaptation; the
  `known func(tag, id)` resolver seam is exactly the anti-fabrication shape
  Throughline needed.
- `WithEgress` outside `WithRetry` (once-per-logical-call) composed cleanly
  with a shared `Swappable` under per-case variants — the documented
  composition held up under a configuration the docs never anticipated
  (N case-scoped stacks over one hot-swap point).
- `injection-detector.DefaultRules` + fencing wired in under an hour,
  including the review-signal (no-block) policy.

---

# Addendum — doc-35 acquisition track (2026-06-10, same day)

Contribution made: `MethodDKIM` + `MethodAuthResultsHeader` added to
`evidence/authenticity` (cf40791) the moment Throughline's DKIM detector
needed them — the upgrade path works same-day.

## Extraction candidates (build when touched next; shapes now proven by use)

1. **`intake-triage` foundation.** Throughline's `app/acquire` grew a generic
   FSM the kit lacks: search → proposal (observation of remote state, full
   provenance) → human-only decision (`proposed → imported | excluded(reason)
   | deferred`, reversible exclusion, AI may only suggest). The human-only
   enforcement + reversible-exclusion semantics are pure logic and belong in
   `foundations/review/` next to review-queue. Extract on second consumer or
   next substantive touch.
2. **`query-planner` investigation primitive.** Brief + known entities +
   date ranges → provider-agnostic query plans with per-query provenance
   (deterministic expansion + optional labeled LLM refinement + detect-only
   DLP warnings). Currently Throughline-local (`app/acquire/planner.go`).
3. **OAuth token-lifecycle core.** Expiry/refresh/revoke state machine is
   pure (clock-injected); only the HTTP exchanges are adapter work.
   Throughline's `adapters/oauth` carries both halves today.
4. **Authenticity grading guidance.** We graded receiver-asserted
   Authentication-Results (0.55/likely_authentic ceiling) far below
   independently verified DKIM (0.95) — worth encoding as documented
   guidance or named constants in `evidence/authenticity` so consumers
   don't re-derive the asserted-vs-verified distinction.
5. **`source-sync-checkpoint` adoption question.** Throughline persists
   cursors via its coverage projection instead of this foundation (deviation
   recorded in its doc 29). Either give the foundation a compelling delta
   (multi-consumer checkpoint semantics) or fold its intent into `coverage`.

---

## Status addendum (H2 close-out, 2026-07-15)

All five doc-35 acquisition-addendum items are CLOSED (pull queue emptied —
see TRACKING.md D35-1..D35-5 for the per-row detail): intake-triage and
query-planner donors deleted from throughline (`app/acquire`), the OAuth
token-lifecycle core extracted to `foundations/security/token-lifecycle` and
the donor rewired (`adapters/oauth`), authenticity grading ceilings encoded
(`ScoreCeiling`/`VerdictCeiling`/`ClampToCeiling`), and the checkpoint
question decided: coverage wins, `source-sync-checkpoint` retired.
Extraction-done = donor-deleted, verified by `task downstream-check`.
