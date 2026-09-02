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

- [ ] `internal/harness`: `Descriptor` (id, tier, detect, instruction files + mode, hook files, event bindings with field maps, `failMode`, scope-merge rule, skills dirs + variant, MCP target, env contract, transcript, `verified_against`), `Registry`, and `Lookup(id)`; `Registry["claude-code"]` reproduces `fathomHookSpecs` event-for-event and the test pins it {id: HP-1, symbol: Lookup, package: internal/harness}
- [ ] `cmd/fathom` derives its hook table from the registry (`hookSpecsFor("claude-code")`), so init, plugin render, reconcile and doctor read one source; `TestPluginRender` and `TestCmdInit_WiresRepoEndToEnd` goldens unchanged {id: HP-2, symbol: hookSpecsFor, package: cmd/fathom}
- [ ] `Codec` (decode stdin per canonical event → `HookInput`; encode `HookOutput` → stdout) with `ClaudeCodec()` replacing the four decode sites and four encode sites in the hook verbs; output bytes pinned against the pre-refactor fixtures {id: HP-3, symbol: ClaudeCodec, package: internal/harness}
- [ ] `ClassifyTool(descriptorID, toolName) ToolClass` (`FileEdit | Shell | DiscoveryRead | DiscoveryGrep | DiscoveryGlob | Other`) replaces the `Edit|Write|MultiEdit|Bash|Grep|Glob|Read` literals in the pre-edit guard and the discovery observer {id: HP-4, symbol: ClassifyTool, package: internal/harness}
- [ ] `renderPostEdit` extracted from `cmdEditHook`: the obligations payload as text, harness-free, so the verb is decode → payload → encode like session-hook already is {id: HP-5, symbol: renderPostEdit, package: cmd/fathom}
- [ ] `renderPreEdit` extracted from `cmdEditHookPreFile` for the same reason {id: HP-6, symbol: renderPreEdit, package: cmd/fathom}
- [ ] `--harness <id>` on the five hook verbs, resolved by `resolveHookHarness` (flag absent ⇒ `claude-code`, so the wired Claude Code command line and its goldens do not change); an unknown id exits 0 with one stderr line (I9 fail-open) {id: HP-7, symbol: resolveHookHarness, package: cmd/fathom}
- [ ] `fathom harness list` and `fathom harness capabilities --json` (optional `--harness id`; law 10), dispatched and listed by `fathom help` {id: HP-8, symbol: cmdHarness, package: cmd/fathom}
- [ ] `schema/harness/schema.cue` validates the exported descriptor JSON with valid and invalid testdata pairs under `task cue-vet` {id: HP-9, symbol: TestHarnessSchemaVetsExport, package: internal/harness, skip_test: true, skip: "the test is the anchor: it runs cue vet over the exported registry; cue-vet is also in task validate"}
- [ ] `RenderCapabilityTable(id)` renders the per-capability vehicle table (hook-inject / hook-block / pull / stanza / mcp / gate / absent) with `unverified on this machine` on every row of an unverified descriptor; `docs/adoption/codex.md` §6 is pinned to it by `codexdoc_test.go` {id: HP-10, symbol: RenderCapabilityTable, package: internal/harness}
- [ ] Stanza hook sentences are generated per descriptor (`stanzaHookSentences`) so the text never promises a channel the harness lacks; the Claude Code stanza stays byte-identical (golden) {id: HP-11, symbol: stanzaHookSentences, package: cmd/fathom}
- [ ] `Detect(root, home)` drives `resolveDoctorHarness`; `doctor --harness` accepts every registry id plus `agents`; MCP registration is read per descriptor as `skipped: unverified` when the descriptor is unverified — never a guessed PASS {id: HP-12, symbol: Detect, package: internal/harness}
- [ ] `fathom init --harness=claude-code|codex|gemini-cli|agents|none`: `initTargetsFor` writes only the descriptor's declared vehicles (`none` writes `.fathom/` and the stanza only), never a user-global file, prints the descriptor's `mcp add` line, and refuses a descriptor past its demotion clock with the reason {id: HP-13, symbol: initTargetsFor, package: cmd/fathom}
- [ ] Leases and rate limits take their harness label from the registry id, not from `CLAUDE*` env sniffing (`harnessLabel`) {id: HP-14, symbol: harnessLabel, package: cmd/fathom}
- [ ] Doctor `gate_posture` row from the descriptor's `failMode` (`advisory, fail-open (§5.3)` on Claude Code; the harness's own posture elsewhere) {id: HP-15, symbol: checkGatePosture, package: cmd/fathom}
- [ ] `DemotionDue(desc, now)`: a descriptor unverified 60 days after its codec landed renders as tier D and `init --harness` refuses it; injected clock, pinned by test {id: HP-16, symbol: DemotionDue, package: internal/harness}

## Wave 2 — descriptors and codecs (unverified until a real session)

- [ ] Codex descriptor + codec: `AGENTS.md`; `.codex/hooks.json` (repo layer; root-level event keys per the verified contract, `features.hooks` noted); Claude-shaped bindings for the session/edit/pre-edit/stop/pre-compact/session-end events; `apply_patch`→FileEdit, `shell`→Shell; `additionalContext` cap; hash-trust step noted; `.agents/skills`; `config.toml [mcp_servers.fathom] url=` printed as the `codex mcp add` line, never written; `verified_against: unverified` (local Codex is 0.146) {id: HP-17, symbol: CodexCodec, package: internal/harness}
- [ ] Codex hook file wiring through the engine: a `wiring.Key` for `.codex/hooks.json` (`KindJSONHook` reused when the verified shape matches Claude's; a new kind with plan/apply/undo otherwise) so `init --harness=codex` leaves a receipt `fleet undo` reverses {id: HP-18, symbol: codexHookKey, package: cmd/fathom}
- [ ] Gemini CLI descriptor + codec (a local install exists): `GEMINI.md`, or `AGENTS.md` via `context.fileName`; `settings.json` hooks; the flat decision plus `hookSpecificOutput.additionalContext` shape; parse-failure-means-allow recorded as `failMode`; `mcpServers.<name>.httpUrl`; `.agents/skills`; `verified_against: unverified` until a real session writes beats {id: HP-19, symbol: GeminiCodec, package: internal/harness}
- [ ] Cursor and Copilot descriptors as data rows only (instruction files, hook file path and `failMode`, skills dirs, MCP target, `mcp add` line), no codec: `init --harness` for them writes stanza and skills and prints the MCP line; the generated table says `hooks: not wired` {id: HP-20, symbol: DataOnlyDescriptors, package: internal/harness}
- [-] Cursor and Copilot codecs {id: HP-21, disposition: deferred, reason: "no local install and no adopter (P20); the descriptors carry the documented field maps as data so a codec is a fixture away", trigger: "an adopter on that harness, or a local install to verify hooks_firing against"}
- [ ] Tier C/D rows (OpenCode, Amp, Zed/ACP, Junie, Warp, Continue, Aider, Windsurf) as data in the generated table: instruction file, skills path, MCP config path, `mcp add` line, and the honest "no injection" line — no codecs {id: HP-22, symbol: LadderRows, package: internal/harness}
- [ ] Every codec round-trips its documented fixtures (`TestCodecsRoundTripDocumentedFixtures`) and never emits `deny` (§5.3 inject-only); each fixture header cites the source URL and date {id: HP-23, symbol: TestCodecsRoundTripDocumentedFixtures, package: internal/harness, skip_test: true, skip: "the test is the anchor"}

## Records

- [ ] `docs/intent/scope.md`: Harnesses boundary restated (Claude Code verified; Codex and Gemini CLI descriptors with codecs, unverified; Cursor and Copilot data rows; tiers C/D documented), compatibility surface gains `--harness`, `fathom harness`, `.codex/hooks.json` writes; `CHANGELOG.md` lines; `docs/mcp-tools.md` and the coding-agents use-case `## Maintained` line name `fathom harness` {id: HP-24, skip_test: true, skip: "docs; acceptance evidence is the scope.md diff and TestMaintainedLinesNameRegisteredToolsAndVerbs"}
- [ ] FA-22 filed in agentic-kit (`docs/plans/fathom-uplift-asks.md`): the harness manifest contract moves to the kit only when the kit deletes `install-intent.sh` and the `integration.md` harness table onto it {id: HP-25, skip_test: true, skip: "a row in another repo's plan; evidence is the kit commit"}
