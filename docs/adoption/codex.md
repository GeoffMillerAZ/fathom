# Adopting fathom with Codex

Fathom's index, MCP surface, and git-native gates are Claude-Code-agnostic —
Codex is a first-class MCP client and gets full read access to the same
index. What's genuinely different is the **enforcement stack**: Claude
Code's hook-based context injection (SessionStart, PostToolUse, Stop) has no
Codex equivalent today, so Codex adoption leans more heavily on the
git-native gates and the standing `AGENTS.md` doctrine. This guide is honest
about that gap rather than pretending symmetry — see §6.

Build/index/daemon setup is identical to the Claude Code path — this guide
does not repeat it. If you haven't built `bin/fathom` or stood up a
`serve-http` daemon yet, do that first via
[docs/adoption/claude-code.md §1–§5](claude-code.md) (or run `fathom init`,
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
- stamped the fathom operating-contract stanza into **both** `CLAUDE.md` and
  `AGENTS.md` from one canonical source (`cmd/fathom/init.go`'s
  `fathomStanza`) — **Codex reads `AGENTS.md` automatically**, so there is
  nothing further to do for the standing-doctrine piece (markers, re-run
  semantics, and the corruption-refusal case are documented once, in
  [docs/adoption/claude-code.md §3.1](claude-code.md#31-the-claudemd--agentsmd-stanza--markers-re-run-semantics-corruption-refusal),
  since the mechanism and the stanza text are identical for both files), and
- if you passed `--skills=codex` (or `=all`), rendered the nine shipped
  `fathom-{plan,verify,onboard,feedback,doctor,impact,rewire,activate,custodian}`
  prompts into `~/.codex/prompts/` — §1.1 below.

The stanza and hook writes go through one wiring engine (law 15): a
receipt lands in `.fathom/receipts.jsonl`, `fathom fleet undo --last`
reverses it, and a marker region a human edited is BLOCKED and named
rather than overwritten — the details are in the Claude Code guide's §3.1
and apply to `AGENTS.md` identically.

`init` does *not* register Codex's MCP client (that's a Codex-side,
interactive step, same reason it doesn't call `claude mcp add` for you
either) and does not touch `lefthook.yml` (hand-curated, printed as a manual
step instead — §4).

### 1.1 Installing Codex prompts: `--skills=codex`

```sh
bin/fathom init --skills=codex
```

Renders the same nine shipped skill sources
(`dist/agents/skills/fathom-{plan,verify,onboard,feedback,doctor,impact,rewire,activate,custodian}/`)
that the Claude Code guide installs as `SKILL.md` files
([docs/adoption/claude-code.md §11](claude-code.md#11-agent-skills--the-nine-shipped-with-the-binary))
into Codex's custom-prompts convention instead: each skill's
`codex-prompt.md` variant is written to `~/.codex/prompts/<name>.md` —
**user-global, not project-scoped**, since that's the only location Codex's
custom-prompts convention documents (no per-repo variant). Invoke one with
`/prompts:<name>`, e.g. `/prompts:fathom-feedback`. `--skills=all` installs
both the Claude Code and Codex shapes in one call, useful if the same repo
is worked from both clients.

**Honest capability note.** These are plain Markdown prompt files, not
Codex's newer native `SKILL.md`-based skills mechanism (`.agents/skills/`
in some CLI versions) — if your Codex CLI has adopted that mechanism, each
installed `codex-prompt.md`'s own header tells you to copy the sibling
`SKILL.md` there instead of using the rendered prompt file; this repo has
no Codex install to verify which mechanism your version supports (same
version caveat as above). Re-running `--skills=codex` is idempotent the
same way the Claude Code path is: every written file carries a
`<!-- fathom:owned -->` marker, so a pre-existing, non-fathom-owned file
with the same name is never clobbered — `init` refuses instead (exit `1`,
naming the path).

---

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

`/rpc/agent` is the same curated 17-tool surface Claude Code registers
against (§10 of the Claude Code guide — incl. `read_source` and the
*experimental* `memory_search`); the full registry (55 tools on this
build) is at `/rpc` if you want everything. Off-surface calls still
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
[docs/adoption/claude-code.md §7.4](claude-code.md#74-pre-push--the-fathom-verify-gate-lefthook).

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

`fathom init` writes the identical stanza fathom puts in `CLAUDE.md` into
`AGENTS.md` — index-first tool discipline, the refresh contract, the
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

### 5.1 The flywheel: `observe` / `instrument --adoption` / `fathom-feedback`

The same local-only friction-capture loop the Claude Code guide documents
([docs/adoption/claude-code.md §12](claude-code.md#12-the-flywheel-observe--instrument---adoption--fathom-feedback))
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
output of `fathom harness capabilities --harness codex`, verbatim. Every
row says *unverified on this machine* because no Codex session has run
against this build here — Codex's own hook contract (11 Claude-shaped
lifecycle events, `.codex/hooks.json`) is carried as **data** in the
descriptor (`fathom harness list --json`) and no codec drives it yet
(*hooks: not wired*, wave 2 of `docs/plans/harness-port.md`). The vehicle
column is what fathom delivers on Codex **today**; an `absent` vehicle is a
documented gap, never an emulation.

<!-- fathom:harness-capabilities codex begin -->
| Capability | Vehicle | Status |
|---|---|---|
| `prime` | `pull` | unverified on this machine — run `fathom prime` at session start (AGENTS.md says so); the SessionStart binding is documented above but not wired |
| `obligations-post-edit` | `mcp` | unverified on this machine — call the `obligations` MCP tool before editing; no PostToolUse hook is wired by this build |
| `guard-pre-edit` | `absent` | unverified on this machine — documented PreToolUse contract, not wired; a bound module's rules still reach Codex via `obligations`/`pack_context` |
| `guard-pre-shell` | `absent` | unverified on this machine — documented PreToolUse contract, not wired |
| `discovery-observe` | `absent` | unverified on this machine — Codex has no Read/Grep/Glob tools to observe; shell searches are unobserved |
| `stop-verify` | `absent` | unverified on this machine — the pre-push verify gate is the backstop |
| `deposit-session-end` | `pull` | unverified on this machine — `fathom thread note` before ending a session |
| `deposit-pre-compact` | `absent` | unverified on this machine — documented PreCompact contract, not wired |
| `mcp-tools` | `mcp` | unverified on this machine |
| `skills` | `pull` | unverified on this machine — `.agents/skills/<name>/SKILL.md` (`fathom init --skills=agents`); `~/.codex/prompts` is the legacy twin |
| `stanza` | `stanza` | unverified on this machine |
| `verify-gate` | `gate` | unverified on this machine |
| `claim-gate` | `absent` | unverified on this machine — not built (vision 26 §4) |
| `session-ledger` | `absent` | unverified on this machine — unmeasured by design (law 12): the rollout transcript format is not stable and no instrument reads it |
<!-- fathom:harness-capabilities codex end -->

What the rows mean beside Claude Code's (`fathom harness capabilities
--harness claude-code` renders that side): the 17-tool `/rpc/agent`
surface is reachable at the same URL, and negotiation itself is no longer
the open question it was: `initialize` echoes the client's own protocol
version from a supported table (`2025-11-25` … `2024-11-05`), the
`MCP-Protocol-Version` header is tolerated, and
`TestServeHTTPNegotiatesWithKitClient` proves `/rpc/agent` end to end
against a Streamable-HTTP client fathom did not write (§2). What stays
`unverified on this machine` is *this Codex build* — confirm it the usual
way; stdio `serve-mcp` always works; the git-native gates (pre-push `fathom verify`, CI `--strict`) are
identical on both; the `codex-cli` compile lane carries a real ambient-token
cost (§3); `fathom init --harness=codex` writes `AGENTS.md` and `.fathom/`
only and prints the `codex mcp add` line rather than running it; the nine
shipped skills reach Codex through `--skills=agents` (`.agents/skills`, the
documented path, unverified here) or the legacy `--skills=codex` prompts
(§1.1); continuity threads are half a loop here (the store and verbs are
identical, nothing deposits automatically); the config custodian's audit
verb and doctor row work identically, with the attention line reaching Codex
only through `fathom prime`; and `fathom doctor --harness=codex` runs every
check, reporting `hooks_installed`/`hooks_firing` as `skipped` (no wired hook
to check) and `mcp_registration` as `skipped: unverified` even when a
`[mcp_servers.fathom]` entry is found — a found entry is evidence of a
registration, not of a session that negotiated the surface.

**The practical upshot:** in Codex, fathom is a fully capable *pull* tool
(every MCP tool works identically) plus two *gate* channels that are
completely harness-agnostic (pre-push, CI). What's missing is the *inject*
tier — the thing that made fathom's measured Claude Code impact come from
configuration rather than agent habit (`docs/design/coding-agent-excellence.md`
§5). Until Codex grows an equivalent hook surface, lean harder on `AGENTS.md`
being genuinely followed, and treat the pre-push/CI gates as the honest
backstop rather than a nice-to-have.

**The measurement boundary (honesty, not a gap to close).** Codex sessions
are outside the impact observatory's instrumentation: the session ledger and
the arm (ON/OFF) data are Claude-transcript/hook-fed, and Codex has neither.
Adoption on Codex is **unmeasured by design**, not merely unmeasured yet —
and law 12 (`docs/design/production-tokenomics.md` §2: no savings claim
without a counterfactual arm) forbids extrapolating Claude-arm numbers to
Codex. Whatever the Claude-side A/B eventually shows, it says nothing about
Codex sessions; a separate instrument would be required to say anything
measured about this harness at all.

---

## 7. You know it works when…

**Troubleshooting starts with `fathom doctor --harness=codex`** — the same
deterministic wiring checklist the Claude Code guide leads its
troubleshooting section with (`docs/adoption/claude-code.md` §13), reading
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
- [ ] Ask Codex to call `list_services` or `capabilities` and confirm a
      real response comes back (not a connection error).
- [ ] Ask Codex to call `find_symbol` for a symbol you know exists in the
      repo and confirm it resolves.
- [ ] `AGENTS.md` in the repo root contains the
      `<!-- fathom:begin v3 sha256:… -->…<!-- fathom:end -->` stanza (from
      `fathom init` — identical to the one in `CLAUDE.md`; a region written
      before v3 reads `<!-- fathom:begin -->` until the next write).
- [ ] `fathom prime` prints the primer (service identity, a live example,
      the thread brief) from the repo root.
- [ ] If using the `codex-cli` compile lane: `fathom providers --profile
      codex` reports the lane healthy.
- [ ] `git push` in the repo runs lefthook's `pre-push` hooks and the
      `fathom-verify` command exits with the expected code (0 pass, 1 a
      real unproven claim, 2 only when the daemon/store is genuinely down).
- [ ] `bin/fathom init --skills=codex` (or `=all`) populates
      `~/.codex/prompts/fathom-{plan,verify,onboard,feedback,doctor,impact,rewire,activate,custodian}.md`
      (§1.1), and `bin/fathom observe --symptom "…"` / `bin/fathom instrument
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
      `bin/fathom init --harness=codex` writes `AGENTS.md` + `.fathom/`
      only and prints the `codex mcp add` line (hooks: not wired, wave 2).

---

## Maintained

This doc covers: `fathom init` (AGENTS.md stanza, `--harness=codex`,
`--skills=codex` prompt install into `~/.codex/prompts/`), `harness`
(`list`/`capabilities` — §6 is its output), `prime` (the Codex pull tier),
`plans next --eligible-only`/`context`, `thread brief`/`note`/`handoff`,
`config audit`, `observe`, `instrument --adoption`, `doctor`
(`--harness=codex`), `serve-mcp`, `serve-http` (`/rpc`, `/rpc/agent`, the
17-tool allowlist), the `codex-cli` provider lane
(`cmd/fathom/llmprofile.go`'s `buildCodexCLIClientProfile` Ping tripwire),
`fathom verify` / `--strict` / `--quiet` (git-native gate, identical to
Claude Code), the nine shipped Codex prompts
(`dist/agents/skills/fathom-{plan,verify,onboard,feedback,doctor,impact,rewire,activate,custodian}`).
Codex-specific `config.toml`/`codex mcp` syntax is
sourced from Codex's own public docs, not this repo — re-verify those
specifics against your installed Codex version when they change. Update
this doc when the codex-cli lane's cost story, the Ping tripwire, the
skills/prompts install story, or the verify gate change.
