# Reach parity — the open standards, the served contract, the documented truth

Design: `docs/vision/25-harness-port.md` §1–2; ADR-029. Companion:
`harness-port.md` (the seam this plan renders through).

Pull-tier work is permitted on every harness today (vision 04 §5 as amended
by ADR-029). This plan makes the binary, the docs, the doctor and the served
tool descriptions agree, on the four standards the field settled on: Agent
Skills under `.agents/skills`, Agent Plugins 1.0, `AGENTS.md`, and MCP over
HTTP proven by a client we did not write. Standards are adopted as vendored
schemas under `cue-vet`, never as hand-written conformance tests.

Verify: `fathom verify --service fathom --plan docs/plans/reach-parity.md`.

## Skills on the standard

- [ ] `skillsTargetsFor(mode)`: bare `--skills` writes `.agents/skills` (spec-only frontmatter) and `.claude/skills`; `--skills=codex` writes `.agents/skills` (the path Codex documents); the legacy `~/.codex/prompts` twin moves behind `--skills=codex-prompts`; the marker guard is unchanged {id: RP-1, symbol: skillsTargetsFor, package: cmd/fathom}
- [ ] `agentsVariant(skill)` strips harness-specific frontmatter to the Agent Skills spec fields and keeps `name` equal to the directory name; the Agent Skills frontmatter schema is vendored as `schema/skills/schema.cue` and `cue vet` runs over every shipped `SKILL.md` frontmatter in `task cue-vet` {id: RP-2, symbol: agentsVariant, package: cmd/fathom}
- [ ] Doctor `skills_installed` checks `.agents/skills` for `codex|gemini-cli|agents` and `.claude/skills` for `claude-code`; a missing skill is named, never summarised {id: RP-3, symbol: checkAgentsSkills, package: cmd/fathom}
- [ ] `fleet reconcile` plans fathom-owned skill drift through the wiring engine (`planOwnedFile`: rewrite only under the `fathom:owned` marker, BLOCK a hand-edited copy by name, receipt + undo) {id: RP-4, symbol: planOwnedFile, package: internal/wiring}
- [ ] `docs/upgrades/0.8.0-skills-on-the-standard.md` and the `### Breaking` line: `--skills=codex` no longer writes `~/.codex/prompts` {id: RP-5, skip_test: true, skip: "migration note; evidence is the file and the CHANGELOG line"}

## One plugin tree, two manifests

- [ ] `renderAgentPluginManifest` emits the Agent Plugins 1.0 `plugin.json` and `mcp.json` (pointing at the loopback daemon) beside `.claude-plugin/plugin.json`, over one `skills/` tree, from the same embedded sources; golden-tested {id: RP-6, symbol: renderAgentPluginManifest, package: cmd/fathom}
- [ ] The Agent Plugins `plugin.json` schema is vendored as `schema/plugin/schema.cue` (with the Claude plugin manifest beside it) and both rendered manifests vet under `task cue-vet` {id: RP-7, symbol: TestPluginManifestsVet, package: cmd/fathom, skip_test: true, skip: "the test is the anchor; cue-vet is also in task validate"}
- [ ] `scripts/ci.sh` runs `claude plugin validate --strict` over the rendered `.claude-plugin` tree when a `claude` binary is on PATH, and prints a named skip otherwise {id: RP-8, skip_test: true, skip: "shell; bash -n is enforced by task scripts-check; the named skip is visible in ci.sh output"}

## The served contract for clients that never read CLAUDE.md

- [x] Every agent-surface tool description carries a "Reach for it when …" sentence and names the arguments its own schema marks required (per-tool truth, not a blanket rule; ADR-006 as amended); the `QuerySymbols` leak in `pack_context` is gone; `TestAgentSurfaceDescriptionsNameRequiredArgs` pins it {id: RP-9, symbol: TestAgentSurfaceDescriptionsNameRequiredArgs, package: internal/mcp, skip_test: true, skip: "the test is the anchor: it iterates the 17-tool allowlist"}
- [x] MCP tool annotations (`readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint`) on every agent-surface tool (16 read-only; `ingest` idempotent and not read-only); `TestMcpToolsDocMatchesRegistry` extended to require them {id: RP-10, symbol: toolAnnotations, package: internal/mcp}
- [x] `outputSchema` on every agent-surface tool, generated from the Go result type (`outputSchemaFor`), and `structuredContent` beside the text content on `tools/call`, so a non-Claude client can machine-check the envelope {id: RP-11, symbol: outputSchemaFor, package: internal/mcp}
- [x] `negotiateProtocolVersion`: `initialize` echoes the client's `protocolVersion` when it is in the supported table, otherwise the newest supported; the `MCP-Protocol-Version` header is tolerated; `tools/list` order is deterministic and pinned {id: RP-12, symbol: negotiateProtocolVersion, package: internal/mcp}
- [x] `TestServeHTTPNegotiatesWithKitClient` drives `/rpc/agent` through agentic-kit's `mcp-client` Streamable-HTTP transport (initialize → tools/list → tools/call) under `httptest`; its failure text is the adoption doc's sentence, replacing every "unverified negotiation" line {id: RP-13, symbol: TestServeHTTPNegotiatesWithKitClient, package: cmd/fathom, skip_test: true, skip: "the test is the anchor; the kit client is the third party"}

## `--json` where the compatibility surface promises it

- [ ] `fathom ingest --json` (read-failed list, degraded/refused reasons, counts) so CI can assert `read_failed == 0` {id: RP-14, symbol: renderIngestJSON, package: cmd/fathom}
- [ ] `fathom init --json` (every write, receipt id, manual steps as data) {id: RP-15, symbol: renderInitJSON, package: cmd/fathom}
- [ ] `fathom thread brief --json` and `fathom thread list --json` {id: RP-16, symbol: renderThreadJSON, package: cmd/fathom}
- [ ] `fathom daemon status --json` and `fathom layers list --json` {id: RP-17, symbol: renderDaemonStatusJSON, package: cmd/fathom}

## Documented truth

- [ ] `docs/adoption/other-harnesses.md` is generated by `RenderAdoptionMatrix` from `fathom harness capabilities --json` (tier, instruction file, skills path, MCP config path and `mcp add` line, `verified_against` or `unverified`) and pinned by a doc test {id: RP-18, symbol: RenderAdoptionMatrix, package: internal/harness}
- [ ] Adoption docs match the binary: `--skills` shapes and `fathom plugin render` in `docs/adoption/claude-code.md` §11, `docs/adoption/codex.md` §1.1/§2/§6 (generated table; Verified-against names the official Codex hooks reference and its date; the vision 16 §3 contradiction resolved), `docs/adoption/README.md`, `docs/use-cases/getting-started.md` (the `.agents/skills` attribution and the session-hook not-indexed line), `docs/use-cases/coding-agents.md` {id: RP-19, skip_test: true, skip: "docs; evidence is codexdoc_test.go and TestNoStaleHelpPhrasesInDocs"}
- [ ] `TestNoStaleHelpPhrasesInDocs` fails on "not yet listed by `fathom help`" and "emits nothing" under `docs/`; the four use-case docs are corrected in the same change {id: RP-20, symbol: TestNoStaleHelpPhrasesInDocs, package: cmd/fathom, skip_test: true, skip: "the test is the anchor"}
- [ ] The nine shipped skills (`fathom-onboard`, `-rewire`, `-doctor`, `-activate`, `-plan`, `-verify`, `-feedback`, `-impact`, `-custodian`) describe `--harness`, `.agents/skills`, `fathom plugin render`, `fathom harness capabilities` and the wiring-drift doctor row where each applies; re-rendered into this repo's `.claude/skills` and `.agents/skills`; `skills_installed` PASS {id: RP-21, skip_test: true, skip: "skill prose; evidence is doctor skills_installed and reconcile_skills_test.go"}
- [ ] `docs/mcp-tools.md`, the use-case `## Maintained` lines and `docs/usage.md` (`--json` envelopes) updated; `TestMcpToolsDocMatchesRegistry` and `TestMaintainedLinesNameRegisteredToolsAndVerbs` green {id: RP-22, skip_test: true, skip: "docs; evidence is the two named tests"}
- [ ] `task stranger-check`: clone the tracked tree into `workspace/tmp`, build with `GOWORK=off`, run `fathom init --harness=none` and `fathom doctor` against a temp store, and fail on any FAIL row — the v0.7.0 lesson as a task, in `task validate` when it runs under two minutes {id: RP-23, skip_test: true, skip: "task + shell; bash -n via scripts-check; evidence is the Taskfile target and its run in the Verified: line"}
