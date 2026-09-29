# AGENTS.md is the one instruction file (ADR-059)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/agents-md.md` proves it. Design:
`docs/design/agents-md.md` (section numbers below are its). Owner decision of 2026-09-29: AGENTS.md is
the single instruction file in our repos, and fathom's product follows with one stanza, in AGENTS.md.
The build lane's reserved observation id is O138.

Ordering (design §3.4): AG-13 widens the obligations corpus, and a recompile reads non-zero guide
coverage from AGENTS.md sources, before AG-22 deletes this repository's CLAUDE files; a silent zero
is not allowed (I38, ADR-027).

## W1 harness, init, rewire, wiring

- [x] The Claude Code descriptor reads AGENTS.md natively (≥2.1.277) and CLAUDE.md, `.claude/CLAUDE.md` and `CLAUDE.local.md` in mode `import`; skills dirs are `.claude/skills` only; verified_against names 2.1.284; `schema/harness` gains mode `import` and retires `mirror`; the adoption matrix is regenerated {id: AG-1, symbol: ImportFiles, package: internal/harness, reachable_from: TestClaudeCodeReadsAgentsMDNativelyAndImportsFromCLAUDE}
- [x] One seam plans the instruction regions for init, reconcile, drift and console apply: the stanza region in AGENTS.md for every harness (`none` included), never a created CLAUDE.md {id: AG-2, symbol: instructionRegions, package: cmd/fathom, reachable_from: TestInstructionRegionsWriteAgentsMDOnlyForEveryHarness}
- [x] The first present root CLAUDE file gets one marked region whose body is the import (`@AGENTS.md`; `@../AGENTS.md` from `.claude/`), none when an unfenced import already exists {id: AG-3, symbol: claudeImportRegion, package: cmd/fathom, reachable_from: TestClaudeImportRegionGoesIntoTheFirstPresentClaudeFile, needs: AG-2}
- [x] Rewire rewrites an existing CLAUDE.md stanza region (tracked or known v1) to the import body and writes the stanza into AGENTS.md, with a receipt `fleet undo` reverses; a drifted region blocks {id: AG-4, symbol: reconcileSpec, package: cmd/fathom, reachable_from: TestRewireMovesTheClaudeStanzaToAnImportAndUndoRestoresIt, needs: AG-3}
- [x] `Region.Remove` plans deletion of a tracked, undrifted region with the removed text in the receipt; drift blocks {id: AG-5, symbol: planRegionRemoval, package: internal/wiring, reachable_from: TestRegionRemoveDeletesOwnedRegionAndUndoRestoresIt}
- [x] `init --harness=gemini-cli` sets `.gemini/settings.json` `context.fileName` to AGENTS.md and GEMINI.md as a wiring key, and writes no GEMINI.md stanza {id: AG-6, symbol: initTargetsForDescriptor, package: cmd/fathom, reachable_from: TestInitGeminiWritesContextFileNameNotAGeminiStanza, needs: AG-1}

## W2 skills, doctor, custodian

- [x] `wiring.Link` plans an owned relative symlink (absent: create; same target: skip; otherwise blocked) with receipt and undo; a real directory of fathom-owned, unedited files is replaced, a foreign entry blocks {id: AG-7, symbol: planLink, package: internal/wiring, reachable_from: TestLinkCreateSkipAndBlock}
- [x] `.claude/skills` holds the files for every mode; `.agents/skills -> ../.claude/skills` is planned only for codex, gemini-cli, agents and bare `--skills`; no writer writes through the link; a real `.agents/skills` with foreign entries blocks {id: AG-8, symbol: skillsTargetsFor, package: cmd/fathom, reachable_from: TestSkillsLinkOnlyForAgentsSkillsReaders, needs: AG-7}
- [x] Every shipped SKILL.md carries only Agent Skills frontmatter fields with name equal to its directory, so the linked bytes are valid for spec-only readers {id: AG-9, symbol: agentsVariant, package: cmd/fathom, reachable_from: TestShippedSkillsCarryOnlyAgentSkillsFields}
- [x] Custodian kind `agents_md_reaches_claude` (optional `allow_import: false`) with schema valid/invalid pairs; the claude-code pack's `claude_code_stanza_present` uses it {id: AG-10, symbol: checkAgentsMDReachesClaude, package: internal/custodian, reachable_from: TestAgentsMDReachesClaude}
- [x] Doctor `stanza_present` FAILs on AGENTS.md region cases and WARNs with remedies on a missing import, a legacy stanza copy, an ancestor CLAUDE file, the claude-md or managed-only setting, or Claude Code before 2.1.277; PASS names the load path {id: AG-11, symbol: claudeLoadPath, package: cmd/fathom, reachable_from: TestCheckStanzaPresentAgentsMD, needs: AG-10}
- [x] Doctor `stanza_loaded` SKIPs with the named agents-md reason where no root CLAUDE file exists; doctor `skills_installed` resolves the `.agents/skills` link and the obligations probe tries AGENTS.md first {id: AG-12, symbol: checkStanzaLoaded, package: cmd/fathom, reachable_from: TestStanzaLoadedSkipsByNameWithNoClaudeFile}

## W3 readers

- [x] The obligations corpus includes AGENTS.md, **/AGENTS.md, .claude/AGENTS.md and .claude/CLAUDE.md, and the exclusion note is gone {id: AG-13, oracle: lexical, path: internal/semantic/layers/obligations/layer.cue, pattern: "\*\*/AGENTS\.md"}
- [x] fathom-owned regions are never compiled as rules, and byte-identical CLAUDE.md/AGENTS.md sections in one directory compile once (AGENTS.md kept) {id: AG-14, symbol: dedupeTwinGuideSections, package: internal/semantic, reachable_from: TestDedupeTwinGuideSectionsKeepsAgentsMD}
- [x] `.claude/AGENTS.md` and `.claude/CLAUDE.md` are root guides read by one bounded stat (no floor, root tier, root-level source) {id: AG-15, symbol: isRootGuidePath, package: internal/semantic, reachable_from: TestRootGuidePathsUnderDotClaude}
- [x] Console and fleet name AGENTS.md: `repoHasGuide`/`isGoverningGuide` add the `.claude/` forms; `fleetWiringNotes` and the ladder read the AGENTS.md stanza; `rootObligationsProbe` is AGENTS.md {id: AG-16, symbol: fleetWiringNotes, package: cmd/fathom, reachable_from: TestFleetWiringNotesReadAgentsMDStanza}
- [x] An edit under a directory with a nested AGENTS.md is served that file's rule, compiled or raw, through the prompt and edit hooks {id: AG-17, symbol: probeAncestorGuides, package: pkg/client, reachable_from: TestProbeAncestorGuidesFindsNestedAgentsMD, needs: AG-13}
- [x] Bench leak list adds `.claude/AGENTS.md` and `CLAUDE.local.md`; RenderArm writes arm text into AGENTS.md when the subject has no root CLAUDE file, and `none` removes AGENTS.md too {id: AG-18, symbol: RenderArm, package: internal/scenariobench, reachable_from: TestRenderArmWritesAgentsMDWithoutClaudeFile}
- [x] `gen-agents-md` replaces `gen-claude-md` (alias with a deprecation line through 0.9.x) and defaults to `workspace/artifacts/AGENTS.md` {id: AG-19, symbol: cmdGenAgentsMd, package: cmd/fathom, reachable_from: TestGenAgentsMdDefaultsToAgentsMD}

## W4 conversion and records

- [x] ADR-059 appended and indexed; scope's compatibility line, I37, the router surface row and footer (Claude Code 2.1.284) and the domain assumption restated (I34 restates with AG-22, when this repository's router becomes AGENTS.md) {id: AG-20, oracle: lexical, path: ADR.md, pattern: "^## ADR-059: AGENTS\.md is the one instruction file"}
- [x] CHANGELOG Breaking lines and the migration note `docs/upgrades/0.9.0-agents-md-one-instruction-file.md` {id: AG-21, oracle: lexical, path: docs/upgrades/0.9.0-agents-md-one-instruction-file.md, pattern: "@AGENTS\.md"}
- [ ] This repo carries AGENTS.md only: root union at most 120 lines, nested guides moved, no CLAUDE file outside testdata and third_party, held by `one_instruction_file` in config audit; I34 restated on AGENTS.md {id: AG-22, oracle: lexical, path: .fathom/concerns.cue, pattern: "allow_import: false", needs: AG-13}
- [ ] Line-anchored citations re-pointed and re-derived at HEAD in tasks-home.json, governing-rules.json, guards-m1.json and lookups-vs.json; `TestHomeCorpusTruthAtHead` green {id: AG-23, oracle: lexical, path: testdata/indexbench/governing-rules.json, pattern: "internal/AGENTS\.md", needs: AG-22}
- [x] Adoption docs, the ten shipped skills and the living docs describe one stanza in AGENTS.md, the import region and the skills link {id: AG-24, oracle: lexical, path: docs/adoption/claude-code.md, pattern: "@AGENTS\.md"}
- [x] Rows naming the two-file layout reworded or re-anchored (AK-3, RP-1, RP-3, IO-24, EW-5, SB-8 path to `.claude/skills`, BP-32, OB-12, XM-20); IA-3 retired as refused with ADR-059 {id: AG-25, oracle: lexical, path: docs/plans/init-and-adoption.md, pattern: "id: IA-3, disposition: refused"}
- [ ] (op) AG-OP1: `fathom doctor` on this checkout after `task deploy`: stanza_present PASS "native (no CLAUDE file, default mode)"; a fresh Claude Code 2.1.284 session shows the AGENTS.md rules; `obligations` with no path serves AGENTS.md-sourced rules {id: AG-OP1, skip_test: true, skip: "operator reading after deploy; evidence is the doctor output and one session transcript line in the plan"}
- [ ] (op) AG-OP2: a Codex session in this repo invokes a fathom skill through the `.agents/skills` link, or the fallback in design §1.4 is taken {id: AG-OP2, skip_test: true, skip: "needs a live Codex session; doctor keeps discovery unverified until then"}
- [ ] (op) AG-OP3: agentic-kit and app-kit converted by their own records, then `task sync-kit` and `task vendor-check` green here {id: AG-OP3, skip_test: true, skip: "kit-side change; evidence is the synced mirror commit"}
