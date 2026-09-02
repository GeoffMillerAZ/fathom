# 25 — The harness port: adapters are data behind one seam

> Owner ask, 2026-09-01: *"Are there adapters that we can build or make
> this more of a hexagonal capability with the more popular agent
> frameworks out there like Claude and Codex and OpenAI and Gemini and
> all of the major popular frameworks and have these adapters ready to
> go but keeping it as DRY as possible?"* Also: *"help with automated
> coding like Claude and Codex"*, *"update any adoption and guidance
> things like skills or documentation."*
>
> Method note: synthesis of a 14-agent evaluation (2026-09-01): four
> external lanes (landscape, harness contracts, field patterns, bench
> corpora), five internal lanes (harness integration, bench machinery,
> kit extension points, open rows, adoption surface), three strategists
> (wedge-first, kit-first, ecosystem-first) and two adversarial critics
> (intent conformance, wheel reinvention). Every verdict cites the lane
> that produced it. Full reports: session scratchpad `research/`,
> distilled into this doc and `26-proof-credibility.md`.
>
> Status: ACCEPTED 2026-09-01 as ADR-029. Plans: `../plans/harness-port.md`,
> `../plans/reach-parity.md`.

---

## 0. The verdict in one paragraph

The extension surface of coding harnesses collapsed onto four open
standards during 2026 — `AGENTS.md`, Agent Skills (`SKILL.md` under
`.agents/skills`), Agent Plugins 1.0 (`plugin.json` + `skills/` +
`mcp.json`), and MCP over HTTP — plus one de-facto wire format: Claude
Code's hook JSON, which Codex, Copilot, VS Code, Gemini CLI, Cursor and
Amp all shim toward (lane E2). Fathom's harness coupling is already thin
and half-factored: every hook verb is stdin-envelope → payload function →
stdout-envelope, the payload layer is vehicle-neutral (`fathom prime`
proves it for the primer), and what is Claude-shaped is one hook table,
four decode sites, four encode sites, one `claude|codex` string switch and
nine files of `.claude/` path literals (lane I1). So the hexagonal port
the owner asked for is not a framework and not a kit package: it is **one
descriptor registry (data) plus one ~100-line codec per harness (code)**
inside fathom, with Claude Code's output held byte-identical by the
existing goldens, and a **degradation ladder** rendered honestly for
every harness that lacks a vehicle. Adapters become configuration. The
kit gets the contract only when a second consumer deletes its own copy
onto it (FA-22, filed; D8 stands).

## 1. What the field settled (evidence, not opinion)

| Fact | Evidence | Consequence for fathom |
|---|---|---|
| `AGENTS.md` is native or opt-in on 13 of 15 harnesses; Claude Code reads `CLAUDE.md` (and can import `AGENTS.md`); Gemini needs a settings key; Zed uses first-match across nine filenames | E2 F5 | The dual `CLAUDE.md`+`AGENTS.md` marker stanza is the right primitive. Add `mode=settings-key` (Gemini) and a first-match shadowing warning (Zed) as descriptor data. |
| `SKILL.md` is honored by 13 of 15; `.agents/skills` is read by Codex, Gemini, Cursor, Copilot, OpenCode, Windsurf, Amp, Zed, Warp, Junie; `.claude/skills` by Claude Code, Cursor, Copilot, OpenCode, Cline, Windsurf, Amp. The pair covers 14 of 15. | E2 F6 | `fathom init --skills` writes both by default. The legacy `~/.codex/prompts` twin retires to an explicit flag. |
| Agent Plugins 1.0 (2026-08, TSC: Amazon, Anysphere, Microsoft, OpenAI, Vercel, Google) standardises `plugin.json` + `skills/` + `mcp.json`; Claude Code keeps `.claude-plugin/plugin.json` | E1 F9, E2 F7 | `fathom plugin render` emits both manifests over one `skills/` tree. No marketplace of our own (F2 forbids it). |
| MCP over HTTP is supported by all 14 MCP-capable harnesses; the config file and key names differ nine ways; tool-name patterns diverge (`mcp__S__T`, `mcp_S_T`, `@S/T`, `mcp:S:T`) | E2 F8 | An `MCPTarget` row per descriptor (path, format, HTTP key, `mcp add` command, tool-name pattern). Hook matchers that name MCP tools are generated per harness. |
| Claude Code's hook envelope is the interchange format. Codex ≥0.152 has 11 Claude-shaped lifecycle events with `hookSpecificOutput`, `additionalContext`, exit-2 blocking, and a hash-trust step. Copilot accepts PascalCase aliases; VS Code reads `.claude/settings.json`. | E2 F1, F2 | Claude-shaped JSON stays the canonical wire; a codec transcodes per target. **`docs/intent/domain.md` assumption "Claude Code and Codex … hook events keep their shapes" has FIRED** and is restated in this change. |
| Subprocess hooks with stdin/stdout JSON exist on 8 of 15; context injection on 7 (Windsurf blocks only); in-process JS hooks on 3 (OpenCode, Amp, Cline SDK — unreachable from a Go binary under the no-Node non-goal); none on 4 (Zed/ACP, Aider, Continue, Warp) | E2 F3 | The degradation ladder is a first-class output: **A** hooks + inject, **B** block-only, **C** JS-only, **D** none. Tiers C and D get obligations through the stanza and the `obligations` MCP tool, and the generated capability table says so. |
| Fail-open is the norm outside Claude Code and Codex: Copilot hooks time out fail-open; Cursor is fail-open unless `failClosed:true`; Gemini treats any non-JSON stdout as Allow | E2 F4 | Every descriptor carries `failMode`. A guard on a fail-open harness is advisory and the doctor row says so (`gate_posture`). agentic-kit's ledger gains "fail-open guards are decoration" (§ in `docs/practices.md`). |
| Harness docs and formats churn quarterly (Windsurf → Devin Desktop, Codex docs moved, Amp removed custom commands, Cursor merging commands into skills) | E2 F11 | Every descriptor carries `verified_against` (doc URL + date + CLI version) or the string `unverified`; the doctor flags drift; nothing in fathom hard-codes a doc URL as behavior. |
| Tree-sitter → SQLite → MCP symbol graphs are a commodity (Graphify 114k stars, CodeGraph 69k, GitNexus 47k). Nobody surveyed verifies "done" against an index, injects cited obligations at edit time, or expires memory by symbol drift. | E1 headline | Reach is how the proof layer gets used; the port exists to carry the proof layer, not to compete on graph-building. |

## 2. The seam (design)

### 2.1 Vocabulary

- **Capability** — a thing fathom can do for a session, independent of any
  harness: `prime` (session primer), `obligations-post-edit`,
  `guard-pre-edit`, `guard-pre-shell`, `discovery-observe`, `stop-verify`,
  `deposit-session-end`, `deposit-pre-compact`, `mcp-tools`, `skills`,
  `stanza`, `verify-gate`, `claim-gate` (26 §4), `session-ledger`.
- **Vehicle** — how a capability reaches a session on a given harness:
  `hook-inject`, `hook-block`, `pull` (the agent runs a verb, e.g.
  `fathom prime`), `stanza` (standing text), `mcp` (a tool call), `gate`
  (lefthook/CI), `absent`.
- **Descriptor** — one harness, as data: identity, detection rule,
  instruction files and mode, hook file(s) and their shape, the event
  bindings (canonical event → harness event name + matcher + stdin/stdout
  field map), `failMode`, scope-merge rule (Claude merges project + user
  scope; a second harness may not), skills dirs and variant, MCP target,
  env contract, transcript availability, `verified_against`.
- **Codec** — per harness: `DecodeInput(event, stdin) → HookInput`,
  `EncodeOutput(event, HookOutput) → stdout`. The Claude codec is a move
  of today's four decode and four encode sites; its output is pinned by
  the existing goldens (`TestPluginRender`, `TestCmdInit_WiresRepoEndToEnd`).
- **ToolClass** — `FileEdit | Shell | DiscoveryRead | DiscoveryGrep |
  DiscoveryGlob | Other`, replacing the `Edit|Write|MultiEdit|Bash|Grep|
  Glob|Read` literals. A descriptor maps harness tool names to classes
  (Codex: `apply_patch` → FileEdit, `shell` → Shell).

### 2.2 The rules

1. **Zero behavior change for Claude Code.** `Registry["claude-code"]`
   reproduces `fathomHookSpecs` byte-identically; every existing golden
   stays green with no edit. If a golden changes, it is not a seam and the
   work stops (plan HP kill).
2. **Additive surface.** `--harness <id>` on the five hook verbs, default
   `claude-code`; `fathom init --harness=claude-code|codex|agents|none`;
   `fathom harness list` and `fathom harness capabilities [--harness id]
   --json`. The compatibility surface (`docs/intent/scope.md`) gains the
   flag and the verb; nothing existing changes shape.
3. **The stanza stays one body.** Its two hook sentences become
   vehicle-neutral, generated from the descriptor of the harness being
   wired, so the text never promises a channel the harness lacks (the
   EP-22 gated-sentence predicate already does this for intent).
4. **Writes go through the wiring engine.** A harness whose hook file
   shape differs (Codex `hooks.json`, Cursor `hooks.json`, Codex
   `config.toml [mcp_servers.fathom]`) needs a new `wiring.KeyKind` with
   plan/apply/undo and a receipt (I13, law 15). This is the one non-thin
   part of any adapter and it is what makes `fleet reconcile` and
   `fleet undo` work for free afterwards.
5. **Honesty by construction.** The capability table for every harness is
   generated from the registry and pinned by test; `docs/adoption/codex.md`
   §6 becomes a rendering of it; an `absent` vehicle prints as absent; a
   descriptor with `verified_against: unverified` renders every row of
   its table as *unverified on this machine* and the doctor says the
   same. No harness gets a guessed PASS.
6. **Detection lives in init and doctor only.** Hook verbs never sniff
   the harness; the wired command spells `--harness`.
7. **Measurement stays Claude-only by design.** The session ledger parses
   Claude Code transcripts; `Descriptor.Transcript == ""` ⇒ the
   `session-ledger` capability is absent and law 12 forbids extrapolating
   a Claude number to another harness. A `TranscriptReader` port is not
   built until a second harness exposes transcripts (I1 F7).
8. **The kit does not get the port yet.** agentic-kit carries harness
   knowledge in three hand-rolled places (`integration.md` stanzas,
   `install-intent.sh`, the fathom-pack); fathom carries the real one. D8
   says a kit package lands when a second consumer deletes its copy onto
   it. Ask FA-22 is filed in the kit with exactly that trigger; the
   descriptor's JSON export is CUE-validated in fathom (`schema/harness`)
   so the shape is ready to move.

### 2.3 The descriptor set at founding

| id | tier | instruction files | hooks | skills | MCP | verified |
|---|---|---|---|---|---|---|
| `claude-code` | A | `CLAUDE.md` (+ `AGENTS.md` mirror) | `.claude/settings.json` (+ user scope, merged) — today's 8 groups | `.claude/skills` (+ `.agents/skills`) | `claude mcp add` / `.mcp.json` | this machine, 2.1.x |
| `codex` | A (unverified) | `AGENTS.md` | `.codex/hooks.json` / `~/.codex/hooks.json`, Claude-shaped events, exit-2, `additionalContext` cap, hash trust | `.agents/skills` | `config.toml [mcp_servers.fathom] url=` | unverified (no local install) |
| `gemini-cli` | A (unverified; codec — a local install exists to verify against) | `GEMINI.md`, or `AGENTS.md` via settings `context.fileName` | `settings.json` hooks; flat decision + `hookSpecificOutput.additionalContext`; parse-failure means allow | `.agents/skills` | `settings.json mcpServers.<name>.httpUrl` | unverified |
| `cursor` | A/B (data row, no codec) | `AGENTS.md`, `.cursor/rules` | `.cursor/hooks.json`; fail-open unless `failClosed` — documented field map, not wired | `.claude/skills`, `.agents/skills` | `.cursor/mcp.json` | unverified |
| `copilot` | B (data row, no codec) | `.github/copilot-instructions.md`, `AGENTS.md` | Claude-format (PascalCase) hooks; VS Code reads `.claude/settings.json`; timeout fail-open — documented field map, not wired | `.agents/skills`, `.claude/skills` | `~/.copilot/mcp-config.json` | unverified |
| `agents` | C/D | `AGENTS.md` | none | `.agents/skills` | printed per-harness `mcp add` line | n/a (the generic pull tier) |

A codec is written only for a harness with a documented Claude-shaped
contract AND a local install to verify against (P20: a named consumer):
Codex and Gemini CLI on this workstation. Cursor and Copilot ship as data
rows whose documented field maps make a codec a fixture away when an
adopter appears. Tier C/D harnesses (OpenCode, Amp, Zed/ACP, Junie, Warp,
Continue, Aider, Windsurf) are documented rows in the generated table, not
descriptors with codecs: stanza + MCP + skills, and an honest "no
injection here" line. ACP editors reach fathom through MCP passthrough
(Zed forwards `context_servers` into `session/new`); an ACP agent is
refused — fathom is not an agent (no chat, no dispatch).

## 3. The sequencing rule, amended (ADR-029)

`docs/vision/04-coding-agents.md` §5 and `24` §6 said: only pull-tier
work for a second harness until the Claude Code numbers are green (scope
G2). Two premises of that rule changed:

1. Codex now has Claude-shaped hooks, so a Codex inject tier is a data
   row plus a codec on the seam, not a second implementation.
2. The seam refactor is zero-behavior and was already registered as
   direction ("payload vs vehicle, never Claude vs Codex; parity becomes a
   config change" — 24 §6).

ADR-029 therefore amends the rule narrowly: **a second harness's inject
tier may ship as a descriptor + codec + fixtures marked `unverified`,
written by `init --harness=<id>` only on that explicit flag, with no
measurement claim.** G2 (the Claude A/B) is untouched: no Claude-tier
instrument is repointed, no savings number is admissible for any harness,
and doctor's `hooks_firing` is the only thing that can turn `unverified`
into a dated `verified_against` — a real session on a real install.

## 4. What this is NOT (refused, with the reason)

- **A generic hooks framework in agentic-kit.** Single consumer; VISION
  "not a framework"; ADR-022/023 precedent. FA-22 carries the trigger.
- **A neutral hook schema.** Claude Code's envelope is what the others
  shim to; inventing a third shape adds a translation nobody asked for.
- **A plugin marketplace or registry listing.** F2 (public release) is
  not promoted; one Agent Plugins manifest plus one `.claude-plugin`
  manifest over one tree is enough for seven clients.
- **Reimplementing `claude|codex|gemini|cursor mcp add`.** Print or
  invoke them.
- **JS plugin hosts for OpenCode/Amp/Cline.** No-Node non-goal.
- **Memory or recall.** Every tier-A harness ships one and they churn
  (Cursor removed theirs). Threads stay evidence of work (ADR-028).
- **An ACP agent.** 36 agents and six client families exist; fathom is
  reachable through MCP passthrough with zero ACP code.
- **Emulating inject-only guards via allow/deny on harnesses without an
  inject channel.** §5.3 inject-only and the no-grep-blocking non-goal;
  declare the vehicle absent instead.
- **Codecs for harnesses nobody here runs.** P20 and P8: a codec without an
  install to verify against is a promise with no consumer population; the
  descriptor's data row carries the documented field map instead.
- **Writing any user-global file from `init --harness`.** Autonomy: global
  agent configuration is ask-first; `init` prints the harness's own
  `mcp add` line and writes repo-scoped files only.

## 5. Promotion and the kill

- Promote a descriptor from `unverified` to `verified_against` only when
  doctor `hooks_firing` sees beats from a real session on that harness.
- Kill: a descriptor still `unverified` 60 days after its codec lands is
  demoted to tier D in the generated table (stanza + MCP only) and its
  `init --harness` write refuses with the reason. Data rows are cheap;
  promises are not.
- The seam itself has one kill: any changed Claude Code golden byte.
