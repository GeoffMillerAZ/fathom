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

- **Promoted 2026-09-27** by ADR-048 (owner directive; vision 37): the work is scope goal
  G15 — each CGO walker's port item in `docs/plans/language-breadth.md` retires its
  incumbent on measured parity (design `docs/design/language-breadth.md` §11), and LB-F19
  removes the CGO tree-sitter modules from `go.mod`. Whether `CGO_ENABLED=0` builds follow is
  measured then, not assumed. This entry is deleted when LB-F19 is proven.

## F2. Public release (H2): public repos, tagged binaries, brew tap, MCP registry, plugin bundle

- Confidence: plausible. Horizon: later.
- Motivation: "the release is the boundary event"; the falsifiable "best" claim is the
  published context-bench. Evidence: `docs/design/product-vision.md` §4 H2.
- Do not build: brew tap, registry listings, marketing, a hosted anything.
- Preserve: `fathom init` works on a cold clone; the in-tree kit mirror (ADR-019) keeps a
  kit-less build possible; no org-specific content (ADR-011); the kit mirrors
  (`third_party/agentic-kit`, the app-kit adoption) carry licences compatible with
  distribution inside an FSL-1.1-ALv2 work (ADR-052; satisfied 2026-09-27, both kits are FSL,
  re-checked on any pin bump); Q29 and Q30 are answered before any public release.
- Promote when: the H1 exit criteria are green (instrument valid ≥4 weeks; injection ≥90%;
  verify gate live in ≥2 repos ≥4 weeks; surface pruned; portability passing; context-bench win).
- Drop if: the owner decides fathom stays a personal force-multiplier (`docs/vision/07` Q4).
- **Promoted in part 2026-09-27** (ADR-049 decision 3, vision 38): the plugin bundle (the
  rendered plugin with a default surface of at most eight tools, install-tested in Claude
  Code and one other client) is scope goal G16, `docs/plans/reach.md`. Publishing it (a
  marketplace listing, the brew tap, the MCP registry) stays here, under the Promote-when
  above and the owner's Q4 call.

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
- **Promoted in part 2026-09-27** (ADR-051 decision 4, U11): the computable subset (a
  revisit or promote condition that names a fathom instrument and a threshold, as a bet
  block does) is evaluated by the architecture drift radar (`docs/plans/change-and-compliance.md`
  CX-21). Prose triggers and plan `trigger:` text stay here, unevaluated.
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
- Promote when: `th-c214dd26` is decided and scope G6 is done. Fired 2026-09-25: G6 held
  and left `scope.md` (PC-20). **Promoted 2026-09-25 (owner) for the gate this repo has:**
  the pre-push verify step now runs `--strict` against its own scratch ingest of HEAD, so a
  claimed row that reads Unknown blocks the push; `scripts/ci.sh` already did. A hosted
  runner stays unbuilt, so the rest of this future waits on a runner.
- Drop if: the repo stays without a CI runner by decision.

## F11. Adopt the kit's `mcp-server` under `internal/mcp`

- Confidence: speculative. Horizon: someday. Evidence: ADR-022.
- Do not build: a wrapper layer. Promote when: the kit gains surface/group/prompts/resources support.
- Drop if: never.

## F12. An LLM-judged evaluation lane on `qa-harness`

- Confidence: plausible. Horizon: later. Evidence: ADR-023, FA-12, ADR-046.
- The lane now exists (`fathom assess`, ADR-046), so ADR-023's trigger fired on 2026-09-25;
  the owner kept evaluation bespoke until there is a corpus to replay.
- Promote when: M-A1's adjudicated sample exists (≥10 adjudicated units per question set).
  Drop if: M-A1 removes every assess question.

## F13. Vectors as an additive in-store column

- Confidence: speculative. Horizon: someday.
- Motivation: hybrid semantic search; "evidence-gated exactly as designed", written for
  DuckDB VSS and never re-checked on SQLite. Evidence: ADR-016, `docs/design/product-vision.md` H3 note.
- Do not build: a second store. Promote when: a query class fails keyword-only on the context bench.
- Drop if: harness-native embeddings make it moot.
- **Still gated by vision 31 (2026-09-27, ADR-049 decision 8):** documentation retrieval is
  built on SQLite FTS5 with doc-to-code links (G20, `docs/plans/knowledge-retrieval.md`), not
  on vectors. KR-M1's doc-question slice is a lexical reading, not vision 31's paired
  concept-query bench, and passing it promotes nothing here.
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
- **Output format promoted 2026-09-27** (ADR-049 decision 9): SARIF for obligations, review
  findings and contract drift, and a check-run payload, emitted by verbs
  (`docs/plans/governance-learning.md` GL-20, GL-21). The separate repository, a forge app
  and any posting service stay here.

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

- **Promoted 2026-09-27** by ADR-049 decision 6 (owner memo N4; vision 38): the work is scope
  goal G19, `docs/plans/spec-interop.md` (a spec-dialect port with plan grammar v2 canonical,
  Spec Kit and OpenSpec adapters in wave 1, Kiro, BMAD, Task Master and GitHub task lists in
  wave 4, the converge report, `certified_by:` as the test-tied tier). The Promote-when had
  not fired (no wired repo had asked); the owner promoted it on the market evidence. This
  entry is deleted when SI-M1 reads PASS.

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
- **Promoted 2026-09-27** by ADR-051 decision 4 (U11, the architecture drift radar): the
  Promote-when had not fired; the owner promoted it. The work is scope goal G25,
  `docs/plans/change-and-compliance.md` CX-19..CX-23. This entry is deleted when CX-M4 reads PASS.

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

## F27. A shared team index snapshot

- Confidence: plausible. Horizon: later.
- Motivation: a teammate starts from a committed, compressed snapshot instead of an ingest
  (the landscape scan's "Later" item). Evidence: `docs/vision/38-proof-layer-roadmap.md` §5.
- Do not build: a snapshot format, a sync, a hosted store.
- Preserve: born-portable stores (ADR-017); store format v0 stays additive; no absolute path
  or `$HOME` in store rows that a snapshot would carry.
- Promote when: F15 fires (a second user adopts) and their cold ingest of a fleet service
  takes longer than 10 minutes.
- Drop if: single-user is accepted in writing (`questions.md` Q5).

## F28. Answer events for background agents

- Confidence: plausible. Horizon: later.
- Motivation: event-driven agents (on a PR, a schedule, a message) could act on fathom's
  impact and verify verdicts. Evidence: the landscape scan's trend 6; vision 38 §5.
- Do not build: a scheduler, a watcher that triggers work, a webhook sender that runs unasked
  (the dispatch non-goal).
- Preserve: verdicts stay verb JSON with stable envelopes; a verb may write a verdict file or
  print a webhook payload only when invoked.
- Promote when: an adopter's background agent consumes a fathom verdict file in two
  consecutive weeks and asks for a stable payload.
- Drop if: the clients' own event systems read verb JSON directly.

## F29. A Rust-compiled wasm grammar or query module under wazero

- Confidence: speculative. Horizon: when the trigger fires.
- Motivation: the cheap experiment if Go or the tags engine becomes the extraction
  bottleneck: same sandbox, same module cap, no FFI. Evidence: ADR-049 decision 11, vision 38 §7.
- Do not build: a Rust worker process, an FFI seam, a rewrite.
- Preserve: extraction behind the wazero module boundary; RT-107's module cap applies to any
  module whatever its source language.
- Promote when: the typed-pass or extraction children cannot meet the 300 MiB ingest target
  after the RT waves, or GC pauses show in hook p99 at fleet scale (RT-M7 GC CPU above 25%
  below the refuse rung).
- Drop if: the RT waves close with ingest at or below 300 MiB and RT-M7 PASS for 30 days.

## F30. An onboarding path for a newcomer

- Confidence: plausible. Horizon: when a second person onboards.
- Motivation: a reading order computed from the index (entrypoints, central symbols,
  governing rules and decisions), each item fresh-verified (U14). Evidence: ADR-051 decision 7.
- Do not build: a tour UI, a curriculum, generated prose.
- Preserve: `entrypoints`, centrality and `obligations` stay verbs whose outputs compose.
- Promote when: F15 fires (a second person onboards to a fleet service).
- Drop if: single-user is accepted in writing (Q5).

## F31. Capability reach (network, filesystem, exec) and capability drift on upgrade

- Confidence: plausible. Horizon: after the dependency plane's second ecosystem.
- Motivation: which code reaches network, filesystem or process execution, and which
  dependency gained a capability in an upgrade (U15). Evidence: ADR-051 decision 7.
- Do not build: a sandbox, a policy engine, a capability database.
- Preserve: `pkg:` keys and the dependency API index keep symbol-level identity so a
  capability table can join them later.
- Promote when: SG-M4 reads PASS for a second ecosystem.
- Drop if: an ecosystem-native capability tool covers the fleet's languages.
