# Feature tuning by a reading

Status: Proposed 2026-11-14 (architect ruling on owner direction).
Decision draft: ADR-110. Work order: `docs/plans/feature-tuning.md`.
This document specifies five tuning cards and no measured improvement.

## 1. Findings

- FT-F1 Maps have a CLI dispatcher, append and compose verbs (`cmd/fathom/mapcmd.go:179`, `:274`, `:408`). Storage adapts thread rows (`cmd/fathom/mapstore.go:133`, `:407`). Expansion deduplicates anchors and resolves freshness (`internal/explmap/expand.go:66`, `internal/explmap/fresh.go:74`). Reuse these paths rather than making another project knowledge store.
- FT-F2 Map display already exists (`cmd/fathom/mapshow.go:117`). Pointer injection, brief selection and extended-surface membership have authored proof rows (`docs/plans/exploration-maps.md#XM-10`, `XM-11`, `XM-12`). Computed descriptions and shared layer freshness remain open rows XM-15 and XM-16. A checked row in that plan is not a new measurement here.
- FT-F3 Semantic packs load through `LoadLayerIn` and union discovered packs (`internal/semantic/packs.go:88`, `:168`, `:273`). Compilation validates drafts in `Compiler.verifyDraft` (`internal/semantic/compile.go:969`). Citation sweep exists (`internal/semantic/sweep.go:96`). Scope-trap precision has an existing bar of 0.90 (`docs/design/semantic-layers.md:242`).
- FT-F4 Obligations already ride edit paths (`cmd/fathom/edithook_pre.go:66`, `cmd/fathom/edithook.go:892`). Layer prose is governed meaning compiled ahead of time, not a query-time oracle (`docs/design/semantic-layers.md:32`). The evidence plane keeps intent evaluation out of hooks and prime (`docs/design/evidence-plane.md:134`).
- FT-F5 Resource reporting and day summaries exist (`cmd/fathom/resources.go:348`, `:1008`). Cache events have an exporter (`cmd/fathom/metrics_cache.go:29`). RT-M1 and RT-M15 remain reading contracts, not assumed health (`docs/plans/responsible-tenant.md#RT-M1`, `RT-M15`).
- FT-F6 Automatic freshness answers from the current index and queues refresh without blocking the read (`docs/guides/fleet-index.md:296`). Index age and stale-answer correctness must therefore be measured separately.
- FT-F7 Knob ownership, select/confirm splits and default-on gates already exist as work orders (`docs/plans/feature-labs.md#LAB-32`, `LAB-36`, `docs/plans/realignment.md#TUN-1`, `TUN-2`, `TUN-3`). BN-D8 forbids firing as the objective (`docs/design/proof-program.md#BN-D8`). This design supplies feature-specific inputs to that program.

Intent: I7 requires honest claim freshness, I12 bounds priming, I35 bounds obligations, I53 bounds hooks and I62 keeps models off code-fact verdicts (`docs/intent/invariants.md`). No live-map or pack-map promotion is asserted. F50 and F64 remain conditional, and F65 forbids a committed generated repository map (`docs/intent/futures/f45-f53.md:53`, `docs/intent/futures/f62-f67.md:27`, `:37`). No new scheduler, chat product or map storage is proposed.

## 2. Design decisions

### FT-D1 Five cards, one existing tuning program

Create one feature-purpose card each for maps, semantic layers, freshness, resource envelope and obligations at the edit. Extend `schema/labs` with a `#TuningCard` referencing the existing `LoadLabs` card ids and quality instruments, not a second list of owned surfaces (`schema/labs/schema.cue:9`, `docs/plans/feature-labs.md#LAB-2`, `LAB-3`). Store these five records in `calibration/labs.d/tuning/` and load them into the existing adoption report. Maps references lab-subagent-brief and lab-session-brief, while semantic layers and edit obligations reference lab-obligations (`calibration/labs.d/lab-subagent-brief.yaml:4`, `calibration/labs.d/lab-session-brief.yaml:4`, `calibration/labs.d/lab-obligations.yaml:4`). Freshness and resources reference their quality instruments and declare firing inapplicable. The surface closure owner stays unchanged.

Every card names purpose metric, measured population, eligibility source, a with/without-capability comparison, knob locations, immutable inputs, $0 reading, registered bar, kill condition and default state. Each result joins the WPF conformance receipt. A zero denominator, missing baseline or incomplete transcript is unmeasured, not a win. Cards emit PASS, FAIL or INCONCLUSIVE and preserve each component measure.

Bars below are proposed acceptance policy unless an existing row is cited. They are not observed numbers. Register task counts, minimum effect, uncertainty method and time window before collection. No knob changes on a count of firing. A confirmed null rejects the proposed change, not necessarily the whole feature. A feature's existing kill remains with its existing card.

### FT-D2 Maps and the orientation brief

Purpose: avoid deriving the same anchored answer twice while preserving correct work. Measure:

- Repeat-exploration rate: later-session read/search actions whose question is already answered by a prior stored map, divided by all later-session read/search actions in the registered project window. Freeze the earlier map at that session boundary and validate its anchors against the later subject revision. Count stale, ambiguous and unclassifiable candidates separately. A repeated file read alone is not repeated derivation.
- Tokens to the first correct edit: input plus output tokens through the first edit that passes the preregistered outside test or accepted fix oracle. Sessions with no correct edit remain censored failures, with their spent tokens reported. Never select only successful sessions.
- Coverage: oracle orientation questions answerable by anchored map entries divided by all registered questions. Also report anchors resolved over anchors attempted, so a large map cannot inflate useful coverage.
- Freshness: fresh, stale, unresolved and unknown map anchors at delivery, plus elapsed age. A fresh map header does not make every entry fresh.

Knobs: pointer versus expansion, entry cap, selection by task overlap, recent-use horizon, composition depth within the existing limit, primer share and map injection on/off. Do not change default-surface membership. XM-15 and XM-16 own computed descriptions and shared freshness. INB-1 owns sibling read-map delivery. FTU measures them rather than cloning that work.

The orientation brief is a request-time deterministic projection over existing stored maps and facts. It answers entry points, known routes, declared ownership and governing rules. Every sentence carries a typed source anchor and subject revision. Render source paths relative to the repository. Unknown ownership is printed unknown, not guessed from the last editor. Label declared ownership separately from code facts and preserve the weakest input tier. Routes retain their own evidence (`docs/guides/fleet-index.md:248`).

Use fixed templates, stable ordering and a bounded payload inside the caller's current budget. Missing anchors suppress the assertion and leave an explicit unknown line. Overflow names the omitted count and recovery pointer. Keep I12's whole-primer ceiling, and do not expand the default primer without a winning reading. The full brief is optional and on request. Provider `none` emits the identical brief. No model summarizes it, writes a map, infers intent or evaluates a lifecycle condition.

Reading: a pinned orientation question set and answers written independently of fathom, including questions whose answer is unknown. Compare the brief with a cold start and a maps-without-brief arm. First run deterministic anchor matching over public fixtures and existing recorded sessions at $0. Report accuracy, answer coverage and bytes/tokens to the first correct answer. Live agent tokens to correct edit stay unmeasured until transcript evidence and separately authorized outcomes exist.

Bar: every asserted sentence resolves to its anchor, no precision loss versus cold start, and repeat exploration falls on the registered development sample. A candidate may advance only if its token estimate is lower and no correctness component regresses. Kill the brief variant on a false asserted fact, no exploration reduction, or a budget breach. A $0 template reading alone cannot establish live outcome benefit or change defaults.

This uses current stored maps. Saved-query maps, governing-rationale joins and instruction-file replacement remain with F50, XM-17 and XM-20/XM-M4. It does not promote pack-carried maps (F64) or generate a committed static map (F65). The brief can consume live-map results later through the same anchor interface without building them now.

### FT-D3 Semantic layer card

Purpose metric: scoped-answer precision against independently labelled source-and-scope pairs. Report recall, unknown residue, eligible use rate and authoring cost as separate components. Authoring cost includes operator minutes, input bytes, edits to fixtures, compiler usage already recorded and accepted claims. Sunk compilation spend is labelled historical. No paid recompile occurs in the $0 reading.

Knobs: pack selection, corpus scope, existing section/digest boundaries, raw fallback, payload order within evidence tier and serving caps. Knobs cannot remove citations, widen intent into the obligations corpus, promote evidence tiers or auto-accept drafted checks (I27, I62 and `docs/design/evidence-plane.md:107`). A model-authored fixture is not an outside oracle.

Reading: reuse compile scope traps with frozen compiler responses and independent labels, plus a pack-authoring task timed by the operator. Keep compile/serve precision separate from W4 use. Reuse the layer freshness sweep and XM-16's shared check, not a second freshness algorithm.

Bar: scoped precision at least the existing 0.90 gate, no unsupported anchor, and no increase in recorded authoring cost at matched coverage. Use rate is reported only for validated eligibility. Kill the candidate on a scope escape, precision below the bar or added authoring cost without coverage gain. Query-time verdicts remain deterministic.

### FT-D4 Freshness and resource cards

These are separate cards because faster refresh can consume more resources.

| Card | Purpose and knobs | $0 reading | Bar and kill |
|---|---|---|---|
| Freshness | Index age at answer, stale-answer rate and refresh latency. Knobs are existing refresh coalescing, backoff and freshness disclosure. | Move a public fixture HEAD, issue an anchored query, then observe queue acceptance and completion under an injected clock. Compare every assertion with HEAD through a compiler or fixture truth, not another fathom answer. | Zero undisclosed HEAD contradictions. Meet the preregistered refresh-latency bound without violating resource bars. Kill a change that masks a stale answer or increases refresh backlog at matched load. |
| Resource envelope | Peak daemon footprint, CPU seconds per completed ingest and hook p95, with input size and graph stage. Knobs are current queue concurrency, cache/admission settings and payload limits. | Reuse resource/day logs, metrics and the tenant harness. Isolate one ingest window for CPU accounting. Do not call process-wide cumulative CPU an ingest measurement. | Meet the applicable RT-M1, RT-M4, RT-M8 and RT-M15 clauses and the declared hook budget. Kill the candidate on an exit rung, a budget breach or an unpriced resource regression. |

Freshness denominator is answer assertions with independently checkable HEAD truth. Report stale-but-disclosed and undisclosed contradictions separately, along with uncheckable assertions. Index age is answer time minus last successful ingest time. Refresh latency runs from the admitted subject-change event to serving that revision, with coalesced, rejected and unfinished requests disclosed. Never confuse index freshness, semantic citation drift and installed-binary freshness.

Use OS footprint on macOS and the proven OS resident gauge on Linux/WSL, named separately, not interchangeable heap numbers (`docs/guides/fleet-index.md:354`). CPU seconds per ingest is unknown until a correlated per-job input exists. The first reading can be INCONCLUSIVE and name that gap. Do not add a daemon sampler or table to fill it.

RT-M1's seven complete UTC days cannot be replaced with a short fixture window. Retain its own verdict and day coverage. RT-M15 remains the graph-rewarm readiness gate. The hook deadline is descriptor timeout minus max(1 second, 20 percent), with no retries after timeout (I53). Record hook p95 per harness/channel alongside the existing tail measures. There is no single invented fleet-wide latency number.

### FT-D5 Obligations at the edit and reading-only knob changes

Purpose: the correct scoped rule reaches an eligible edit and leads to an oracle-correct action. Knobs are existing ordering within tier, scope selection and payload budget. Preserve zero output on a clean file, the rule/word caps and disclosed overflow in I35. A candidate cannot increase exposure by relabelling clean edits eligible.

Reading: use BEN-4 holdouts and AD-20/AD-21 eligibility/compliance evidence through LAB-11. Run frozen-response replay at $0 for exact target, deadline, scope and budget correctness, and report forward delivered/acted-on lift separately. Bar: no wrong-scope rule, no cap regression and positive acted-on lift against held-out delivery at the registered uncertainty bound. Kill the variant on wrong-scope advice, budget breach or null/harm after the registered forward window. Fired counts never meet this bar.

All five cards use BN-D8's development-first selection. LAB-36 owns the knob decision record, TUN-1 owns typed knob data, TUN-3 owns held-out confirmation and TUN-2 owns default-on. At most the two profiles selected under WP-D4 proceed to paid outcomes. A tuning reading records a keep, revert or inconclusive decision even when it changes no knob.

## 3. Risks

- FT-R1 Repeated exploration can be legitimate validation. Label answer-equivalent actions independently, audit near misses and report unknown classifications.
- FT-R2 Orientation questions can reward enumeration by construction. Follow BN-D18's independent task questions and native-tier oracles, including unknowns, and publish failed questions.
- FT-R3 Stored maps may be stale precisely when reused. Score answer-time anchors and count unresolved entries instead of excluding them from coverage.
- FT-R4 Resource improvement can be shifted to another process or a later window. Include children and complete refresh windows under the current tenant clauses.
- FT-R5 Eligibility or token telemetry may be absent on a harness. Print unmeasured with coverage, retain WPF invalidation and make no savings claim from byte estimates.

## 4. Sequence

1. Reconcile the five cards with seed cards, freeze the independent question/label sets and register the reading bars.
2. Add deterministic map metrics and the optional anchored brief over existing inputs. Run its $0 anchor/coverage reading.
3. Run semantic, freshness, resource and obligations readings through existing instruments after their WPF requirements are met.
4. Record knob decisions through LAB-36 and BN-D8. Defaults and paid outcomes wait on their existing gates.

## 5. Decision models

No new judged site applies. Feature metrics, orientation sentences and keep/revert checks derive from anchors, independent labels and preregistered arithmetic. Provider `none` leaves all five readings available. There is no calibration band for a deterministic verdict. Operators opt out by keeping the experimental profile off. A model may author a layer only through its existing compiler and acceptance gates, outside these $0 readings. It never supplies a missing owner, resolves a stale anchor or grades a primary metric.
