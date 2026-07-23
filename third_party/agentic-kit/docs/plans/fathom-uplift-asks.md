# Fathom uplift asks — kit seams surfaced by Phase K (2026-07-22)

**Mission** — Phase K (fathom vision 11 §3.K, "kit uplift") requires fathom to
be a **consumer, not the kit**: when Phase K's own work surfaces a kit-shaped
seam, the discipline is to file the ask and stop, not fork the code into
fathom. Two candidates surfaced while executing K1 (the `internal/vector` →
`vectorstore` donor deletion, this repo's `handoff-h4-sextant.md` Stage 4) and
K4 (auditing fathom's non-adoption of `kits/eval-runner`/`foundations/quality/qa-harness`):
fathom carries two evidence-agnostic *kernels* — a proof-gate aggregator
(`internal/verify`) and an extraction-accuracy harness (`calibration/`) — whose
typing and rollup/scoring logic is generic, but each currently has exactly
**one** consumer (fathom itself). Per D8 discipline, a single consumer does
not justify a kit package: **both rows below park with a named-consumer
trigger. Neither is built in this pass.**

**Write scope** — this repo's `docs/plans/` only (this file). No kit Go/CUE
code is added by Phase K; if either row's trigger fires, that is a future
plan's scope, not this one's.

## Parked — subscribe-when triggers (D8: no consumer, no code)

- [-] **Proof-gate kernel** — generalize fathom's `internal/verify` gate-aggregation shape: `Task{ID, Title, SpecIDs, Expect}` / `Criterion{Name, Status, Detail}` with a `CriterionStatus` enum (`pass|fail|unknown|n/a`) and a `Disposition` concept for the parked/deferred row states that today only live as unparsed prose in Markdown annotation blocks (`{disposition: deferred, reason: ..., trigger: ...}` — see this repo's own `2026-07-fathom-asks.md` rows, or fathom's `tmp/TRACKING.md`); plus the deterministic rollup `VerifyPlan(plan, ev Evidence) Report{Tasks []TaskVerdict, Summary{Total,Pass,Fail,Unknown}}` (`internal/verify/verify.go:29-143`). The kernel would ship the typing and the pure aggregation algorithm ONLY — fathom's `Evidence` interface (`FindSymbol`/`TestReaches`/`Reachable`/`Contradictions`/`SpecImplementors`, all index-backed) stays in fathom; the kit package would never gain an evidence implementation, exactly as `evidenceladder.Tier1`'s doc comment already draws that line for the ladder side. Motivating observation: this is the same shape as `foundations/quality/eval-gate`'s pass/fail rollup but keyed on static code evidence instead of judge verdicts — two structurally similar gates with zero shared typing today. {id: FA-11, disposition: deferred, reason: "fathom internal/verify is the kernel's only consumer; no second repo has a Task/Criterion/Disposition-shaped gate today", trigger: "a second named consumer"}
- [-] **Eval-harness kernel** — generalize the SHAPE of fathom's `calibration/` extractor-accuracy harness (`calibration/baseline.sh`, `calibration/corpus.yaml`): a golden/proxy-oracle corpus manifest (pinned source + expected counts) driven against a caller-supplied pipeline, producing a recall/precision report per category, with the proxy-vs-real-oracle distinction (`calibration/README.md`'s "not ground truth yet" honesty note) kept explicit in the type. This is a deliberately DIFFERENT shape from `kits/eval-runner`/`foundations/quality/qa-harness` (LLM-judge-scored agent-scenario evaluation, budget-guarded, cassette-replayed) — fathom's harness scores mechanical extraction recall against a static corpus, never an LLM judge call. Today it is a bash script, not a Go package, and has exactly one caller (`task calibrate*` in fathom's own Taskfile); no kernel exists to extract yet. {id: FA-12, disposition: deferred, reason: "fathom's calibration/ harness is bash, not a Go package, and has one consumer (fathom's own extractor calibration); no second repo runs a golden-corpus recall harness today", trigger: "a second named consumer"}

## Design constraints

- **D8 holds**: neither row gets typed, ships, or gets a package skeleton in
  this pass — filing the row IS the deliverable (per Phase K's K2 task: "file
  asks, don't build").
- If FA-11's trigger fires: the kernel package must stay evidence-agnostic —
  no `Evidence` implementation, no fathom-specific `core.SymbolNode`/tier
  vocabulary — mirroring the `evidenceladder.Tier1` precedent (kit ships the
  contract, the index-backed adapter stays downstream).
- If FA-12's trigger fires: the kernel must keep the proxy-oracle honesty
  boundary a first-class type distinction (a report must be able to say "this
  number came from a proxy, not ground truth"), not silently upgrade a grep
  proxy into a claimed oracle.
- Both rows require a **second named consumer**, not merely a second
  hypothetical use case — per this repo's own D8 doctrine, "duplication must
  demonstrably hurt first" (see FA-10's identical trigger discipline).
