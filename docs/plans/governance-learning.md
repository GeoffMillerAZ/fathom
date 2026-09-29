# Governance that learns: the plan (vision 38, ADR-049, ADR-051)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/governance-learning.md` proves it.
Design: `docs/design/governance-learning.md` (section numbers below are its). Goals G16
(GL-M1, GL-M4) and G21 (GL-M2, GL-M3, GL-M5). U4 (rule effectiveness, ADR-051) lives here.

Risk carried from the feature audit (2026-09-27): the learning signals are empty on the
reference machine (0 memories, 0 human claim verdicts, 0 live certificate declarations).
GL-10 builds the signal source before any threshold row, and GL-M2 is not read before 30 days
of signal exist.

## Wave 1: every client's rules and hooks, reviewer output

- [x] The `RulesSource` port feeding the existing compile path; every compiled rule still passes I27; `[rules.sources]` levels `off`/`warn`/`error` (§3) {id: GL-1, symbol: RegisterRulesSource, package: internal/semantic, reachable_from: TestRulesSourceLevels}
- [x] Cursor rules: `.cursor/rules/*.mdc` frontmatter `globs` become scope, `alwaysApply` repo scope {id: GL-2, symbol: cursorRulesRead, package: internal/semantic, reachable_from: TestCursorRulesScopes, needs: "GL-1"}
- [x] Copilot instructions: `.github/copilot-instructions.md` and `.github/instructions/*.instructions.md` with `applyTo` scope {id: GL-3, symbol: copilotInstructionsRead, package: internal/semantic, reachable_from: TestCopilotInstructionsApplyTo, needs: "GL-1"}
- [x] Kiro steering: `.kiro/steering/*.md` with `inclusion` and `fileMatchPattern`; `manual` compiled but not served unasked {id: GL-4, symbol: kiroSteeringRead, package: internal/semantic, reachable_from: TestKiroSteeringInclusion, needs: "GL-1"}
- [x] `BUGBOT.md` (nested, directory-scoped), served to review verbs by default {id: GL-5, symbol: bugbotRulesRead, package: internal/semantic, reachable_from: TestBugbotRulesDirectoryScope, needs: "GL-1"}
- [x] `GEMINI.md` (nested) read as `AGENTS.md` is {id: GL-6, symbol: geminiRulesRead, package: internal/semantic, reachable_from: TestGeminiRulesNested, needs: "GL-1"}
- [x] Cursor hook codec for the flat snake_case `.cursor/hooks.json` shape; `fathom harness hooks --harness cursor` prints entries (§4) {id: GL-7, symbol: cursorCodec, package: internal/harness, reachable_from: TestCursorCodecRoundTrip}
- [x] Copilot hook codec on the harness port {id: GL-8, symbol: copilotCodec, package: internal/harness, reachable_from: TestCopilotCodecRoundTrip}
- [ ] Gemini CLI verified from a real session's beats; the descriptor leaves `unverified` only then (ADR-029) {id: GL-9, oracle: lexical, path: docs/adoption/gemini-cli.md, pattern: "^Verified live on [0-9]{4}-[0-9]{2}-[0-9]{2}"}
- [x] SARIF for obligations and review findings through `verify.RenderSARIF` with their own rule vocabularies (§8) {id: GL-20, symbol: obligationsSARIF, package: cmd/fathom, reachable_from: TestObligationsAndReviewSARIF}
- [x] A check-run payload (name, conclusion, summary, at most 50 annotations) written to stdout or a file, never posted {id: GL-21, symbol: renderCheckRun, package: internal/verify, reachable_from: TestCheckRunPayloadCapped}

## Wave 2: the signal source, rule effectiveness, learning

- [x] The signal source: served, complied, violated and blocked events per rule from the edit and PreToolUse hooks, appended to a rotated JSONL under `~/.fathom/` (16 MiB), never the transcript (§5) {id: GL-10, symbol: recordRuleOutcome, package: cmd/fathom, reachable_from: TestRuleOutcomeEventsRecorded}
- [x] Rule effectiveness (U4): per-rule counts and rates with n; classes ignored, never triggered, effective, contested; remedies as proposals (§5) {id: GL-11, symbol: RuleEffectiveness, package: internal/semantic, reachable_from: TestRuleEffectivenessClasses, needs: "GL-10", library: true, reason: "rule-effectiveness library: GL-18 (the console page) and GL-13 (promotion), both open, are the rows that call it"}
- [x] Published thresholds: `.fathom/learning.cue` schema with valid/invalid testdata, defaults in the binary (§6) {id: GL-12, oracle: lexical, path: schema/learning/schema.cue, pattern: "^#Learning:"}
- [ ] Promotion and retirement under the thresholds; retirement closes the validity window, never deletes {id: GL-13, symbol: applyLearning, package: internal/semantic, reachable_from: TestLearningPromotesAndRetires, needs: "GL-11,GL-12"}
  <!-- library landed 53143690; product wiring open: nothing reads the GL-10 outcome log, loads .fathom/learning.cue (LoadLearningThresholds) or calls applyLearning, so no claim is ever promoted or retired. GL-11's RuleEffectiveness has the same gap. -->
- [ ] Every promotion or retirement announced in the attention line and `fathom intent audit` with its signals; none silent {id: GL-14, symbol: learningAnnouncements, package: cmd/fathom, reachable_from: TestLearningNeverSilent, needs: "GL-13"}
  <!-- library landed 0a333ae5; product wiring open: the attention line and `fathom intent audit` read intent.KindLearning ledger rows, but learningAnnouncements, their only writer, is never called (waits on GL-13). -->

## Wave 3: validity windows everywhere, the rule-effectiveness view, layered diffs

- [x] Every claim-ending path (compile supersede, `fathom claim refute`, the review reconcile on the CLI and MCP, the findings importer's revocation and closure) sets `valid_to`, and a test proves none deletes (SU-19 activated) (§7); `fathom purge` is retention, not a claim-ending path, and is out of this row {id: GL-15, symbol: closeClaimValidity, package: internal/semantic, reachable_from: TestNoClaimEndingPathDeletes}
- [ ] Memories, rules-file rules, learned-rule states and doc links gain `valid_from`/`valid_to`, additively {id: GL-16, symbol: PutMemoryValidity, package: internal/store, reachable_from: TestValidityWindowsBeyondClaims}
- [ ] `as_of` on `staleness_report` and the review verbs, beside `obligations` {id: GL-17, symbol: asOfFilter, package: internal/mcp, reachable_from: TestAsOfAcrossVerbs, needs: "GL-16"}
- [ ] The rule-effectiveness page on the console (U4) {id: GL-18, symbol: renderRuleEffectiveness, package: cmd/fathom, reachable_from: TestConsoleRuleEffectivenessPage, needs: "GL-11"}
- [ ] Ignored rules proposed for a hook, never-triggered rules proposed for pruning, each with its evidence {id: GL-19, symbol: ruleRemedies, package: internal/semantic, reachable_from: TestRuleRemediesAreProposals, needs: "GL-11"}
  <!-- library landed 811c0b09; product wiring open: no verb or console page renders ruleRemedies (GL-18 is the console page). -->
- [x] `review_diff` layers changed files by blast radius (leaf, shared callee, cross-service provider) without a model (§8) {id: GL-22, symbol: layerDiffByBlastRadius, package: internal/mcp, reachable_from: TestReviewDiffLayers}

## Refused (recorded so they are not re-proposed)

- [-] Silent learning {id: GL-X1, disposition: refused, reason: "every state change is announced with its signals (ADR-049 decision 9)"}
- [-] Writing to any client's rules file {id: GL-X2, disposition: refused, reason: "law 15; fathom reads rules files, never edits them"}
- [-] Posting to a forge or running a review service {id: GL-X3, disposition: deferred, reason: "the output format is promoted; the F14 repository is not", trigger: "F14's Promote-when fires"}
- [-] Learning from prose inference over transcripts {id: GL-X4, disposition: refused, reason: "ADR-041 decision 8; counts and tool events only"}

## Measurements (op)

- [ ] (op) GL-M1: rules read from at least 6 client formats on fixtures, each compiled rule citing its source line {id: GL-M1, oracle: lexical, path: docs/bench/readings/governance/rules-formats.md, pattern: "^GL-M1 verdict: PASS", needs: "GL-2,GL-3,GL-4,GL-5,GL-6"}
- [ ] (op) GL-M2: after 30 days of signal, promotions and retirements under the published thresholds with fewer than 1 in 10 promotions refuted within 30 days {id: GL-M2, oracle: lexical, path: docs/bench/readings/governance/learning.md, pattern: "^GL-M2 verdict: PASS", needs: "GL-14"}
- [ ] (op) GL-M3: `as_of` answers for claims, memories and learned-rule states on a seeded history {id: GL-M3, oracle: lexical, path: docs/bench/readings/governance/as-of.md, pattern: "^GL-M3 verdict: PASS", needs: "GL-17"}
- [ ] (op) GL-M4: hooks verified live in at least 3 clients (descriptor state `verified` from real beats) {id: GL-M4, oracle: lexical, path: docs/bench/readings/governance/hooks-live.md, pattern: "^GL-M4 verdict: PASS", needs: "GL-9"}
- [ ] (op) GL-M5: SARIF or the check payload consumed by one external reviewer {id: GL-M5, oracle: lexical, path: docs/bench/readings/governance/reviewer.md, pattern: "^GL-M5 verdict: PASS", needs: "GL-20,GL-21"}
