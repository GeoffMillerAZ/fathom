# Claude Code mods: fathom's evidence where the person works, through an optional fathom-ui plugin

Status: accepted 2026-10-16 (ADR-072). Plan: `docs/plans/claude-code-mods.md` (MOD-). Goal G40. Roadmap:
`docs/vision/38-proof-layer-roadmap.md` §9.7 (after readouts; the tenancy program follows, as RT-M1 reads).
Assessment: `docs/assessments/2026-10-16-claude-code-mods.md`. Constraints carried in: the `No Node toolchain`
non-goal and its carve-out (`docs/intent/scope/non-goals.md`); ADR-071 (readouts own every view of tool output, the
shell-read count and the grep reading; F75, F78); SEC-1 and SEC-2 (no hook runs a command a repository names); I9,
I14, I35, I38, I53, I62; ADR-059 (instruction files); ADR-063 (the observatory owns dashboards); no net tool growth;
the dispatch-loop non-goal; the table freeze until RT-M1 reads PASS; the hold on model spend; ADR-011 (generic
names only); the determinism contract (`internal/AGENTS.md`).

Claude Code facts come from two local sources and nothing else: the plugin typings Claude Code 2.1.301 writes
beside a mod ("typings" below, cited by line; the file says the surface is early access and "may change between
releases without notice", line 4), and the changelog (cited by version). A fact neither source states is marked
UNVERIFIED with the reading that settles it.

## 1. The question, and the verdict

Fathom tells the agent a lot the person never sees: the rules that govern an edit, how far the edit reaches, whether
the index is behind, whether a claimed plan row is proven, whether fathom is installed at all. Mods (changelog
2.1.287) let a plugin draw inside Claude Code's own interface. The verdict: ship an optional second plugin that only
shows, in two steps. First the parts that need only Go and markdown (an install check, a scout subagent, a status
line, a committed marketplace manifest), then one small TypeScript module that draws four things (evidence rows, the
evidence band, the proof line, a setup check). The surface with the most value per line is M2, evidence rows: it
reads what fathom's hooks already produced, so it adds no daemon call and no token. Every mod carries a reading
that can remove it, and none changes what the agent sees, is allowed or runs (ADR-072 decision 1).

## 2. What Claude Code offers, and what fathom uses

| Surface | What it can do | Source | Used here |
|---|---|---|---|
| Hooks module | a plugin's TS or JS module run in process, in an environment of its own with no DOM and no Node; the one import is type-only and empty at run time | typings 14-20; changelog 2.1.287 | M1, the host |
| `classic.<Event>` | fires for every settings-hook event; the chain is managed hooks, hooks modules, then the other settings hooks; the result carries `additionalContext`, one entry per hook | typings 1264-1327 | M2 and M4 read fathom's own hook output |
| `ui.render` on `ToolResult` | redraws a tool's result block on every attached surface; a rewrite of `output` is checked against the tool's schema; the stored result is untouched | typings 10362-10369 | M2 adds one line under the engine's own drawing and never rewrites `output` |
| `ui.render` on `AbovePrompt` | the band above the prompt, terminal and desktop only; it yields to a survey | typings 10673-10690 | M3 |
| `InfoNotice` | one dim status line under the logo, terminal only | typings 10582-10589 | not used (section 12 corrects the research) |
| `$.ui.log` | appends one dim line to the transcript, not sent to the model | typings 2432-2435 | M4 |
| Pane and `$.ui.open` | a pane a plugin opens | typings 2533, 9771 | `/proof`, `/fathom` |
| `$.command.register` | a command that runs without a model turn | typings 3174 | `/proof`, `/fathom` |
| `$.mcp.call` | calls a connected MCP server's tool with the engine's own connection, no permission prompt | typings 2749-2768 | `/proof` calls `verify_plan` |
| `$.http.fetch` | http or https to whatever the host reaches | typings 3555-3573 | the loopback `/statusz` only (I14) |
| `$.process.run` | runs an argv with no shell, 30 s default timeout | typings 3583-3598 | `fathom doctor --json` only |
| `$.state`, `$.store` | session values any plugin may read; a JSON file kept between sessions | typings 3428-3467 | `$.state` only |
| `tool.call`, `tool.check`, `agent.spawn` | hold, deny or answer a call; rewrite a subagent's prompt | typings 4043-4070, 255-285 | none (ADR-072 decision 1) |
| `$.ui.ask` | the question dialog; rejects in a `-p` run | typings 2449-2463 | none |
| Hook budget | 10 s per dispatch with `$` calls excluded; a hook that fails is skipped | typings 5236-5251, 4052 | every hook fails open |
| `claude plugin validate --json` | a mod's hooks and calls, each gating hook with its `.catch`; hooks skipped where an organization manages Claude Code | changelog 2.1.290, 2.1.299 | MOD-M2 |
| `claude plugin test` | a mod's `*.test.ts` cases per surface (terminal, desktop, vscode, mobile) in an environment with no fs, network or process | typings 28-41 | MOD-M2 |
| Plugin `agents/*.md` | `omitClaudeMd`, `effort`, `autoCompactWindow` | changelog 2.1.271, 2.1.267, 2.1.296; typings 543-550 | M6 |
| Plugin `settings.json` | default settings shipped by a plugin; the `subagentStatusLine` payload carries `agentType` and effort | changelog 2.1.49, 2.1.293, 2.1.214 | M7 |
| MCP Apps | drawn by other clients, not by Claude Code (the research); needs public HTTPS | UNVERIFIED locally | rejected |
| Headless | nothing draws under `-p` or in cloud sessions (the research) | `$.ui.ask` rejects in `-p` (typings 2454); the rest UNVERIFIED | every mod is optional; the hooks still serve the agent |

## 3. Two plugins (Q2)

| Plugin | Holds | Works without the other |
|---|---|---|
| `fathom` (core, today's render) | skills, hooks (each starts `command -v fathom >/dev/null 2>&1 \|\| exit 0`, `cmd/fathom/hooknoop_test.go:95`), the MCP registration with the default surface, `agents/fathom-scout.md` (M6), `settings.json` holding only `subagentStatusLine` (M7) | yes |
| `fathom-ui` (new) | its manifests, a hooks file naming one module, `register.ts`, and its test file `register.test.ts`; nothing else | draws "daemon down" and the install line; it reads no fathom fact itself |

Each has its own `pluginVersion` and template digest pin (`cmd/fathom/pluginrender.go:43`, `:120-132`); the committed
marketplace lists both. Why two: an organization can skip a mod's hooks (changelog 2.1.299; the managed-mods-only
setting the research names is UNVERIFIED locally) and still keep fathom whole; a person who wants only the
agent-facing parts installs one plugin; and a break in an early-access surface cannot take the core down.

## 4. The Node carve-out (Q1)

Allowed: render writes one dependency-free TypeScript hooks module and its `claude plugin test` cases, embedded in the
binary as text (`cmd/fathom/pluginui/register.ts` and `register.test.ts`, read by `go:embed`), executed only by Claude
Code. The module imports only `import type { Register } from 'claude-code'`, has no package manifest, uses `h()` calls
from `$.ui.resolve` rather than JSX, and stays within 150 lines; render substitutes two constants (the loopback base
from `--mcp-url`, the core server's name). Refused: any Node, npm or TypeScript install, run or compile in the build,
the gate or CI; a dependency; a bundler; a second file of logic. `.fathom/guard.cue`'s refusal of `npm`, `npx`,
`yarn`, `pnpm` and `bun` stands. The type file Claude Code writes beside a loaded mod (typings 5-7) is never rendered
and is ignored in the committed tree.

Proof without Node in the gate. Go holds what Go can check: render goldens and digest pins, the allowlist scan and
the line cap at render (MOD-13), and fixtures of the hook payloads the module reads, held equal to the Go renderers
(MOD-14, MOD-15). Claude Code checks what only it can run: `claude plugin validate --strict --json` and `claude
plugin test` over those fixtures (MOD-M2), then one live session (MOD-M3). Both are owner-run readings, never gate
steps, because the gate has no Claude Code.

## 5. The allowlist (ADR-072 decision 4)

| Allowed | Why |
|---|---|
| Registrations on `classic.PreToolUse`, `classic.PostToolUse`, `classic.Stop`, `turn.complete` | read the result `next(e)` returns and hand it back unchanged |
| `ui.render` on `ToolResult`, `AbovePrompt`, `Pane` | the three drawings |
| `$.command.register` for `/fathom` and `/proof` | zero-turn commands |
| `$.state` under the module's own keys | per-session summaries keyed by tool-use id; any plugin may read them, and they hold only counts and rule text the agent already received |
| `$.ui.resolve`, `$.ui.log`, `$.ui.open` | drawing |
| `$.http.fetch` to the rendered loopback base only | I14; the band's one GET |
| `$.mcp.call` to the core plugin's fathom server for `verify_plan` only | `/proof` |
| `$.process.run(["fathom", "doctor", "--json"])` | `/fathom`; fathom's own binary with a fixed argv, the same trust as every hook command, never a command a repository names (SEC-1, SEC-2) |

| Refused | Why |
|---|---|
| `tool.call`, `tool.check`, `agent.spawn` hooks; a changed classic result | decision 1: display-only, never a gate |
| `$.model` | the budget hold and the determinism contract |
| `$.store` | the table freeze: no second store of fathom facts |
| `$.tool.register` | no net tool growth |
| `$.agent`, `$.fs`, `$.ui.ask`, `$.ui.notify`, `$.process.spawn`, any other argv | out of scope for a display; `$.ui.notify` is a pushed event (the dispatch-loop non-goal) |

Render refuses a module outside the list and names the reference (MOD-13); `claude plugin validate --json` must list
no gating hook (MOD-M2).

## 6. The portfolio

| Code | Name | Surface | What the person sees | Phase or owner | Rows |
|---|---|---|---|---|---|
| M1 | Mod host | render writes `fathom-ui` | nothing alone; carries M2 to M5 | 2 | MOD-12, MOD-13 |
| M2 | Evidence rows | classic Pre and PostToolUse, the result block | `⚖ 3 rules · ↯ 41 callers, 12 tests [rules]` under a governed edit; `stale index` on a fathom answer | 2 | MOD-14, MOD-16 |
| M3 | Evidence band | the band | `fathom ● @d276556`, `◐ 3 behind`, `○ daemon down`, `? not indexed` | 2 | MOD-6, MOD-17 |
| M4 | Proof line and `/proof` | `$.ui.log` at Stop; a pane | `✗ fathom: RH-6 [x] unproven · /proof` | 2 | MOD-15, MOD-18 |
| M5 | Setup check `/fathom` | a pane | doctor's checklist, or the install line when the binary is missing | 2 | MOD-19 |
| M6 | fathom-scout | core `agents/` | answers of at most 40 lines with receipts | 1 | MOD-5, MOD-M1 |
| M7 | Status line | core `subagentStatusLine`; a user segment | `fathom ● @sha` on each worker row | 1 | MOD-6 to MOD-9 |
| M8 | Install check | a script and RH-6 | the rendered plugin installs and answers | 1 | MOD-4; RH-6 (`docs/plans/reach.md`) |
| M9 | Grep-answerable reading | Go over the discovery log | none | not built | section 7.10 |
| M10 | Read-routing hint | Go PreToolUse context | `fathom: X file:line · 6 callers` | F75 (Q5) | none |
| M11 | Spawn brief | `agent.spawn` | pre-resolved symbols in a subagent's prompt | deferred (Q6) | MOD-X1 |
| M12 | Tests to run | Go hint and a pane | "the diff reaches 7 packages; run these first" | deferred | MOD-X2 |
| M13 | Symbol blast hold | `tool.check` and a question | `[Proceed] [Cancel] [Callers]` | deferred (gates) | MOD-X3 |
| M14 | Review verdict pane | a pane | `review_diff` sections | deferred | MOD-X4 |
| M15 | Committed marketplace | the repository root | the install line works | 1 | MOD-10, MOD-11 |

## 7. Each mod

### 7.1 M8, the install check (Q4)

`scripts/plugin-install-check.sh` renders the core plugin into a scratch HOME under `workspace/tmp/`, runs `claude
plugin validate --strict` on it, loads it with `--plugin-dir` and asserts the plugin and its MCP server are listed
(the listing command's flags are UNVERIFIED and recorded by the script). It starts no model turn, so it spends
nothing, and exits 2 with the reason when `claude` is not on PATH; no gate runs it. RH-6 is then one ordinary
owner session in that HOME: the tools listed and one answered (Q4), recorded as
`docs/bench/readings/reach/install-claude-code.md` with RH-6's own oracle (`^Installed and answered on`,
`docs/plans/reach.md:17`). The mods plan adds no RH-6 row of its own.

### 7.2 M6, fathom-scout

Render writes `agents/fathom-scout.md` into the core plugin. Frontmatter: `tools` lists the plugin's eight MCP tools by
their plugin-scoped names, read from the registry's default surface (`renderPluginSurface`,
`cmd/fathom/pluginrender.go:112`), never hand-copied; `omitClaudeMd: true` (typings 543-550); `effort: low`. The
body carries routing only (which tool answers which question shape, as the AGENTS.md stanza does) and the answer
contract: at most 40 lines, a `file:line` receipt per claim, the index's freshness named. Whether `omitClaudeMd` also
drops an AGENTS.md that Claude Code loads in CLAUDE.md's place (ADR-059) is UNVERIFIED and does not matter here: the
scout needs routing, not project instructions. The plugin-scoped tool name prefix is UNVERIFIED; MOD-M3 records it
and render holds it in one constant. Kill: MOD-M1.

### 7.3 M7, the status line and `fathom statusline`

`fathom statusline [--subagent] [--json]` reads the status-line JSON on stdin defensively (the project directory when
present, else the working directory; fields beyond `agentType` and effort are UNVERIFIED), makes one loopback GET of
`/statusz?format=json&root=<dir>` (MOD-6) under the hook deadline (`internal/hookguard`, RT-7), and prints exactly
one line: `fathom ● @<sha>` (indexed at HEAD), `fathom ◐ <n> behind @<sha>` (or `◐ behind unknown @<sha>`),
`fathom ○ daemon down`, or `fathom ? not indexed`. Never blank and never `0 behind` for an unknown (I38). Speed: CPU
time p95 at most 100 ms (MOD-8); the research reads the refresh as at least once a second with a 300 ms debounce
(UNVERIFIED). The core plugin's `settings.json` holds only `subagentStatusLine`: `command -v fathom >/dev/null 2>&1 &&
fathom statusline --subagent || echo 'fathom: not on PATH'`. A plugin cannot ship `statusLine` (the research,
UNVERIFIED), so `docs/usage.md` shows the person how to add the same verb as a segment of their own status line.

MOD-6 extends `/statusz` additively: `root=<dir>` answers the one service whose root holds `dir`, with
`head_commit` and `behind` (commits between the indexed commit and HEAD, through the read-only git wrapper, under a
200 ms deadline); a miss reads `behind: null` with `behind_reason`, never 0. Today the JSON carries each service's
indexed commit (`cmd/fathom/statusz.go:70-73`) but no HEAD.

### 7.4 M15, the committed marketplace (Q3)

The repository commits `.claude-plugin/marketplace.json` at its root and the rendered trees it names
(`plugins/fathom/`, then `plugins/fathom-ui/`). Render gains a root for the manifest so sources are relative to it
(today it writes beside `--out`, `cmd/fathom/pluginrender.go:381-394`). A test renders into a temporary directory and
requires the committed bytes, with the build-version metadata normalized (the digest already excludes it,
`cmd/fathom/pluginrender.go:134-141`), so the committed tree is never hand-maintained. The README's install line
names the binary first, because every hook and the MCP server need it, then `/plugin marketplace add` with the
repository and `/plugin install` for `fathom` and, optionally, `fathom-ui`. The marketplace name stays generic
(`fathom-plugins`, ADR-011). Not done here: a catalog listing, the brew tap, the MCP registry (F2, RH-X1, Q30).

### 7.5 M1, the host

`register.ts` exports one `register` (typings 62-64). Every hook fails open: an overrun or a throw skips it (typings
4052, 5244), and the module adds no `.catch` that refuses. Render writes it only when the allowlist scan passes
(MOD-13). The test file covers every behaviour in sections 7.6 to 7.9 over the committed fixtures, per surface.

### 7.6 M2, evidence rows

On `classic.PreToolUse` and `classic.PostToolUse` for Edit, Write and MultiEdit, the module awaits `next(e)`, finds
fathom's own entries in `additionalContext` by their header (the obligations header and the blast trailer the edit
hooks write: `cmd/fathom/edithook_pre.go`, `cmd/fathom/edithook_blast.go`), stores a parsed summary in `$.state` by
tool-use id, and returns the result unchanged. The result block then draws one collapsed line under the engine's own
drawing: `⚖ 3 rules · ↯ 41 callers, 12 tests [rules]`, the button opening the rule lines (at most six, I35). A clean
file draws nothing. When the band's last reading is "daemon down" it draws `⚖ rules not checked (daemon down)`,
never nothing (I38). A fathom tool's result whose structured content says `index_stale`
(`internal/mcp/outputschema.go:137`) gets a `stale index` chip. MOD-14 holds the payload text the module parses equal
to the Go renderer, so a change to the hook text fails the gate before the module misreads it. Premise: MOD-M3 reads
whether Claude Code already shows hook context to the person and whether the core's command hooks reach a module's
classic result; either reading against M2 refuses it in the same change.

### 7.7 M3, the evidence band

One row in the band, terminal and desktop only, from MOD-6's answer: the same four states as the status line. It
refreshes on `turn.complete` and after an Edit or Write result, never on a timer (the dispatch-loop non-goal; no
background job), and yields to a survey (typings 10679). Unreachable never reads as `0` (I38). Kill: MOD-M4.

### 7.8 M4, the proof line and `/proof`

On `classic.Stop` the module awaits `next(e)`; when the result carries the stop hook's entry naming newly claimed rows
that fail verification (`cmd/fathom/stophook.go:40-49`), it writes one `$.ui.log` line, `✗ fathom: RH-6 [x] unproven
· /proof`, and returns the result unchanged, so the agent gets the stop hook's advisory (or its block under
`--block`) exactly as today. `/proof` opens a pane with `verify_plan` over the plans the stop entry named, through
`$.mcp.call`. No uncheck or status button (the dispatch-loop non-goal, ADR-049 decision 4). Kill: MOD-M5.

### 7.9 M5, the setup check `/fathom`

A zero-turn command. It runs `fathom doctor --json` through `$.process.run` with a 20 s timeout and draws doctor's
checks as a list (`cmd/fathom/doctor.go`). When `fathom` cannot start, the pane shows the install line and the docs
path: the only surface that speaks when the binary is missing, since every hook exits 0 silently then. The command's
final name (`/fathom` or a plugin-scoped form) is UNVERIFIED; MOD-M2 records it. Proof: MOD-M3 (c), with the binary
off PATH.

### 7.10 Owned elsewhere, or later

- M9 is not built. ADR-071's corpus reading puts bare-identifier greps at 13.5% of 29,643 greps, an upper bound on
  the index-answerable share; a classifier cannot read the non-goal's 30% condition as fired under that bound. If a
  later corpus reading puts the bound at 30% or more, the exact classification belongs to F75's promotion.
- M10 is F75's hint (Q5): it ships only as the recorded firing of the `No grep-blocking` condition, with readouts'
  RO-29 counting the shell reads; this plan has no row for it.
- M11 (Q6), M12, M13, M14 and readouts behind `tool.call` (F78) are deferred rows MOD-X1 to MOD-X5, each with its
  trigger. M11, M13 and F78 change what the agent sees or is allowed, so each also needs an amendment of ADR-072
  decisions 1 and 4.

## 8. Reconciliation with readouts

| Item | Owner | Where |
|---|---|---|
| Any view of a command's output (`updatedToolOutput`, a result replacement) | readouts | ADR-071 decision 2; RO-19, RO-20 |
| Evidence beside a Bash result for the agent (receipt, test delta, compile groups, command obligations) | readouts | RO-5, RO-23 to RO-25 |
| Shell reads counted in instrument B | readouts | RO-29 |
| The index-answerable share of greps | readouts' corpus reading; F75 | ADR-071 context; the non-goal's notes |
| The read-routing hint (M10) | F75 | Q5 |
| One command lexer | readouts | RO-3; MOD-M1 reuses it to count the parent's shell reads |
| Readouts in process behind `tool.call` | F78 | MOD-X5, deferred |
| Doctor's readouts row | readouts (RO-18) | `/fathom` shows whatever doctor prints (MOD-19) |
| Evidence rows on Edit and Write, the stale chip, the band, the proof line, `/fathom`, the scout, the status line, the marketplace, the install check | mods | MOD- |

Sequencing: mods phase 1 starts when readouts phase 0 lands (RO-3 to RO-18 verified, RO-M0 filed). RO-M1's 14 days
of observe need time, not a wave, so they run beside it. ADR-071's rule that no other program's wave starts while a
readouts row is runnable reads a row waiting on its own measurement window or gate as not runnable; mods waves fill
those windows (roadmap §9.7).

## 9. Distribution and versions

- Layout: `.claude-plugin/marketplace.json`, `plugins/fathom/`, `plugins/fathom-ui/`; regenerated by render, held by
  MOD-10's test; the generated type files Claude Code writes are ignored.
- Versions: each plugin's `pluginVersion` bumps on a template change, with its digest pin moved in the same change.
- Licence and naming: the rendered trees carry the repository's own licence; the plugin's name is already public
  through the module path and the release archives. Q-MOD-1 answered yes 2026-10-16 (the owner-architect, under the
  owner's delegation): a self-hosted manifest names nothing the public repository and its releases do not already
  name, so it is outside Q30's "public naming"; a catalog listing and a brew tap wait on F2 and Q30.

## 10. Measurement

| Reading | Question | Bar | Kill | Cost |
|---|---|---|---|---|
| MOD-8 | does the status line stay cheap | CPU p95 at most 100 ms over 200 runs against a fake daemon | the row stays open | $0, a test |
| MOD-M1 | does the scout save the parent a re-read | over at least 30 spawns in natural sessions, the parent reads a file the scout cited within five calls after at most 50% | CUT removes the scout from render in the same change | $0 |
| MOD-M2 | does the module do what this design says on Claude Code's own test host | every terminal case passes; zero gating hooks; only allowlisted calls | FAIL holds every fathom-ui render until fixed | $0 |
| MOD-M3 | do M2's premise and M5's purpose hold live | (a) hook context not already shown to the person, (b) the core's hooks reach the module's classic result, (c) `/fathom` shows the install line with the binary off PATH, (d) the band's states seen | (a) or (b) against: M2 refused in the same change | $0, one owner session |
| MOD-M4 | does the band cut stale-index surprises | stale-index observations in the 30 days after at most half the 30 days before, with at least 4 before | CUT removes the band | $0 |
| MOD-M5 | does the proof line stop unproven claims earlier | claimed-unproven rows reaching the pre-push `fathom verify` (the verify ledger) over 14 days after at most half the 14 days before, with at least 4 before | CUT removes the line and `/proof` | $0 |

Every bar is fixed here before any run (K19). What these figures are not: no token saving is claimed (no saving
without an A/B, P12), and no figure is per person (I58).

## 11. Rejected, and why

| Proposal | Why not |
|---|---|
| Answering a call in place: a grep answered from the index, Grep rewritten to `search`, an auto-approver | the grep-blocking non-goal; it would corrupt instrument B's discovery count; a mod may approve calls, fathom's never does |
| A HUD or cost meter | the generic token-dashboard non-goal; two public status-line projects already hold about 28,600 and 13,300 stars |
| Fathom's verbs re-registered with `$.tool.register` | no net tool growth |
| `$.model` routers or summaries | the budget hold and the determinism contract |
| A monitor or channel pushing events | the dispatch-loop non-goal |
| A module's own view of tool output (a whole-file read summarized) | readouts own output views behind ADR-071's contract (a read of a file the session edited stays raw; the outline class is gated by RO-M0 and RO-M3); a second engine would bypass it. This corrects the research, which rejected summaries outright: readouts' `read-outline` is planned behind those gates |
| An MCP App | Claude Code does not draw one; it needs public HTTPS and auth, against I14. Revisit when fathom has an authenticated remote surface |
| A live published-page dashboard | a published page cannot call a local MCP server, and the observatory owns dashboards (ADR-063) |
| An LSP plugin | the IDE language-services non-goal; eleven official LSP plugins exist (the research) |
| Uncheck or status-setting buttons | the dispatch-loop non-goal; ADR-049 decision 4 |
| `$.store` counters | a second store of fathom facts, against the table freeze; readings come from existing logs |

## 12. Open facts, and corrections to the research

| Fact | Status | Settled by |
|---|---|---|
| Claude Code already shows hook `additionalContext` to the person | UNVERIFIED | MOD-M3 (a) |
| A plugin's command hooks run beneath another plugin's module, so `classic.*` results carry them | UNVERIFIED (the chain names "the other settings hooks", typings 1277-1278) | MOD-M3 (b) |
| VS Code draws the result block | the typings say every attached surface (10369); live UNVERIFIED | MOD-M3 records it |
| The plugin-scoped MCP tool names and server name | UNVERIFIED | MOD-M3 records them; render holds one constant |
| Which keys a plugin's `settings.json` keeps | UNVERIFIED | MOD-4's validate run and MOD-M3 |
| `subagentStatusLine` fields beyond `agentType` and effort | UNVERIFIED | MOD-7 reads defensively |
| SubagentStart's input carries the task prompt | UNVERIFIED | MOD-X1's trigger |
| `tests select` speed and output shape | UNVERIFIED | MOD-X2's trigger |
| The managed-mods-only setting's name | UNVERIFIED locally; organizations can skip a mod's hooks (changelog 2.1.299) | Q2 holds either way |
| Official marketplace submission | UNVERIFIED; out of scope | F2 |
| Claude Code 2.1.302, installed locally, has no changelog entry | UNVERIFIED | MOD-M2 records the version it ran on |
| "No competitor" findings | negative search results, a hypothesis | none |

Corrections found while recording: `InfoNotice` is the dim line under the logo, terminal only (typings 10582-10589),
so M4 uses `$.ui.log`; `/statusz` JSON does carry each service's indexed commit (`cmd/fathom/statusz.go:70-73`),
and only HEAD and the distance are missing (MOD-6); `$.ui.ask` rejects in a `-p` run (typings 2454); the 44%
shell-read share the research could not find is F75's evidence (`docs/intent/futures/f74-f79.md`); the research's
rejection of output summaries is reconciled with readouts (section 11).

## 13. Jev, weighed

No site. The module formats facts Go computed; a typed judgment could order the rule lines in an evidence row, but
the row shows counts, not choices, and a judgment never asserts a code fact (I62). With provider `none` nothing
changes. The measurement that would open a site: M11 built, and its choice of which symbols to pre-resolve contested
on written cases; ADR-056 R3's calibration would come first, and any A/B waits on the budget lift.

## 14. Risks, and the measurement that kills each

| Risk | Measurement | Kill |
|---|---|---|
| The early-access surface changes under the module | MOD-M2 re-read on the Claude Code version each fathom-ui render ships against | a failing module is not rendered; the core is untouched |
| A strip disagrees with what the agent received | MOD-14's fixture parity | any mismatch is a defect that blocks the render |
| The band reads healthier than the truth | MOD-M2's I38 cases (unknown never 0, down never blank) | a failing case blocks the render |
| The committed tree drifts from the render | MOD-10's test | the gate fails |
| A stranger installs a prerelease plugin without the binary | `/fathom` (MOD-19) and MOD-M3 (c) | the README names the binary first |
| A mod adds lag | the 10 s budget skips an overrun (typings 5244); the band makes one GET per turn | MOD-M3 notes visible lag; any lag over a second removes the band |
