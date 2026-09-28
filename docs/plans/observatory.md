# observatory — the value-realization UI over the impact verbs

> Governing source: docs/vision/17-observatory-ui.md (this plan is its
> binding promotion) — IO-28's successor after its trigger fired
> 2026-08-08. **Covenant (ADR-0070 class):** failing test first,
> `task validate` green before commit, dated CHANGELOG entry, boxes flip
> only on `fathom verify`.

## A — Model and primitives

- [x] BuildObservatoryModel folds registry, fleet rows, ledger, arm/narrate state and observations into one value; a dead daemon or absent instrument degrades to named reasons, never zeros {id: OB-1, symbol: buildObservatoryModel, package: cmd/fathom, reachable_from: TestBuildObservatoryModel_DegradedIsNamed}
- [x] the absence primitive: every rendered metric is value-or-named-reason; rendering a nil metric fabricates nothing {id: OB-2, symbol: metricCell, package: cmd/fathom, reachable_from: TestMetricCell_AbsenceNamed}
- [x] viz comes from the kit (owner redirect 2026-08-08, D13): appkit/ui/viz Sparkline and Gauge render trends and shares; the hand-rolled SVG primitives were deleted the same day they were written {id: OB-3, symbol: TestObservatoryServicePage, package: cmd/fathom, skip_test: true, skip: "adoption row - the kit's own suites pin Sparkline/Gauge; the anchored render test exercises their use here (a Test* is not reached by a test)"}

## B — The screens

- [x] GET /observatory: fleet tiles with wiring, freshness, traffic and open observations; arm and narrate banners; every warning carries its remedy command; ?format=json returns the model {id: OB-4, symbol: observatoryHandler, package: cmd/fathom, reachable_from: TestObservatoryFleetPage}
- [x] GET /observatory/service?id=S: adoption gauge, hook heartbeat with template versions, freshness with the re-ingest remedy {id: OB-5, symbol: observatoryServicePage, package: cmd/fathom, reachable_from: TestObservatoryServicePage}
- [x] GET /observatory/impact: verify catches, discovery substitutions and obligations served each with denominators and date ranges; the counterfactual pilot rendered as a state machine; story rows only under the lens banner {id: OB-6, symbol: observatoryImpactPage, package: cmd/fathom, reachable_from: TestObservatoryImpactPage}

## C — The verb and the docs

- [-] fathom brief writes a dated local markdown value brief from the same impact model: real numbers with denominators, version stamps, and an explicit not-measured section; refuses nothing silently {id: OB-7, disposition: refused, reason: "retired by SR-14 (surface reckoning): the fathom brief CLI verb (cmdBrief) is removed for 0 lifetime calls; renderBriefMD/briefTuning survive for the console's /brief page"}
- [x] usage docs and the fathom-impact skill point at the observatory and the brief {id: OB-8, oracle: lexical, path: docs/usage.md, pattern: "/observatory/brief"}

## C2 — Owner asks landed mid-flight (2026-08-08)

- [x] GET / is a landing page: every browsable and machine surface linked from the root; unknown paths still 404 {id: OB-11, symbol: mountConsole, package: cmd/fathom, reachable_from: TestLandingPage}
- [x] the observatory is an app-kit app end to end: dashboard/stat/viz components, kit tokens via task ui:css (pinned binary, no Node), the UI doctrine in CLAUDE.md and AGENTS.md, UI guards in .fathom/guard.cue, the ui-appkit dialect pack installed, and the honest-absence tile promoted UP into the kit as stat.Absent {id: OB-12, symbol: TestObservatoryServicePage, package: cmd/fathom, skip_test: true, skip: "wiring row - the committed input.css/@source chain, the guard file, the installed layer dir, and app-kit commit af47262; the anchored render test exercises the kit components (a Test* is not reached by a test)"}

## C3 — The brief reads as a story (owner ask, 2026-08-08)

- [x] the brief opens with a plain-language short version — what fathom is, its scale, what it demonstrably did, what it has NOT shown, and the single highest-value configuration change — then five numbered questions each closing with an explicit **Reading:** line that says whether the number is evidence of help, of no help, or of nothing yet {id: BR-1, symbol: renderBriefMD, package: cmd/fathom, reachable_from: TestRenderBriefMD_HonestSections}
- [x] every count in the brief carries thousands separators through the observatory's existing formatter — no duplicate helper was added {id: BR-2, symbol: fmtThousands, package: cmd/fathom, reachable_from: TestFmtThousands}
- [x] the closing section is a work order derived from live state only: each entry names what is switched off, what switching it on would let a future brief say, and either the exact command or a note explaining why there is none; a fully configured fleet yields no entries rather than invented advice {id: BR-3, symbol: briefTuning, package: cmd/fathom, reachable_from: TestBriefTuningIsGroundedInState}
- [x] the brief PAGE renders the same emphasis the markdown carries — bold, emphasis and code spans — with escaping applied before tag emission so no instrument note can inject markup {id: BR-4, symbol: mdInline, package: cmd/fathom, reachable_from: TestMdInlineEscapesBeforeEmitting}

## D — Deferred with triggers

- [-] live-updating pages (JS/websockets) {id: OB-9, disposition: deferred, reason: "a fresh GET is enough until someone leaves the page open on a wall", trigger: "first wall-display or long-poll use case"}
- [-] any hosted or cross-org sharing of observatory pages {id: OB-10, disposition: deferred, reason: "law 13 - sharing is a human verb; the brief file is the sharing mechanism", trigger: "an org explicitly asks, and then only as an export a human sends"}
