# Phase D — Expert depth (vision 11 §3.D)

Make the conscience real: the claims machinery's unwired levers.

Design (normative):
- **D1 brief as claims consumer.** After deterministic packing, `brief`
  looks up `fresh` semantic_claims whose claim_links (kind=scopes) match
  the requested paths/symbols and appends them verbatim — citation, tier,
  staleness state included. Never a claim in any non-fresh state without
  its state labeled. Instrumented: claims-block hit rate visible in the
  call log (kill: <5% of brief calls after 30 days → revert wiring, keep
  flag, record why).
- **D2 claim_feedback.** A `feedback` action on the obligations tool (the
  already-hot channel, 764 calls — NOT a new tool): {action:"refute"|
  "corroborate", claim_id, note} → claim_events. ONE automated caller wired:
  the post-compile sweep records refute events for claims whose anchors no
  longer resolve (today it only flips status silently — the event trail is
  what refutation durability needs). Kill: claim_events still 0 rows 30
  days post-wiring → kill the automation, keep the CLI escape hatch.
- **D3 eval-gate enforcement.** `fathom compile` (non-dry-run) refuses any
  layer lacking a passing eval RESULTS record unless
  `--unsafe-skip-eval-gate` is passed; every use of the flag is logged into
  compile_runs. "Passing eval RESULTS record" resolves per pack source:
  embedded packs self-declare via the manifest's `evaled` field (their
  eval/ is deliberately not compiled into the binary — the field is checked
  against `eval/RESULTS.md` at pack-authoring time); a PROJECT pack's
  declaration is additionally verified against its on-disk
  `eval/RESULTS.md` at load time — `evaled: true` with no readable RESULTS
  file loads as not-evaled, with a warning, so a one-line layer.cue lie
  cannot walk through the gate. **Scoping note (shared with A2's
  idempotency guard, docs/plans/activation.md):** the gate lives in
  `cmd/fathom`'s direct `--store` lane only; the HTTP/daemon
  `compile_layer` path (`internal/mcp/semantic_tools.go`) is UNGUARDED
  today — guarding it is deferred until a daemon-side compile consumer
  exists (today's only spending caller is the operator CLI), and
  `--unsafe-skip-eval-gate` without `--store` is refused loudly. Kill:
  flag used ≥3× in normal operation in 60 days → gate ergonomics wrong,
  redesign.

## Tasks

- [x] brief appends fresh scoped claims with citation/tier/state; fake-store test covers fresh, stale-labeled, and no-claims cases {id: D1-1, symbol: registerBrief, package: internal/mcp}
- [x] claims-block hit-rate visible per call-log row (claims_attached count field) {id: D1-2, symbol: registerBrief, package: internal/mcp}
- [x] obligations tool feedback action writing claim_events; contract-suite round-trip for the event write across all store backends {id: D2-1, symbol: ClaimFeedback, package: pkg/client}
- [x] Sweep records refute events (not just status flips) for anchor-dead claims; test proves event trail + refutation durability together {id: D2-2, symbol: SweepClaims, package: pkg/client}
- [x] compile refuses un-evaled layers; --unsafe-skip-eval-gate logged into compile_runs; tests cover refusal, override, and logging {id: D3-1, symbol: cmdCompile, package: cmd/fathom}
