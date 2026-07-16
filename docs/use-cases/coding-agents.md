# Fathom for coding agents — the canonical wiring guide

How to make Claude Code (and Codex, and any MCP-speaking agent) measurably
more effective and cheaper with fathom. This is the one document to follow;
older recipes (getting-started's manual steps, the June integration notes)
defer to it.

**The loop, in one breath:** fathom *indexes* your repos deterministically,
*serves* code intelligence over MCP, *injects* governing rules at the moment
of edit, *gates* claimed work against the index at session end / push / CI,
and *measures* its own channels — so the agent spends tokens reasoning, not
rediscovering, and "done" means proven, not asserted.

```
index (ingest, hash-gated) ──► serve (MCP: /rpc/agent, 15 tools)
        ▲                              │
        │ auto-refresh on HEAD drift   ▼
measure (instrument, call log) ◄── inject (edit-hook: obligations)
        ▲                              │
        └── gate (stop-hook · pre-push verify · CI --strict) ◄┘
```

## 1. Wire a repo in ten minutes

```sh
fathom init            # from the repo root
```

Idempotent and additive. It creates `.fathomignore`, ingests the repo into
the daemon, wires the three hooks into the project's `.claude/settings.json`
(SessionStart → `session-hook`, PostToolUse Edit|Write → `edit-hook`,
Stop → `stop-hook`), and stamps the same fathom stanza into `CLAUDE.md`
**and** `AGENTS.md` (Codex reads the latter). It then prints the two steps
it will not do for you:

1. **Register the MCP surface** (once per machine, user scope):
   ```sh
   claude mcp add --transport http --scope user fathom http://127.0.0.1:7575/rpc/agent
   ```
   `/rpc/agent` is the curated 15-tool surface — the full ~80-tool registry
   stays one endpoint away at `/rpc`, and off-surface calls still execute
   (they're just logged `off_surface` so demand is measurable).
2. **Arm the push gate** (per repo, if you use lefthook — printed snippet):
   ```yaml
   pre-push:
     commands:
       fathom-verify:
         run: |
           for f in docs/plans/*.md; do case "$f" in *README*) continue;; esac
             fathom verify --service <svc> --plan "$f" || exit 1
           done
   ```
   In CI add `--strict` (claimed-but-Unknown blocks too). Exit-code contract:
   0 pass · 1 a claimed `[x]` failed verification (block) · 2 operational
   (daemon down, unindexed — **fail-open at pre-push, fail-closed in CI**).

One always-on daemon serves every repo and session (launchd example in
`docs/usage.md`); ingest is hash-gated (~0.2s when clean) and self-refreshes
on git-HEAD drift.

## 2. The channels, ranked by force

| Channel | Mechanism | Force | Harness |
|---|---|---|---|
| CI `verify --strict` | claimed `[x]` must verify against the index | blocking | **any** (git-native) |
| pre-push `verify` | same, fail-open on operational errors | blocking | **any** |
| `verify --require-all` | wave gate: everything claimed-and-verified or explicitly skipped | blocking, on demand | **any** |
| Stop hook | tasks claimed `[x]` *this session* that fail verification surface at session end (`--block` to refuse ending) | advisory → blocking | Claude Code |
| edit-hook | obligations governing the edited file injected ≤150 tokens, zero-output-on-clean | injected context | Claude Code |
| session-hook | index status + "resolve via fathom before grep" primer | injected context | Claude Code |
| MCP tools | the 15-tool agent surface | pull (agent choice) | any MCP client |
| CLAUDE.md / AGENTS.md stanza | standing doctrine | advisory | Claude Code / Codex |

The ordering is the strategy: **configuration beats habit.** Fathom's own
telemetry showed agents choose grep over tools (0–7% pulled share), so the
durable channels are the ones that inject and block — the pull surface
exists for when the agent does reach for it. For Codex today: the git-native
gates and MCP work identically; the hook channels are Claude-Code-specific
(AGENTS.md carries the doctrine instead).

## 3. What goes in CLAUDE.md — and what deliberately doesn't

`fathom init` maintains a marker-delimited stanza (safe to regenerate):
index-first tool discipline, the refresh contract, the verify command. Keep
it lean. **Do not** hand-copy coding rules into CLAUDE.md that the
obligations layer should carry instead: compiled obligations are *scoped*
(only injected when a governed file is edited), *cited* (file#line), and
*swept* (they flag themselves stale when their source drifts) — a CLAUDE.md
paragraph is none of those and costs context on every session. Rule of
thumb: CLAUDE.md says *how to work here*; obligations say *what binds this
file*.

Compile the obligations layer per repo (binding docs only — CLAUDE.md,
ADR.md, plan rules; never design essays):

```sh
fathom compile --service <svc>                 # dry-run: sections + cost estimate
fathom compile --service <svc> --dry-run=false # cents-scale; digest-gated thereafter
```

Recompiles cost ~$0 for unchanged sections; run-level budgets from the
layer manifest are enforced (refusal and mid-run stop are ledgered and
resumable).

## 4. Done-means-proven, minimally

Write plans as Markdown checklists in `docs/plans/` (grammar reference:
`docs/plans/README.md`). The contract is one annotation per line:

```markdown
- [ ] Parser handles v2 keys {id: G2-1, symbol: ParseMarkdownPlan, package: internal/verify}
- [-] cmd oracle {id: D-1, disposition: deferred, reason: "no consumer yet", trigger: "skip share >30% after 4 weeks"}
```

`[x]` is a claim; the gate checks the symbol exists, a test statically
reaches it, and contradictions are absent — with citations. Every escape
(`skip_test`) carries its justification (`skip:`), every deferral its
reason and trigger. The Stop hook closes the loop at session granularity:
an agent cannot quietly end a session on a disproven claim.

## 5. The cost story, honestly

**What it saves.** Discovery is where agent tokens die: one `symbol_context`
call (~300–800 tokens) replaces a grep → 4–6 file reads sweep (5–20k
tokens). The June 2026 integration assessment measured discovery agents
eliminated outright, ~148→32 tool calls per comparable worker, and wave
wall-clock roughly halved — with the honest caveat that per-wave worker
token totals did not collapse, and the ×10 discovery claim awaits the
context-bench before it's stated as fact.

**What it costs.** Ingest: ~0.2s hash-gated refresh, cents of CPU. Serving:
p95 ≤ ~90ms per hook call. Obligations compile: cents per repo *change*
(digest-gated). Hooks: ≤150 injected tokens on governed edits, zero on
clean ones.

**Subscription-backed compiles.** `fathom compile --dry-run=false` needs an
LLM. By default that's the Anthropic API (`ANTHROPIC_API_KEY`, metered).
Set `FATHOM_LLM_PROVIDER=claude-cli` (with a locally logged-in `claude` CLI)
to run compiles against a Claude Pro/Max subscription instead — no API key
needed. The `cost_usd` a `compile_layer` run reports is the same
API-equivalent accounting either way; under `claude-cli` the actual marginal
spend is $0 (it draws from the subscription's plan limits, not a metered
key). `FATHOM_CLAUDE_BIN` overrides the resolved `claude` binary path when
PATH doesn't include it (e.g. a launchd daemon).

**Measure yours, don't trust ours:**

```sh
fathom instrument     # injection coverage vs the ≥90% gate, per-day rollup,
                      # liveness alarms (a silent channel is an alarm, not a mystery)
```

The daemon call log (`workspace/logs/mcp-calls.jsonl`) tags every call with
`origin` (hook/CLI plumbing vs agent choice) and `off_surface`, so adoption
numbers and tool-kill decisions run on clean data.

## 6. Honesty boundaries (read once, they're load-bearing)

- `test_reaches` is **static** reachability — never "the test passes."
  `runtime_pass` stays Unknown until CI evidence is ingested as a tier.
- An empty index is "cannot verify" (exit 2) — never "all claims false."
- Obligations are served **flagged** when their citations drift, never
  silently dropped; `⚠ index behind working tree` means re-read the file.
- The index reflects the last ingest: after heavy uncommitted work, trust
  `staleness_report` or read the file you're changing.
- A committed Go change reaches the daemon only after its binary restarts
  (`fathom version` vs `git rev-parse HEAD` when a tool misbehaves).

## Maintained

CLI: `fathom init`, `ingest`, `compile`, `verify` (`--strict`,
`--require-all`), `instrument`, `session-hook`, `edit-hook`, `stop-hook`,
`gen-plan-cue`. MCP: the `/rpc/agent` surface (15 tools — see
`docs/mcp-tools.md`). When any of these change, update this doc's examples
and this line.
