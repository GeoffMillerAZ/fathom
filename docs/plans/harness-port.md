# Harness port — adapters are data behind one seam (ADR-029)

Design: `docs/vision/25-harness-port.md`. Decision: ADR-029. Companion plan
(pull-tier parity, plugin manifest, served contract): `reach-parity.md`.

The seam is `internal/harness`: a descriptor registry (data) plus one codec
per harness (code). Claude Code's registry entry reproduces the hook table
byte-identically and every existing golden stays green with no edit — that
is the proof the refactor is a seam, and any changed Claude Code golden byte
kills the wave (rule 1). Codecs exist only for harnesses with a documented
Claude-shaped contract AND a local install to verify against (Codex 0.146
and Gemini CLI are installed on this workstation; Cursor and Copilot are
not): P20 — build for a named consumer. Every non-Claude descriptor ships
`unverified`; `init --harness=<id>` writes its declared vehicles only on
that explicit flag; no measurement claim (ADR-029 §5). Envelopes come from
the verified contract notes gathered 2026-09-02 from each harness's primary
documentation; a field the documentation does not show is not encoded.

Verify: `fathom verify --service fathom --plan docs/plans/harness-port.md`.

## Wave 1 — the seam (zero behavior change)

- [x] `internal/harness`: `Descriptor` (id, tier, detect, instruction files + mode, hook files, event bindings with field maps, `failMode`, scope-merge rule, skills dirs + variant, MCP target, env contract, transcript, `verified_against`), `Registry`, and `Lookup(id)`; `Registry["claude-code"]` reproduces `fathomHookSpecs` event-for-event and the test pins it {id: HP-1, symbol: Lookup, package: internal/harness}
- [x] `cmd/fathom` derives its hook table from the registry (`hookSpecsFor("claude-code")`), so init, plugin render, reconcile and doctor read one source; `TestPluginRender` and `TestCmdInit_WiresRepoEndToEnd` goldens unchanged {id: HP-2, symbol: hookSpecsFor, package: cmd/fathom}
- [x] `Codec` (decode stdin per canonical event → `HookInput`; encode `HookOutput` → stdout) with `ClaudeCodec()` replacing the four decode sites and four encode sites in the hook verbs; output bytes pinned against the pre-refactor fixtures {id: HP-3, symbol: ClaudeCodec, package: internal/harness}
- [x] `ClassifyTool(descriptorID, toolName) ToolClass` (`FileEdit | Shell | DiscoveryRead | DiscoveryGrep | DiscoveryGlob | Other`) replaces the `Edit|Write|MultiEdit|Bash|Grep|Glob|Read` literals in the pre-edit guard and the discovery observer {id: HP-4, symbol: ClassifyTool, package: internal/harness}
- [x] `renderPostEdit` extracted from `cmdEditHook`: the obligations payload as text, harness-free, so the verb is decode → payload → encode like session-hook already is {id: HP-5, symbol: renderPostEdit, package: cmd/fathom}
- [x] `renderPreEdit` extracted from `cmdEditHookPreFile` for the same reason {id: HP-6, symbol: renderPreEdit, package: cmd/fathom}
- [x] `--harness <id>` on the five hook verbs, resolved by `resolveHookHarness` (flag absent ⇒ `claude-code`, so the wired Claude Code command line and its goldens do not change); an unknown id exits 0 with one stderr line (I9 fail-open) {id: HP-7, symbol: resolveHookHarness, package: cmd/fathom}
- [x] `fathom harness list` and `fathom harness capabilities --json` (optional `--harness id`; law 10), dispatched and listed by `fathom help` {id: HP-8, symbol: cmdHarness, package: cmd/fathom}
- [x] `schema/harness/schema.cue` validates the exported descriptor JSON with valid and invalid testdata pairs under `task cue-vet` {id: HP-9, symbol: TestHarnessSchemaVetsExport, package: internal/harness, skip_test: true, skip: "the test is the anchor: it runs cue vet over the exported registry; cue-vet is also in task validate"}
- [x] `RenderCapabilityTable(id)` renders the per-capability vehicle table (hook-inject / hook-block / pull / stanza / mcp / gate / absent) with `unverified on this machine` on every row of an unverified descriptor; `docs/adoption/codex.md` §6 is pinned to it by `codexdoc_test.go` {id: HP-10, symbol: RenderCapabilityTable, package: internal/harness}
- [x] Stanza hook sentences are generated per descriptor (`stanzaHookSentences`) so the text never promises a channel the harness lacks; the Claude Code stanza stays byte-identical (golden) {id: HP-11, symbol: stanzaHookSentences, package: cmd/fathom}
- [x] `Detect(root, home)` drives `resolveDoctorHarness`; `doctor --harness` accepts every registry id plus `agents`; MCP registration is read per descriptor as `skipped: unverified` when the descriptor is unverified — never a guessed PASS {id: HP-12, symbol: Detect, package: internal/harness}
- [x] `fathom init --harness=claude-code|codex|gemini-cli|agents|none`: `initTargetsFor` writes only the descriptor's declared vehicles (`none` writes `.fathom/` and the stanza only), never a user-global file, prints the descriptor's `mcp add` line, and refuses a descriptor past its demotion clock with the reason {id: HP-13, symbol: initTargetsFor, package: cmd/fathom}
- [x] Leases and rate limits take their harness label from the registry id, not from `CLAUDE*` env sniffing (`harnessLabel`) {id: HP-14, symbol: harnessLabel, package: cmd/fathom}
- [x] Doctor `gate_posture` row from the descriptor's `failMode` (`advisory, fail-open (§5.3)` on Claude Code; the harness's own posture elsewhere) {id: HP-15, symbol: checkGatePosture, package: cmd/fathom}
- [x] `DemotionDue(desc, now)`: a descriptor unverified 60 days after its codec landed renders as tier D and `init --harness` refuses it; injected clock, pinned by test {id: HP-16, symbol: DemotionDue, package: internal/harness}

## Wave 2 — descriptors and codecs (unverified until a real session)

- [x] Codex descriptor + codec: `AGENTS.md`; `.codex/hooks.json` (repo layer; the verified contract's `hooks.<Event>` nesting — Claude's, so the json-hook editor is reused; `features.hooks` noted); Claude-shaped bindings for the session/edit/pre-edit/stop/pre-compact/session-end events; `apply_patch`→FileEdit, `shell`→Shell; `additionalContext` cap; hash-trust step noted; `.agents/skills`; `config.toml [mcp_servers.fathom] url=` printed as the `codex mcp add` line, never written; `verified_against: unverified` (local Codex is 0.146) {id: HP-17, symbol: CodexCodec, package: internal/harness}
- [x] Codex hook file wiring through the engine: a `wiring.Key` for `.codex/hooks.json` (`KindJSONHook` reused when the verified shape matches Claude's; a new kind with plan/apply/undo otherwise) so `init --harness=codex` leaves a receipt `fleet undo` reverses {id: HP-18, symbol: codexHookKey, package: cmd/fathom}
- [x] Gemini CLI descriptor + codec (a local install exists): `GEMINI.md`, or `AGENTS.md` via `context.fileName`; `settings.json` hooks; the flat decision plus `hookSpecificOutput.additionalContext` shape; parse-failure-means-allow recorded as `failMode`; `mcpServers.<name>.httpUrl`; `.agents/skills`; `verified_against: unverified` until a real session writes beats {id: HP-19, symbol: GeminiCodec, package: internal/harness}
- [x] Cursor and Copilot descriptors as data rows only (instruction files, hook file path and `failMode`, skills dirs, MCP target, `mcp add` line), no codec: `init --harness` for them writes stanza and skills and prints the MCP line; the generated table says `hooks: not wired` {id: HP-20, symbol: DataOnlyDescriptors, package: internal/harness}
- [-] Cursor and Copilot codecs {id: HP-21, disposition: deferred, reason: "no local install and no adopter (P20); the descriptors carry the documented field maps as data so a codec is a fixture away", trigger: "an adopter on that harness, or a local install to verify hooks_firing against"}
- [x] Tier C/D rows (OpenCode, Amp, Zed/ACP, Junie, Warp, Continue, Aider, Windsurf) as data in the generated table: instruction file, skills path, MCP config path, `mcp add` line, and the honest "no injection" line — no codecs {id: HP-22, symbol: LadderRows, package: internal/harness}
- [x] Every codec round-trips its documented fixtures (`TestCodecsRoundTripDocumentedFixtures`) and never emits `deny` (§5.3 inject-only); each fixture header cites the source URL and date {id: HP-23, symbol: TestCodecsRoundTripDocumentedFixtures, package: internal/harness, skip_test: true, skip: "the test is the anchor"}

## Records

Wave 1 landed the `CHANGELOG.md` lines, the coding-agents `## Maintained` line
and `docs/mcp-tools.md` naming `fathom harness`; wave 2 (HP-18, HP-20) let
`docs/intent/scope.md` restate the Harnesses boundary (Codex and Gemini CLI
codecs, unverified; Cursor and Copilot data rows; tiers C/D as ladder rows) and
list the `.codex/hooks.json` / `.gemini/settings.json` writes in the
compatibility surface. Verbatim contracts the codecs were built from:
`docs/adoption/other-harnesses.md` is generated by RP-18, never hand-written.

- [x] `docs/intent/scope.md`: Harnesses boundary restated (Claude Code verified; Codex and Gemini CLI descriptors with codecs, unverified; Cursor and Copilot data rows; tiers C/D documented), compatibility surface gains `--harness`, `fathom harness`, `.codex/hooks.json` writes; `CHANGELOG.md` lines; `docs/mcp-tools.md` and the coding-agents use-case `## Maintained` line name `fathom harness` {id: HP-24, skip_test: true, skip: "docs; acceptance evidence is the scope.md diff and TestMaintainedLinesNameRegisteredToolsAndVerbs"}
- [x] FA-22 filed in agentic-kit (`docs/plans/fathom-uplift-asks.md`): the harness manifest contract moves to the kit only when the kit deletes `install-intent.sh` and the `integration.md` harness table onto it {id: HP-25, skip_test: true, skip: "a row in another repo's plan; evidence is the kit commit"}

## Wave 3 — the events that advise, and Codex for real (2026-09-12)

Claude Code 2.1.270 offers 33 events and Codex 0.154 twelve; fathom wires eight. Every
event below advises or records and never blocks (§5.3); each maps onto both harnesses
through the descriptor, and the doctor row names every event fathom leaves unwired with
its reason. INTENT.md is not a harness feature but the entry document of Anthropic's
SDLC playbook (intent → spec → plan → diff → PR → incident); the router under
`docs/intent/` is its durable form, so fathom renders the playbook's one-screen file
from the router rather than adopting a second corpus.

- [ ] `SubagentStart` → `fathom session-hook --event subagent-start`: a ≤600-byte primer (identity line + the opening-moves table) as `additionalContext`, so subagents that never see SessionStart still start index-first; a beat is recorded (`subagentPrimer`) {id: HP-26, symbol: subagentPrimer, package: cmd/fathom}
- [ ] `PostToolUseFailure` (matcher: the fathom MCP tools under both the plain and the plugin-scoped names) → `fathom session-hook --event tool-failure`: one remedy line from the daemon and staleness diagnosis doctor already computes; silent when the failure is not fathom's (`toolFailureAdvice`) {id: HP-27, symbol: toolFailureAdvice, package: cmd/fathom}
- [ ] `FileChanged` (matcher `CLAUDE.md|AGENTS.md|.fathom/**`) → `fathom config audit --hook`, the same attention beat `ConfigChange` writes (`fileChangedHook`) {id: HP-28, symbol: fileChangedHook, package: cmd/fathom}
- [ ] `WorktreeCreate` → `fathom session-hook --event worktree-create`: one advisory line naming the service the worktree belongs to and that ingest will not re-root it (PC-33) (`worktreeAdvice`) {id: HP-29, symbol: worktreeAdvice, package: cmd/fathom}
- [ ] `CwdChanged|DirectoryAdded` → `fathom session-hook --event cwd-changed`: the identity line for the new root's service, or the not-indexed line (`cwdChangedPrimer`) {id: HP-30, symbol: cwdChangedPrimer, package: cmd/fathom}
- [ ] `Setup` → `fathom doctor --brief`: FAIL rows only, ≤5 lines, exit 0 (`doctorBrief`) {id: HP-31, symbol: doctorBrief, package: cmd/fathom}
- [ ] Doctor `hooks_coverage`: for the detected harness, every event the descriptor knows against what fathom wires, one reason per unwired event (advisory, never FAIL); the descriptor carries the event catalog with reasons (`checkHooksCoverage`) {id: HP-32, symbol: checkHooksCoverage, package: cmd/fathom}
- [ ] Codex descriptor at the 0.154 contract (12 events incl. SubagentStart/Stop, PostCompact, Interrupt, PermissionRequest; the `hooks` wrapper shape; project `.codex/hooks.json` honored under directory trust); `fathom init --harness=codex` wires the eight fathom rows plus wave 3's; `TestCodexHooksFixtureFromLiveSession` replays a stdin captured from a real headless `codex exec` session on this machine {id: HP-33, symbol: TestCodexHooksFixtureFromLiveSession, package: internal/harness, skip_test: true, skip: "the test is the anchor; the fixture header names the codex-cli version and date the descriptor is verified against"}
- [ ] Codex wired on this workstation: `.codex/hooks.json` in this repo, the MCP server registered for Codex, one headless session's beats on file, `doctor --harness=codex` hooks_installed and hooks_firing PASS, the descriptor's `verified_against` dated {id: HP-34, skip_test: true, skip: "operator wiring; evidence is doctor --harness=codex and the descriptor field"}
- [ ] `fathom intent render --playbook` writes a root `INTENT.md` (Problem, Proposed outcome, Affected users, Constraints, Open questions — each a pointer into `docs/intent/`) inside fathom markers, policed by `config audit` pointers (`renderPlaybookIntent`) {id: HP-35, symbol: renderPlaybookIntent, package: cmd/fathom}
- [ ] The nine skills (SKILL.md and codex-prompt.md twins) and `docs/adoption/{claude-code,codex,other-harnesses}.md` describe the new events, `hooks_coverage`, the verified Codex lane and INTENT.md; `docs/mcp-tools.md` verbs current {id: HP-36, skip_test: true, skip: "docs and skill prose; evidence is codexdoc_test.go, the adoption matrix test and skills_installed"}
