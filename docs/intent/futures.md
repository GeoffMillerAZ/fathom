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
  distribution inside an FSL-1.1-ALv2 work (ADR-052, amended 2026-09-29: the check holds, because
  both mirrors carry FSL-1.1-ALv2, agentic-kit from `53024ff` and app-kit from `d4512ca`, mirrored at
  `9fa58e6`; re-check on any pin bump that changes either licence); Q29 and Q30 are
  answered before any public release.
- Promote when: the H1 exit criteria are green (instrument valid ≥4 weeks; injection ≥90%;
  verify gate live in ≥2 repos ≥4 weeks; surface pruned; portability passing; context-bench win).
- Drop if: the owner decides fathom stays a personal force-multiplier (`docs/vision/07` Q4).
- **Promoted in part 2026-09-27** (ADR-049 decision 3, vision 38): the plugin bundle (the
  rendered plugin with a default surface of at most eight tools, install-tested in Claude
  Code and one other client) is scope goal G16, `docs/plans/reach.md`. Publishing it (a
  marketplace listing, the brew tap, the MCP registry) stays here, under the Promote-when
  above and the owner's Q4 call.
- **Note 2026-09-29** (the Claude OS comparison, ADR-049 amendment): screenshots of the console up front in
  the README, and listings in MCP directories, wait here under the same Promote-when and Q4.

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
- Note (2026-09-29, incident maps, revised the same day): incident maps are not the form this future takes. They
  became committed declarations, not live maps, and moved to F56 (ADR-058, `docs/design/incident-maps.md`); an
  earlier note here that gated them on live maps and on rows XM-26 to XM-30 is withdrawn. This future stays
  precedent compiled from a non-git corpus, gated on the resolved-finding count above. Related: F52, F56.

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
- **Note 2026-09-29** (the Claude OS comparison, ADR-049 amendment): a browsable shared skills and packs
  library waits here; it needs adopters first.

## F7. The `ai-derived` evidence tier

- Confidence: speculative. Horizon: someday.
- Motivation: the first non-citable claim kind. Evidence: ADR-016.
- Do not build: the tier. Preserve: `tier_profile` column (ADR-017); tiers declared in CUE.
- Promote when: a non-citable claim kind is proposed with its own eval. Drop if: never needed.
- **Named 2026-09-29 (ADR-056):** a decision-port template that passes R3's pruning-and-gating bar may graduate
  to a judged layer whose rows are claims at this tier; J0 plus that template's calibration reading is the eval
  this entry asks for, and the graduation needs its own ADR because `schema/semantic/schema.cue` rejects the tier
  today. Nothing is built (`docs/plans/judged-predicates.md` JP-16).

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

**Read again 2026-09-29 (ADR-056):** `typesafe` is one provider of an optional port, never a default and never a
dependency; every new decision site is opt-in per repository even with a provider set, calibration is per
(site, provider, resolved model), and Jev's signups were paused on 2026-09-22 with OpenRouter access in alpha, so
nothing may depend on it (P24). The entry's own drop-if stands.

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
- **Named 2026-09-29 (ADR-056):** review narrowing (which callers depend on a change) and migration-site
  disambiguation stay under this entry, with the same Promote-when (a frontier over 100 sites plus an A/B corpus);
  the earlier "plan-only" promise for them is dropped.

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

## F32. Review Kit merges into fathom

- Confidence: speculative. Horizon: someday. Evidence: ADR-055 (D3), `docs/vision/39-feature-map-and-review.md` §4.
- Motivation: the owner's instinct that review is superpowered by fathom is right, but the
  2026-09-28 decision is that the two stay a family (one contract, a shared bundle), not one
  codebase — ADR-040's authority boundary (fathom never approves, disputes, tailors or closes),
  the hosted/org-feature conflict with fathom's non-goals and tenancy, Review Kit's
  works-with-no-fathom independence rule, and fathom's own surface shrink (ADR-053) all argue
  against it today.
- Do not build: absorbing Review Kit's standards, dispositions, admission, signed execution or
  hosted view into fathom's codebase.
- Preserve: the versioned evidence contract (ADR-055 decision 8) as the seam; Review Kit's
  standalone (no-fathom) path.
- Promote when: at least 70% of Review Kit's requirements evaluated in a quarter depend on a
  fathom evidence lane; or at least 5 adopters exist and every one installs both; or the
  evidence contract takes more than 3 breaking changes in a quarter (the seam costs more than
  it saves); or the owner decides to sell one product (then bundle first, merge code last).
- Drop if: the versioned evidence contract holds stable and adopters keep choosing one tool or
  the other rather than both.

## F33. Review Kit moves into its own repository beside fathom

- Confidence: likely. Horizon: later.
- Motivation: Review Kit lives inside the `platform-engineering` monorepo today; the owner's
  answer of 2026-09-28, on the same review-versus-merge question ADR-055 settles, is that it
  eventually moves out to sit beside fathom, but not now. Evidence: ADR-055 decision 8 and
  Consequences; `docs/vision/39-feature-map-and-review.md` §9.
- Do not build: the split, a new repository, or any repo-scoped tooling before it is promoted.
- Preserve: the versioned evidence contract (F32's seam) works the same whether Review Kit
  lives in the monorepo or its own repository — the split is packaging, not a contract change.
- Promote when: the family bundle (ADR-055 decision 8, one installer, shared naming, shared
  licence) ships; or Review Kit's release cadence diverges from platform-engineering's; or a
  second adopter installs Review Kit standalone.
- Drop if: Review Kit stays tied to monorepo-only consumers.

## F34. `fathom ask`: judged predicates over spans the graph selects

- Confidence: plausible. Horizon: later. Evidence: ADR-056, `docs/design/judged-predicates.md` section 3.
- Motivation: level 10's own question generalized: an agent asks a typed model about N code units without reading
  N files, fathom being the planner (it selects rows, computes what is computable, assembles the unit and its
  context, builds options from index facts) and the model a semantic predicate on the residue only.
- Do not build: the verb, an MCP tool, a computable-operator grammar (existing tools answer those), a model that
  decides what is computable, or any answer that prunes before R3's bar.
- Preserve: opt-in per site (`ask` is in `decide.OptInSites`), provider none returns the computed rows and the
  redacted request plan, a CLI verb plus a lazily loaded skill adds no schema bytes.
- Promote when: J0 (`docs/plans/judged-predicates.md` JP-M1) passes and the model beats the lexical baseline.
- Drop if: J0 fails, or the 40-handlers cell shows arm C under 40 percent fewer tokens than control at recall and
  precision within 0.05.

## F35. Tier 2 of the test-run hook: a failure-kind judgment

- Confidence: plausible. Horizon: later. Evidence: ADR-056, `docs/design/test-run-hook.md` section 5.
- Motivation: after the deterministic line names the failing tests and the changed symbols they reach, a choice
  over code bug, environment, timing, outdated expectation or other can settle itself from where the fix lands.
- Do not build: the site, or any display of `test_expectation_outdated` before it clears R3's bar (it licenses
  editing a test to match buggy output).
- Preserve: the hook's line is byte-identical with any provider; the parse keeps a settle-able failure record.
- Promote when: the hook has logged at least 100 failing runs later followed by green, and J0 has run.
- Drop if: arm C fails to beat arm B by 10 percent on turns or correctness, or agreement stays under 0.8 after
  one template revision.

## F36. Question-directed traversal (`localize kind: question`)

- Confidence: plausible. Horizon: later. Evidence: ADR-056, `docs/design/judged-predicates.md` section 4.
- Motivation: exploration as a best-first walk of the typed neighbours, scored against the agent's own question,
  strong nodes never pruned; its natural delivery is as a builder of exploration maps.
- Do not build: a scorer before the deterministic traversal beats control, or a new tool.
- Preserve: `localize`'s loop and hop budget; the pruned list with reasons.
- Promote when: the maps A/B (`docs/plans/exploration-maps.md` XM-M2) passes, so a delivery channel exists.
- Drop if: provider-none traversal fails to beat control by 25 percent on tokens-to-correct-answer, or J0 kills.

## F37. Files at scale: judged semantic columns (level 9)

- Confidence: plausible. Horizon: later. Evidence: ADR-056, `docs/design/judged-predicates.md` section 5.
- Motivation: one cheap question per unit, run only on units whose content hash changed and scoped to a live
  map's footprint, for the residue no computation labels (test kind, concern membership, role among declared
  layers). Computable labels are refused as model work.
- Do not build: the judged layer, the job, or any column that prunes; nothing security-shaped is ever served.
- Preserve: content-hash freshness, the memo registry's byte budget, options from index facts.
- Promote when: J0 passes, RT-M1 reads PASS, and a named consumer's own bench metric judges it.
- Drop if: J0 fails, or a graduated label changes no consumer's ranking measurably within 30 days.

## F38. When-to-compact advice

- Confidence: plausible. Horizon: someday. Evidence: ADR-056, `docs/design/carry-across.md` section 6.
- Motivation: a task-switch judgment at the prompt, shown to the user only; the act of compacting belongs to a
  harness that owns compaction.
- Do not build: advice without carry-across, or anything that claims fathom can compact.
- Preserve: the computed working set as a CLI a harness can call.
- Promote when: carry-across passes (CA-M2) and the route site is re-admitted.
- Drop if: carry-across fails its A/B.

## F39. Fitted recalibration and site auto-disable

- Confidence: plausible. Horizon: later. Evidence: ADR-056, `docs/design/judgment-foundations.md` section 8.
- Motivation: a per-site map from raw provider confidence to a calibrated band, and a site that disables itself
  when its rolling Brier score falls below the base rate.
- Do not build: a fit before the data, or auto-disable without a minimum n per window (it flaps).
- Preserve: the calibration reading per (site, template, provider, resolved model).
- Promote when: one (site, provider, model) holds at least 100 labels.
- Drop if: no site reaches 100 labels within a year.

## F40. The judged-telemetry sampler, queue, background judge, judgments table and durable memo

- Confidence: plausible. Horizon: someday. Evidence: ADR-056, `docs/design/judged-telemetry.md` section 4.
- Motivation: more labels than settlement supplies, and a memo where the llm lane makes repeats costly.
- Do not build: any of them before RT-M1 reads PASS, or a lane parallel to the assess lanes.
- Preserve: spend labels that keep judging spend out of every A/B unit (I46); the owner's call to unfreeze assess
  or supersede it, made before this is built.
- Promote when: RT-M1 reads PASS and tier 2 (F35) or `ask` (F34) needs more labels than settlement supplies.
- Drop if: settlement plus batched adjudication supply every template's labels for 90 days.

## F41. Guard proposals from rule outcomes

- Confidence: plausible. Horizon: later. Evidence: ADR-056, `docs/design/judged-telemetry.md` section 5.
- Motivation: a rule with a checkable condition that is repeatedly not met becomes a deterministic guard proposal
  a human accepts.
- Do not build: a proposal that sets `enforce: true` on its own, or model-judged hunks written into tests.
- Preserve: the renamed rule-outcome events (`docs/plans/judgment-foundations.md` JF-1).
- Promote when: JF-1 has shipped and 30 days of relabeled outcomes show such a rule.
- Drop if: no rule has a checkable condition after 90 days.

## F42. MCP exposure of test-run or ask output

- Confidence: plausible. Horizon: someday. Evidence: ADR-056 rule R6, `docs/design/test-run-hook.md` section 8.
- Motivation: the CLI and hook are the delivery; an MCP parameter is the pull path if agents demonstrably want it.
- Do not build: a new tool; the exposure is a parameter of `localize` (kind: test_output) or `test_impact`.
- Preserve: the extended surface and an unchanged tool count.
- Promote when: hook or CLI use reaches at least 5 percent of sessions.
- Drop if: use stays under 5 percent for 180 days after the hook and the CLI both exist.

## F43. A `--plan-only` export for other harnesses

- Confidence: plausible. Horizon: later. Evidence: ADR-056, `docs/design/judged-predicates.md` section 3.
- Motivation: fathom plans and redacts the request; any harness, or Review Kit's typed-judgment lane, makes the
  call with its own key, so fathom has zero egress.
- Do not build: the export before a consumer names itself.
- Preserve: the redacted request plan as a stable shape inside `ask`.
- Promote when: a named consumer asks for it.
- Drop if: no consumer asks within a year of `ask` shipping.

## F44. Set algebra over exploration maps

- Confidence: plausible. Horizon: later. Evidence: ADR-056, `docs/design/exploration-maps.md` section 2.
- Motivation: "the base map minus what is already done" for a brief.
- Do not build: intersection or difference; union by reference is the only composition.
- Preserve: maps are DAGs of references with anchored entries, never copies.
- Promote when: a real brief needs a base minus what is already done.
- Drop if: union covers every observed handoff for 60 days.

## F45. ADR-to-rule pair judgments

- Confidence: speculative. Horizon: someday. Evidence: ADR-056 decision 12.
- Motivation: proposing arch rules for adopters whose ADRs name packages but who have no arch rules.
- Do not build: it before an adopter needs it; negation-shaped questions are a typed model's weakness.
- Preserve: options built from the repository's own package names.
- Promote when: an adopter repository has many ADRs that name packages and no arch rules.
- Drop if: no such adopter appears within a year.

## F46. Doc-drift triage

- Confidence: plausible. Horizon: later. Evidence: ADR-056 decision 12, `docs/plans/knowledge-retrieval.md`.
- Motivation: ordering flagged doc spans by how likely the changed symbol made them wrong; deterministic order
  stays the default.
- Do not build: a model that marks a doc wrong (drift means the target moved, nothing more).
- Preserve: doc-to-code links and their freshness flags.
- Promote when: knowledge-retrieval doc linking ships and more than 20 spans are flagged per week.
- Drop if: flags stay at 20 a week or fewer for 90 days.

## F47. An injection screen for served third-party text

- Confidence: speculative. Horizon: someday. Evidence: ADR-056 decision 12.
- Motivation: a typed flag on text fathom serves that came from outside the repository.
- Do not build: it before fathom serves third-party text by default; it can only flag.
- Preserve: served text carries its source.
- Promote when: fathom serves third-party text by default, or an injection incident occurs.
- Drop if: fathom still serves no third-party text after a year.

## F48. Skill suggestion at the prompt

- Confidence: speculative. Horizon: someday. Evidence: ADR-056 decision 12.
- Motivation: the same prompt-classification shape as route, choosing among installed skills.
- Do not build: it while route is off by default.
- Preserve: the route site's deterministic precheck.
- Promote when: route is re-admitted (`docs/plans/judgment-foundations.md` JF-8).
- Drop if: route is dropped.

## F49. Friction episode classification and product feedback

- Confidence: speculative. Horizon: someday. Evidence: ADR-056 decision 12, `docs/design/judged-telemetry.md` section 5.
- Motivation: classifying the unclassified residue of a work-intelligence friction map (U3), and aggregated
  counts and taxonomy labels for fathom's own maintainers through the research bundle's closed allowlist, run by
  a human (P13, I14).
- Do not build: either before its consumer exists or before labels worth sharing exist.
- Preserve: deterministic friction classes first; the research bundle's allowlist.
- Promote when: U3 exists (classification), or labels worth sharing exist (feedback).
- Drop if: U3 is refused, or no labels worth sharing exist within a year.

## F50. Durable live maps: saved queries in the layers' vocabulary

- Confidence: plausible. Horizon: later. Evidence: ADR-056, `docs/design/exploration-maps.md` sections 3 and 3b,
  `docs/plans/exploration-maps.md` XM-D1, XM-D4, XM-D5, and for documentation use XM-17 to XM-22, XM-D7, XM-D8.
- Motivation: a live map is a saved, re-run query (a working set) that stays correct as code changes, unlike a
  wiki diagram or a document list: the auth surface for a security review, the payment path from entry point to
  data store, release blockers, the public API without a live certificate, a migration's remaining sites, the
  hot zone, everything that reads a flag, an active program's footprint, dead-code candidates, the product's
  core loop. It queries layers, scopes an expensive judged column to the maps that need it, and graduates to a
  named concern when many agents reuse it.
- Do not build: a live map before RT-M1 reads PASS and either the handoff A/B (XM-M2) reads PASS or a named
  consumer of a non-handoff use case with its own bench metric exists (R5: storage stays in existing thread
  fields), a second freshness mechanism, an idle unnamed map that never decays, or a layer
  owned by a map (a layer keeps its own freshness and lifecycle: candidate, active, retired).
- Preserve: maps compose by reference (a DAG, depth capped at 4), a named map needs a purpose line, one
  freshness machine shared with layers, permanent only when named and used.
- Promote when: RT-M1 reads PASS and either XM-M2 reads PASS or a named consumer of a non-handoff use case
  (a concern scope, an incident map, a structural claim, a saved question) has its own bench metric (XM-D1's
  trigger, the same). Amended 2026-09-29: a kill of the handoff maps (XM-M1 reads KILL, so XM-M2 never runs)
  leaves durable live maps unaffected, because their value is independent of sub-agent re-derivation; P20 still
  demands the named consumer. The root rows of `docs/plans/exploration-maps.md` wave 3 and wave 4 therefore
  need XM-D1 and RT-M1, not XM-M2, relying on G2-6 so that a deferred target blocks.
- Drop if: no live map is mounted in at least 3 runs within 180 days of the first one shipping.

## F51. Model proposals for feature-map verdicts

- Confidence: plausible. Horizon: someday. Evidence: ADR-055 decision 4 (amended 2026-09-29), ADR-056 decision 12,
  `docs/plans/feature-map.md` FM-10, FM-11, FM-M2.
- Motivation: a Choice over keep, improve, merge and ditch given a feature's computed evidence row could ease
  reviewing many features. The owner decided on 2026-09-29 that this is deferred, not refused: verdicts stay
  human until a calibration earns the proposal path.
- Do not build: the proposal path or any display of a model's verdict before J0 passes and a calibration against
  the owner's 86 labeled verdicts reaches the bar; a ditch proposal never auto-applies (it routes into the
  surface-reckoning pattern) and a model never anchors the one decision ADR-053 requires to be evidence-driven.
- Preserve: verdicts as human-authored cited claims in `docs/features.md` (FM-9), the computed evidence row as
  the proposal's input, and the 86 labeled verdicts as calibration data (committed as
  `docs/assessments/data/2026-09-27-feature-scorecard.json`, also FM-M1's hand-built oracle). The critics' estimate is that 86 labels
  over four classes cannot support the 0.8 bar, which is why this waits rather than runs.
- Promote when: J0 (`docs/plans/judged-predicates.md` JP-M1) reads PASS and a calibration against the owner's 86
  labeled verdicts reaches agreement of at least 0.8 at n of at least 3 (FM-M2's bar).
- Drop if: J0 kills the code-reading sites, or the calibration misses the bar and the owner's labeled verdicts
  have not grown within a year.

## F52. Review Kit concern packs declare their scope as live-map queries

- Confidence: plausible. Horizon: later. Evidence: ADR-055 decision 8, ADR-056, `docs/design/exploration-maps.md`
  section 3c, `docs/plans/exploration-maps.md` XM-23 to XM-25.
- Motivation: a review or well-architected concern ("every outbound network call", "every write path to table
  X", "every flag reader") is a set of code, and findings are the violations inside it. A pack whose scope is a
  live-map query reviews the current code and catches code that newly enters the scope, where a fixed list goes
  stale; paired with a check it makes a periodic review a standing property.
- Do not build: it before live maps exist and the evidence contract has a scope field; any conversion, scoring
  or judgment of a finding by fathom (ADR-040, ADR-055: findings stay under Review Kit's authority); a concern
  scope that only works with fathom present.
- Preserve: the versioned evidence contract as the seam (ADR-055 decision 8), a scope as a query in the layers'
  vocabulary, findings as point claims outside maps, and a pack with no scope field working exactly as today.
  The Review Kit side is a future on its own plan; the fathom side is the evidence-contract row XM-23.
- Promote when: live maps land (F50 promoted, XM-D1) and the evidence contract has a scope field.
- Drop if: Review Kit declines to carry a scope field on its concern packs.

## F53. `propose_check`: a typed choice over the closed check kinds for a prose rule

- Confidence: speculative. Horizon: someday. Evidence: ADR-057, `docs/design/served-not-kept.md` section 8,
  `docs/plans/served-not-kept.md` PR-D7.
- Motivation: a prose rule that names no test has no check, so its keeping cannot be recorded. A decision-port
  site could choose, for a prose rule, which of the closed check kinds fits (the pinned test, a forbidden path,
  an import boundary and the other guard checks), and a human accepts the proposal. It chooses only the kind:
  the arguments (globs, test names) still have to be generated, and generation is the port's weak side (ADR-056).
- Do not build: it before J0 passes and the labeled set reaches 50 (about 21 rules have a check today); any
  auto-accept; a model that sets kept or broken (R7, I62); a check kind outside the closed set.
- Preserve: the closed set of guard check kinds; rules labeled by whether a check already exists; human
  acceptance of every proposal.
- Promote when: J0 (`docs/plans/judged-predicates.md` JP-M1) reads PASS, the labeled set reaches 50, and prose
  rules are at least half of served firings.
- Drop if: J0 kills the site, or the labeled set stays under 50 for a year.

## F54. Orchestrator constraints on a sub-agent

- Confidence: speculative. Horizon: someday. Evidence: ADR-057, `docs/design/served-not-kept.md` section 6,
  `docs/plans/served-not-kept.md` PR-D8, `docs/design/exploration-maps.md`.
- Motivation: a handoff map id could carry `constraints: [{check}]` so the orchestrator's requirements ("the
  store stays pure Go") reach the checks that run in the sub-agent's hooks.
- Do not build: it before handoff maps exist and it is known whether hooks fired inside a sub-agent carry
  `agent_id` (PR-V3); a constraint the sub-agent may waive; a second store for constraints.
- Preserve: map entries anchored to symbol and content hash; checks drawn only from the closed check kinds.
- Promote when: the handoff maps exist (XM-M1 reads BUILD with re-derivation at least 15 percent of sub-agent
  reads, and XM-M2 reads PASS) and PR-V3 shows sub-agent hooks carry `agent_id`.
- Drop if: the maps handoff channel is removed (XM-M1 KILL or a missed XM-M2).

## F55. Fathom as the code engine under an agent-memory tool

- Confidence: speculative. Horizon: someday. Evidence: `docs/assessments/2026-09-27-fathom-and-claude-os.md`,
  ADR-049 amendment 2026-09-29, ADR-028.
- Motivation: an agent-memory tool with a weak code index (Claude OS indexes three languages and links files by
  shared names) could ask fathom for callers, impact and rules through an adapter that answers its
  structural-index calls with fathom's JSON; the pattern could then be offered to other memory tools.
- Do not build: an adapter, a demo or a pull request against another project before the trigger; anything that
  makes an agent-memory tool a dependency of fathom (ADR-028: a peer process, never a dependency); a pairing that
  hides fathom's name and its rules and proof features.
- Preserve: the JSON CLI output, the HTTP API and the Go client as the integration surface; a kit-less build.
- Promote when: a project with a licence file and a release in the last 90 days asks for fathom answers, or a
  user asks for the pairing.
- Drop if: Claude OS stays inactive past 2027-03-31 and no second tool asks.

## F56. Incident maps: pre-shaped incident declarations and the suspect list

- Confidence: plausible. Horizon: later. Evidence: ADR-058, `docs/design/incident-maps.md`,
  `docs/plans/incident-maps.md`.
- Motivation: in the first five minutes an on-call engineer or agent needs owners, what changed, prior incidents,
  knobs, reach and the runbook. A team commits a declaration ahead of time (symptoms, footprint, window, runbook
  path, links, blind spots); a pasted trace or alert finds it; every facet reads a value, none, or not computed.
- Do not build: a map kind or the `map` verb for it; probes or MCP next queries (F57); attach or any connector
  (ADR-058 refuses them); drills, drill state, a timeline or an incident-tool export (the command-center non-goal);
  committers as owners; a stored commit table before RT-M1.
- Preserve: the unprepared view as the floor; "git-visible changes to this repo only, not cause"; links never
  fetched; the declaration editable only by people.
- Promote when: IM-M0 reads BUILD (stage 1), then IM-M2 reads KEEP and a named consumer is recorded (stage 2).
- Drop if: IM-M0 reads KILL on both arms; IM-M1 recall@20 falls under 0.5 or its median list exceeds 25 rows; or the
  pointer's expansion stays under 10 percent over 30 days of pointers shown.

## F57. Incident probes and MCP next queries: command templates rendered, never run

- Confidence: speculative. Horizon: later. Evidence: ADR-058, `docs/design/incident-maps.md` section 6.4.
- Motivation: the owner wants incident maps to reach CLI commands and other data sources; a declaration could carry
  read-only command or tool-call templates bound to the incident's facts.
- Do not build: anything fathom executes; a template that passes a shell, an interpreter, a launcher, an endpoint or
  credential flag, or a secret-reading verb; operator-local MCP aliases in a committed file; a probe with no
  sensitivity or no pinned target.
- Preserve: the section 6.4 rendering contract (rendering is the gate because bypass modes exist).
- Promote when: F56 is promoted, Q32 is answered and an inject-only sensor can match a run argv to a rendered probe
  id, and a consumer whose runbooks lack the commands asks for templates.
- Drop if: F56 is dropped, or no consumer asks within 180 days of F56's stage 2 shipping, or the sensor's first
  30-day reading shows under 10 percent of rendered probes run.

## F58. Upstream drift of vendored and pinned dependencies, restricted to what the product reaches

- Confidence: plausible. Horizon: later. Evidence: ADR-060 decision 3, `docs/vision/41-stewardship.md` section 4.2,
  `docs/plans/stewardship.md` SV-M4 and SV-D1, ADR-050 decision 4, `docs/plans/system-graph.md` SG-37 to SG-39.
- Motivation: Renovate and Dependabot say a new version exists; nobody says whether it changes anything the product
  uses. For a vendored mirror the next version is often already on disk as a sibling checkout (agentic-kit and
  app-kit here), so upgrade impact over the used closure could put one line on the attention line: "app-kit moved
  and changed 3 files you reach".
- Do not build: polling, a registry query or any fetch (P23, I56); a version-availability feed (Renovate's and
  Dependabot's job); a significance rule other than a removed or re-signatured reached symbol, a changed file inside
  the used closure, or a licence-file change; a model's opinion of significance.
- Preserve: `contract_drift {pkg, from, to}` as the one upgrade-impact answer (ADR-050); the used closure's JSON
  (`fathom report used`, SV-7) as its filter; `.vendor-commit` stamps and `task vendor-check` as the pin record.
- Promote when: `docs/plans/system-graph.md` SG-39 is proven and SV-M4 shows at least one reached upstream change in
  a quarter.
- Drop if: SV-M4 shows no reached upstream change in two quarters, or a dependency bot ships reach-restricted
  upgrade impact that reads fathom's closure.

## F59. Promises as code: SLOs, alerts and dashboards linked to the code that keeps them

- Confidence: plausible. Horizon: later. Evidence: ADR-060 decision 5, `docs/vision/41-stewardship.md` section 4.4,
  `docs/plans/stewardship.md` SV-D5, ADR-050 (the link model), ADR-044 (conservation planes), ADR-055 decisions 7 and
  8 (Review Kit's domains and the evidence contract), ADR-058 (incident declarations).
- Motivation: a service's promises form a chain (feature, route, the SLI metric emitted on the handler's reach, the
  SLO, alert and dashboard that consume it, the runbook), and each break is a code-grounded fact: an alert or
  dashboard on a metric no code emits, a route with no SLI or no SLO, a refactor that stops emitting a metric an alert
  consumes. Review Kit's reliability domain judges readiness against a Standard and needs these facts.
- Do not build: before a named consumer exists (P20; the 2026-09-30 census found no alert-rule, SLO-spec or
  dashboard file on this workstation); live SLI values, burn rates or alert state (the ops system's, a rendered link
  at most); readiness grades or level criteria in fathom (a Review Kit Standard); an organization-wide dashboard
  (hosted, a non-goal); a separate Reliability Kit (P21); a tool per kind (law 4).
- Preserve: the contract-key grammar stays additive, so a `metric:` kind is one more key kind (ADR-050 decision 1);
  conservation planes stay registrable; readiness facts travel through the versioned evidence contract; a level
  renders only from a committed reading (ADR-054).
- Promote when: a named consumer commits alert rules, SLO specs or dashboards, and `docs/plans/system-graph.md` SG-2
  and SG-M1 have landed.
- Drop if: no consumer within 365 days of 2026-09-30, or Review Kit's reliability domain declines fathom evidence.

## F60. Runtime topology graph: typed cross-artifact edges from proxies, IaC, policy and containers

- Confidence: plausible. Horizon: later. Evidence: the owner 2026-10-02 (the excellence directive, language-breadth
  design section 18), `docs/vision/38` and ADR-050 (the system graph), `docs/plans/system-graph.md` (SG-2, SG-M1),
  F59 (a later consumer).
- Motivation: proxy route to Kubernetes Service to Deployment to image to Dockerfile to repo, and an IaC resource to the
  policy that governs it. When every format contributes typed cross-artifact edges, cross-repo impact answers "what
  breaks in production".
- Do not build: before RT-M1 (the table freeze lifts) and SG-2, no new edge table; this is not a goal of the excellence
  program, which only extracts each format well.
- Preserve: edge kinds stay additive (`service_ref` exists; `image_ref` arrives with wave tl and `route_ref` with wave
  px; `policy_ref` is named only, not built), and the contract-key grammar stays additive (ADR-050 decision 1).
- Promote when: RT-M1 reads PASS and `docs/plans/system-graph.md` SG-2 has landed, and at least two of the proxy, IaC,
  policy and container formats are extracted at L3 with their reference edges.
- Drop if: no format beyond containers reaches L3 within 365 days of 2026-10-02, or ADR-050's link model is superseded
  by one that already carries these edges.
