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

- [ ] index-backed surface matching: the audit's oracle resolves symbol → FILE, so file-scoped surface globs (`cmd/fathom/*hook*.go`, `Taskfile.yml`, `CLAUDE.md`) match plan rows instead of being reported unmatchable. Closes 12 of 14 surfaces; keeps U12's narrow rule, so it adds findings only where a row genuinely pins no test {id: RC-10, symbol: FileOracle, package: internal/intent, reachable_from: TestFileScopedSurfacesMatchViaIndex}

## Kit asks (agentic-kit)

- [ ] FA-20/FA-21 filed with this evidence: D10's `CoverageManifest` discipline has no mechanical counterpart for deterministic reads, and three Project Intent repos need one. NOT promoted yet — D13 requires two consumers and fathom is one {id: RC-11, skip_test: true, skip: "cross-repo ask; acceptance is the entry in agentic-kit/docs/plans/fathom-uplift-asks.md"}

## Kill criteria (law 5)

Delete this plan's machinery if, after 60 days, no manifest has ever been
incomplete in a real run on any adopting repo, or every incompleteness had a
single cause a narrower fix would have covered. Measured by an
`incomplete_coverage` count on the existing `intent-cli` and `doctor` beats.
