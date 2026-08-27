# Coverage before conclusion (ADR-027)

Design: `docs/design/reader-coverage.md`. Grammar: `docs/plans/README.md`.
Boxes flip to `[x]` only on `fathom verify --service fathom --plan
docs/plans/reader-coverage.md` exit 0.

The proven defect: `chmod 000 docs/intent/invariants.md` makes `fathom intent
audit` report 0 invariants (from 37) and 85 unproven claims (from 169), with
nothing in `uncomputable` naming the file. Every row below is scored against
that: **no failure mode may make the repository look healthier than it is.**

## Wave A — the reading floor

- [x] `internal/readset`: `Set.Read(path)` returns bytes plus a recorded outcome — `read`, `absent`, `unreadable(errno)`, or `skipped(category)` — and `Manifest()` returns them sorted with `Complete()` false when any read failed. Mirrors `fusion.ReadFailure`'s vocabulary deliberately; does NOT import it (the ingest path's refusal semantics are its own) {id: RC-1, symbol: ReadFile, package: internal/readset, reachable_from: TestReadSetRecordsAbsentUnreadableAndSkipped}
- [x] `intent.Load` reads every corpus file through a `readset.Set` and carries the manifest on `Corpus.Coverage`; an unreadable file is NEVER a silently smaller corpus {id: RC-2, symbol: Load, package: internal/intent, reachable_from: TestUnreadableInputNeverLooksHealthier}
- [x] counts over an incomplete manifest are LOWER BOUNDS and render as such (`≥ N (k source(s) unreadable: <path>: <errno>)`) everywhere they surface: the verb's three sections, the JSON (`coverage` object with `complete`, `unreadable`, `skipped`), and `RunAudit`'s uncomputable list {id: RC-3, symbol: RenderCount, package: internal/readset, reachable_from: TestRenderCountIsALowerBoundWhenBlind}
- [x] the plan-contract reader, the ADR-citation reader and the guard reader all route through a `readset.Set`, so U11/U12/U13 cannot silently shrink either {id: RC-4, symbol: planRowContracts, package: cmd/fathom, reachable_from: TestPlanContractsNameUnreadablePlans}
- [x] **the regression test the defect never had**: for the audit, make an input unreadable and assert the output does NOT improve — invariant count not lower without a note, unproven count not lower without a note, and the path plus errno named {id: RC-5, symbol: RunAudit, package: internal/intent, reachable_from: TestUnreadableInputNeverLooksHealthier}

## Wave B — instruments that disagree or mislead

- [x] doctor's `unproven_claims` reconciles with the verb: U12 and U13 wired in (neither needs the index), U4 still skipped and NAMED, and the non-zero path carries the `NOT COMPUTED` list its zero path already carried. Today: doctor 12, verb 88, no explanation {id: RC-6, symbol: checkUnprovenClaims, package: cmd/fathom, reachable_from: TestDoctorUnprovenReconcilesWithVerb}
- [x] `binary_fresh` distinguishes an UNREADABLE `go.mod` from one declaring another module: today it reports "go.mod does not declare github.com/geoffmilleraz/fathom" against a repo whose go.mod declares exactly that on line 1, then SKIPs instead of FAILing {id: RC-7, symbol: fathomSourceTree, package: cmd/fathom, reachable_from: TestFathomSourceTreeDistinguishesUnreadableFromForeign}
- [x] `guard_dead_scopes` stats the filesystem before calling a scope dead: a scope matching real files fathom does not INDEX (`cmd/fathom/ui/**` — three CSS files) is reported as unindexed-not-dead, and its remedy no longer says to delete a correct guard {id: RC-8, symbol: filesOnDiskUnder, package: cmd/fathom, reachable_from: TestDeadScopeDistinguishesUnindexedFromAbsent}
- [x] U12's `NOT COMPUTED` output collapses: one line naming the surfaces it could not match and why, never eleven near-identical lines drowning the section a reader most needs {id: RC-9, symbol: SurfaceRowsUncovered, package: internal/intent, reachable_from: TestUncomputableNotesCollapse}

## Wave C — capability

- [x] U14 `surface_globs_match_nothing`: a sensitive-surface row whose globs match NO file in the tree has stopped guarding, silently — no check, no build error, no test would notice. Found live: the router declared `internal/instrument*`, `internal/flywheel*` sensitive because it holds "the numbers the program steers by (law 12)"; neither path exists, the code is in `internal/metrics`, and the repo's most measurement-critical surface had been unguarded for as long as the row did. "Could not look" is never "matched nothing" {id: RC-12, symbol: SurfaceGlobsMatchNothing, package: internal/intent, reachable_from: TestSurfaceGlobsMatchNothingIsAFinding}
- [x] the router's `Done needs` column names TESTS as symbols, not prose. Six rows named a real test in words a machine cannot resolve ("registry–doc sync test", "the brief byte cap test", "the carry-every-field test"); each is now the verified symbol. Surfaces naming no `Test*` fell 12 → 6, and fixing the dead glob above surfaced 9 genuinely unpinned rows in measurement code that U12 could not previously see (62 → 71) {id: RC-13, skip_test: true, skip: "docs row; acceptance is `fathom intent audit` reporting 6 surfaces without a named test where it reported 12, and U14 at 0"}
- [-] index-backed surface matching (symbol → FILE via the oracle) so file-scoped globs match package-scoped plan rows. Deferred, and RC-12/RC-13 are why: the real blocker was never matching, it was that the router named its tests in PROSE — a surface U12 could match but whose `Done needs` names no test lets it say a row is unpinned without saying what to pin it to. Naming the tests cost a docs edit and halved the gap; the index work would have bought the other half at the cost of a second probe per audit. Revisit if the remaining 4 all-file-scoped surfaces start carrying claimed rows {id: RC-10, disposition: deferred, reason: "the cheaper fix (RC-13) closed most of the gap; the index probe is not yet earned", trigger: "a claimed plan row lands in a surface whose globs are all file-scoped"}

## Kit asks (agentic-kit)

- [x] FA-20/FA-21 filed with this evidence: D10's `CoverageManifest` discipline has no mechanical counterpart for deterministic reads, and three Project Intent repos need one. NOT promoted yet — D13 requires two consumers and fathom is one {id: RC-11, skip_test: true, skip: "cross-repo ask; acceptance is the entry in agentic-kit/docs/plans/fathom-uplift-asks.md"}

- [x] the last two conflation sites: the ADR-citation concern reports `unable-to-check` naming the file rather than a clean hold it never earned, and the Codex-config check distinguishes "no config" from "a config that exists and could not be read" — a reader told "no config found" never thinks to check permissions {id: RC-14, symbol: checkADRReferencesResolve, package: internal/custodian, reachable_from: TestADRReferencesNamesUnreadableCitingFiles}

## Kill criteria (law 5)

Delete this plan's machinery if, after 60 days, no manifest has ever been
incomplete in a real run on any adopting repo, or every incompleteness had a
single cause a narrower fix would have covered. Measured by an
`incomplete_coverage` count on the existing `intent-cli` and `doctor` beats.
