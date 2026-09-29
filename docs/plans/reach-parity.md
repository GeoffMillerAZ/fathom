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

- [x] `skillsTargetsFor(mode)`: every repo-scoped mode writes `.claude/skills` (spec-only frontmatter, held by test); bare `--skills`, `--skills=codex` and `--skills=agents` add `.agents/skills` (the path Codex documents) as a link to it (ADR-059); the legacy `~/.codex/prompts` twin moves behind `--skills=codex-prompts`; the marker guard is unchanged {id: RP-1, symbol: skillsTargetsFor, package: cmd/fathom}
- [x] `agentsVariant(skill)` strips harness-specific frontmatter to the Agent Skills spec fields and keeps `name` equal to the directory name; the Agent Skills frontmatter schema is vendored as `schema/skills/schema.cue` and `cue vet` runs over every shipped `SKILL.md` frontmatter in `task cue-vet` {id: RP-2, symbol: agentsVariant, package: cmd/fathom}
- [x] Doctor `skills_installed` checks `.agents/skills` (resolving its link to `.claude/skills`, ADR-059) for `codex|gemini-cli|agents` and `.claude/skills` for `claude-code`; a missing skill is named, never summarised {id: RP-3, symbol: checkAgentsSkills, package: cmd/fathom}
- [x] `fleet reconcile` plans fathom-owned skill drift through the wiring engine (`planOwnedFile`: rewrite only under the `fathom:owned` marker, BLOCK a hand-edited copy by name, receipt + undo) {id: RP-4, symbol: planOwnedFile, package: internal/wiring}
- [x] `docs/upgrades/0.8.0-skills-on-the-standard.md` and the `### Breaking` line: `--skills=codex` no longer writes `~/.codex/prompts` {id: RP-5, oracle: lexical, path: docs/upgrades/0.8.0-skills-on-the-standard.md, pattern: "no longer writes"}

## One plugin tree, two manifests

- [x] `renderAgentPluginManifest` emits the Agent Plugins 1.0 `plugin.json` and `mcp.json` (pointing at the loopback daemon) beside `.claude-plugin/plugin.json`, over one `skills/` tree, from the same embedded sources; golden-tested {id: RP-6, symbol: renderAgentPluginManifest, package: cmd/fathom}
- [x] The Agent Plugins `plugin.json` schema is vendored as `schema/plugin/schema.cue` (with the Claude plugin manifest beside it) and both rendered manifests vet under `task cue-vet` {id: RP-7, symbol: TestPluginManifestsVet, package: cmd/fathom, skip_test: true, skip: "the test is the anchor; cue-vet is also in task validate"}
- [x] `scripts/ci.sh` runs `claude plugin validate --strict` over the rendered `.claude-plugin` tree when a `claude` binary is on PATH, and prints a named skip otherwise {id: RP-8, oracle: lexical, path: scripts/ci.sh, pattern: "claude plugin validate --strict"}

## The served contract for clients that never read CLAUDE.md

- [x] Every agent-surface tool description carries a "Reach for it when …" sentence and names the arguments its own schema marks required (per-tool truth, not a blanket rule; ADR-006 as amended); the `QuerySymbols` leak in `pack_context` is gone; `TestAgentSurfaceDescriptionsNameRequiredArgs` pins it {id: RP-9, symbol: TestAgentSurfaceDescriptionsNameRequiredArgs, package: internal/mcp, skip_test: true, skip: "the test is the anchor: it iterates the 17-tool allowlist"}
- [x] MCP tool annotations (`readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint`) on every agent-surface tool (16 read-only; `ingest` idempotent and not read-only); `TestMcpToolsDocMatchesRegistry` extended to require them {id: RP-10, symbol: toolAnnotations, package: internal/mcp, reachable_from: TestAdoptionServiceScoped}
- [x] `outputSchema` on every agent-surface tool, generated from the Go result type (`outputSchemaFor`), and `structuredContent` beside the text content on `tools/call`, so a non-Claude client can machine-check the envelope {id: RP-11, symbol: outputSchemaFor, package: internal/mcp, reachable_from: TestAdoptionServiceScoped}
- [x] `negotiateProtocolVersion`: `initialize` echoes the client's `protocolVersion` when it is in the supported table, otherwise the newest supported; the `MCP-Protocol-Version` header is tolerated; `tools/list` order is deterministic and pinned {id: RP-12, symbol: negotiateProtocolVersion, package: internal/mcp, reachable_from: TestAdoptionServiceScoped}
- [x] `TestServeHTTPNegotiatesWithKitClient` drives `/rpc/agent` through agentic-kit's `mcp-client` Streamable-HTTP transport (initialize → tools/list → tools/call) under `httptest`; its failure text is the adoption doc's sentence, replacing every "unverified negotiation" line {id: RP-13, symbol: TestServeHTTPNegotiatesWithKitClient, package: cmd/fathom, skip_test: true, skip: "the test is the anchor; the kit client is the third party"}

## `--json` where the compatibility surface promises it

- [x] `fathom ingest --json` (read-failed list, degraded/refused reasons, counts) so CI can assert `read_failed == 0` {id: RP-14, symbol: renderIngestJSON, package: cmd/fathom}
- [x] `fathom init --json` (every write, receipt id, manual steps as data) {id: RP-15, symbol: renderInitJSON, package: cmd/fathom}
- [x] `fathom thread brief --json` and `fathom thread list --json` {id: RP-16, symbol: renderThreadJSON, package: cmd/fathom}
- [x] `fathom daemon status --json` and `fathom layers list --json` {id: RP-17, symbol: renderDaemonStatusJSON, package: cmd/fathom}

## Documented truth

- [x] `docs/adoption/other-harnesses.md` is generated by `RenderAdoptionMatrix` from `fathom harness capabilities --json` (tier, instruction file, skills path, MCP config path and `mcp add` line, `verified_against` or `unverified`) and pinned by a doc test {id: RP-18, symbol: RenderAdoptionMatrix, package: internal/harness}
- [x] Adoption docs match the binary: `--skills` shapes and `fathom plugin render` in `docs/adoption/claude-code.md` §11, `docs/adoption/codex.md` §1.1/§2/§6 (generated table; Verified-against names the official Codex hooks reference and its date; the vision 16 §3 contradiction resolved), `docs/adoption/README.md`, `docs/use-cases/getting-started.md` (the `.agents/skills` attribution and the session-hook not-indexed line), `docs/use-cases/coding-agents.md` {id: RP-19, oracle: lexical, path: docs/adoption/codex.md, pattern: "\*\*Verified against\*\*"}
- [x] `TestNoStaleHelpPhrasesInDocs` fails on "not yet listed by `fathom help`" and "emits nothing" under `docs/`; the four use-case docs are corrected in the same change {id: RP-20, symbol: TestNoStaleHelpPhrasesInDocs, package: cmd/fathom, skip_test: true, skip: "the test is the anchor"}
- [x] The nine shipped skills (`fathom-onboard`, `-rewire`, `-doctor`, `-activate`, `-plan`, `-verify`, `-feedback`, `-impact`, `-custodian`) describe `--harness`, `.agents/skills`, `fathom plugin render`, `fathom harness capabilities` and the wiring-drift doctor row where each applies; re-rendered into this repo's `.claude/skills` (`.agents/skills` is a link to it); `skills_installed` PASS {id: RP-21, oracle: lexical, path: .claude/skills/fathom-onboard/SKILL.md, pattern: "--harness"}
- [x] `docs/mcp-tools.md`, the use-case `## Maintained` lines and `docs/usage.md` (`--json` envelopes) updated; `TestMcpToolsDocMatchesRegistry` and `TestMaintainedLinesNameRegisteredToolsAndVerbs` green {id: RP-22, oracle: lexical, path: docs/usage.md, pattern: "`--json` envelope"}
- [x] `task stranger-check`: clone the tracked tree into `workspace/tmp`, build with `GOWORK=off`, run `fathom init --harness=none` and `fathom doctor` against a temp store, and fail on any FAIL row — the v0.7.0 lesson as a task, in `task validate` when it runs under two minutes {id: RP-23, oracle: lexical, path: Taskfile.yml, pattern: "clean-clone-check, stranger-check"}
