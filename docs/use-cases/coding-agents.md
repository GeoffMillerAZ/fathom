# Fathom for coding agents — the canonical wiring guide

How to wire Claude Code (and Codex, and any MCP-speaking agent) into
fathom: equal answers, 2–3× faster, cheaper on most tasks, plus the
governed capabilities (verify, obligations, review) no grep loop has —
see §5 for exactly what is and isn't measured. This is the one document to follow;
older recipes (getting-started's manual steps, the June integration notes)
defer to it.

**The loop, in one breath:** fathom *indexes* your repos deterministically,
*serves* code intelligence over MCP, *injects* governing rules at the moment
of edit, *gates* claimed work against the index at session end / push / CI,
and *measures* its own channels — so the agent spends tokens reasoning, not
rediscovering, and "done" means proven, not asserted.

```
index (ingest, hash-gated) ──► serve (MCP: /rpc/agent, 19 tools)
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
the daemon, wires the hooks into the project's `.claude/settings.json`
(SessionStart → `session-hook`, PostToolUse Edit|Write → `edit-hook`,
PreToolUse Edit|Write|Bash → `edit-hook --pre` — silent unless
`.fathom/guard.cue` names the file/command, see §2 — Stop → `stop-hook`),
scaffolds `.fathom/guard.cue` and `.fathom/modules.cue` **commented out**
(nothing guarded or bound by default), and stamps the same fathom stanza
into `CLAUDE.md` **and** `AGENTS.md` (Codex reads the latter). It then
prints the two steps it will not do for you:

1. **Register the MCP surface** (once per machine, user scope):
   ```sh
   claude mcp add --transport http --scope user fathom http://127.0.0.1:7575/rpc/agent
   ```
   `/rpc/agent` is the eight-tool default surface (`/rpc/extended` the 21-tool
   opt-in superset) — the full registry
   (`docs/mcp-tools.md`) stays one endpoint away at `/rpc`, and off-surface
   calls still execute (they're just logged `off_surface` so demand is
   measurable).
2. **Arm the push gate** (per repo, if you use lefthook — printed snippet):
   ```yaml
   pre-push:
     commands:
       fathom-verify:
         run: |
           for f in docs/plans/*.md; do case "$f" in *README*) continue;; esac
             fathom verify --service <svc> --plan "$f" --quiet || exit 1
           done
   ```
   In CI add `--strict` (claimed-but-Unknown blocks too). Exit-code contract:
   0 pass · 1 a claimed `[x]` failed verification (block) · 2 operational
   (daemon down, unindexed — **fail-open at pre-push, fail-closed in CI**).

One always-on daemon serves every repo and session (launchd example in
`docs/usage.md`); ingest is hash-gated (~0.2s when clean) and self-refreshes
on git-HEAD drift. `fathom init --skills` (bare = `.agents/skills` +
`.claude/skills`; `=claude`, `=codex`/`=agents` for one path, `=codex-prompts`
for the deprecated `~/.codex/prompts` twin) also installs the nine shipped
skills (activate, custodian, doctor, feedback, impact, onboard, plan,
rewire, verify); `fathom plugin render` packages the same nine with the
hook template and the MCP registration as one plugin tree carrying both the
Claude Code and the Agent Plugins 1.0 manifests. Every file `init` or
`fathom fleet reconcile --apply` writes goes through one plan → apply →
receipt (`.fathom/receipts.jsonl`; `fathom fleet undo` reverses it — law
15); `fathom doctor` then proves the wiring, and `fathom prime` is the
pull-tier brief for a harness with no SessionStart hook (Codex).

## 2. The channels, ranked by force

| Channel | Mechanism | Force | Harness |
|---|---|---|---|
| CI `verify --strict` | claimed `[x]` must verify against the index (claimed-but-Unknown fails too); **experimental** — no CI runner yet, `scripts/ci.sh` is run by hand | blocking | **any** (git-native) |
| pre-push `verify --quiet` | same, failing rows + summary only, fail-open on operational errors | blocking | **any** |
| `verify --require-all` | wave gate: everything claimed-and-verified or explicitly skipped | blocking, on demand | **any** |
| Stop hook | tasks claimed `[x]` *this session* that fail verification surface at session end (`--block` to refuse ending) | advisory → blocking | Claude Code |
| edit-hook | obligations governing the edited file injected ≤150 tokens, zero-output-on-clean | injected context | Claude Code |
| edit-hook --pre (guarded edits, phase O) | for scopes/commands named in `.fathom/guard.cue`, the SAME injection served BEFORE the edit/command — silent for everything else, zero daemon calls on a miss | injected context | Claude Code |
| session-hook | index status + question-shape routing primer (SB-7: where/who-calls/what-breaks/what-governs/what-was-known via fathom, a literal string/config value/log line via grep) | injected context | Claude Code |
| MCP tools | the eight-tool agent surface | pull (agent choice) | any MCP client |
| CLAUDE.md / AGENTS.md stanza | standing doctrine | advisory | Claude Code / Codex |

The ordering is the strategy: **configuration beats habit.** Fathom's own
telemetry showed agents choose grep over tools (0–7% pulled share), so the
durable channels are the ones that inject and block — the pull surface
exists for when the agent does reach for it. For Codex today: the git-native
gates and MCP work identically; the hook channels are Claude-Code-specific
(AGENTS.md carries the doctrine instead).

## 3. What goes in CLAUDE.md — and what deliberately doesn't

`fathom init` maintains a marker-delimited stanza (safe to regenerate):
index-first tool discipline, the refresh contract, the verify command. After
the first write the begin line carries a digest — `<!-- fathom:begin v3
sha256:… -->` — so a region a human hand-edited is blocked and named rather
than overwritten (a digest-less `v1` region is adopted and upgraded, not
drift). Keep it lean. **Do not** hand-copy coding rules into CLAUDE.md that the
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
reaches it, and contradictions are absent — with citations. Function
values are edges (`mux.HandleFunc`, route tables, `http.HandlerFunc`,
middleware, method values) and a call inside a closure counts for the
enclosing test; same-named methods need `package:`; a discovery gap is
named on stderr. Every escape
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

**2026-07-24 update — fathom's own home turf, measured** (docs/plans/
home-bench.md H3-1): the follow-up bench flipped the corpus to
index-native tasks (obligations, verify, blast radius, claim provenance,
concern search, staleness, contradictions) — fathom's home turf, not
grep's. It did **not** win decisively: success 12/13 vs 12/13, equal, and
tokens 1.071× (a premium, sensitive to one documented protocol call). The
honest surviving pitch, unchanged in kind from above: equal answers,
cheaper on 8/13 tasks (an outlier-driven aggregate premium on the rest),
2–3× faster wall time — and the governance capabilities themselves
(verify gate, obligations, staleness honesty) remain unmeasured by either
bench.

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
  `runtime_pass` reads an attested run (`fathom tests ingest`), informational, Unknown when none exists.
- An empty index is "cannot verify" (exit 2) — never "all claims false."
- Obligations are served **flagged** when their citations drift, never
  silently dropped; `⚠ index behind working tree` means re-read the file.
- The index reflects the last ingest: after heavy uncommitted work, trust
  `staleness_report` or read the file you're changing.
- A committed Go change reaches the daemon only after its binary restarts
  (`fathom version` vs `git rev-parse HEAD` when a tool misbehaves).

## Maintained

CLI: `fathom init` (`--skills`, `--harness`, `--json`), `harness` (`list`/`capabilities`/`ladder`/`matrix`
— the registry every hook verb's `--harness` names, ADR-029; `matrix` renders the
adoption matrix `docs/adoption/other-harnesses.md` pins), `plugin` (`render` — one tree,
both manifests), `ingest` (`--json`), `compile`, `layers`
(`list` (`--json`)/`show`/`install` — see `docs/layers.md`), `verify` (`--quiet`, `--strict`, `--require-all`),
`instrument`, `edit-hook` (`--pre`; the claim-moment verify on a plan `[x]`),
`stop-hook`, `doctor` (its stanza-loaded row reads the InstructionsLoaded beats), `gen-plan-cue`,
`config` (incl. `config audit` and `config audit --hook`), `capabilities`, `thread` (`brief`/`note`/
`point`/`list`/`close`/`promote`/`handoff`; `brief --json`, `list --json`), `rate`, `providers`, `smoke`,
`session-hook`, `session-end-hook`, `pre-compact-hook`, `prime`, `fleet` (`report`/
`reconcile`/`receipts`/`undo`) — every one listed by `fathom help` (the table
renders from the dispatch table) and answering `fathom <verb> -h`.
MCP: the `/rpc/agent` surface (the eight-tool default floor, surface reckoning
SR-33 — see `docs/mcp-tools.md`).
Phase O (`.fathom/guard.cue`, `.fathom/modules.cue`) is covered in
[platform-modules.md](platform-modules.md). When any of these change,
update this doc's examples and this line.
