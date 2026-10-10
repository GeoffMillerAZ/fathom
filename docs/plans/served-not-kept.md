# Served is not kept: the commit pin line (ADR-057)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/served-not-kept.md` proves it. Design:
`docs/design/served-not-kept.md` (section numbers below are its). Goal G33. Futures F53 (`propose_check`) and F54
(orchestrator constraints on a sub-agent). Built: wave 1 (PR-1 to PR-7, the line in record mode), PR-9 to PR-12;
open: PR-13 (the HEAD stamp) and the readings PR-M1 and PR-M2.

The accepted core is the commit pin line (PR-1 to PR-7) with two ledger fixes that ride SR-61 (PR-10, PR-11) and
JF-1 landing alone. PR-8 (a silent pin loss blocks at pre-push) was accepted on 2026-09-29 by the owner-architect and confirmed by the owner the same day (Q34
answered); PR-9 builds it. K4 fired on PR-M0's replay (O161): PR-12 exempts a retired pin only when the range
cites a decision naming it. PR-M0 is
a $0 replay that must pass before PR-3 is built (kill K1); PR-M1 is 30 days of record mode; PR-M2 is a bench that
needs the owner's approval of about $76 (`docs/intent/autonomy.md`). The rows after "Deferred and refused" record
what was considered so it is not re-proposed.

## Wave 0: facts and the $0 replay

- [x] (op) PR-V1: whether Claude Code and Codex PostToolUse input for a shell command carries its stdout (git's `[branch sha]` summary line) and whether context added there reaches the model {id: PR-V1, oracle: lexical, path: docs/bench/readings/served-not-kept/hook-facts.md, pattern: "^PR-V1 reading:"}
- [x] (op) PR-M0: the $0 replay over guards-pilot 6 and 8 cell diffs flags every violating i28 cell and no oracle-clean cell, and publishes the fragment evaluators' would-have-fired and false-fire counts {id: PR-M0, oracle: lexical, path: docs/bench/readings/served-not-kept/pr-m0-replay.md, pattern: "^PR-M0 verdict: (PASS|FAIL)"}

## Wave 1: the commit pin line

- [x] A storeless pin-loss check over base to head: the CV-7 guarded set without reach, returning only loss kinds and never `edited`; a test moved unchanged to another changed file is not a loss {id: PR-1, symbol: PinLosses, package: internal/conserve, reachable_from: TestPinLossesI28RenameIsLossAndMoveIsNot, needs: "PR-M0"}
- [x] A guard's `done needs:` tests join the guarded set under their own reason {id: PR-2, symbol: GuardedTests, package: internal/analysis/integrity, reachable_from: TestGuardedTestsIncludesGuardDoneNeeds}
- [x] On a `git commit` seen by PostToolUse(Bash), the hook reads the sha from git's summary line, checks parent to commit, and adds one line per lost pin naming the test, its naming source, the loss kind and a head location; silent when nothing is lost; no waiver text {id: PR-3, symbol: commitPinLine, package: cmd/fathom, reachable_from: TestCommitPinLineNamesSourceAndIsSilentOnKeep, needs: "PR-1,PR-2,PR-V1"}
- [x] The stage reads blobs through one `git cat-file --batch` process, stays inside a 250 ms sub-budget and its descriptor deadline (I53), and on a miss skips silently with a `skipped` beat, never a kept one {id: PR-4, symbol: runCommitPinWithDeadline, package: cmd/fathom, reachable_from: TestCommitPinSkipsSilentlyOverBudget, needs: "PR-3"}
- [x] One `pin` beat per checked commit in hooks.jsonl carries session, commit, loss kinds, test ids, source cites and ms, and no source text {id: PR-5, symbol: emitPinBeat, package: cmd/fathom, reachable_from: TestPinBeatCarriesNoSourceText, needs: "PR-3"}
- [x] `[hooks] pin_check` is `off`, `record` (default) or `feedback`, and `off` and `record` leave the payload byte-identical {id: PR-6, symbol: Load, package: internal/config, reachable_from: TestConfig_HooksPinCheckDefaultsRecord, needs: "PR-3"}
- [x] `docs/configuration.md` documents `[hooks] pin_check` and what the line does and does not do {id: PR-7, oracle: lexical, path: docs/configuration.md, pattern: "pin_check", needs: "PR-6"}

## Wave 2: the pre-push gate (owner decision)

- [x] (op) PR-8: the owner accepts or rejects blocking a silent pin loss at pre-push, recorded on the `PR-8 decision.` line of ADR-057 (Q34) {id: PR-8, oracle: lexical, path: ADR.md, pattern: "^\*\*PR-8 decision\.\*\* (accepted|rejected)"}
- [x] `fathom conserve --strict-plane integrity` exits 1 only on a loss whose naming source still names the test at head, prints a retired pin for review without failing, and lefthook's refactor-conserve uses it {id: PR-9, symbol: cmdConserve, package: cmd/fathom, reachable_from: TestConserveStrictIntegrityBlocksOnlySilentPinLoss, needs: "PR-1,PR-8"}
- [x] A retired pin is exempt at pre-push only with a decision cite in the range: an added line naming the test in a decision record, or a dated decision line; plan rows never count (ADR-057 K4, O161) {id: PR-12, symbol: decisionCites, package: internal/conserve, reachable_from: TestPinLossRetirementNeedsDecisionCite, needs: "PR-9"}

## Wave 3: ledger rows that ride SR-61

- [x] The enforce deny path writes `blocked` to the rule-outcome ledger {id: PR-10, symbol: recordBlockedOutcome, package: cmd/fathom, reachable_from: TestEnforceDenyWritesBlockedOutcome, needs: "surface-reckoning.md#SR-61"}
- [x] A rule served by the pre hook still writes its `served` row on the paired post call; the guard stamp keeps suppressing only the SR-48 ask {id: PR-11, symbol: trackRuleOutcomeAt, package: cmd/fathom, reachable_from: TestPreServedRuleStillRecordsServed, needs: "judgment-foundations.md#JF-1"}
- [ ] A PreToolUse(Bash) stamp of HEAD before a git command lets the pin check find the new commit without git's summary line, the `no_summary` skip share falls under 10% (744 of 745 checks skipped to 2026-10-10) {id: PR-13, symbol: stampHeadBeforeBash, package: cmd/fathom, reachable_from: TestPinCheckFindsCommitWithoutSummaryLine}

## Wave 4: readings

- [ ] (op) PR-M1: 30 days of record mode on owner traffic, every fire adjudicated by the owner, K2 not tripped, stage p95 at most 250 ms, zero new I53 misses, skip rate named {id: PR-M1, oracle: lexical, path: docs/bench/readings/served-not-kept/pr-m1-owner-traffic.md, pattern: "^PR-M1 verdict: (PASS|FAIL)", needs: "PR-6"}
  <!-- PR-M1's 30-day record clock restarts when PR-13 lands: a record of 99.9% skips measures nothing (ADR-067). -->
- [ ] (op) PR-M2: the bench, about $76 of agent spend, held with all bench spend since 2026-09-29 (the owner's earlier approval is suspended, `docs/intent/autonomy.md`): part A is the enforced-arm rerun ADR-038 owes, part B is the pin line in feedback mode against the same config with the line off, oracle-graded, 5 trials per task per arm {id: PR-M2, oracle: lexical, path: docs/bench/readings/served-not-kept/pr-m2-bench.md, pattern: "^PR-M2 verdict: (PASS|FAIL)", needs: "PR-3,PR-M0"}

## Deferred and refused (recorded so they are not re-proposed)

- [-] `test_ran`: a pinned test ran and passed after the last in-scope edit {id: PR-D1, disposition: deferred, reason: "85 to 94 percent base rate on code scopes, a false-kept hole on [no tests to run], and pre-push already runs every test (docs/bench/readings/served-not-kept/test-ran-base-rate-2026-09-29.md)", trigger: "a served-population reading under 70 percent on code scopes that the pin line does not already cover"}
- [-] Turn-end feedback through a one-shot Stop block {id: PR-D2, disposition: deferred, reason: "no verified in-turn channel and Stop runs at 7.1 s of 8 s", trigger: "pin losses from sessions that ended without a commit, and a PR-V2 reading of Stop block behavior"}
- [-] `enforce: warn` for guards and the FATHOM_ENFORCE=warn bench arm {id: PR-D3, disposition: deferred, reason: "every checkable guard here is enforce true and adopter guards carry no checks, so no rule would use it", trigger: "an adopter wants a shadow period for a checkable guard, or the owner approves arm W after PR-M0"}
- [-] `fathom claim waive` with a review receipt {id: PR-D4, disposition: deferred, reason: "nothing to waive until the line is live", trigger: "PR-M1 records 3 or more owner-judged legitimate fires in 30 days"}
- [-] The open promise set, ledger schema 2 and an internal/promise registry {id: PR-D5, disposition: deferred, reason: "no accepted check needs state across hooks", trigger: "a revived check needs it"}
- [-] GL-11 reclassification and the D-8 trigger from rule outcomes {id: PR-D6, disposition: deferred, reason: "the rule-outcome ledger has never held a row", trigger: "SR-61 deployed and 30 days of rows; exclude enforce true rules"}
- [-] Jev `propose_check` over the closed check kinds {id: PR-D7, disposition: deferred, reason: "about 21 labeled rules against a bar of 50", trigger: "J0 passes and the labeled set reaches 50, with prose rules at least half of served firings (F53)"}
- [-] Orchestrator constraints carried to sub-agents {id: PR-D8, disposition: deferred, reason: "no consumer yet", trigger: "the handoff maps exist (XM-M1 reads BUILD at 15 percent or more re-derivation and XM-M2 reads PASS) and sub-agent hooks carry agent_id, PR-V3 (F54)"}
- [-] Unserved scanning of guard and layer rules, as a receipt-only counter {id: PR-D9, disposition: deferred, reason: "it checks rules the agent never saw", trigger: "PR-M2 shows retrieval misses dominate"}
- [-] Per-agent keeping tables as a view keyed by agent identity {id: PR-D10, disposition: deferred, reason: "no volume, and I58 keys any per-author figure by agent identity, never a person", trigger: "K6 clears: at least 20 pin checks a week on registered roots"}
- [-] A model setting kept or broken {id: PR-X1, disposition: refused, reason: "R7, I62 and ADR-056 d13: a judgment never sets a status"}
- [-] A waiver or bypass command inside the feedback line {id: PR-X2, disposition: refused, reason: "a sanctioned bypass beside the fact becomes the cheapest exit, which is how i28 failed"}
- [-] Turn-end lines delivered on the next UserPromptSubmit {id: PR-X3, disposition: refused, reason: "it arrives after the agent reported done, which is the served-but-ignored channel again"}
- [-] Guard and layer evaluators over the whole file after each edit {id: PR-X4, disposition: refused, reason: "fragment-tuned checks misfire on existing content and read kept vacuously for advisory rules"}
- [-] A per-edit single-file pin check {id: PR-X5, disposition: refused, reason: "a test moved to another file reads as deleted"}
- [-] `denied_retry`, `doc_touched` and `verify_row` promise rows {id: PR-X6, disposition: refused, reason: "a tautology, a proxy for the rule, and a duplicate of the verify ledger"}
- [-] Automatic suppression of a rule's checks at a waiver rate {id: PR-X7, disposition: refused, reason: "model-written waivers would switch checking off; a human proposal instead"}
- [-] Moving the thread evaluators into a shared check registry {id: PR-X8, disposition: refused, reason: "a refactor with no consumer"}
- [-] Promises parsed from agent prose {id: PR-X9, disposition: refused, reason: "GL-X4 and ADR-041 decision 8: counts and tool events only, never inference over prose"}
- [-] A new MCP tool or top-level verb for promises {id: PR-X10, disposition: refused, reason: "none is needed: a line, a beat and a conserve flag cover it (ADR-053 holds the surface)"}
- [-] A pin inferred rather than named: a test chosen from `test_impact` reach, a blast-radius trailer or the tests near an edited file {id: PR-X11, disposition: refused, reason: "precision over coverage: a pin is only a test that a governing source already names (decision 3); an inferred pin fires on tests no rule depends on, and a false fire teaches the agent to skip the line (K2)"}
