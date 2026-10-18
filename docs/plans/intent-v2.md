# Intent practice v2 — plan (ADR-074)

Request (owner, 2026-10-17): "I think I saw both INTEND.md and INTENT.md. We should consolidate that too, right?" ...
"I think every time we have planning discussions and decisions and things like that, I think it's just as important to
keep track of the rejected ideas." ... "I almost feel like it's a better standard to write these things in data with
contracts, and then have some sort of a template view if it ever wants to be viewed by humans" ... "I really don't
think we should be doing things because I've said so."

Decision: ADR-074 (fathom's part of the intent-practice-v2 design, V1 to V11 and T1 to T4; research `wf_888f8f2d-57b`,
review `wf_a6ee03dd-707`). The records pass (the ADR, the dated lines under ADR-018 and ADR-026, F9 measured, F86, the
futures repair, the narrowed non-goal, the deprecation) landed with this plan; the rows below are the code wave. Kit-side
work (doctrine, templates, the bounded review script, the intent-design docs-search step) is agentic-kit's plan, not this
one. Grammar: `docs/plans/README.md`. Symbols are provisional until grounded with `find_symbol` at row start; a new
function named here is the row's intended anchor, and the row is re-anchored, never waived, if the code lands elsewhere.
No store table, cache or background job (the table freeze), and no model spend.

## A. Retire the unread view, check the read one (ADR-074 decision 1)

- [x] `INTENT.md` is deleted and the `intent-render-check` gate step leaves `Taskfile.yml`'s `validate:unleased`; `fathom intent render --playbook` prints a deprecation notice naming ADR-074 on stderr, writes nothing and exits 0 for one minor release (Breaking: the region leaves the compatibility surface; the `### Breaking` line and `scope.md` item change in the same commit) {id: IV-1, gone: pattern "intent-render-check", paths: "Taskfile.yml"}
- [x] `fathom intent render --playbook` is gone: no `cmdIntentRender` and no `intent render` dispatch remain under `cmd/`; `fathom intent render` answers as any unknown `intent` subcommand does {id: IV-10, needs: IV-1, gone: pattern "cmdIntentRender", paths: "cmd/"}
- [x] `fathom intent index --check`: every `## ADR-NNN` heading of `ADR.md` has exactly one row in `docs/intent/decisions/README.md` and every row names a heading; drift exits 1 naming each id and which side lacks it; a step in `task validate` {id: IV-2, symbol: cmdIntentIndex, package: cmd/fathom, reachable_from: TestCmdIntentIndex_CheckNamesRowHeadingDrift}

## B. Edit-time intent, generated (ADR-074 decision 6; T4)

- [x] `fathom intent export --guard --write` writes `.fathom/guard.cue`'s intent entries from the router's sensitive-surface table, each with its `intent: <glob> @ <router blob sha>` provenance note, leaving hand-written entries untouched; `--check` exits 1 naming the first drifted entry and runs in `task validate` (one map, not two) {id: IV-3a, symbol: cmdIntentExport, package: cmd/fathom, reachable_from: TestIntentExportGuard_CheckFailsOnDriftWriteIsIdempotent}
- [x] An edit under a generated entry is served the invariant ids plus each one's rung (I19 with rung test, say) in at most 2 of the 6 slots; I35's count, word and payload caps are unchanged and `TestObligationsCorpusNeverWidensToIntent` stays green {id: IV-3b, needs: IV-3a, symbol: renderPreEdit, package: cmd/fathom, reachable_from: TestRenderPreEdit_IntentIdsAndRungInAtMostTwoSlots}
- [x] Repo-wide invariants (those no surface row names) are listed once in the session-start primer and never at an edit; the whole-primer cap holds {id: IV-3c, needs: IV-3a, symbol: composePrimer, package: cmd/fathom, reachable_from: TestComposePrimer_NamesRepoWideInvariantsOnce}
- [x] The stanza's obligations sentence names what is delivered (the governing rules, and the invariant ids of the surfaces `guard.cue` covers), never "matched invariants injected on every edit"; the fathom-owned stanza region is regenerated here and its digests re-recorded {id: IV-4, symbol: stanzaHookSentences, package: cmd/fathom, reachable_from: TestStanzaIntentSentenceNamesWhatIsDelivered}

## C. CUE declares the parsed form, the gate vets it (ADR-074 decisions 4 and 5)

- [x] `schema/intent` (a `schema.cue` with `testdata/valid.cue` and a commented `testdata/invalid.cue`) declares a future's parsed form: Confidence (`likely|plausible|speculative`), Horizon (`next|later|someday`), optional Value (`low|medium|high|very-high` with a why) and Basis (`measured|argued|guess`, measured citing a path, instrument or commit), and its conditions; the futures parser's output vets against it, and `fathom intent audit` names a future whose fields fail {id: IV-5, symbol: ParseFutures, package: internal/intent, reachable_from: TestParseFutures_OutputVetsAgainstSchemaIntent}
- [x] The gate vets every future under `docs/intent/futures/`: an off-list value or a missing Confidence or Horizon fails `task validate` naming the id (baseline 2026-10-17: 8 off-list and 2 missing before ADR-074's repair, 0 after) {id: IV-5b, needs: IV-5, symbol: ParseFutures, package: internal/intent, reachable_from: TestRepoFuturesVetAgainstSchemaIntent}
- [x] `schema/plan` declares every key the parser reads (`needs`, `certified_by`, `strength`, the `oracle: ledger` keys, `accept`, `retires` among them) and a gate test round-trips every active plan through `gen-plan-cue` with no loss (baseline 2026-10-17: 49 of 75 lose data); ADR-018 decision 3's claim then holds {id: IV-6, symbol: renderPlanCUE, package: cmd/fathom, reachable_from: TestGenPlanCue_RoundTripsEveryActivePlan}

## D. Honest readings (ADR-074 decision 7)

- [x] `obligations` never serves a rule whose source file no longer exists at the indexed commit: the rule is withheld and counted in `layer_coverage`, with a remedy naming the recompile, never silently dropped (I38; platform-engineering served 6 such rules on 2026-10-17) {id: IV-7, symbol: staleRuleTest, package: internal/mcp, reachable_from: TestObligationsTool_RuleFromDeletedSourceFileWithheldAndCounted}
- [x] `fathom intent audit` U4 checks a named test against the repository's own test files at HEAD, not a daemon index that may be stale; an index-only miss is never reported as a missing test (baseline 2026-10-17: 181 of 229 unproven claims were the stale index) {id: IV-8, symbol: auditInvariants, package: internal/intent, reachable_from: TestAudit_U4ReadsTheRepositoryNotAStaleIndex}

## E. Readings

- [x] (op) Does splitting `ADR.md` into per-file records force a paid adr-rules recompile? A reading with a verdict line; if it does, the split waits for the owner's spend approval (`docs/intent/autonomy.md`), and if it does not, the split is its own change (ADR-074 Revisit when) {id: IV-9, oracle: lexical, path: docs/bench/readings/intent-v2/adr-split.md, pattern: "^Verdict: (recompile needed|no recompile needed)"}
- [ ] (op) The 30-day readings, due 2026-11-17. K1: the `docs/intent` share of hook injections (baseline 1.8%); KEEP at 10% or more with injected bytes inside I35, else revert to the hand list. K6: docs search before design recalls the 6 known misses (baseline: misses E10 and c1); KEEP at 6 of 6, else revert the intent-design step {id: IV-M1, needs: IV-3b, oracle: lexical, path: docs/bench/readings/intent-v2/k1-k6.md, pattern: "^K[16]: (KEEP|KILL)", min: 2}
