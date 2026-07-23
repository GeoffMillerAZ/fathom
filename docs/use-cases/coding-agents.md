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

Obligations is one of several **module packs** (`docs/layers.md` is the
catalog reference: `adr-rules` and `spec-bindings` ship alongside it, plus
`findings` for review conclusions). `fathom layers list|show|install`
inspects and installs packs — embedded ones and a project's own
`.fathom/layers/` — without writing any Go:

```sh
fathom layers list                 # embedded + project packs, source-labeled
fathom layers show adr-rules       # full resolved definition + prompt provenance
fathom layers install ./my-pack    # validates against #Layer, copies into .fathom/layers/
```

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
token totals did not collapse. The first context-bench run (2026-07-23,
docs/plans/falsifiability.md B1-3) settled the ×10 discovery claim:
**retracted**. Measured, 15 real tasks × 2 subagent arms: equal success
(15/15 both arms), 1.38× aggregate / 1.79× mean per-task end-to-end token
reduction, a 2.81× mean (2.23× median) wall-time reduction (fathom slower
on 4/15 tasks), and **dollar parity** — claude-reported cost $2.43
(fathom) vs $2.46 (grep), because the token win concentrates in
0.1×-priced cache reads while fathom pays more 1h cache writes. The
honest pitch is "same answers, ~1.4–1.8× fewer tokens at roughly equal
dollars, ~2–3× faster on most tasks, plus the governed capabilities
(verify_plan, obligations, staleness honesty) that have no grep
equivalent and which this bench did not measure" — not ×10. (The bench's
corpus is deliberately grep-native — every ground truth is grep-derivable
— so it measured grep's home turf; grep answered even the cross-repo
tasks.)

**What it costs.** Ingest: ~0.2s hash-gated refresh, cents of CPU. Serving:
p95 ≤ ~90ms per hook call. Obligations compile: cents per repo *change*
(digest-gated). Hooks: ≤150 injected tokens on governed edits, zero on
clean ones.

**Provider matrix.** `fathom compile --dry-run=false` (and `serve-http`'s
`compile_layer`) need an LLM, selected via `FATHOM_LLM_PROVIDER` — one
construction site (`buildLLMClient`, `cmd/fathom/compile.go`), six lanes: two
metered API-key families, two subscription-CLI families riding a local
login, and two enterprise endpoints. The `cost_usd` a `compile_layer` run
reports is the same API-equivalent accounting across every lane; only the
subscription-CLI lanes (`claude-cli`, `codex-cli`) have $0 actual marginal
spend (they draw from a plan's limits, not a metered key) — see the cost
column below for the honest caveat on each.

| Lane | Auth mode | Env vars | Marginal-cost story |
|---|---|---|---|
| `anthropic` (default) | API key | `ANTHROPIC_API_KEY` | Metered, per-token Anthropic API pricing. |
| `claude-cli` | Subscription CLI (OAuth) | `FATHOM_CLAUDE_BIN` (optional binary override) | $0 marginal — draws from a Claude Pro/Max plan's limits. `--safe-mode` suppresses ambient context to near-zero, so `cost_usd`'s API-equivalent accounting stays close to the real token shape. |
| `openai` | API key | `OPENAI_API_KEY` (optional for local/compatible hosts), `FATHOM_OPENAI_BASE_URL` (optional, e.g. Ollama/vLLM) | Metered, per-token OpenAI (or compatible-host) pricing; $0 against a self-hosted local model. |
| `azure-openai` | API key (or Entra bearer passthrough) | `AZURE_OPENAI_API_KEY`, `AZURE_OPENAI_ENDPOINT`, `AZURE_OPENAI_DEPLOYMENT`, `AZURE_OPENAI_API_VERSION` (optional, defaults to the adapter's pinned GA version) | Metered against the Azure resource's negotiated/enterprise pricing, not the public OpenAI price list. |
| `bedrock` | AWS credential chain (no static keys required) | none fathom-specific — the kit chain resolves explicit env, `AWS_PROFILE`/shared files (incl. `aws sso login`), `credential_process`, IMDSv2; `FATHOM_COMPILE_MODEL` required (a Bedrock model or inference-profile ID) | Metered against AWS Bedrock's per-token pricing for the chosen model/inference-profile, billed through the AWS account, not a separate API key. |
| `codex-cli` | Subscription CLI (ChatGPT plan OAuth) | `FATHOM_CODEX_BIN` (optional binary override) | Real cost, not $0: the undocumented ambient-context suppression this adapter depends on still carries **~10k input tokens of overhead per call** (~300k for a 31-section compile) against ChatGPT plan limits, roughly **60% cache-discounted at steady state** — a materially heavier ambient footprint than `claude-cli`, which suppresses ambient context to near-zero via `--safe-mode`. Construction also Pings the CLI (zero-token, ~300ms, local-only) as a liveness tripwire for that dependence; a degraded CLI fails construction loudly instead of silently losing the system prompt. |

`FATHOM_LLM_PROVIDER` unset or an unrecognized value: unset defaults to
`anthropic`; an unrecognized value is a typed configuration error listing
all six lanes, never a silent fallback.

**Prefer a config file over raw env for anything beyond one lane.** The env
vars above still work exactly as documented (they are the top precedence
tier), but `~/.fathom/config.toml` / project `.fathom/config.toml` give you
named, switchable profiles — including AWS GovCloud and Azure Government —
plus `fathom config --explain` (which tier resolved every key, from where),
`fathom providers` (auth health, zero tokens), and `fathom smoke` (one real
completion's latency/cost, opt-in spend). `fathom init` scaffolds a starter
file. Full guide, precedence rules, and secrets doctrine:
**[docs/configuration.md](../configuration.md)**.

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

CLI: `fathom init`, `ingest`, `compile`, `layers` (`list`/`show`/`install`
— see `docs/layers.md`), `verify` (`--strict`, `--require-all`),
`instrument`, `session-hook`, `edit-hook`, `stop-hook`, `gen-plan-cue`,
`config`, `providers`, `smoke`. MCP: the `/rpc/agent` surface
(15 tools — see `docs/mcp-tools.md`). When any of these change, update this
doc's examples and this line.
