# Future directions

Things that might happen. NOT requirements; nothing here authorizes building anything.
An entry exists so today's design does not wall off a plausible tomorrow when keeping the
door open is free, and so agents stop re-proposing what has been considered.

Confidence: **likely** (a commitment or sponsor statement exists) · **plausible** (fits
the vision, no commitment) · **speculative** (must not influence design; listed only to
stop re-proposal). Horizon: **next** · **later** · **someday**. Only likely and plausible
futures may justify a free option today. The repo's own sequence is
`docs/design/product-vision.md` §4 (H1 → H2 → H3) and `docs/vision/08-roadmap.md`.

## F1. CGO-free binary: the nine CGO tree-sitter languages move to the wazero engine (XT-6b)

- Confidence: likely. Horizon: later.
- Motivation: cross-compiled single-binary releases, no C toolchain at build time.
  Evidence: ADR-025 "Consequence", `Taskfile.yml` header, `docs/vision/11` A5 ("XT-6b not yet fired").
- Do not build: a migration ahead of per-language calibration; a second grammar build chain.
- Preserve: every new language lands on `internal/extractor/tsengine` (query-driven `.scm`),
  never on the CGO bindings; `internal/store` stays pure Go.
- Promote when: a language's wazero recall on the calibration corpus meets the CGO
  incumbent's, for each of the nine, or a release must be cross-compiled.
- Drop if: the wasm32-wasi tree-sitter core stops building or a language cannot reach
  parity on the corpus.

## F2. Public release (H2): public repos, tagged binaries, brew tap, MCP registry, plugin bundle

- Confidence: plausible. Horizon: later.
- Motivation: "the release is the boundary event"; the falsifiable "best" claim is the
  published context-bench. Evidence: `docs/design/product-vision.md` §4 H2.
- Do not build: brew tap, registry listings, marketing, a hosted anything.
- Preserve: `fathom init` works on a cold clone; the in-tree kit mirror (ADR-019) keeps a
  kit-less build possible; no org-specific content (ADR-011).
- Promote when: the H1 exit criteria are green (instrument valid ≥4 weeks; injection ≥90%;
  verify gate live in ≥2 repos ≥4 weeks; surface pruned; portability passing; context-bench win).
- Drop if: the owner decides fathom stays a personal force-multiplier (`docs/vision/07` Q4).

## F3. Federation over portable stores (H3)

- Confidence: plausible. Horizon: someday.
- Motivation: one module = one store; orchestration composes above the engine.
  Evidence: `docs/design/product-vision.md` §4 H3, ADR-016 "federation composes above the engine".
- Do not build: a multi-tenant engine; the DuckDB `ATTACH` mechanism as written (engine gone; needs a SQLite re-check).
- Preserve: born-portable stores (ADR-017); `pkg/platform` as the join point.
- Promote when: ≥2 real external module consumers, or ≥100 external installs plus ≥60%
  internal fathom-share sustained four weeks.
- Drop if: no external module consumer appears after F2.

## F4. Layer 2: the failure-mode playbook (incident precedent)

- Confidence: plausible. Horizon: later.
- Motivation: incident investigation with precedent, compiled from a non-git folder corpus.
  Evidence: `docs/design/product-vision.md` §3–4; `fathom findings status` tracks the entry condition.
- Do not build: the pack, the prompt, or a replay harness before the corpus exists.
- Preserve: findings persist as `kind:"finding"` claims; non-git watermarks (ADR-017).
- Promote when: the obligations gate has passed AND ≥15–20 resolved findings exist.
- Drop if: two replay attempts put the true root-cause class in the top-3 for <70% of a 20-incident set.
- Trigger status (2026-09-24, CV-M10 / docs/plans/refactor-conservation.md): still not fired,
  but half the condition cleared. The findings eval gate now passes (`docs/layers.md`'s
  `findings` row — `TestFindingsEval_Precision`: precision 1.00 / recall 1.00, 2 consecutive
  production-model runs, `schema/semantic/layers/findings/eval/RESULTS.md`); the ≥15–20
  resolved-finding half is unmeasured here and, on a fixture-scale corpus, presumed well
  below the bar. The resolved-finding count is BuildStatus's own tally across every
  finding-kind claim regardless of origin, so an imported review-kit finding (once `fathom
  findings import` lands, FL-10) counts toward the same ≥15–20 threshold this future is
  watching — the findings loop widens the funnel, it does not promote anything by itself.

## F5. Provider pack type (TX-3)

- Confidence: speculative. Horizon: someday.
- Motivation: explain an external technology's resource semantics; the first corpus that
  drifts against an upstream. Evidence: ADR-024.
- Do not build: the pack type, the upstream watermark kind.
- Preserve: `stratum` is already a `#Layer` field.
- Promote when: a named consumer plus one concrete provider an indexed service needs.
- Drop if: platform modules (ADR-024 amendment) cover the need.

## F6. Concern-pack library growth (TX-4): review packs, compliance packs

- Confidence: plausible. Horizon: later.
- Motivation: one substrate, N data-defined packs. Evidence: ADR-024, `docs/vision/02-shape.md`.
- Do not build: any pack without its consumer.
- Preserve: the admission case names a real corpus, eval fixtures, and a kill criterion.
- Promote when: per pack, a real consumer with a real corpus and eval fixtures.
- Drop if: a pack sits at 0 production claims 30 days after its compile (`docs/vision/11` A4).

## F7. The `ai-derived` evidence tier

- Confidence: speculative. Horizon: someday.
- Motivation: the first non-citable claim kind. Evidence: ADR-016.
- Do not build: the tier. Preserve: `tier_profile` column (ADR-017); tiers declared in CUE.
- Promote when: a non-citable claim kind is proposed with its own eval. Drop if: never needed.

## F8. Hosted read-only expert host (law 9)

- Confidence: plausible. Horizon: later.
- Motivation: hardened `serve-http` with auth and a read-only mode is the hosted surface,
  not a new server. Evidence: `docs/vision/12` §3, `docs/plans/gke-module-host.md`.
- Do not build: a writable hosted surface, tenancy, billing.
- Preserve: `Server.strictSurfaces` (ADR-014 amendment) as the hard gate for that shape.
- Promote when: the auth + read-only rows are proven and a first internal consumer session is stamped (law 6).
- Drop if: no external request in writing (`docs/vision/07` R1).

## F9. Machine-evaluated plan triggers

- Confidence: speculative. Horizon: someday.
- Motivation: `trigger:` on deferred plan rows is recorded, never evaluated, in v0. Evidence: ADR-018 "Rejected".
  The 2026-08-26 design panel checked: plan trigger vocabulary does not repeat across the three
  consumer repos, so this has NOT fired; ADR-026's annotated *intent* conditions are a different
  corpus on a different ground (`docs/plans/evidence-plane.md` EP-31 records the distinction).
- Do not build: a trigger engine. Preserve: `trigger:` stays free text.
- Promote when: the same trigger vocabulary repeats across two repos' plans.
- Drop if: `needs:` eligibility and threads already answer the question.

## F10. CI fails closed (`--strict` with its own store)

- Confidence: likely. Horizon: next.
- Motivation: CI owns determinism; "daemon down" is not a valid CI state.
  Evidence: `docs/design/coding-agent-excellence.md` §5.3, `scripts/ci.sh`.
- Do not build: a CI service or runner config before the claims are reconciled.
- Preserve: `verify --store <tmp>` keeps working; ambient-fathom's claimed-Unknown rows
  are reconciled first (`th-c214dd26`).
- Promote when: `th-c214dd26` is decided and scope G6 is done.
- Drop if: the repo stays without a CI runner by decision.

## F11. Adopt the kit's `mcp-server` under `internal/mcp`

- Confidence: speculative. Horizon: someday. Evidence: ADR-022.
- Do not build: a wrapper layer. Promote when: the kit gains surface/group/prompts/resources support.
- Drop if: never.

## F12. An LLM-judged evaluation lane on `qa-harness`

- Confidence: speculative. Horizon: someday. Evidence: ADR-023, FA-12.
- Promote when: a second consumer needs LLM-enriched artifacts graded. Drop if: never.

## F13. Vectors as an additive in-store column

- Confidence: speculative. Horizon: someday.
- Motivation: hybrid semantic search; "evidence-gated exactly as designed", written for
  DuckDB VSS and never re-checked on SQLite. Evidence: ADR-016, `docs/design/product-vision.md` H3 note.
- Do not build: a second store. Promote when: a query class fails keyword-only on the context bench.
- Drop if: harness-native embeddings make it moot.
- **Superseded 2026-09-19 (ADR-041 decision 2, `docs/vision/31-substrate.md` §3 D5/D7):**
  the live vector adapter (`internal/vector`, `WithVector`) is deleted, not parked — this
  row's old "Promote when" bar is replaced by vision 31 §5's kill criterion (a paired
  concept-query bench beating lexical+graph at ≤ 2× tokens, committed ledger, n ≥ 3);
  "semantic search" in fathom means compiled claims until that bench runs and passes.

## F14. CI/forge integration as its own repo

- Confidence: plausible. Horizon: later. Evidence: `docs/handoff/repo-strategy.md` §2.
- Do not build: the repo. Preserve: the seam is a subcommand that takes a diff, calls
  fathom, posts results; review output stays verb JSON.
- Promote when: the first non-fleet consumer needs it, or its release cadence diverges.
- Drop if: lefthook remains the only delivery.

## F15. A second user or second machine

- Confidence: plausible. Horizon: later.
- Motivation: every instrument assumes one person, one machine, one loopback daemon.
  Evidence: `docs/vision/20` §7 Q1.
- Do not build: tenancy, auth, sync.
- Preserve: service-scoped exclusions; absolute store and call-log defaults (O11/O12 fixes);
  portable stores; ledgers keyed by something other than `$HOME` when touched.
- Promote when: a colleague adopts, as one deliberate hostile test.
- Drop if: single-user is accepted in writing (see `questions.md` Q5).

## F17. A saved-query discovery router

- Confidence: speculative. Horizon: someday. Evidence: declined in `docs/vision/24`.
- Listed to stop re-proposal. Promote when: `fathom prime` and the opening-moves table
  measurably fail to route discovery. Drop if: never asked for again.

## F18. Cross-repo intent federation (one corpus over many repos)

- Confidence: speculative. Horizon: someday. Evidence: refused in `docs/design/evidence-plane.md` §9.
- Do not build: a fleet-wide intent index, cross-repo condition evaluation.
- Preserve: one repo, one router, one corpus; the parser takes a root, never a fleet.
- Promote when: two repos' conditions reference each other's artifacts and a human asks for the join.
- Drop if: never asked for again.

## F19. Anchored-memory export into harness memory formats

- Confidence: plausible. Horizon: later.
- Motivation: a second harness vendor shipped cited, just-in-time re-verified memory
  (2026-01) and Claude Code's auto-memory carries typed, timestamped frontmatter — the
  anchored-memory thesis validated externally. Evidence: `docs/vision/26-proof-credibility.md` §5.
- Do not build: a second memory surface; any write into a harness's memory file that is
  not inside fathom's own markers (law 15).
- Preserve: memories carry anchors and a read-time freshness state (ADR-028's authority
  boundary); the harness descriptor records where a harness keeps memory (`internal/harness`).
- Promote when: an adopter asks for fathom facts inside the harness's memory, or the
  session brief measurably loses to the harness's own recall on the thread bet (G3).
- Drop if: harness memory formats keep churning quarterly (Cursor removed theirs in 2.1).

## F21. A `typesafe` adapter for the decision port (TypeSafe AI's Jev)

**Landed 2026-09-22**, ahead of the promote condition below, on owner direction —
ADR-042's amendment of the same date; `[decide] provider = "typesafe"`,
`docs/configuration.md` §"The decision port". The A/B this row originally gated
promotion on still applies before `typesafe` becomes any repo's *default*
provider (vision 32 §4) — kept below as the still-live bar for that, not for
existence.

**This row's own drop-if fired for the `route` site, 2026-09-24** (CV-M11,
`docs/bench/readings/decision-port-calibration-2026-09-24.md`, ADR-042
amendment): joined against 82 adjudicated route decisions, the vendor's
calibration was right only 45.7% of the time at confidence ≥ 0.9 — the
"0.9 not right nine times in ten" condition below, triggered. Scoped to
`route` alone: `certify_suggest`'s own spot-check stayed 5/5, and this is not
a drop of the adapter or of `typesafe` generally. The response is a downgrade,
not a removal — `route_confidence` stays at 0.9 (raising it would not satisfy
a non-monotonic miscalibration) and the prompt hook's route line is now
always hedged, never firm, at any confidence (ADR-042's 2026-09-24 wave-2
amendment). Revisit `route`'s own use specifically when a routing change
passes the calibration check on a new sample; the row otherwise stands.

- Confidence: plausible. Horizon: when access exists. Evidence: ADR-042, `docs/vision/32-decision-port.md`, the 2026-09-22 assessment.
- Promote when: an API key exists AND one site's A/B (compile pre-filter, obligations rank, or routing) shows a gain over the `llm` adapter at n ≥ 3. Drop if: the vendor's calibration fails our own adjudicated cells (0.9 not right nine times in ten) or the service is withdrawn.
- Preserve: the port's three kinds mirror the vendor primitives; an adapter is an HTTP client, never a change to a site.

## F20. Spec-scaffold interop: scaffold task checklists as plan rows and a converge report

- Confidence: plausible. Horizon: later.
- Motivation: the spec-driven scaffolds (Spec Kit, OpenSpec, BMAD, Kiro) emit task
  checklists whose "verify" step is LLM-judged; fathom's gate is the only deterministic
  one. The inverse of claimed-unproven — code nothing claims — is what their converge
  step does by LLM. Evidence: `docs/vision/26-proof-credibility.md` §5, E1 F7.
- Do not build: a spec authoring workflow; a separate SDD binary (non-goal).
- Preserve: plan grammar v2 stays markdown-authored; `ParseMarkdownPlan` is the seam a
  second checklist dialect would plug into.
- Promote when: a wired repo uses one of the scaffolds and asks for its checklist gated.
- Drop if: the scaffolds converge on a checklist format fathom already parses.

## F22. A cross-language port ledger

- Confidence: plausible. Horizon: when a port happens.
- Motivation: when a team moves a service to another language for engineering merit
  (vision 34 §5), the language-neutral inventory keys (`route:`, `rpc:`, `env:`, `cli:`)
  make the port auditable: complete when the source inventory is closed in the target and
  behaviour certificates are live against both. Evidence: ADR-044, plan row CV-50.
- Do not build: symbol-to-symbol mapping between languages; a translator.
- Preserve: inventory keys stay language-neutral; behaviour certificates resolve to keys.
- Promote when: a fleet service ports to another language. Drop if: ports stop happening.

## F23. Decision-port roles in refactoring: frontier ordering and same-concept questions

- Confidence: plausible. Horizon: after the ledger ships.
- Motivation: ordering attention over hundreds of sites after the deterministic type-check
  split, and a same-concept probability for renames where typed edges are missing, are the
  two places a calibrated fast model could help (ADR-044 decision 8). Evidence: plan rows
  CV-51, CV-52; the one ranking use at the edit measured negative (O113).
- Do not build: a model deciding a move, a strength, scope or safety.
- Preserve: every site is an explicit `[decide] sites` opt-in; calibration check first.
- Promote when: a frontier over 100 sites exists with an A/B corpus (ordering), or file-tier
  precision is published and a JavaScript or Python rename corpus exists (same-concept).
- Drop if: either A/B is null or negative against the deterministic baseline.

## F24. Architectural decisions compiled into refutable dependency rules

- Confidence: plausible. Horizon: later.
- Motivation: ADR-035's "governance compiles to tests": an ADR stating a dependency
  direction or layering becomes a rule the call graph refutes on every ingest, instead of
  a sentence served at the edit. Evidence: plan row CV-53; archconformance rules exist.
- Do not build: a new LLM layer without a consumer (TX-4).
- Promote when: adr-rules is compiled in two or more repositories and one asks for
  enforcement. Drop if: arch rules authored by hand cover the need.

## F25. Data, schema and infrastructure refactors

- Confidence: speculative. Horizon: when a consumer migrates data.
- Motivation: these refactors are irreversible or stateful (store formats, SQL DDL,
  Terraform state addresses, Kubernetes apiVersion moves) and need a different
  conservation law (reversibility, state-address continuity). Evidence: plan row CV-54.
- Promote when: a named consumer's migration. Drop if: no consumer in two quarters.

## F26. Security, performance and concurrency conservation

- Confidence: plausible. Horizon: when a regression of that kind is observed.
- Motivation: a moved handler can lose its authorization middleware while its route key
  survives; a refactor of locking or loops loses race safety or throughput silently.
  test-kit now records `env.race`; bench suites exist. Evidence: plan row CV-55.
- Promote when: such a regression is observed in the fleet. Drop if: never observed.
