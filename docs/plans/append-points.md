# Collision-free append points: the plan (F88)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/append-points.md` proves it. Design:
`docs/design/append-points.md` (section numbers below are its). Evidence: the F82 replay,
`docs/bench/readings/f82-overlap-replay-2026-10-19.md` (247 of 263 conflicted merges in 30 days on an append point).

Parallel lines stop colliding where they only add: a change note is a file of its own, a generated file merges by
regeneration, a language wave's foundation rows go in a sibling plan, and the extractor epoch is derived from files
nobody shares. Kit rows land in agentic-kit first and arrive here by `task sync-kit`.

## Wave 0: the record

- [x] The decision record: an ADR (its id reserved at build) records D1 to D4, amends ADR-048 decision D2 (the integrator's one epoch bump), and F88 moves from `docs/intent/futures/f86-f91.md` to a scope goal {id: AP-1, oracle: lexical, path: ADR.md, pattern: "^## ADR-[0-9]+: Collision-free append points"}

## Wave 1: change-note fragments in the kit (section 3)

- [x] `changes.md` sections 4, 6, 8 and 11: one change-note fragment per change under a directory the repo names, one heading and its lines, folded at release with `### Breaking` first, no committed `[Unreleased]` view, and why fragments are neither a generated changelog nor a second log {id: AP-2, oracle: lexical, path: third_party/agentic-kit/docs/project-intent/changes.md, pattern: "change-note fragment"}
- [ ] `release-notes -roll x.y.z -date D` folds the fragments in bytewise filename order under the pointer, computes `Release impact:`, orders the categories with Breaking first and deletes the fragments; `release-notes Unreleased` prints the same fold and writes nothing {id: AP-3, symbol: foldFragments, package: third_party/agentic-kit/cmd/release-notes, reachable_from: TestFoldFragmentsSortedBreakingFirst}
- [ ] `release-notes -check` refuses a fragment with no heading, two headings, an unknown heading or no line, an `[Unreleased]` section holding more than the pointer, and a fragment line already in a released block {id: AP-4, symbol: checkFragments, package: third_party/agentic-kit/cmd/release-notes, reachable_from: TestCheckFragmentsRefusesBadHeadingsAndStrayUnreleasedLines}
- [x] `intent-release` folds fragments at steps 1 to 5, `intent-doctor` rows 11 and 12 accept the pointer layout and count fragment lines, `intent-prune` pass 5 collapses fragments, and `kit-new` writes a fragment in a fragments repo {id: AP-5, oracle: lexical, path: third_party/agentic-kit/.claude/skills/intent-release/SKILL.md, pattern: "fragment", needs: "AP-3"}
- [x] `operating.md` section 4 (parallel agents): a parallel line adds a file and never appends to a shared one, and a generated file merges by regeneration, never by hand {id: AP-6, oracle: lexical, path: third_party/agentic-kit/docs/project-intent/operating.md, pattern: "merges by regeneration"}

## Wave 2: fathom adopts the fragments (section 3)

- [x] Today's `[Unreleased]` lines move to `docs/changelog/unreleased/`, one fragment per category, between waves; `CHANGELOG.md`'s `[Unreleased]` keeps only the pointer line {id: AP-7, oracle: lexical, path: CHANGELOG.md, pattern: "^Unreleased changes wait as fragments under docs/changelog/unreleased/", needs: "AP-2"}
- [x] `task validate` runs the fragment check {id: AP-8, oracle: lexical, path: Taskfile.yml, pattern: "release-notes -check", needs: "AP-4,AP-7"}
- [ ] `fathom plans archive` counts a row id named in a fragment as absorbed {id: AP-9, symbol: readAbsorptionFiles, package: cmd/fathom, reachable_from: TestReadAbsorptionFilesReadsChangeFragments, needs: "AP-7"}
- [ ] The custodian ADR-reference concern resolves citations in fragments {id: AP-10, symbol: adrCitingFiles, package: internal/custodian, reachable_from: TestADRCitingFilesIncludeChangeFragments, needs: "AP-7"}
- [x] The `AGENTS.md` change-records stanza, `CHANGELOG.md`'s header and the router's "what changed" row name the fragments directory {id: AP-11, oracle: lexical, path: AGENTS.md, pattern: "docs/changelog/unreleased/", needs: "AP-7"}

## Wave 3: generated files merge by regeneration (section 4)

- [ ] PROVENANCE.md's tables carry no `**total**` row {id: AP-12, symbol: RenderProvenanceTables, package: internal/extractor/tsengine, reachable_from: TestProvenanceTablesCarryNoTotalRow}
- [x] `.gitattributes` names `docs/languages.md` and `internal/extractor/tsengine/grammars/PROVENANCE.md` with `merge=fathom-regen` {id: AP-13, oracle: lexical, path: .gitattributes, pattern: "^docs/languages\.md merge=fathom-regen$"}
- [x] `task merge-drivers` sets the driver (`true`, keep the current side) and `task regen` runs every generator in a fixed order; `scripts/cloud-setup.sh` runs `task merge-drivers` {id: AP-14, oracle: lexical, path: Taskfile.yml, pattern: "^  merge-drivers:", needs: "AP-13"}
- [ ] The waves guide's integrator steps: `task merge-drivers` at clone, `task regen` after a merge, and a bump file per wave instead of the integrator's epoch bump {id: AP-15, oracle: lexical, path: docs/guides/agent-waves.md, pattern: "task regen", needs: "AP-14,AP-19"}

## Wave 4: sibling plan files for language waves (section 5)

- [ ] The support console reads row marks from `language-breadth.<wave>.md` siblings as well as `language-breadth.md` {id: AP-16, symbol: PlanRowMarks, package: internal/support, reachable_from: TestPlanRowMarksReadsLanguageBreadthSiblings}
- [ ] The worker recipe (design `language-breadth.md` section 13) puts a wave's new foundation rows in `docs/plans/language-breadth.<wave>.md`, with no F.NN number, and queue needs in the cross-plan form {id: AP-17, oracle: lexical, path: docs/design/language-breadth.md, pattern: "docs/plans/language-breadth\.<wave>\.md", needs: "AP-16"}

## Wave 5: the derived extractor epoch (section 6)

- [ ] The data digest: sha256 over the sorted paths and bytes of the embedded dialect manifests and queries and `grammars.lock.json` {id: AP-18, symbol: EmbeddedDataDigest, package: internal/extractor, reachable_from: TestEmbeddedDataDigestCoversManifestsQueriesAndLock}
- [ ] The epoch is the base 67 plus the number of `internal/fusion/epochs/*.md` bump files, then 12 hex of sha256 over the sorted bump files and the data digest; a new bump file or one data byte changes it {id: AP-19, symbol: epochOf, package: internal/fusion, reachable_from: TestEpochOfChangesWithABumpFileOrADataByte, needs: "AP-18"}
- [ ] `FingerprintHash` keeps its formula and salts with the derived epoch, so an ingest after either change re-extracts every file {id: AP-20, symbol: FingerprintHash, package: internal/fusion, reachable_from: TestFingerprintHashSaltsWithTheDerivedEpoch, needs: "AP-19"}

## Measurement (section 8)

- [ ] (op) AP-M1: 30 days after the last build row lands, the F82 replay (same script and classifier, from a clone where `task merge-drivers` ran) counts append-point conflicted merges at most 61 a month (25% of 247), also read per 100 merges (247 of 523 today) {id: AP-M1, oracle: lexical, path: docs/bench/readings/append-points-replay.md, pattern: "^AP-M1 verdict: (PASS|FAIL)", needs: "AP-8,AP-9,AP-10,AP-11,AP-12,AP-14,AP-15,AP-17,AP-20"}

## Deferred and refused (recorded so they are not re-proposed)

- [-] A blank line between plan rows {id: AP-D1, disposition: deferred, reason: "simulated over the 105 plan-conflict merges of the replay it cleared 13", trigger: "adjacent-row plan conflicts reach 10 a month after AP-17"}
- [-] `ADR.md` split into one file per record {id: AP-D2, disposition: deferred, reason: "5 append conflicts in 30 days against line-pinned fixtures (O1335)", trigger: "ADR.md append conflicts reach 8 a month"}
- [-] Per-dialect epoch salts {id: AP-D3, disposition: deferred, reason: "F73 does not build them before IXS-M2 reads under 0.5", trigger: "F73's Promote when"}
- [-] Per-dialect registration in place of the connector's shared per-language maps (`typedEdgeProducers`, `extClassMap`, `langRowHint`) {id: AP-D4, disposition: deferred, reason: "their files conflict in 7 of 247 append-point merges in 30 days", trigger: "the per-language map files conflict in 8 merges a month"}
- [-] `merge=union` for `CHANGELOG.md` or plans {id: AP-X1, disposition: refused, reason: "it keeps both sides of edits too, so a collapsed line or an edited row survives twice"}
- [-] A committed `[Unreleased]` view rendered from the fragments {id: AP-X2, disposition: refused, reason: "no reader the on-demand render does not serve, and it brings the append point back (ADR-074); wins if a consumer must read the notes from the file at HEAD"}
- [-] A lock or a per-file slot reserved for an append point {id: AP-X3, disposition: refused, reason: "F88 and F82 reject them for now; a file per change needs neither"}
