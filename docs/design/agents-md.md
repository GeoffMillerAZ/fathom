# AGENTS.md is the one instruction file (design for ADR-059, plan AG-)

Base: fresh clone of `main` at f383a64a, 2026-09-29. Procedure: intent-design (router -> "new
subsystem / architecture change" row, plus the `CLAUDE.md`, `AGENTS.md` sensitive-surface row).
Owner decision, 2026-09-29: AGENTS.md is the single instruction file in our repos, and we do not
maintain CLAUDE.md. fathom's product follows: one stanza, in AGENTS.md.
Platform facts were confirmed on 2026-09-29 from code.claude.com/docs/en/memory.md and the installed
Claude Code 2.1.284 binary. Status: accepted as ADR-059; the work order is `docs/plans/agents-md.md`
(AG-). Symbols were resolved with the fathom MCP (`find_symbol`, `search`) and by reading the tree.

## 0. Conflicts and fired triggers (stated before designing)

Records consulted: the router, `invariants.md`, `scope.md`, `domain.md`, `principles.md`
(grep), `futures.md` (grep), `autonomy.md`, and `decisions/README.md`, plus ADR-026 §4,
ADR-029, ADR-049 (with its amendments) and ADR-053 in full.

**Conflicts**
1. **Compatibility surface (`scope.md`).** The surface lists "The `<!-- fathom:begin -->`…
   `<!-- fathom:end -->` stanza region in `CLAUDE.md` and `AGENTS.md`". Writing the region
   into AGENTS.md alone is a **breaking change**: a `### Breaking` line and a migration note.
2. **The obligations layer's corpus rationale.** `internal/semantic/layers/obligations/layer.cue`
   and its `schema/` twin say: "AGENTS.md is deliberately NOT included: fathom writes the same
   stanza into both files". This decision removes that premise. If our repos convert and the
   corpus stays as it is, the corpus holds **no instruction file at all**. `obligations` would
   then silently lose every root and nested guide rule, a silent zero that ADR-027 and I38
   forbid. The corpus must widen in the same wave, and before our repos convert (§4 ordering).
3. **Plan row IA-3** (`init-and-adoption.md`, open) says the stanza is "emitted into both
   CLAUDE.md and AGENTS.md". That contradicts the decision. It retires to `[-] refused` and
   points at ADR-059.
4. **Claimed rows whose text names the two-file layout:** AK-3, RP-1, RP-3, IO-24, EW-5, SB-8,
   BP-32 and OB-12. XM-20 is open and needs the same treatment. The capabilities survive, so
   these rows are reworded or re-anchored in the same change, as ADR-053 decision 5 does. SB-8's
   lexical path moves from `.agents/skills/...` to `.claude/skills/...`.
5. **Invariant I34 ("`CLAUDE.md` ≤120 lines") and I37** name CLAUDE.md. Both are restated on
   AGENTS.md. I37's `stanza_loaded` evidence clause also changes (item 8).
6. **Router sensitive-surface row** "stanza paragraphs identical in both files (`intent-doctor`
   15)". This row is replaced. agentic-kit's `intent-doctor` check 15 and `intent-init` ("add the
   project stanza to CLAUDE.md and AGENTS.md") are kit-side and change in the kit (§3.3).
7. **The ADR-029 Claude Code descriptor is now false on two counts.** It lists CLAUDE.md as
   `native` and AGENTS.md as `mirror`, but AGENTS.md is native since 2.1.277. It lists
   `.agents/skills` as a skills directory, but Claude Code discovers only `.claude/skills`.
   ADR-029 makes adapters data, so this is a data correction under ADR-029, not a supersession.
   Its trigger "Claude Code stops being the interchange envelope" has not fired: the hook
   envelopes are untouched, and so are the hook goldens.
8. **PC-25 `stanza_loaded` (claimed).** InstructionsLoaded does not fire for AGENTS.md when
   Claude Code loads it through the setting. In a repo with no CLAUDE.md, the row can therefore
   never PASS. It keeps its rule (SKIP, never FAIL) and gains a named reason. It never
   synthesizes a PASS.

**Fired triggers**
- `domain.md` assumption: "The obligations corpus stays tiny and binding (`CLAUDE.md`, `ADR.md`,
  `docs/plans/README.md`)". The file list is now false and is restated to AGENTS.md. The intent
  of the assumption (tiny and binding) still holds.
- `domain.md` harness-envelope assumption: its tripwire is "the router footer's tool versions".
  The footer reads "Claude Code 2.1.245". A platform behavior that fathom's wiring depends on
  changed at 2.1.277, so the footer moves to Claude Code 2.1.284. The envelope assumption itself
  holds.
- ADR-049's consequence, "the obligations corpus widens only to files a client actually loads".
  Claude Code now loads AGENTS.md, so ADR-049 licenses the widening. No record conflicts with it.

**Checked and clear**
- Non-goals: none is crossed.
- Futures: none is matched. F2's plugin bundle carries its own `skills/`, and F19 is untouched.
- Autonomy: user-global configuration is ask-first. Consequences:
  - fathom never writes the `agents-md@builtin` setting. That setting is ignored in project
    settings anyway.
  - The conversion never touches `~/.claude`, `~/.agents` or `~/.codex`.
- Law 15 / I13: every new write goes through `internal/wiring` with a receipt.
- ADR-011: nothing org-specific.

## 1. Product rule: init, rewire, doctor, Codex, skills

### 1.1 What init and rewire write
1. **One stanza, in AGENTS.md.** These paths plan the region into `AGENTS.md` only:
   - `fathom init` for every `--harness`, including `none`;
   - `fleet reconcile --apply`;
   - the console's apply (`serviceWiringSpec`).

   AGENTS.md is created if it is absent. **No path creates a `CLAUDE.md`.**
2. **CLAUDE.md is at most an import.** Look at the repo root for `CLAUDE.md`,
   `.claude/CLAUDE.md` and `CLAUDE.local.md`, in that order, and take the first one present (the
   "root CLAUDE file"). Init plans **one** fathom-marked region in that file, whose body is the
   single import line:
   - `@AGENTS.md` in `CLAUDE.md` and `CLAUDE.local.md`;
   - `@../AGENTS.md` in `.claude/CLAUDE.md`, because Claude Code resolves imports relative to
     the importing file.

   The markers are law 15's ownership claim. The body is one line and is never a second stanza.
   No region is planned when an unfenced import line resolving to the root AGENTS.md already
   sits outside a region.

   Interpretation: "the root CLAUDE.md" in the owner rule means that first-present file. That is
   the only reading compatible with "never creates a CLAUDE.md".
3. **Migration: rewire removes the CLAUDE.md copy and adds the import, in one item.** The desired
   body of the existing CLAUDE.md stanza region becomes the import line. The region may be a
   tracked v3 region whose digest matches, or a v1 body adopted through `Region.Known` or
   `AdoptUntracked`. The wiring engine already rewrites a region body with a receipt, and
   `fleet undo` restores the stanza.
   - A drifted region is BLOCKED and named (law 15).
   - If a hand-written import already exists outside the region, the stack's one engine
     addition applies: `Region.Remove` plans deletion of the tracked, undrifted region. The
     receipt holds the removed text, and drift blocks the deletion.
   - fathom never deletes an adopter's CLAUDE.md, because it did not create that file. When the
     file carries nothing but fathom's import region, doctor says so ("deleting it lets Claude
     Code read AGENTS.md natively").
4. **Service identity.** `stanzaNamedService` reads AGENTS.md first. The import region names no
   service.
5. **Gemini CLI.** Gemini reads AGENTS.md only through the `context.fileName` settings key, and
   that key replaces the default list. So `init --harness=gemini-cli` plans
   `.gemini/settings.json` `context.fileName: ["AGENTS.md", "GEMINI.md"]` as a wiring key with a
   receipt, instead of writing a stanza into GEMINI.md. This stays within ADR-029 decision 5: a
   repo-scoped file, written only on the explicit flag.

### 1.2 Doctor
The `stanza_present` row is rebuilt. It uses `doctorWarn`, which does not fail the gate. The row
checks, in order:
- **FAIL** (remedy `fathom init --path <root>`): AGENTS.md is absent, has no region, or has a
  corrupted marker pair. These are today's three cases, now read on AGENTS.md.
- **WARN** (remedy `fathom init --path <root>`, which adds or rewrites the import region):
  - the root CLAUDE file exists without the import; or
  - it still carries a fathom stanza region (the legacy two-copy layout).
- **WARN** (remedy names the file and the setting; ask-first, and fathom never writes it):
  - A `CLAUDE.md`, `.claude/CLAUDE.md` or `CLAUDE.local.md` exists in an **ancestor** directory
    outside the repo, which under the default mode suppresses every AGENTS.md below it.
    `~/.claude/CLAUDE.md` does not count.
  - The effective `pluginConfigs["agents-md@builtin"].options.instructionFiles` is
    `claude-md` or `managed-only`, and no import exists. Doctor reads the managed settings
    first, then the user settings; project settings are ignored.
  - No import exists, and `claude --version` reports a version before 2.1.277. The check is
    bounded and cancellable. An unresolvable binary prints "version unknown" and is never
    guessed.
- **PASS**: the reason names the path AGENTS.md takes. The possible reasons are:
  - "native (no CLAUDE file, default mode)";
  - "imported by CLAUDE.md";
  - "mode claude-md-and-agents-md".

  WARN also applies in and-mode if a legacy duplicate stanza would load twice.

The ancestor walk and the settings read are doctor's alone. The repo-scoped half is the new
custodian kind `agents_md_reaches_claude`. In the claude-code pack it replaces the check of
`claude_code_stanza_present`, and the concern id is kept so law-13 overrides keep working. The
kind holds in two cases:
- no root CLAUDE file exists; or
- the first present root CLAUDE file carries an unfenced import resolving to the root AGENTS.md.

With `allow_import: false` it also requires that no CLAUDE file exists anywhere in the
agent-config corpus. Our three repos set that option.

Because the kind is file-local, the existing FileChanged `config audit --hook` catches `/init`
writing a CLAUDE.md mid-session. It raises an attention line at that moment, which is useful
because `/init` still writes CLAUDE.md.

The `stanza_loaded` row:
- In a repo with no root CLAUDE file, it SKIPs with this reason: "Claude Code (≥2.1.277) loads
  AGENTS.md through the agents-md setting without an InstructionsLoaded event; no harness-side
  evidence exists here; stanza_present covers the file side".
- Where `stanza_present` WARNs that Claude Code will not load AGENTS.md (an ancestor CLAUDE file,
  the `claude-md` setting, a release before 2.1.277), the same SKIP says so instead: "no
  harness-side evidence exists here, and stanza_present warns Claude Code will not load
  AGENTS.md: <its reasons>", with its remedies. The named reason above claims a load only where
  nothing warns.
- A beat for AGENTS.md arriving through a CLAUDE.md import PASSes.

The InstructionsLoaded hook stays wired: it still fires for CLAUDE files, for imports, and for
nested CLAUDE.md.

### 1.3 Codex
Codex reads AGENTS.md natively. It merges from the repo root down to the working directory,
honors `AGENTS.override.md`, and applies a 32 KiB `project_doc_max_bytes` cap. Its skills live in
`$REPO_ROOT/.agents/skills` (and cwd-to-root, plus `~/.agents/skills`), per
developers.openai.com/codex/skills.md as cited in `docs/adoption/codex.md` §1.1. It does not read
`.claude/skills`.

`init --harness=codex` is unchanged for the stanza, which goes to AGENTS.md. Its skills get the
link described in §1.4.

Converting nested CLAUDE.md files to nested AGENTS.md is a strict gain for Codex: nested
CLAUDE.md files were invisible to it, and now it reads them from the root down to cwd. The
`codex_agents_md_fits` concern (32 KiB) stays as it is.

### 1.4 Skills layout
- **`.claude/skills/<name>/SKILL.md` is canonical.** It holds real files, for every `--skills`
  mode. A new test pins every shipped `SKILL.md` in `skillsdist.FS` to the Agent Skills spec
  fields (`agentSkillsFields`) with `name` equal to the directory. The linked bytes are therefore
  valid for spec-only readers. All ten comply today: they carry only `name` and `description`.
- **`.agents/skills` is a relative directory link to `../.claude/skills`.** It is a single link,
  so a new skill reaches Codex without a rewire. It is written only when the selected harness
  set includes a harness whose descriptor reads `.agents/skills` and not `.claude/skills`:
  `codex`, `gemini-cli`, the generic `agents` tier, and the modes that stand for them (bare
  `--skills` = `all`, `codex`, `agents`). For `claude` it is not written at all, and neither for
  cursor or copilot, which read `.claude/skills`.
- **The link is a wiring-owned object.** The new kind `wiring.Link` behaves like this:
  - absent: create it;
  - same target: skip;
  - a real directory or a different target: BLOCKED, never replaced.

  The receipt records the link, and `fleet undo` removes it.
- **No writer writes through the link.** `installSkillsReport`, `skillOwnedFiles` and
  `ownedSkillFiles` lstat `.agents/skills` and treat a link to `.claude/skills` as one owned
  vehicle. Nothing is double-counted in drift, and nothing flip-flops between the Claude and
  agents renderings.
- **Migration of a real `.agents/skills` directory.** The fathom-owned SKILL.md files there
  (marker present, digest equal to the ledger) are removed, and the directory is replaced by the
  link. This happens only when no foreign entry remains. Otherwise the plan is BLOCKED and names
  the foreign entries: never a merge.
- `fathom plugin render` keeps `renderSkill(skillsTargetAgents, …)`. The plugin bundle is its
  own tree.
- **Honest limits.**
  - Whether Codex follows a directory link has not been verified on this machine. The skills
    discovery itself is already marked unverified.
  - Doctor's codex `skills_installed` row keeps saying "discovery unverified" until a Codex
    session invokes a skill through the link. That is op row AG-OP2.
  - If Codex does not follow the link, AG-7 falls back to the spec-only rendered copy, which is
    today's behavior, for Codex only.
- **Repo evidence for the link.** This repo's `.claude/skills/fathom-plan/SKILL.md` and
  `.agents/skills/fathom-plan/SKILL.md` already differ from each other and from
  `dist/agents/skills`. Two copies drift, and a link cannot.

## 2. Every place fathom treats instruction files (exact symbols)

Legend:
- **CHANGE**: edit needed.
- **INHERITS**: correct once the corpus or predicates change, with a test to prove it.
- **OK**: already AGENTS.md-aware.

| Area | Symbol (file) | Today | Action |
|---|---|---|---|
| Obligations corpus | `corpus.include` in `internal/semantic/layers/obligations/layer.cue` and `schema/semantic/layers/obligations/layer.cue` | `["CLAUDE.md","ADR.md","docs/plans/README.md","**/CLAUDE.md"]`; AGENTS.md deliberately excluded | CHANGE: add `AGENTS.md`, `**/AGENTS.md`, `.claude/AGENTS.md`, `.claude/CLAUDE.md`; delete the exclusion paragraph; extend `TestObligationsCorpusIncludesNestedInstructionFiles` (`TestObligationsCorpusNeverWidensToIntent` must still hold) |
| Stanza is not a rule | `SplitMarkdownSections` (`internal/semantic/sections.go`) | the fathom region compiles as a "rule" | CHANGE: skip `<!-- fathom:begin … -->`…`<!-- fathom:end -->` spans (fathom's own text; its truth is `stanza_tools_served`) |
| Twin dedupe | `orderSectionsByGoverningScope` (`compileorder.go`); new `dedupeTwinGuideSections` | a legacy repo carrying both files compiles every rule twice | CHANGE: byte-identical sections of `CLAUDE.md`/`AGENTS.md` in one directory compile once, keeping the AGENTS.md copy |
| Root vs nested guide | `scopeFloorDir`, `instructionFileBases` (`compile.go`); `docCompileTier` (`compileorder.go`); `isRootLevelSource` (`serve.go`); new `isRootGuidePath` shared by all three | `instructionFileBases` already has AGENTS.md (OK), but `.claude/CLAUDE.md` would floor to `.claude/` | CHANGE: `.claude/{AGENTS,CLAUDE}.md` are root guides |
| `.claude/` reach | `skipDir` (`internal/connector/connector.go`) prunes `.claude`; `probeAncestorGuides` (`pkg/client/obligations_corpus.go`) | `.claude/*.md` is never indexed | CHANGE: new `rootGuideExtras(root)` (internal/semantic): one bounded stat per literal root include, used by the compile corpus and `probeAncestorGuides`. `.claude` stays pruned (it holds worktrees and settings) |
| Raw fallback, coverage | `rawfallback.go` floors, `layercoverage.go` root check, `rulessource.go` readers | via `scopeFloorDir`/`instructionFileBases` | INHERITS: test that an edit under `internal/` gets the `internal/AGENTS.md` rule raw when uncompiled |
| Prompt hook | `cmdPromptHook` (`prompthook.go`), `promptcandidates.go` | no instruction-file literal; asks `obligations` | INHERITS |
| Edit hook | `cmdEditHook`/`cmdEditHookBashWrite` (`edithook*.go`; `edithook_pre.go:437` "never a CLAUDE* literal") | via `obligations` and the sentinel | INHERITS: `TestEditHookServesNestedAgentsMDRule` |
| Root obligations | `rootObligationsProbe` (`internal/mcp/semantic_tools.go`) | `"CLAUDE.md"` bait path | CHANGE: `"AGENTS.md"` |
| Console | `isGoverningGuide`, `repoHasGuide` (`cmd/fathom/console_projects.go`) | base name `CLAUDE.md`/`AGENTS.md`; root only | CHANGE: AGENTS.md first; add `.claude/` forms |
| Console ladder | `RealizationLadder`, `LadderFacts.StanzaNote` (`internal/console/ladder.go`) | "CLAUDE.md stanza present" | CHANGE: evidence names AGENTS.md |
| Fleet notes | `fleetWiringNotes` (`cmd/fathom/fleetreport.go`), consumed by `capabilityprobe.go`, `console_fleet.go`, `observatory*.go` | reads root CLAUDE.md only; reports "no CLAUDE.md" as unwired | CHANGE: read the AGENTS.md region; `fleetRow.StanzaNote` doc |
| Reconcile | `stanzaFiles` var, `stanzaNamedService`, `stanzaPlan`, `staleStanzaFiles`, `reconcileSpec`, `analyzeWiringFor` (`reconcile.go`); `serviceWiringSpec` (`observatory_apply.go`); `wiringDriftFor` (`wiringdrift.go`) | fixed pair `CLAUDE.md`,`AGENTS.md` | CHANGE: replace the var with one seam `instructionRegions(root, id, svc, intentOn) []wiring.Region` (the AGENTS.md stanza plus the import region when a root CLAUDE file exists); branding check reads both files |
| Init | `initTargetsFor` (`none` → both), `initTargetsForDescriptor`, `initWiringSpecFor`, `stanzaRegionsFit`, `fitStanzaBody`, `stanzaRegionFor`, `fathomStanza` doc, `printManualSteps` text (`init.go`, `inittargets.go`); `verbs.go` init summary | both files | CHANGE: via `instructionRegions`; new `claudeImportRegion` |
| Harness data | `claudeCode`, `codex`, `geminiCLI`, `agents` (`internal/harness/descriptors.go`); `Descriptor.StanzaFiles`, new `Descriptor.ImportFiles` (`harness.go`); `#InstructionFile.mode` (`schema/harness`) | claude-code: CLAUDE.md native, AGENTS.md mirror, skills `.claude`+`.agents` | CHANGE: AGENTS.md `native` (note: ≥2.1.277, default mode, no CLAUDE file); CLAUDE.md mode `import`; skills `.claude/skills` only; `verified_against` 2.1.284; `CapStanzaLoaded` note; regenerate `docs/adoption/other-harnesses.md` (`RenderAdoptionMatrix`) |
| Wiring engine | `Region`, `planRegion`, `placeRegion`, `Undo` (`internal/wiring`) | no region removal, no link kind | CHANGE: `Region.Remove`; new `Link` + `planLink` + apply/undo |
| Doctor | `checkStanzaPresent` (`doctor_packrows.go`); `checkStanzaLoaded`, `stanzaLoadedFiles` (`doctor_stanza_loaded.go`); `doctorObligationsProbeFile`, `obligationsProbeCandidates`, `checkSkillsInstalled` (`doctor.go`); `checkAgentsSkills` (`doctor_skills.go`) | both-files stanza; CLAUDE.md probe first; `.agents/skills` checked as real files | CHANGE: §1.2; new `claudeLoadPath(root)` (ancestors, effective mode, version); probe AGENTS.md first; skills rows resolve the link |
| Config audit | `checkStanzaPresent` (`internal/custodian/audit.go`) OK; `AgentConfigFiles`/`gitAgentConfigFiles` OK (the pathspec `*/AGENTS.md` covers nested and `.claude/`); `checkStanzaToolsServed` OK (reads both, still right for legacy adopters) | — | CHANGE: new `KindAgentsMDReachesClaude` + `checkAgentsMDReachesClaude` (`custodian.go`, `audit.go`, `load.go`, `custodian_schema.cue` with valid/invalid testdata); `packs/claude-code.cue`; this repo's `.fathom/concerns.cue` `router_thin` → `file: "AGENTS.md"` plus `one_instruction_file {kind: agents_md_reaches_claude, allow_import: false}` |
| Skills | `skillsTargetsFor`, `skillDest`, `installSkillsReport` (`skillsinstall.go`); `skillOwnedFiles`, `skillDriftFiles` (`reconcile_skills.go`); `ownedSkillFiles` (`wiringdrift.go`); `agentSkillsFields` | two rendered copies | CHANGE: §1.4 |
| InstructionsLoaded | `recordInstructionsLoaded` (`sessionhook_instructions.go`) | records a beat per load | OK (unchanged); only doctor's reading changes |
| gen verb | `cmdGenClaudeMd` (`genclaudemd.go`), `main.go` dispatch, `verbs.go` | writes `workspace/artifacts/CLAUDE.md` | CHANGE: `gen-agents-md` (`cmdGenAgentsMd`), default output `workspace/artifacts/AGENTS.md`; the old name stays as an alias with a deprecation line through 0.9.x (law 4: net zero) |
| Benches | `corpusBaselineFiles` (`internal/contextbench/corpus.go`); `RenderArm`, `appendInstructionsText` (`internal/scenariobench/arm.go`); `StripFathom` OK; `instructionFileNames` (indexbench) OK; `governanceBasenames` (scenariobench) OK | the leak list lacks `.claude/AGENTS.md` and `CLAUDE.local.md`; arm text is appended to CLAUDE.md, creating one if absent | CHANGE: add the two leak files. Write arm text into AGENTS.md when the subject has no root CLAUDE file: creating a CLAUDE.md would silently suppress the subject's AGENTS.md under the default mode, which would change the arm. `none` also removes AGENTS.md |
| Docs phrase tests | `TestStanzaNamesOnlyRegisteredSurfaceTools` (renders and compares both files); `TestThisRepoStanzaStaysPlainWhileItsChannelIsUnwired`; `TestCheckStanzaPresent`, `doctor_packrows_test.go`; `init_test.go`, `init_wiring_test.go`, `init_stanzabudget_test.go`, `fleetwiring_test.go`, `adopterfootprint_test.go`; `skillsinstall_test.go`, `doctor_skills_test.go`; `TestHomeCorpusTruthAtHead`; `TestLoadGoverningRules_RealFixtureParses` | assert the two-file layout | CHANGE: AGENTS.md stanza plus import body; this repo's AGENTS.md and nested AGENTS.md. Synthetic fixtures that write their own `CLAUDE.md#L…` stay, because adopters still have CLAUDE.md |

## 3. Repo conversion rules (fathom, agentic-kit, app-kit)

### 3.1 Merge and delete
- **Union, deduplicated, in one voice.** Root AGENTS.md = CLAUDE.md ∪ AGENTS.md:
  - identical sections kept once;
  - near-duplicates reduced to the more specific wording;
  - self-references ("full guidance: CLAUDE.md") dropped;
  - titles say AGENTS.md.

  There is exactly one fathom region and one app-kit region: marker pairs are kept, and the
  bodies are byte-identical today.
- **Budget.** The repo's own `max_lines` concern applies where one exists (fathom: 120, I34).
  Otherwise the project-intent living-doc budget applies (~150 lines). Codex's 32 KiB always
  applies. The project-intent stanza stays at 6 lines or fewer. Content over budget moves to a
  routed doc; it is never truncated.
- **Nested guides:** `git mv <dir>/CLAUDE.md <dir>/AGENTS.md` (keeps history).
- **Delete** every `CLAUDE.md`, `.claude/CLAUDE.md` and `CLAUDE.local.md` outside `testdata/`
  and `third_party/`. Mirrors change only through `task sync-kit` after each kit converts (I15),
  never by hand.
- **Links, not copies.** `.claude/skills` holds real files. `.agents/skills` becomes a link to
  `../.claude/skills` in any repo whose owner runs Codex.
- **Never touch global configuration.** Global symlinks (for example
  `~/.agents/skills/intent-*` pointing at `<kit>/.agents/skills/…`) keep resolving through the
  repo link.

### 3.2 fathom
- **Root:** CLAUDE.md is the base (119 lines; it already carries the appkit region, and the
  stanza is byte-identical).
  - Drop AGENTS.md's "Repo rules" list (a lossy copy of Non-negotiables) and its "Full agent
    guidance: CLAUDE.md" line.
  - Retitle to "# AGENTS.md — fathom".
  - Re-point the Path-scoped table to `cmd/fathom/AGENTS.md`, `docs/AGENTS.md`,
    `internal/AGENTS.md` and `schema/AGENTS.md`, and note that Claude Code loads them on demand
    and Codex root-down.
  - Result: ≤120 lines, about 8.3 KB.
- **Nested:** the four files above move with `git mv`.
- **Stays:** `internal/custodian/testdata/codefacts/fixture/{CLAUDE,AGENTS}.md` (test input).
- **Gates:**
  - `.fathom/concerns.cue`: `router_thin` → AGENTS.md, plus `one_instruction_file`.
  - `.fathom/guard.cue:91` already scopes both names (OK).
- **Line-anchored citations re-pointed and re-derived at HEAD:**
  - `testdata/contextbench/tasks-home.json`: `CLAUDE.md#L15`, `#L10`, `#L40`; `TestHomeCorpusTruthAtHead` re-derives.
  - `testdata/indexbench/governing-rules.json`: 40 labels across `CLAUDE.md`, `internal/`, `cmd/fathom/`, `docs/` and `schema/CLAUDE.md`.
  - `testdata/scenariobench/guards-m1.json`: `CLAUDE.md#L26`, `docs/CLAUDE.md#L72`, `internal/CLAUDE.md#L3`.
  - `testdata/scenariobench/lookups-vs.json`.
- **Living prose re-pointed:**
  - `docs/intent/**`, `docs/adoption/{claude-code,codex}.md` (§3.1 stanza, §1.1 skills, §5), `docs/use-cases/*`, `docs/usage.md`, `docs/mcp-tools.md`, `docs/architecture.md`, `docs/app-kit.md`, `docs/guides/*`, `docs/bench/README.md`, `README.md`;
  - the ten `dist/agents/skills/*/SKILL.md` texts;
  - Go comments citing `internal/CLAUDE.md` and similar;
  - `lefthook.yml` and `Taskfile.yml` comments; `scripts/*.sh` comments.
- **History left as written:** ADR bodies, released CHANGELOG blocks, `docs/dogfood-observations.md` (frozen), `.fathom/observations.md` entries, `docs/bench/scenarios-ledger.jsonl`, `docs/bench/readings/*`, `incidents.md` lines.
- **Plan rows:** claimed rows are reworded or re-anchored (item 4 of §0), and IA-3 is retired.
  After that, `fathom verify --strict` on every top-level plan exits 0.

### 3.3 agentic-kit and app-kit (kit-side; each kit records its own decision; fathom mirrors after)
- **agentic-kit**
  - Merge CLAUDE.md (152 lines) with AGENTS.md (67 lines), budget ~150 lines and ≤32 KiB.
  - Move `.agents/skills/intent-*` (today the canonical copies, with `.claude/skills/intent-*`
    as links into them) to real directories in `.claude/skills`. `.agents/skills` becomes the
    link.
  - `install-intent.sh` moves to `scripts/`. Its global symlinks keep resolving through the
    link.
  - Kit products change:
    - `intent-init` writes the project stanza into AGENTS.md only;
    - `intent-doctor` check 15 becomes "the stanza is in AGENTS.md and no CLAUDE file exists, or
      the root one imports it";
    - `docs/project-intent/README.md` §10 and "one stanza text shared by both tools";
    - `cmd/kit-new` `host_install.go` (`instructions = "CLAUDE.md"`) → AGENTS.md;
    - the host guard's denied-paths list.
- **app-kit**
  - CLAUDE.md is byte-identical to AGENTS.md (135 lines), so delete it.
  - The `.agents/skills/fathom-*` copies become the link.
  - Kit products change:
    - `cmd/appkit-mcp` `kitdocs.go` `loadExtra("claude","CLAUDE.md")` → AGENTS.md;
    - `internal/adoption/check.go` ("matching CLAUDE.md/AGENTS.md routers") → AGENTS.md, with
      CLAUDE.md absent or importing;
    - `tools/releasecandidate` file list;
    - the host-routing fixtures.
- **Order:** fathom first (product plus its own repo), then the kits, then `task sync-kit` and
  `task vendor-check` in fathom (`TestPackFilesMatchSource`).

### 3.4 Ordering inside fathom (a silent zero is not allowed)
AG-13 (corpus widening) lands and a recompile reads non-zero guide coverage **before** AG-22
deletes the CLAUDE files. Evidence: `console` knowledge coverage, or
`fathom layers status obligations` naming `AGENTS.md` sources. That makes AG-22 depend on AG-13
through `needs:`. The move changes every guide section's path, and the digest gate keys on the
section's subject, so without AG-26 that recompile pays a model call for text it already compiled.
AG-26 carries the claims of a section whose content digest and prompt identity match one compiled
at another path, re-cited to the new path and lines, with no model call. The carry is one-to-one
(`internal/semantic/carry.go`): exactly one departed subject and exactly one current section with
the digest, one scope floor, no declared-scope rules file, and one line shift that re-anchors every
citation; anything else compiles.

## 4. The record

### 4.1 ADR-059 (append to `ADR.md`; add the index row)

## ADR-059: AGENTS.md is the one instruction file; a CLAUDE.md fathom meets carries only an import (2026-09-29)

**Status.** Accepted 2026-09-29 on the owner's decision of the same date ("AGENTS.md is the single
instruction file in our repos; do not maintain CLAUDE.md"). **Type.** Product / architecture.

This record:
- corrects ADR-029's Claude Code descriptor data (instruction files, skills directories,
  `verified_against`), with no change to ADR-029's decisions;
- executes ADR-049's consequence "the obligations corpus widens only to files a client actually
  loads";
- supersedes the obligations layer's corpus note excluding AGENTS.md.

Plan: `docs/plans/agents-md.md` (AG-). Design: this document, to be committed as
`docs/design/agents-md.md`.

**Context.**
- **Claude Code.** Since 2.1.277 it reads AGENTS.md natively through `agents-md@builtin`. In the
  default `claude-md-or-agents-md` mode, a project with no `CLAUDE.md`, `.claude/CLAUDE.md` or
  `CLAUDE.local.md` in the working directory or any parent loads its AGENTS.md files where
  CLAUDE.md would load. `~/.claude/CLAUDE.md` does not count. InstructionsLoaded does not fire
  for an AGENTS.md loaded that way. Only `.claude/skills` is discovered. The setting is honored
  only in user and managed settings.
- **Codex.** It reads AGENTS.md from the root down and skills from `.agents/skills`.
- **fathom today.** fathom writes one stanza into two files and renders skills twice, and the
  in-repo copies have already drifted. Its obligations layer compiles CLAUDE.md and excludes
  AGENTS.md, so converting a repo would empty its governing corpus silently.

**Decision.**
1. **One stanza, in AGENTS.md.** Every write path puts the stanza region in AGENTS.md only:
   `init` (every `--harness`), `fleet reconcile --apply` and the console apply. For Gemini CLI,
   init also sets `context.fileName` through the wiring engine. No path creates a CLAUDE.md.
2. **CLAUDE.md is at most an import.** The first present of `CLAUDE.md`, `.claude/CLAUDE.md` and
   `CLAUDE.local.md` gets one fathom-marked region whose body is the import line (`@AGENTS.md`,
   or `@../AGENTS.md` from `.claude/`). It gets none when an unfenced import already exists.
3. **Migration rewrites the region, never the file.** An existing CLAUDE.md stanza region is
   rewritten in place to the import body. It carries a receipt, and `fleet undo` restores it. A
   drifted region blocks, per law 15. fathom never deletes an adopter's CLAUDE.md.
4. **Doctor proves the path.** `stanza_present` FAILs when AGENTS.md lacks a well-formed region.
   It WARNs, with the remedy, when Claude Code will not load AGENTS.md:
   - the root CLAUDE file has no import;
   - a legacy stanza copy remains;
   - an ancestor CLAUDE file exists;
   - the effective setting is `claude-md` or `managed-only`;
   - the Claude Code version is older than 2.1.277.

   fathom never writes the setting. `stanza_loaded` SKIPs with a named reason where no CLAUDE
   file exists.
5. **Instruction files are first-class wherever fathom reads them.**
   - The obligations corpus includes AGENTS.md at the root, under `.claude/` and nested, beside
     CLAUDE.md.
   - fathom-owned regions are not compiled.
   - Byte-identical twin sections compile once.
   - Probes, console guide predicates, fleet notes and benches name AGENTS.md first.
6. **One skills copy.** `.claude/skills` holds the files, and its frontmatter is held to the
   Agent Skills fields by test. `.agents/skills` is a wiring-owned relative link to it, written
   only for a harness that reads `.agents/skills` and not `.claude/skills`. No writer writes
   through the link. A real `.agents/skills` with foreign entries blocks.
7. **Our repos carry AGENTS.md only:** fathom, agentic-kit and app-kit. Living citations are
   re-pointed; history is left as written.

**Alternatives rejected.**
- *Keep both stanzas.* Two copies to keep identical. Under the default mode, the CLAUDE.md copy
  also suppresses AGENTS.md. The owner refused this.
- *Create an import-only CLAUDE.md in every adopter.* It restores the InstructionsLoaded evidence,
  but it re-adds the retired file and switches off native AGENTS.md loading.
- *Write the agents-md setting.* That is user-global configuration (ask-first), and project
  settings ignore it.
- *Keep rendering two skill copies.* They drift, and the fathom-plan copies here already differ.
- *Delete a CLAUDE.md that carries only fathom's import.* fathom did not create it (law 15).
- *Follow `@` imports in the compile.* No measured need; not built.

**Consequences.**
- **Breaking:**
  - what init and reconcile write (compatibility surface: the stanza region);
  - `gen-claude-md` becomes `gen-agents-md`.

  Migration note: `docs/upgrades/0.9.0-agents-md-one-instruction-file.md`.
- **Changed:**
  - the harness descriptors and the `schema/harness` mode `import`;
  - the skills layout;
  - the obligations corpus;
  - doctor `stanza_present`;
  - the claude-code pack's check kind.
- **Restated:**
  - invariants I34 and I37;
  - the router's sensitive-surface row;
  - the obligations-corpus assumption in `domain.md`;
  - the router footer (Claude Code 2.1.284).
- **Evidence and history:**
  - `stanza_loaded` loses harness evidence in AGENTS.md-only repos.
  - A converted subject is a new bench corpus revision; historical cells are unchanged.
- **Kits:** agentic-kit's and app-kit's own products change in their own records, and fathom
  mirrors them by `task sync-kit`.

**Revisit when.**
- Claude Code changes the agents-md contract: its default mode, discovery of `.agents/skills`, or
  InstructionsLoaded for AGENTS.md.
- Codex reads `.claude/skills`, or is shown not to follow the link.
- A supported harness reads CLAUDE.md but not AGENTS.md.
- An adopter asks in writing for a CLAUDE.md-only layout.
- `stanza_loaded` SKIPs on every row of our fleet for 30 days: retire or replace the row.

Index row: `| ADR-059 | accepted | product / architecture | AGENTS.md is the one instruction file;
a CLAUDE.md fathom meets carries only an @AGENTS.md import; one skills copy with .agents/skills as
a link; instruction files first-class in every reader | the agents-md contract changes; Codex reads
.claude/skills or not the link; a harness reads CLAUDE.md but not AGENTS.md; an adopter asks for
CLAUDE.md-only; stanza_loaded SKIPs fleet-wide for 30 days |`

### 4.2 Compatibility surface and CHANGELOG
- **`scope.md`, compatibility-surface line replaced with:**

  > The `<!-- fathom:begin -->`…`<!-- fathom:end -->` stanza region in `AGENTS.md`, and the
  > fathom-marked `@AGENTS.md` import region in a root `CLAUDE.md`, `.claude/CLAUDE.md` or
  > `CLAUDE.local.md` that fathom did not create (fleet reconcile compares content, law 15).
  > Evidence: ADR-059.

  The Harnesses boundary line adds "Claude Code reads the AGENTS.md stanza natively (≥2.1.277)".
- **`[Unreleased]`, `### Breaking`, first in the section:**
  - `fathom init`, `fleet reconcile --apply` and the console apply write the fathom stanza into
    `AGENTS.md` only. A root `CLAUDE.md`, `.claude/CLAUDE.md` or `CLAUDE.local.md` gets one
    marked region holding `@AGENTS.md`, and an existing CLAUDE.md stanza region is rewritten to
    it on the next rewire. fathom never creates a CLAUDE.md. Migration:
    `docs/upgrades/0.9.0-agents-md-one-instruction-file.md` (ADR-059).
  - `fathom gen-claude-md` is renamed `gen-agents-md` (output `workspace/artifacts/AGENTS.md`).
    The old name runs with a deprecation line through 0.9.x.
- **`### Changed`:**
  - `.claude/skills` is the only skills copy. `.agents/skills` is a link to it, written only for
    Codex, Gemini CLI or the generic AGENTS.md tier.
  - `doctor stanza_present` checks AGENTS.md and warns, with the remedy, when Claude Code will not
    load it.
  - The obligations layer compiles AGENTS.md (root, `.claude/`, nested). fathom's own stanza is
    no longer compiled as a rule.
  - In `fathom harness capabilities --json`, Claude Code reads AGENTS.md natively and CLAUDE.md
    in `import` mode.
- **`### Fixed`:** the Claude Code descriptor no longer claims `.agents/skills` discovery.
- **Release impact: minor** (pre-1.0: Breaking bumps minor). This rides 0.9.0, which already
  carries Breaking lines.
- Doctor's `--json` shape is unchanged; only the row's statuses and reasons change.

### 4.3 Plan `docs/plans/agents-md.md` (prefix AG, unused before; grammar v2, one line each)
Lanes and observation ids (reserved one per build worker; an unused id stays unused):
- W1 = O138: harness, init, rewire, wiring.
- W2 = O139: skills, doctor, custodian.
- W3 = O140: readers.
- W4 = O141: conversion and records.

```
## W1 harness, init, rewire, wiring (O138)
- [ ] The Claude Code descriptor reads AGENTS.md natively (≥2.1.277) and CLAUDE.md in mode `import`; skills dirs are `.claude/skills` only; verified_against 2.1.284; `schema/harness` gains mode `import`; the adoption matrix is regenerated {id: AG-1, symbol: ImportFiles, package: internal/harness}
- [ ] One seam plans the instruction regions for init, reconcile, drift and console apply: the stanza region in AGENTS.md for every harness (`none` included), never a created CLAUDE.md {id: AG-2, symbol: instructionRegions, package: cmd/fathom}
- [ ] The first present root CLAUDE file gets one marked region whose body is the import (`@AGENTS.md`; `@../AGENTS.md` from `.claude/`), none when an unfenced import already exists {id: AG-3, symbol: claudeImportRegion, package: cmd/fathom, needs: AG-2}
- [ ] Rewire rewrites an existing CLAUDE.md stanza region (tracked or known v1) to the import body with a receipt `fleet undo` reverses; a drifted region blocks {id: AG-4, symbol: reconcileSpec, package: cmd/fathom, needs: AG-3}
- [ ] `Region.Remove` plans deletion of a tracked, undrifted region with the removed text in the receipt; drift blocks {id: AG-5, symbol: planRegion, package: internal/wiring}
- [ ] `init --harness=gemini-cli` sets `.gemini/settings.json` `context.fileName` to AGENTS.md and GEMINI.md as a wiring key, and writes no GEMINI.md stanza {id: AG-6, symbol: initTargetsForDescriptor, package: cmd/fathom, needs: AG-1}
## W2 skills, doctor, custodian (O139)
- [ ] `wiring.Link` plans an owned relative symlink (absent: create; same target: skip; otherwise blocked) with receipt and undo {id: AG-7, symbol: planLink, package: internal/wiring}
- [ ] `.claude/skills` holds the files for every mode; `.agents/skills -> ../.claude/skills` is planned only for codex, gemini-cli, agents and bare `--skills`; no writer writes through the link; a real `.agents/skills` with foreign entries blocks {id: AG-8, symbol: skillsTargetsFor, package: cmd/fathom, needs: AG-7}
- [ ] Every shipped SKILL.md carries only Agent Skills frontmatter fields with name equal to its directory {id: AG-9, symbol: agentSkillsFields, package: cmd/fathom}
- [ ] Custodian kind `agents_md_reaches_claude` (optional `allow_import: false`) with schema valid/invalid pairs; the claude-code pack's `claude_code_stanza_present` uses it {id: AG-10, symbol: checkAgentsMDReachesClaude, package: internal/custodian}
- [ ] Doctor `stanza_present` FAILs on AGENTS.md region cases and WARNs with remedies on a missing import, a legacy stanza copy, an ancestor CLAUDE file, the claude-md or managed-only setting, or Claude Code before 2.1.277; PASS names the load path {id: AG-11, symbol: claudeLoadPath, package: cmd/fathom, needs: AG-10}
- [ ] Doctor `stanza_loaded` SKIPs with the named agents-md reason where no root CLAUDE file exists; doctor `skills_installed` resolves the `.agents/skills` link and probes obligations on AGENTS.md first {id: AG-12, symbol: checkStanzaLoaded, package: cmd/fathom}
## W3 readers (O140)
- [ ] The obligations corpus includes AGENTS.md, **/AGENTS.md, .claude/AGENTS.md and .claude/CLAUDE.md, and the exclusion note is gone {id: AG-13, oracle: lexical, path: internal/semantic/layers/obligations/layer.cue, pattern: "\\*\\*/AGENTS\\.md"}
- [ ] fathom-owned regions are never compiled as rules, and byte-identical CLAUDE.md/AGENTS.md sections in one directory compile once (AGENTS.md kept) {id: AG-14, symbol: dedupeTwinGuideSections, package: internal/semantic}
- [ ] `.claude/AGENTS.md` and `.claude/CLAUDE.md` are root guides read by one bounded stat (no floor, root tier, root-level source) {id: AG-15, symbol: isRootGuidePath, package: internal/semantic}
- [ ] Console and fleet name AGENTS.md: `repoHasGuide`/`isGoverningGuide` add the `.claude/` forms; `fleetWiringNotes` and the ladder read the AGENTS.md stanza; `rootObligationsProbe` is AGENTS.md {id: AG-16, symbol: fleetWiringNotes, package: cmd/fathom}
- [ ] An edit under a directory with a nested AGENTS.md is served that file's rule, compiled or raw, through the prompt and edit hooks {id: AG-17, symbol: probeAncestorGuides, package: pkg/client, needs: AG-13}
- [ ] Bench leak list adds `.claude/AGENTS.md` and `CLAUDE.local.md`; RenderArm writes arm text into AGENTS.md when the subject has no root CLAUDE file, and `none` removes AGENTS.md too {id: AG-18, symbol: RenderArm, package: internal/scenariobench}
- [ ] `gen-agents-md` replaces `gen-claude-md` (alias with a deprecation line through 0.9.x) and defaults to `workspace/artifacts/AGENTS.md` {id: AG-19, symbol: cmdGenAgentsMd, package: cmd/fathom}
## W4 conversion and records (O141)
- [ ] ADR-059 appended and indexed; scope's compatibility line, I34, I37, the router surface row and footer (Claude Code 2.1.284) and the domain assumption restated {id: AG-20, oracle: lexical, path: ADR.md, pattern: "^## ADR-059: AGENTS\\.md is the one instruction file"}
- [ ] CHANGELOG Breaking lines and the migration note `docs/upgrades/0.9.0-agents-md-one-instruction-file.md` {id: AG-21, oracle: lexical, path: docs/upgrades/0.9.0-agents-md-one-instruction-file.md, pattern: "@AGENTS\\.md"}
- [ ] This repo carries AGENTS.md only: root union at most 120 lines, nested guides moved, no CLAUDE file outside testdata and third_party, held by `one_instruction_file` in config audit {id: AG-22, oracle: lexical, path: .fathom/concerns.cue, pattern: "allow_import: false", needs: AG-13}
- [ ] Line-anchored citations re-pointed and re-derived at HEAD in tasks-home.json, governing-rules.json, guards-m1.json and lookups-vs.json; `TestHomeCorpusTruthAtHead` green {id: AG-23, oracle: lexical, path: testdata/indexbench/governing-rules.json, pattern: "internal/AGENTS\\.md"}
- [ ] Adoption docs, the ten shipped skills and the living docs describe one stanza in AGENTS.md, the import region and the skills link {id: AG-24, oracle: lexical, path: docs/adoption/claude-code.md, pattern: "@AGENTS\\.md"}
- [ ] Rows naming the two-file layout reworded or re-anchored (AK-3, RP-1, RP-3, IO-24, EW-5, SB-8 path to `.claude/skills`, BP-32, OB-12, XM-20); IA-3 retired as refused with ADR-059 {id: AG-25, oracle: lexical, path: docs/plans/init-and-adoption.md, pattern: "id: IA-3, disposition: refused"}
- [ ] (op) AG-OP1: `fathom doctor` on this checkout after `task deploy`: stanza_present PASS "native (no CLAUDE file, default mode)"; a fresh Claude Code 2.1.284 session shows the AGENTS.md rules; `obligations` with no path serves AGENTS.md-sourced rules {id: AG-OP1, skip_test: true, skip: "operator reading after deploy; evidence is the doctor output and one session transcript line in the plan"}
- [ ] (op) AG-OP2: a Codex session in this repo invokes a fathom skill through the `.agents/skills` link, or the fallback in §1.4 is taken {id: AG-OP2, skip_test: true, skip: "needs a live Codex session; doctor keeps discovery unverified until then"}
- [ ] (op) AG-OP3: agentic-kit and app-kit converted by their own records, then `task sync-kit` and `task vendor-check` green here {id: AG-OP3, skip_test: true, skip: "kit-side change; evidence is the synced mirror commit"}
```

### 4.4 Futures, questions, incidents
- Futures: none. Following `@` imports in the obligations compile is refused in the ADR, not
  parked.
- Questions: none.
- Incidents: none; nothing broke.

### 4.5 Done needs (surface evidence)
- `go test -race ./cmd/fathom ./internal/...`.
- For the prompt and edit hooks: mutation-RED tests on `instructionRegions` and
  `claudeImportRegion`, and the hook goldens unchanged.
- `fathom config audit` exit 0.
- `fathom doctor` on this repo.
- `TestObligationsCorpusNeverWidensToIntent`.
- `task cue-vet` for `schema/harness` and the custodian schema.
- `fathom verify --strict` exit 0 on every top-level plan.
- `intent-doctor`, once the kit's check 15 ships.
