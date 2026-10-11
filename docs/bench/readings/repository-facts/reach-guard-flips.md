# RF-M6: how many proven rows in the fleet pass only through a heuristic edge?

**Verdict: nine proven rows in two services rest on a guess, and RF-5 refuses them.** Across the
2,737 claimed rows the current verify proves in the seven fleet services that can flip, nine pass
`reachable_from` (edge_wired) or `wired` only through a call edge of resolution tier `heuristic`: six
in fathom's own plans and three in platform-engineering's. The other 2,728 hold over typed edges
alone. Five of fathom's six are rows whose symbol is a type or a constant (a type or a constant is
never the target of a typed call edge, so only a syntax-only reference "reached" it); the sixth and
the three platform-engineering rows passed through a name the syntax pass matched to the wrong
function. The count is small and every row is named below, so RF-5 ships with a `### Breaking` line
and this migration note.

RF-M6 count: 9

## Binaries

| binary | git commit | sha256 (first 16) |
|---|---|---|
| base (the current verify) | `c39875f3` (origin/main) | `fe9142e6358a7f39` |
| guard (a scratch build with RF-5) | `6af1ed42` (side branch `rf5-side`, RF-5 written test-first; the RF-5 commit after this reading carries the same product code) | `363033f56ff061ea` |

Both were built with `go build ./cmd/fathom` (CGO on, as the gate builds), $0, no model call.

## Service HEADs

The fleet holds 19 services (`fathom fleet report`, 2026-10-11). A row can flip only if a plan in
`docs/plans/` claims it `[x]` with a `reachable_from:`, or with a `symbol:` that `wired` evaluates
(a Go function in a service with a `package main`). Twelve services hold no such row and were not
ingested: car-assistant, growing-gardens-bi, iam_horcrux, planning, pulumi-eks, pulumi_eks_component,
study, throughline and workstation_as_code have no plan; cue-wasm and imbue have no claimed row with
either key; my-gods-eye's 11 symbol rows name no Go source and it has no `reachable_from`. The seven
below were measured at the owner's committed HEAD.

| service | HEAD | plans | rows | claimed rows proven by base | of those, proven with a passing edge_wired or wired | flips |
|---|---|---|---|---|---|---|
| agentic-kit | `53024ffa6343` | 29 | 361 | 87 | 42 | 0 |
| app-kit | `81a1352da387` | 4 | 62 | 41 | 20 | 0 |
| conjure | `2d95c9795d11` | 6 | 136 | 64 | 42 | 0 |
| fathom | `c39875f301d0` | 69 | 2949 | 1991 | 1495 | 6 |
| geoffmiller-cloud | `66ef3be8c69f` | 11 | 37 | 28 | 17 | 0 |
| platform-engineering | `84f9d4101045` | 25 | 874 | 343 | 225 | 3 |
| resume_as_code | `04be1238bae1` | 4 | 202 | 183 | 160 | 0 |
| **total** | | 148 | 4621 | 2737 | 2001 | **9** |

## Method

1. Each service's committed HEAD was cloned read-only from its registered root (the daemon's
   `fathom fleet report` paths) into a scratch tree, the services side by side as on the operator's
   machine, so a module's `replace => ../<sibling>` resolves as it does there. fathom's HEAD was a
   detached worktree at `c39875f3`. No owner checkout was edited or ingested; agentic-kit's
   `core.bare` read `false` after every step.
2. One scratch store at a time: `fathom ingest --store <scratch> --service <svc> --path <clone>`
   (the base binary; the ingest is the same for both), under a shared lock.
3. `fathom verify --strict --json --store <scratch> --service <svc>` over every `docs/plans/*.md`
   of the service, once with each binary, from the clone's root (lexical oracles read it), each run
   writing its own scratch ledger. The store was deleted before the next service.
4. A flip is a row claimed `[x]` that the base proves (status pass) and the guard does not, where
   every criterion that changed from pass is `edge_wired` or `wired`. No row changed on any other
   criterion.

Three modules failed the scratch ingest's typed pass, each for a reason of the clone, not of the
service: app-kit's `spikes/wails-gallery` and geoffmiller-cloud's `studio` (go/packages returned no
package), and platform-engineering's `portal/ui` (an embedded `app.css` the clone lacks, generated
and uncommitted). A row reached only through those packages would read heuristic here and typed on
the operator's tree. None of the nine runs through them, and no other row in those services flipped.

A first guard build (`afb078cf`) also flipped five resume_as_code rows (RA-1, RA-142, RA-180, RA-187,
RA-188). Each names a type and one method (`symbol: Viz` finds the type `Viz` and the method
`Corpus.Viz`), and `wired` judged the bare name, which resolves to the type, "reached" only by a
reference to it. RF-5 now judges the function the row names, and all five pass over typed edges
(`product entry point resume_as_code::cmd/lambda/main.main reaches resume_as_code::cmd/corpus/Corpus.Corpus.Viz`).

## The rows (migration note)

Each row passed under the base and is refused under RF-5. The edge is the first heuristic edge on
the reach, as the refusal now names it. Every edge is still served with its tier named
(`symbol_context`, `blast_radius`, `find_callers` with `include_heuristic`); only verify stops passing
on it.

| service | plan | row | criterion | the heuristic edge it rested on |
|---|---|---|---|---|
| fathom | `docs/plans/ambient-fathom.md` | TH-1 | edge_wired (`reachable_from: TestThreadBriefComposesDerivedAndDeposited`) | `cmd/fathom/main.derivedInFlight -> cmd/fathom/main.run` (syntax-only: a local closure named `run`, which runs git, bound to the CLI's `main.run`) |
| fathom | `docs/plans/language-breadth.md` | LB-F377 | edge_wired (`reachable_from: TestIngest_ReactComponents`) | `internal/fusion/fusion.TestIngest_ReactComponents -> internal/core/core.KindJSComponent` (syntax-only: a constant) |
| fathom | `docs/plans/observatory-console.md` | OC-1 | edge_wired (`reachable_from: TestValue_TruthStatesRoundTripJSON`) | `internal/console/console_test.TestValue_TruthStatesRoundTripJSON -> internal/console/console.Value` (type-ref: a type) |
| fathom | `docs/plans/observatory-console.md` | OC-13 | edge_wired (`reachable_from: TestEventBus_ReplayBoundAndKeepalive`) | `internal/console/console_test.TestEventBus_ReplayBoundAndKeepalive -> internal/console/console.EventBus` (type-ref: a type) |
| fathom | `docs/plans/refactor-conservation.md` | CV-24 | edge_wired (`reachable_from: TestReview_ConservationDimension`) | `internal/analysis/review/review_test.TestReview_ConservationDimension -> internal/analysis/review/review.DimConservation` (syntax-only: a constant) |
| fathom | `docs/plans/test-certifications.md` | TC-10 | edge_wired (`reachable_from: TestReview_CertificationsDimension`) | `internal/analysis/review/review_test.TestReview_CertificationsDimension -> internal/analysis/review/review.DimCertifications` (syntax-only: a constant) |
| platform-engineering | `docs/plans/phase-3a-capstan.md` | P3A-3 | wired (`symbol: Attest`) | `cli/cmd/cmd.ciDoctorDiagnose -> capstan/witness/conformance/conformance.checks` (syntax-only: `checks[name]()` calls a local map the syntax pass bound to the unexported package variable of that name) |
| platform-engineering | `docs/plans/phase-3a-capstan.md` | P3A-31 | wired (`symbol: Strongest`) | the same edge |
| platform-engineering | `docs/plans/phase-3a-capstan.md` | P3A-33 | wired (`symbol: Attest`) | the same edge |

What each owner does:

- **A row whose `reachable_from` test never reaches the symbol through typed edges** (TH-1): name a
  test that does. TH-1 is re-anchored to `TestThreadNoteMapEntryAnchoredWithHash`, which runs
  `fathom thread note` through `cmdThread`.
- **A row whose symbol is a type or a constant with a `reachable_from`** (LB-F377, OC-1, OC-13, CV-24,
  TC-10): no typed call edge ever reaches a type or a constant, so the criterion cannot pass. The rows
  are unchecked with a reason naming RF-5; each re-closes when its row anchors on the function that
  does the work.
- **A row whose product wiring was a mis-bound name** (P3A-3, P3A-31, P3A-33): no product binary
  calls `witness.Attest` or `Registry.Strongest`. Call them from a product entry point, or declare
  `library: true` with a reason that predates the claim. That fix is platform-engineering's.
- **A non-Go row with a `reachable_from`**: every call edge outside Go's typed pass is heuristic today (a
  syntax-only call or a file-resolved import match, `core.EdgeConfidenceOf`), so such a row cannot pass
  `edge_wired` until its language gains a typed pass. No fleet row carries one (the count above). Its
  `test_reaches` criterion still reads every edge with its tier named.
- `library: true` and `skip_test` are not migrations: neither is added to pass this guard.
