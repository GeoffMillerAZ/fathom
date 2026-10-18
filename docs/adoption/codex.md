# Adopting fathom with Codex

Fathom's index, MCP surface, and git-native gates are Claude-Code-agnostic —
Codex is a first-class MCP client and gets full read access to the same
index. Since 2026-09-12 the **enforcement stack** works here too, and that
is a measurement rather than a reading of the documentation: a headless
`codex exec` session on codex-cli 0.153.1 ran fathom's own hook entries and
left beats for SessionStart, PreToolUse and PostToolUse, and its five stdin
captures are committed as fixtures (`internal/harness/testdata/codecs/codex/
*.live.in.json`). Two things are worth knowing before you start, both
measured the same day: hooks load **only** from `$CODEX_HOME/hooks.json` —
a project `<repo>/.codex/hooks.json` is not read at all on this build — and
because fathom never edits user-global configuration unasked, `fathom init
--harness=codex` writes no hook file and `fathom harness hooks --harness
codex` prints the entries for you to paste. See §6.

Build/index/daemon setup is identical to the Claude Code path — this guide
does not repeat it. If you haven't built `bin/fathom` or stood up a
`serve-http` daemon yet, do that first via
[docs/adoption/claude-code.md, Install through Run the daemon](claude-code.md) (or run `fathom init`,
which wires the parts common to both clients), then come back here for the
Codex-specific registration and honesty boundaries.

> **Version caveat.** Codex's MCP configuration surface has moved fast and
> differs by CLI version. Everything below reflects the documented `config.toml`
> schema and `codex mcp add` CLI as of this writing; treat any flag or field
> name here as **"verify against your Codex version"** (`codex --version`,
> `codex mcp --help`) before depending on it — this repo has no Codex install
> to test against.

---

## 1. What `fathom init` already did for you

If you ran `fathom init` in the repo (per the Claude Code guide), it already:

- wrote `.fathomignore`,
- ingested the repo into the daemon,
- stamped the fathom operating-contract stanza into `AGENTS.md`, the one
  instruction file (ADR-059; `cmd/fathom/init.go`'s `fathomStanza`) —
  **Codex reads `AGENTS.md` automatically**, and so does Claude Code
  2.1.277 and later, so there is nothing further to do for the
  standing-doctrine piece (markers, re-run semantics, the `@AGENTS.md`
  import a root CLAUDE file gets, and the corruption-refusal case are
  documented once, in
  [docs/adoption/claude-code-internals.md §3.1](claude-code-internals.md#31-the-agentsmd-stanza-and-the-agentsmd-import--markers-re-run-semantics-corruption-refusal)), and
- if you passed `--skills=codex` (or a bare `--skills`), rendered the ten
  shipped `fathom-{plan,verify,onboard,feedback,doctor,impact,rewire,activate,custodian,findings}`
  skills into `<repo>/.claude/skills/` and linked `<repo>/.agents/skills` —
  the path Codex documents — to it, §1.1 below.

The stanza and hook writes go through one wiring engine (law 15): a
receipt lands in `.fathom/receipts.jsonl`, `fathom fleet undo --last`
reverses it, and a marker region a human edited is BLOCKED and named
rather than overwritten — the details are in the Claude Code guide's §3.1
and apply to `AGENTS.md` identically.

`init` does *not* register Codex's MCP client (that's a Codex-side,
interactive step, same reason it doesn't call `claude mcp add` for you
either) and does not touch `lefthook.yml` (hand-curated, printed as a manual
step instead — §4).

### 1.1 Installing the skills: `--skills=codex` (`.agents/skills`)

```sh
bin/fathom init --skills=codex          # <repo>/.claude/skills/<name>/SKILL.md + .agents/skills -> ../.claude/skills
bin/fathom init --skills                # bare = all: the same
bin/fathom init --skills=codex-prompts  # the deprecated ~/.codex/prompts twin, explicit only
```

Renders the ten shipped skill sources
(`dist/agents/skills/fathom-{plan,verify,onboard,feedback,doctor,impact,rewire,activate,custodian,findings}/`)
once, into `.claude/skills`
([docs/adoption/claude-code.md, Skills](claude-code.md#skills)),
and makes the path Codex's skills documentation lists —
`$REPO_ROOT/.agents/skills` (also every directory from cwd up to the repo
root, and `$HOME/.agents/skills` for personal skills) — a relative link to it
(ADR-059: one copy cannot drift the way two rendered copies did). Every
shipped `SKILL.md` carries only the Agent Skills spec's six frontmatter
fields with `name` equal to the directory (held by test, vetted against
`schema/skills/schema.cue`), so the bytes Codex reads through the link are
spec-valid. The link is written through the wiring engine (receipted,
`fleet undo` removes it); an earlier real `.agents/skills` holding only
fathom's unedited copies is replaced by it, and one file fathom did not write
there blocks it by name. Invoke a skill with `$fathom-<name>` or `/skills`;
Codex also invokes a skill implicitly when its description matches.
Whether Codex follows a directory link is unverified on this machine (plan
row AG-OP2): if it does not, file an observation.

**Since 0.8.0** (`docs/upgrades/0.8.0-skills-on-the-standard.md`):
`--skills=codex` no longer writes `~/.codex/prompts/`. Codex marks custom
prompts deprecated ("use skills"); the twin still renders behind
`--skills=codex-prompts` — the one user-global write `--skills` can make —
and is invoked as `/prompts:<name>`. Re-running any mode is idempotent the
same way the Claude Code path is: every written file carries a
`<!-- fathom:owned -->` marker, so a pre-existing, non-fathom-owned file
with the same name is never clobbered — `init` refuses instead (exit `1`,
naming the path), and `fathom fleet reconcile` BLOCKS it by name.

**Verified against.** The skills discovery table and the custom-prompts
deprecation are from https://developers.openai.com/codex/skills.md and
https://learn.chatgpt.com/docs/custom-prompts.md, fetched 2026-09-02
(latest release rust-v0.152.1, 2026-09-01; the installed `codex-cli` here
is 0.153.1). Whether *this* Codex build loads `.agents/skills` has still not
been exercised on this machine — the 2026-09-12 verifying session proved
hooks and MCP, not skills — and `fathom doctor --harness codex` says so on
its `skills_installed` row. Verifying one thing does not verify its
neighbours.

---

Codex **cloud** tasks take the same committed setup script as Claude Code on
the web; [cloud-sessions.md §2](cloud-sessions.md#2-codex-cloud) has the
environment steps and what is unverified there.
A cloud task whose harness takes only a stdio command reaches the VM's daemon through
`fathom serve-mcp --url <daemon rpc url>`, which opens no store.

## 2. Register fathom as an MCP server for Codex

Codex reads MCP server definitions from `config.toml`:

- Global: `~/.codex/config.toml` (applies to every session)
- Project-scoped: `.codex/config.toml` in the repo (only honored for
  **trusted** projects — set `trust_level = "trusted"` if you keep it there)

The transport is selected by which keys are present: `command` means stdio,
`url` means a remote (streamable HTTP) server.

### Option A — stdio (`serve-mcp`), guaranteed-compatible

The single-session pattern; HTTP (Option B) is the shared one. Same
single-writer-lock caveat as the Claude Code stdio path applies: one
`serve-mcp` process per concurrent Codex session will race for the store
lock, so this is a single-session pattern.

```toml
[mcp_servers.fathom]
command = "/abs/path/bin/fathom"
args = ["serve-mcp", "--store", "/abs/path/workspace/db/fleet.sqlite"]
```

Or via the CLI helper:

```sh
codex mcp add fathom -- /abs/path/bin/fathom serve-mcp --store /abs/path/workspace/db/fleet.sqlite
codex mcp list      # verify against your Codex version — exact output format varies
```

### Option B — the shared `serve-http` daemon (streamable HTTP)

If you already run the shared daemon for Claude Code (recommended — one
index, one lock-holder, §5 of the Claude Code guide), point Codex at it
instead of spawning a second stdio process:

```toml
[mcp_servers.fathom]
url = "http://127.0.0.1:7575/rpc/agent"
```

or:

```sh
codex mcp add fathom --url http://127.0.0.1:7575/rpc/agent
```

`/rpc/agent` is the same eight-tool default surface Claude Code registers
against (§10 of the Claude Code guide); `/rpc/extended` is the 21-tool opt-in
superset, and the full registry
is at `/rpc` if you want everything. Off-surface calls still
execute and are only flagged in the call log — surfaces are never a
security boundary.

**What is proven.** fathom's `/rpc/agent` negotiates with a Streamable-HTTP
MCP client fathom did not write (agentic-kit's `mcp-client`, 2025-11-25
stateless mode): initialize → tools/list → tools/call succeed end to end
under `httptest` — `TestServeHTTPNegotiatesWithKitClient` in `cmd/fathom`
runs on every `task validate`, and its failure text is this sentence.
`initialize` echoes any of `2025-11-25`, `2025-06-18`, `2025-03-26`,
`2024-11-05` and answers `2025-11-25` otherwise; the `MCP-Protocol-Version`
header is tolerated on every request. What is NOT implemented: SSE streaming
responses, `Mcp-Session-Id` resumability, and the 2026-07-28 modern era
(`server/discover`, per-request `_meta`) — a client that requires any of
those falls back to Option A. Confirm your Codex build the usual way
(`codex mcp list`, then a real tool call).

---

## 3. The codex-cli provider lane (for `fathom compile`)

If you want fathom's semantic-layer compile step
(`fathom compile --dry-run=false`, `serve-http`'s `compile_layer`) to ride
your ChatGPT plan instead of a metered API key, `codex-cli` is one of six
provider lanes (`cmd/fathom/llmprofile.go`):

```toml
# ~/.fathom/config.toml
[profiles.codex]
provider = "codex-cli"
# bin = "/custom/path/to/codex"   # optional — overrides FATHOM_CODEX_BIN
```

or via env: `FATHOM_LLM_PROVIDER=codex-cli fathom compile --service <svc> --dry-run=false`.

**The Ping tripwire.** Constructing this client doesn't just shell out — it
Pings the installed `codex` CLI first
(`agentic-kit/foundations/llmclient/codexcli`, ~300ms, zero tokens,
local-only) and **fails construction loudly** if the CLI doesn't answer.
This exists because the adapter depends on undocumented CLI behavior
(system-prompt mapping, ambient-context suppression) that a Codex CLI
upgrade could silently drop — fathom would rather refuse to compile than
risk a silent quality regression from a degraded CLI. If `fathom compile
--profile codex` fails at construction, that's the tripwire firing, not a
bug — check `codex --version` and that you're logged in.

**Cost — read this before you assume "subscription = free."** Unlike
`claude-cli` (near-zero ambient overhead via `--safe-mode`), the `codex-cli`
lane has a **real, non-trivial marginal cost against your plan's limits**:
roughly **10k input tokens of ambient overhead per call** (~300k for a
31-section full compile), around **60% cache-discounted at steady state**.
It draws from ChatGPT plan limits rather than a metered key, so `cost_usd`
in a `compile_layer` result is the same API-equivalent accounting every
lane uses, not an actual dollar charge — but the token volume is real and
materially heavier than the `claude-cli` lane's footprint. Budget
accordingly, especially for a first full-repo compile.

```sh
fathom providers --profile codex   # health check, $0 (a Ping, not a completion)
fathom smoke --profile codex       # one real tiny completion — confirms it actually works, small $
```

---

## 4. Gates: plain git hooks, same as Claude Code

The pre-push `fathom verify` gate is git-native — it runs on `git push`
regardless of which agent produced the commits, Codex included. Nothing
Codex-specific to configure — use the exact same lefthook block as the
Claude Code guide, verbatim (it runs `fathom verify … --quiet`: failing
rows plus the summary, one line per green plan):
[docs/adoption/claude-code-internals.md §7.4](claude-code-internals.md#74-pre-push--the-fathom-verify-gate-lefthook).

In CI, `fathom verify --strict` (claimed-but-Unknown also blocks) with exit
2 treated as a failure — CI builds its own store rather than trusting a
live daemon, so this is identical regardless of which agent's commits are
being checked (*experimental here*: this repo has no CI runner yet;
`scripts/ci.sh` is the manual strict edition). See `docs/plans/README.md`
for the plan-checklist grammar (`{id, symbol, package, ...}` annotations)
that `verify` gates against — it too has nothing Claude-Code-specific in
it, and neither do the reachability facts behind it: a function passed as
a value (`mux.HandleFunc`, route tables, middleware) is an edge, and a
call inside a closure counts for the enclosing test — the old "call it at
the test's top level" rule is retired
([spec-driven-development.md §3](spec-driven-development.md)).

---

## 5. What `AGENTS.md` carries (and what it can't)

`fathom init` writes its one stanza into `AGENTS.md` (ADR-059) —
index-first tool discipline, the refresh contract, the
`fathom verify` command. This is a **pull** channel: it's advisory text a
capable agent reads and (hopefully) follows, not an injected or enforced
one. It is the *only* fathom-adoption channel available to Codex beyond raw
MCP tool availability and the git-native gates — there is no Codex
equivalent of a PostToolUse or Stop hook today, so obligations
injection and the done-means-proven session check (§6) don't reach a Codex
session the way they reach a Claude Code one.

**The pull tier — what a Codex session runs by hand.** The stanza itself
now instructs it: `fathom prime` prints the exact primer Claude Code's
SessionStart hook injects (service identity + freshness, a live
`find_symbol` example, the continuity-thread brief, the custodian
attention line) — run it at session start. Then:

```sh
fathom prime                                   # the session-start primer, on demand
fathom plans next --service <svc> --eligible-only   # the work frontier, minus rows blocked on an unmet needs:
fathom plans context --service <svc> --row <id>     # the row's ctx: pack, budgeted
fathom thread brief                            # this repo's open items (≤2,400 bytes)
fathom thread note "…"                         # before ending: file what the next session must know
fathom thread handoff                          # a harness-neutral Markdown handoff of the open work
fathom config audit                            # the custodian's evidence (no attention line without the hook)
```

`thread note`/`brief`/`list` are scoped to this repo's service id by
default (`--all-repos` widens; `--thread` overrides) — same store, same
verbs, whichever harness wrote to it.

**Claude files beside `AGENTS.md`.** Codex reads `AGENTS.md` root-down and needs no CLAUDE
file. A repository that also serves Claude Code keeps each CLAUDE.md import-only: exactly
`@AGENTS.md`, beside the root AGENTS.md and every nested one (ADR-059 amendment 2026-10-02);
the custodian kind `agents_md_reaches_claude` with `import_only: true` holds it
([claude-code-internals.md §3.1](claude-code-internals.md#31-the-agentsmd-stanza-and-the-agentsmd-import--markers-re-run-semantics-corruption-refusal)).

**Reports that read and never act.** `fathom plans lines` (branches, worktrees and stashes as
landed, at-risk, idle or active), `fathom gc` (its report-only `strays` and `binaries`),
`fathom plans status` (an archivable plan names its `fathom plans archive` command) and
`fathom report used <dir>` (a vendored dependency's used closure) are plain CLI verbs, so they
work the same from Codex ([usage/stewardship.md](../usage/stewardship.md)). `fathom ingest`
prints a `pass-times (wall/cpu)` line, and how far each language extracts is the generated
excellence board in `docs/languages.md`.

### 5.1 The flywheel: `observe` / `instrument --adoption` / `fathom-feedback`

The same local-only friction-capture loop the Claude Code guide documents
([docs/adoption/claude-code-internals.md §12](claude-code-internals.md#12-the-flywheel-observe--instrument---adoption--fathom-feedback))
works identically here — neither verb is a hook, so there is no
Claude-Code-specific machinery to be missing in this one:

```sh
bin/fathom observe --symptom "…" --tool <tool> --expected "…" --actual "…" --repro "…"
bin/fathom instrument --adoption
```

`fathom observe` appends a structured entry (symptom → evidence → root
cause → fix → status → impact) to `.fathom/observations.md`; `fathom
instrument --adoption` reports traffic/gate stats plus the open-observation
count, run from either harness's shell. The `fathom-feedback` Codex prompt
(§1.1 above, invoked as `/prompts:fathom-feedback`) carries the same
discipline as its Claude Code `SKILL.md` sibling — file friction the
moment you hit it, don't let it die unrecorded. Both verbs are plain
CLI/file operations: nothing hosted, nothing phones home, nothing
Codex-specific to configure.

---

## 6. Honest capability table

This table is **generated** from fathom's harness registry (ADR-029,
`internal/harness`) and pinned by `cmd/fathom/codexdoc_test.go`: it is the
output of `fathom harness capabilities --harness codex`, verbatim. The rows
read *verified* because a real session earned that word on 2026-09-12 (see
**Verified against** below), not because anyone typed it here — the status
column is computed from the descriptor's `verified_against` field, and it
said *unverified on this machine* on every row the day before.

Codex's hook contract is twelve Claude-shaped lifecycle events (the eleven
of 2026-09-02, plus `Interrupt`), carried as **data** in the descriptor
(`fathom harness list --json`) and driven by a codec since wave 2 of
`docs/plans/archive/harness-port.md`. Of the twelve, fathom wires seven; `fathom
doctor --harness=codex`'s `hooks_coverage` row names the five it leaves
alone and why. The vehicle column is what fathom delivers on Codex
**today**; an `absent` vehicle is a documented gap, never an emulation.

<!-- fathom:harness-capabilities codex begin -->
| Capability | Vehicle | Status |
|---|---|---|
| `prime` | `hook-inject` | verified — SessionStart entry in CODEX_HOME/hooks.json, gated to startup/clear; runs only after `/hooks` trust. MEASURED firing 2026-09-12 — `fathom prime` stays the fallback for an untrusted or unwired install |
| `obligations-post-edit` | `hook-inject` | verified — PostToolUse on apply_patch (first file of the patch); MEASURED firing 2026-09-12 on a real patch. The `obligations` MCP tool is the fallback until the hook is trusted |
| `guard-pre-edit` | `hook-inject` | verified — PreToolUse on apply_patch, inject-only (no permissionDecision); MEASURED firing 2026-09-12 |
| `guard-pre-shell` | `hook-inject` | verified — PreToolUse on Bash (shell and exec_command), inject-only |
| `discovery-observe` | `absent` | verified — Codex has no Read/Grep/Glob tools to observe; shell searches are unobserved |
| `stop-verify` | `hook-inject` | verified — advisory; `--block` renders {"decision":"block"} — the pre-push verify gate is the backstop |
| `deposit-session-end` | `hook-silent` | verified — SessionEnd reason is always "other", which the verb does not deposit on — `fathom thread note` before ending a session |
| `deposit-pre-compact` | `hook-silent` | verified — PreCompact entry writes one thread pointer; injects nothing |
| `mcp-tools` | `mcp` | verified |
| `skills` | `pull` | verified — `.agents/skills/<name>/SKILL.md` (`fathom init --skills=agents`); `~/.codex/prompts` is the legacy twin. Discovery on this build is still unverified — the verifying session did not invoke a skill |
| `stanza` | `stanza` | verified |
| `verify-gate` | `gate` | verified |
| `claim-gate` | `absent` | verified — the PostToolUse hook IS wired here, but it serves obligations only — the claim check rides Claude Code's edit payload and is not ported; the pre-push verify gate is the backstop (vision 26 §4) |
| `session-ledger` | `absent` | verified — unmeasured by design (law 12): the session ledger and the ON/OFF arm take no Codex row; the rollout's token_count events are for cost only, never a savings claim; the reader, CodexSessionCost (PE-14, outcome economics), is a library no verb or hook calls yet |
| `stanza-loaded` | `absent` | verified — no instructions-loaded event is documented; the stanza check reads the file, not the harness |
| `config-audit` | `absent` | verified — run `fathom config audit` by hand; no config-change event is documented |
| `subagent-prime` | `hook-inject` | verified — SubagentStart in CODEX_HOME/hooks.json injects the ≤600 B index-first primer; wired and documented, but no subagent ran in the verifying session — the one wave-3 row on this harness that is still unmeasured |
| `tool-failure-advice` | `absent` | verified — Codex documents no post-failure event — a failed fathom tool call gets no remedy line; run `fathom doctor` |
| `worktree-advice` | `absent` | verified — Codex documents no worktree event |
| `cwd-prime` | `absent` | verified — Codex documents no cwd-change or directory-added event |
| `setup-doctor` | `absent` | verified — Codex documents no setup event — run `fathom doctor --brief` by hand after wiring |
| `prompt-obligations` | `absent` | verified — Codex documents a UserPromptSubmit-shaped event but fathom has not ported prompt-hook to it yet — `fathom prime` covers identity/freshness, and `obligations` is a call away |
| `obligations-post-shell` | `absent` | verified — not ported to Codex yet — PostToolUse on apply_patch answers only the patch's own file |
| `test-run-line` | `absent` | verified — not ported to Codex yet — the test-run hook reads Claude Code's Bash envelope only |
| `readout` | `absent` | verified — not ported to Codex yet — the readout hook reads Claude Code's Bash envelope only |
<!-- fathom:harness-capabilities codex end -->

`subagent-prime`'s ≤600 B primer now carries an addition (RC-28, docs/vision/
28-receipts.md §4, not yet reflected in the generated table above —
`internal/harness/descriptors.go`'s `CapSubagentPrime` note is the row to
update next): when the parent session already established discoveries
(call-log pointers since its own recorded start), a "last session
established: …" brief, resolved fresh, is appended within 1,200 bytes
TOTAL — the 600 B cap stays the ceiling when there is nothing to add. Codex
has no mid-session pull-form event of its own for this; `fathom discoveries`
is that pull form here too, same as `fathom prime` is for `SessionStart`.

**Verified against — a session, 2026-09-12.** `codex-cli 0.153.1` on this
workstation, one headless `codex exec --json` run. What it proved, and what
it disproved:

- **Hooks fire.** SessionStart, PreToolUse and PostToolUse (on a real
  `apply_patch`), Stop and SessionEnd all ran fathom's wired commands. Their
  beats carry `harness=codex` in `~/.fathom/hooks.jsonl`, which is what
  `fathom doctor --harness=codex`'s `hooks_firing` row reads.
- **The envelope is what the documentation said.** All five stdin captures
  are committed under `internal/harness/testdata/codecs/codex/*.live.in.json`
  with their version and date in the file, and
  `TestCodexHooksFixtureFromLiveSession` replays every one through
  `CodexCodec`. The codec needed no change — including the load-bearing
  assumption that `apply_patch` reports its whole patch in
  `tool_input.command` and never a `file_path`.
- **The MCP surface answers.** The session called `list_services` over
  `/rpc/agent` and got all 20 services back, so streamable-HTTP negotiation
  with Codex is now a measurement rather than a hope.
- **The project hook layer does NOT exist on this build.** `touch` hooks
  placed in `<repo>/.codex/hooks.json` on `SessionStart`, `Stop`,
  `SessionEnd` and `UserPromptSubmit` fired **zero** times — in a trusted
  project, with `[features] hooks = true`, and with
  `--dangerously-bypass-hook-trust` — while the identical entries under
  `$CODEX_HOME/hooks.json` fired every time. So Codex hooks are user-global
  on 0.153.1: there is no per-repo hook wiring to have, every fathom entry
  fires in every repo, and the hook verbs resolve which repo they are in
  from the envelope's `cwd`.
- **SessionEnd's timeout is clamped to 3s** with a notice, and its `reason`
  is always `"other"` — which the deposit verb does not deposit on, so
  `fathom thread note` remains the way to leave a handoff on Codex.

That settles the contradiction this repo carried until 2026-09-02, when an
earlier edition of this section said Codex had "no hook mechanism" while
`docs/vision/16-skills-pack.md` §3 said its hooks were "real but narrow —
PreToolUse/PostToolUse intercept only Bash calls, ~10 events". Neither was
right: Codex hooks are real, are **twelve** Claude-shaped events
(`SessionStart`, `SessionEnd`, `SubagentStart`, `SubagentStop`,
`UserPromptSubmit`, `PreToolUse`, `PermissionRequest`, `PostToolUse`,
`PreCompact`, `PostCompact`, `Stop`, `Interrupt`), and their tool hooks
intercept shell (`Bash`), `apply_patch`, MCP tools and local function tools.

**Wiring them, then.** `fathom init --harness=codex` writes `AGENTS.md` and
`.fathom/` and deliberately writes no hook file: the only file that works is
user-global, and fathom does not edit user-global configuration unasked
(`docs/intent/autonomy.md`). Print the entries and paste them yourself:

```sh
fathom harness hooks --harness codex     # 8 entries, ready to merge
$EDITOR ~/.codex/hooks.json              # or $CODEX_HOME/hooks.json
```

Then trust them in `/hooks` (Codex records trust per hook hash, so a changed
command re-prompts) and confirm with `fathom doctor --harness=codex`:
`hooks_installed` reads the file the harness actually loads, `hooks_firing`
reads the beat log.

What the rows mean beside Claude Code's (`fathom harness capabilities
--harness claude-code` renders that side): the eight-tool `/rpc/agent`
surface is reachable at the same URL, and negotiation itself is no longer
the open question it was: `initialize` echoes the client's own protocol
version from a supported table (`2025-11-25` … `2024-11-05`), the
`MCP-Protocol-Version` header is tolerated, and
`TestServeHTTPNegotiatesWithKitClient` proves `/rpc/agent` end to end
against a Streamable-HTTP client fathom did not write (§2). *This Codex build* is now
verified (above); stdio `serve-mcp` always works; the git-native gates (pre-push `fathom verify`, CI `--strict`) are
identical on both; the `codex-cli` compile lane carries a real ambient-token
cost (§3); `fathom init --harness=codex` writes `AGENTS.md` and `.fathom/`
only, prints the `codex mcp add` line rather than running it, and prints the
hook entries rather than writing a user-global file; the ten
shipped skills reach Codex through `--skills=codex` (= `--skills=agents`:
`.agents/skills`, the documented path, unverified here) or the deprecated
`--skills=codex-prompts` twin (§1.1); continuity threads are half a loop here (the store and verbs are
identical, nothing deposits automatically); the config custodian's audit
verb and doctor row work identically, with the attention line reaching Codex
only through `fathom prime`; and `fathom doctor --harness=codex` runs every
check, with `hooks_installed`, `hooks_firing` and `mcp_registration` now
reporting real verdicts rather than `skipped: unverified` — that demotion
existed for a descriptor no session had run, and keeping it after one had
would have made the verified state unreachable by construction.

**The practical upshot:** in Codex, fathom is a fully capable *pull* tool
(every MCP tool works identically), two harness-agnostic *gate* channels
(pre-push, CI), and — once you have pasted and trusted the entries — a
working *inject* tier: the thing that made fathom's measured Claude Code
impact come from configuration rather than agent habit
(`docs/design/coding-agent-excellence.md` §5). Three real differences
remain, none of them a gap fathom can close: the hooks are user-global
rather than per-repo, six of Claude Code's events have no Codex equivalent
at all (`PostToolUseFailure`, `InstructionsLoaded`, `ConfigChange`,
`FileChanged`, `WorktreeCreate`, `CwdChanged`, `Setup` — the capability
table says what each costs you), and the measurement boundary below.

**The measurement boundary (honesty, not a gap to close).** Codex sessions
are outside the impact observatory's instrumentation: the session ledger and
the arm (ON/OFF) data are Claude-transcript/hook-fed, and Codex has neither.
Adoption on Codex is **unmeasured by design**, not merely unmeasured yet —
and law 12 (`docs/design/production-tokenomics.md` §2: no savings claim
without a counterfactual arm) forbids extrapolating Claude-arm numbers to
Codex. Whatever the Claude-side A/B eventually shows, it says nothing about
Codex sessions; a separate instrument would be required to say anything
measured about this harness at all. One such reader exists as a library,
for cost only: `harness.CodexSessionCost` (PE-14) reads a session's token
totals and tool-call count from its rollout file under `~/.codex/sessions`,
read-only and incrementally, so outcome economics can price a Codex session
per proven row once a verb calls it (none does yet; PE-14 is open). It says
what a session cost, never what fathom saved — there is still no ON/OFF arm
on Codex.

---

## 7. You know it works when…

**Troubleshooting starts with `fathom doctor --harness=codex`** — the same
deterministic wiring checklist the Claude Code guide leads its
troubleshooting section with (`docs/adoption/claude-code.md` Troubleshooting), reading
`skipped` (never a guessed pass) for the two Claude-only hook checks per
the capability table above:

```sh
bin/fathom doctor --harness=codex [--json]
```

```sh
# Daemon up (same as Claude Code path)
curl -s http://127.0.0.1:7575/healthz   # → ok
```

- [ ] `codex mcp list` (or reading `~/.codex/config.toml`) shows a `fathom`
      entry — **verify against your Codex version's exact output.**
- [ ] Ask Codex to call `find_symbol` for a symbol you know exists in the
      repo and confirm it resolves.
- [ ] `AGENTS.md` in the repo root contains the
      `<!-- fathom:begin v3 sha256:… -->…<!-- fathom:end -->` stanza (from
      `fathom init` — the stanza's one file, ADR-059; a region written
      before v3 reads `<!-- fathom:begin -->` until the next write).
- [ ] `fathom prime` prints the primer (service identity, a live example,
      the thread brief) from the repo root.
- [ ] If using the `codex-cli` compile lane: `fathom providers --profile
      codex` reports the lane healthy.
- [ ] `git push` in the repo runs lefthook's `pre-push` hooks and the
      `fathom-verify` command exits with the expected code (0 pass, 1 a
      real unproven claim, 2 only when the daemon/store is genuinely down).
- [ ] `bin/fathom init --skills=codex` (or a bare `--skills`) populates
      `.claude/skills/fathom-{plan,verify,onboard,feedback,doctor,impact,rewire,activate,custodian}/SKILL.md`
      and links `.agents/skills` to it
      (§1.1; `--skills=codex-prompts` for the deprecated `~/.codex/prompts` twin),
      and `bin/fathom observe --symptom "…"` / `bin/fathom instrument
      --adoption` (§5.1) both run cleanly from a Codex-driven shell — neither
      is hook-dependent, so there's nothing Codex-specific to verify beyond
      the CLI itself working.
- [ ] `bin/fathom doctor --harness=codex` reports every applicable row
      `pass` (the two Claude-only hook rows and `mcp_registration` read
      `skipped` — the latter `skipped: unverified` until a real Codex
      session negotiates the surface here; `gate_posture` states Codex's
      documented fail mode) — or, faster than this checklist, just run it
      first.
- [ ] `bin/fathom harness capabilities --harness codex` prints §6's table;
      `bin/fathom init --harness=codex` writes `AGENTS.md` + `.fathom/` only
      and prints both the `codex mcp add` line and the reason it wrote no
      hook file.
- [ ] `bin/fathom harness hooks --harness codex` prints 8 entries; after you
      merge them into `$CODEX_HOME/hooks.json` and trust them in `/hooks`,
      `bin/fathom doctor --harness=codex` shows `hooks_installed` PASS, and
      `hooks_firing` PASS once one session has run.

---

## Maintained

This doc covers: `fathom init` (AGENTS.md stanza, `--harness=codex`,
`--skills=codex` into `.agents/skills/`, `--skills=codex-prompts` into
`~/.codex/prompts/`), `harness` (`list`/`capabilities` — §6 is
`capabilities`' output — and the adoption matrix subcommand), `plugin render` (the Agent Plugins 1.0 manifests
Codex's plugin loader reads beside Claude Code's), `prime` (the Codex pull tier),
`plans next --eligible-only`/`context`, `thread brief`/`note`/`handoff`,
`config audit`, `observe`, `instrument --adoption`, `doctor`
(`--harness=codex`), `serve-mcp`, `serve-http` (`/rpc`, `/rpc/agent`, the
eight-tool default surface), the `codex-cli` provider lane
(`cmd/fathom/llmprofile.go`'s `buildCodexCLIClientProfile` Ping tripwire),
`fathom verify` / `--strict` / `--quiet` (git-native gate, identical to
Claude Code), the ten shipped Codex prompts
(`dist/agents/skills/fathom-{plan,verify,onboard,feedback,doctor,impact,rewire,activate,custodian,findings}`).
Codex-specific `config.toml`/`codex mcp` syntax is
sourced from Codex's own public docs, not this repo — re-verify those
specifics against your installed Codex version when they change. Update
this doc when the codex-cli lane's cost story, the Ping tripwire, the
skills/prompts install story, or the verify gate change.
