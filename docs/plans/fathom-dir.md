# The `.fathom/` directory and tech packs v1: the plan (ADR-061)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/fathom-dir.md` proves it. Design: `docs/design/fathom-dir.md`
(section numbers below are its). Decision: ADR-061 (accepted 2026-10-19 with amendments A1-A7). Goal G35. Futures: F5 (promoted), F27, F62 to F65.

Every reading here is $0 (no provider or agent spend). No row adds an MCP tool or a verb (law 4, law 10) or a store
table, cache or background job (ADR-049 decision 2). Every existing `.fathom/` file keeps its name, location and
meaning; a legacy read is never dropped by a row of this plan. ADR-061 was accepted 2026-10-19 with amendments A1-A7; accepting does not schedule (roadmap section 9.7). Symbols in
open rows are the names the design proposes; a row that lands under another name updates its anchor before it is
claimed.

## Wave 1: one resolver, one keyed schema, the local zone (sections 4 and 5)

- [x] `internal/projectdir` names every project-scope file once and joins it under a root; the hand joins in `cmd/` and `internal/` route through it, and a test refuses a new literal `.fathom` join outside the package {id: FD-1, symbol: File.Path, package: internal/projectdir, reachable_from: TestProjectDirIsTheOneSpelling}
- [x] The Config panel reads platform-module bindings from `modules.cue` through the resolver (it read `bindings.cue`, which nothing writes, so it always reported no bindings) {id: FD-2, symbol: buildConfigPanel, package: cmd/fathom, reachable_from: TestConfigPanelReadsModulesCue, needs: "FD-1"}
- [x] `schema/project` defines `#Project`, composing the existing `.fathom/` schemas by top-level key, closed, with valid and invalid testdata; `task cue-vet` vets this repository's `.fathom/*.cue` as one value {id: FD-3, oracle: lexical, path: schema/project/schema.cue, pattern: "^#Project:"}
- [x] `projectdir.Load` parses each `.fathom/*.cue` file on its own and validates each top-level key against its sub-schema: a syntax error names its file, a failing key names its file and key, every other key still loads (I9, I38); two files setting one key conflict by name; an unknown key is a doctor warning; `risk.cue` and `learning.cue` map by file name {id: FD-4, symbol: Load, package: internal/projectdir, reachable_from: TestProjectLoadIsolatesAFailingKey, needs: "FD-1,FD-3"}
- [x] `guardspec`, `bindspec`, `capability` and `custodian` read their keys from FD-4's value with no second parse, byte-identical on this repository and on the fauxtown fixtures {id: FD-5, symbol: LoadGuards, package: internal/guardspec, reachable_from: TestLoadGuardsThroughProjectValueByteIdentical, needs: "FD-4"}
- [x] One doctor row reports every `#Project` key (file, status, entry count) and any unknown key; the existing guard, modules, capabilities and concerns rows keep their names and verdicts {id: FD-6, symbol: checkProjectKeys, package: cmd/fathom, reachable_from: TestDoctorProjectRowNamesEveryKey, needs: "FD-4"}
- [x] `fathom init` and `fleet reconcile` write `.fathom/.gitignore` as a wiring-owned region (receipted, undoable) naming `owned.json`, `leases.jsonl`, `scip/`, `receipts.jsonl` and `memories.jsonl` {id: FD-7, symbol: localZoneIgnoreRegion, package: internal/wiring, reachable_from: TestInitWritesLocalZoneIgnore}
- [x] This repository's tenancy allow list moves to `internal/lint/tenancy/allow.txt`: a development artifact leaves the product directory {id: FD-8, oracle: lexical, path: internal/lint/tenancy/tenancy_test.go, pattern: "^const allowlistPath = .allow[.]txt.$"}
- [x] `docs/configuration.md` carries the four-zone table naming every file of section 2 {id: FD-9, oracle: lexical, path: docs/configuration.md, pattern: "^## Where fathom keeps things"}

## Wave 2: tech packs v1 (section 7)

- [ ] `schema/techpack` defines `#Pack`: the `pack: {name, version, about}` header the custodian's packs already carry, `guards` (the existing `#Guard`), `generated`, `formats`, `concerns` and `orientation`, with valid and invalid testdata; a kit pack carrying `layer_rules`, `capabilities`, `bindings`, `exclude_packs` or `orientation` is invalid {id: FD-10, oracle: lexical, path: schema/techpack/schema.cue, pattern: "^#Pack:"}
- [ ] `techpack.Discover` resolves the repository's Go and CUE pins offline (a local `replace`, `vendor/`, the module cache at the required version; never `go.work`) and returns each `fathom-pack/` carrying a `pack` header with its name, version, content hash and path; a pinned module absent from disk is disclosed and never fetched (I56) {id: FD-11, symbol: Discover, package: internal/techpack, reachable_from: TestDiscoverReadsPinsOfflineNeverGoWork, needs: "FD-10"}
- [ ] `techpack.RunFixtures` reads a pack's fixtures as pure data (guard scope positives and negatives, generated pairs, format samples) and returns a per-section reading; a section with no fixtures, or a failing one, is withheld and named {id: FD-12, symbol: RunFixtures, package: internal/techpack, reachable_from: TestRunFixturesWithholdsUnfixturedSections, needs: "FD-10"}
- [ ] `fathom rules fixtures <dir>` prints the reading, the harness a kit runs in its own CI {id: FD-13, symbol: cmdRulesFixtures, package: cmd/fathom, reachable_from: TestCmdRulesFixturesPrintsReading, needs: "FD-12"}
- [ ] Before an edit, discovered pack guards are served tagged `[name@version]` after the repository's guards and bound module rules, at most 2 per pack inside I35, displaced lines counted per source {id: FD-14, symbol: cmdEditHookPreFile, package: cmd/fathom, reachable_from: TestEditHookPreServesPackGuardsBelowRepoRules, needs: "FD-11,FD-12"}
- [ ] An edit to a declared generated output gets one line naming its source and the regenerate command, rendered and never run {id: FD-15, symbol: GeneratedLineFor, package: internal/techpack, reachable_from: TestEditHookPreGeneratedLineNeverRuns, needs: "FD-14"}
- [ ] `GeneratedHandEdits` reads pack and project `generated` declarations before its same-directory heuristic {id: FD-16, symbol: GeneratedHandEdits, package: internal/analysis/integrity, reachable_from: TestGeneratedHandEditsReadsDeclarationsFirst, needs: "FD-11"}
- [ ] `formats` maps a glob to a dialect id only when the dialect is embedded or trusted at user scope (ADR-048, I55); an unknown id is disclosed and ignored; the discovered packs' identity joins the ingest watermark, and only after RT-M1 reads PASS and lifts the table freeze (no new column before; A4) {id: FD-17, symbol: Classify, package: internal/fusion, reachable_from: TestClassifyHonorsPackFormatsForKnownDialectsOnly, needs: "FD-11"}
- [ ] `LoadLayerIn`'s union reads discovered packs' `layers/` with precedence embedded, then tech pack, then project; `#Layer.stratum` gains `"tech"` {id: FD-18, symbol: LoadLayerIn, package: internal/semantic, reachable_from: TestLoadLayerInUnionsTechPackLayers, needs: "FD-11"}
- [ ] A repository with no discovered pack produces byte-identical edit-hook, obligations and ingest output (a golden over fauxtown) {id: FD-19, symbol: Discover, package: internal/techpack, reachable_from: TestNoTechPackIsByteIdentical, needs: "FD-14,FD-15,FD-17"}
- [ ] The `tech-packs` capability (default on, design section 13 question 1) and `exclude_packs` in `modules.cue` {id: FD-20, symbol: Default, package: internal/capability, reachable_from: TestCapabilityTechPacksAndExcludePacks, needs: "FD-11"}
- [ ] Doctor names each discovered pack, its pin, its served and withheld sections and its exclusion; `fathom layers strata` lists the tech stratum {id: FD-21, symbol: checkTechPacks, package: cmd/fathom, reachable_from: TestDoctorTechPacksRow, needs: "FD-11,FD-12"}
- [ ] The rule-outcome log records, for a served generated line, whether the session's next edit touched a declared source (`next_edit_source`) or the output again (`next_edit_output`), from beats at zero spend {id: FD-22, symbol: recordRuleOutcome, package: cmd/fathom, reachable_from: TestRuleOutcomeNextEditAfterGeneratedLine, needs: "FD-15"}
- [ ] The custodian's shipped packs (`claude-code`, `codex`, `lefthook`, `taskfile`) load as built-in tech packs validated by `#Pack`; `packs:` in `concerns.cue` enables them and a kit pack's `concerns` alike; `fathom config audit` output is byte-identical on this repository (K10) {id: FD-25, symbol: LoadPack, package: internal/custodian, reachable_from: TestBuiltInPacksValidateAsTechPacksByteIdentical, needs: "FD-10"}
- [ ] The project pack's `orientation` entries (an anchor, a path or `path#Symbol`, plus why) load through FD-4; `fathom config audit` names every anchor that no longer resolves and never drops one silently (K9) {id: FD-26, symbol: Load, package: internal/projectdir, reachable_from: TestOrientationAnchorsResolveOrAreNamed, needs: "FD-4"}
- [ ] (op) This repository's `.fathom/pack.cue` carries an orientation set, and `AGENTS.md` routes to it with one row paid for by removing a duplicate (K8, router_thin holds) {id: FD-27, oracle: lexical, path: .fathom/pack.cue, pattern: "^orientation:", needs: "FD-26"}
- [ ] (op) Consumer #1, starts when an app-kit release carries the `pack:` header and fixtures (A7; fathom never edits a kit repository, ADR-070): the pinned app-kit pack carries a header and fixtures, and this repository's `.fathom/pack.cue` declares the observatory CSS as generated from `cmd/fathom/ui/input.css` by `task ui:css` and the tenancy allow list from its lint test {id: FD-23, oracle: lexical, path: .fathom/pack.cue, pattern: "^generated:", needs: "FD-15"}
- [ ] Starts with FD-23's app-kit release (A7). The pasted app-kit guards leave `.fathom/guard.cue`; the pinned pack serves them {id: FD-24, oracle: lexical, path: .fathom/guard.cue, pattern: "from app-kit fathom-pack", eq: 0, needs: "FD-23"}

## Wave 3: merges with legacy reads (section 6, H2)

- [x] `layer_rules` gains kind `call_direction` (from and to package globs over call edges, `allow`), checked by the code review and `arch_conformance` run today; `rules/archconformance.cue` is still read, and doctor prints its equivalent `layer_rules` entries once {id: FD-30, symbol: LoadLayerRules, package: internal/guardspec, reachable_from: TestCallDirectionMatchesArchconformanceOnFixture, needs: "FD-4"}
- [ ] `config.toml` gains `[risk]` and `[learning]` with user defaults and `config --explain` provenance, each new field named in `config.ScopeOf` with a class (`TestEveryConfigFieldHasAScopeClass`, A5); a section wins over the legacy `risk.cue` or `learning.cue`, which doctor names {id: FD-31, symbol: Load, package: internal/config, reachable_from: TestConfigRiskLearningSectionsWinOverLegacyFiles}
- [ ] `owned.json` moves to the state directory: read from `.fathom/` only when the new path is absent, written only to the new path; doctor names a stale old copy; fathom deletes neither (I63) {id: FD-32, symbol: LoadLedger, package: internal/wiring, reachable_from: TestLedgerReadsLegacyWritesStateDir, needs: "FD-1"}
- [ ] `leases.jsonl` and `scip/` move the same way {id: FD-33, symbol: leasesPath, package: cmd/fathom, reachable_from: TestLeasesReadLegacyWriteStateDir, needs: "FD-32"}
- [x] The `config.toml` switches whose field is not `ScopeUserOnly` surface as capability ids (`test-run-hook`, `pin-check`, `guard-enforce`) sourced from the same booleans; `auto-refresh` stays user-only (`daemon.auto_refresh`, I64, SEC-2) and is not surfaced (A2); `capabilities.cue` wins on conflict and doctor names the conflict {id: FD-34, symbol: Default, package: internal/capability, reachable_from: TestCapabilitiesSurfaceConfigSwitches}
- [x] The incident-maps, feature-map and stewardship designs name their files as `#Project` keys read through the one loader (ADR-061 decision 2) {id: FD-35, oracle: lexical, path: docs/design/incident-maps.md, pattern: "ADR-061"}

## Readings ($0)

- [ ] (op) FD-M1: the generated-line reading over 60 days of owner traffic from the rule-outcome log. KEEP when at least 20 fires occur and at least half are followed in-session by an edit to a declared source with at most a quarter followed by a re-edit of the output; KILL otherwise, fewer than 20 fires included; bars fixed before the window; a KILL removes the serving path in the same change {id: FD-M1, oracle: lexical, path: docs/bench/readings/fathom-dir/generated-line.md, pattern: "^FD-M1 verdict: (KEEP|KILL)", needs: "FD-22,FD-23", weight: 2}
- [ ] (op) FD-M2: pack guard effectiveness after 60 days (`RuleEffectiveness` classes per pack rule); a pack whose every rule reads never-triggered or ignored is reported to its kit's owner {id: FD-M2, oracle: lexical, path: docs/bench/readings/fathom-dir/pack-guards.md, pattern: "^FD-M2 reading:", needs: "FD-14"}
- [ ] (op) FD-M3: consumer #1 before and after on this repository: the packs discovered from the pins, their fixture readings, `.fathom/layers/ui-appkit/` (the drifted copy) deleted in favor of the pinned layer, and the edit-hook payload p90 inside I35 before and after {id: FD-M3, oracle: lexical, path: docs/bench/readings/fathom-dir/consumer-one.md, pattern: "^FD-M3 verdict: (PASS|FAIL)", needs: "FD-23,FD-24"}

## Not planned (recorded so it is not re-proposed)

- Committing derived index snapshots to any repository or under `.fathom/`: rejected (K5, ADR-061). A shared cache
  keyed by commit waits as F27.
- A `fathom pack` verb, an MCP tool, a pack registry, a fetch verb or a pack host: refused (laws 4 and 10, I56, F2).
- A `fathom migrate` verb: refused while every legacy read stays; weighed only when a release proposes dropping one.
- Moving `.fathomignore` into `.fathom/`, or user-scope state under `~/.fathom/state/`: refused (convention and
  compatibility cost, no reader gain).
- Kit packs carrying `layer_rules`, `capabilities`, `bindings` or `exclude_packs`, and any pack but the project's
  carrying `orientation`: refused (a kit advises; only the repository governs). A kit pack MAY carry `concerns`
  (ADR-061 decision 4, FD-25, applied only when `packs:` names it; A5).
- Tech pack verbs (F62), framework edges as data (F63), pack-carried maps (F64), a generated map readable without
  fathom (F65): futures with their own triggers.
