# Configuration — profiles, `~/.fathom/config.toml`, health, smoke

Canonical guide for fathom's LLM provider configuration: `config.toml` file
format, precedence, named profiles (including AWS GovCloud and Azure
Government), the secrets doctrine, and the three verbs that read it —
`fathom config`, `fathom providers`, `fathom smoke`. Design source:
`docs/plans/config-experience.md`. The other `.fathom/*` files — `ignore`,
`guard.cue`, `capabilities.cue` (+ its `budgets` block), `concerns.cue`,
`modules.cue` — are covered in the scoped-config sections at the end.

## Why a config file

Fathom's compile path (`fathom compile --dry-run=false`, `serve-http`'s
`compile_layer`) needs an LLM, and until this feature existed the only way to
configure one was ~14 environment variables and ~15 overlapping flags — no
file, no discoverability, no way to keep more than one provider set up at a
time. Worse: the always-on daemon (`serve-http`, run via launchd) inherits
**no environment at all** from its plist, so `compile_layer` was silently
disabled on the daemon with no error anywhere. A config file fixes both: it
is a real, inspectable place to name provider setups, and `serve-http` reads
it at startup — no plist surgery required.

For the six provider lanes themselves (auth mode, env vars, marginal-cost
story per lane) see the provider matrix in
[docs/use-cases/coding-agents.md §5](use-cases/coding-agents.md#5-the-cost-story-honestly)
— it is not duplicated here.

## File format and location

Humans write **TOML**. Two layers, both optional:

- **User**: `~/.fathom/config.toml` — your personal profiles, shared across
  every repo.
- **Project**: `<repo>/.fathom/config.toml` — profiles specific to one repo
  (a CI service account, a team-shared endpoint). Typically **not**
  committed unless every profile in it uses `api_key_env`/`api_key_cmd`
  indirection (no literal secret can ever end up in it — see below — but an
  endpoint or region might still be something you'd rather keep local).

`fathom init` scaffolds `~/.fathom/config.toml` with every profile shape
below, commented out, write-if-absent (never touches a file that already
exists).

Validation is **CUE** (`schema/config/schema.cue`), not TOML's own weak
typing: your file is parsed, converted to a CUE value, and unified against
the schema at every load. A typo'd key, a provider value outside the six
known lanes, or (see below) a literal secret fails immediately, naming the
file and the offending key — never a silent no-op.

## Precedence

```
flags > env > project .fathom/config.toml > user ~/.fathom/config.toml > built-in defaults
```

Every environment variable that worked before this feature shipped still
works exactly the same way, and still wins over anything in a config file —
nothing already deployed breaks. The config file's job is to cover
everything env vars never could: multiple named setups, GovCloud/Government
endpoints, credential indirection, and the daemon's own startup.

Precedence is tracked **per field**, not per file: if your user config sets
`provider = "anthropic"` and your project config's same-named profile only
overrides `model`, the merged profile has project's `model` and user's
`provider` — a project layer's partial override never blanks out fields it
didn't mention. Run `fathom config --explain` to see exactly which layer
supplied each value (see below).

## Profiles

A **profile** is a named provider setup:

```toml
default_profile = "work"

[profiles.work]
provider = "anthropic"
model    = "claude-opus-4-1-20250805"   # exact pin — wins over the adapter's own default model
```

Fields (all but `provider` are optional):

| Field | Meaning |
|---|---|
| `provider` | One of the six lanes: `anthropic`, `claude-cli`, `openai`, `azure-openai`, `bedrock`, `codex-cli`. |
| `model` | Exact model pin. Still loses to `FATHOM_COMPILE_MODEL` if that's set (env beats profile). |
| `endpoint` | Base URL override: OpenAI-compatible host (Ollama/vLLM), Azure OpenAI resource endpoint, or a Bedrock `BaseURL` override (FIPS/VPC endpoint). |
| `region` | AWS region, for `provider = "bedrock"`. |
| `deployment` | Azure OpenAI deployment name, for `provider = "azure-openai"`. |
| `api_version` | Azure OpenAI API version; empty defers to the adapter's pinned GA default. |
| `bin` | CLI binary override for `claude-cli`/`codex-cli` (mirrors `FATHOM_CLAUDE_BIN`/`FATHOM_CODEX_BIN`). |
| `api_key_env` | Name of an environment variable to read at construction time. |
| `api_key_cmd` | Argv (no shell) executed at construction time; trimmed stdout is the key. |

Select a profile with `--profile <name>` on `compile`/`config`/`providers`/
`smoke`, or set `default_profile` so it's implicit. With neither a
`--profile` flag nor a `default_profile`, fathom falls back to the bare
environment — exactly its pre-config-file behavior.

### The env-overlay

Even when a named profile is active, the classic environment variables
(`FATHOM_LLM_PROVIDER`, `FATHOM_COMPILE_MODEL`, `AZURE_OPENAI_ENDPOINT`,
`AWS_REGION`, …) still win over that profile's own fields — they sit at the
env precedence tier, above any file. Think of the environment as an
*implicit, unnamed profile* that always overlays whichever named profile you
selected.

Two caveats worth reading once:

- **`FATHOM_LLM_PROVIDER` clobbers the provider of *every* profile.** With
  it exported, `--profile gov-bedrock` still resolves to the env var's
  provider — and the *rest* of the profile's fields still apply, so you can
  end up constructing, say, an `openai` client carrying a Bedrock profile's
  leftover fields. `fathom config --explain` makes this visible
  (`provider … [env:FATHOM_LLM_PROVIDER]`); the practical advice is to
  **unset `FATHOM_LLM_PROVIDER` once you adopt profiles** — the file gives
  you everything the env var did.
- **A `FATHOM_COMPILE_MODEL` precedence corner:** the env var set to
  *exactly* the built-in default model string is indistinguishable from
  unset, so in that one corner a profile's `model` pin wins instead of the
  env var. Any other value behaves per the normal precedence (env beats
  profile). If you actually want the default model, leave the variable
  unset.

## Secrets doctrine

**There is no `api_key` field.** The schema does not define one, and
`#Profile` is a closed CUE struct, so pasting a literal key into
`config.toml` is a hard validation error at load — not a silently-ignored
key, not a working-but-regrettable choice. Use one of:

- **`api_key_env`** — names an environment variable, read at LLM-client
  *construction* time (never at config-load time, so a rotated credential is
  picked up without re-parsing the file).
- **`api_key_cmd`** — an argv-exec'd command (no shell interpretation),
  bounded to a 10-second timeout, whose trimmed stdout is the key:

  ```toml
  [profiles.vault]
  provider     = "openai"
  api_key_cmd  = ["op", "read", "op://vault/openai/credential"]
  ```

  or, on macOS, the system keychain directly:

  ```toml
  api_key_cmd = ["security", "find-generic-password", "-w", "-s", "openai-fathom"]
  ```

Neither the resolved key nor a failing command's stdout/stderr is ever
logged or embedded in an error message.

## GovCloud and Azure Government

Both are **profiles, not modes** — no separate code path, just fields:

```toml
# AWS Bedrock, GovCloud region (standard host derives correctly)
[profiles.gov-bedrock]
provider = "bedrock"
region   = "us-gov-west-1"
model    = "anthropic.claude-3-5-sonnet-20241022-v2:0"

# AWS Bedrock, GovCloud FIPS endpoint (explicit override)
[profiles.gov-bedrock-fips]
provider = "bedrock"
region   = "us-gov-west-1"
endpoint = "https://bedrock-runtime-fips.us-gov-west-1.amazonaws.com"
model    = "anthropic.claude-3-5-sonnet-20241022-v2:0"

# Azure OpenAI, Azure Government
[profiles.gov-azure]
provider    = "azure-openai"
endpoint    = "https://<resource>.openai.azure.us"
deployment  = "<deployment-name>"
api_key_env = "AZURE_OPENAI_API_KEY"
```

## `fathom config` — effective configuration and provenance

```sh
fathom config                 # the resolved default profile, human text
fathom config --profile gov-bedrock
fathom config --explain       # every key annotated with its source
fathom config --json --explain
```

`--explain` is the flagship feature: every field is printed as
`value  [source]`, where source is `flag`, `env:<VAR>`, `project:<path>`,
`user:<path>`, or `default`. This is the fastest way to answer "why is
fathom using *that* model" without grepping your shell profile.

## `fathom providers` — auth health, zero spend

```sh
fathom providers               # every configured profile
fathom providers --profile gov-bedrock
fathom providers --json
```

Builds a `Pinger` for every selected profile and probes them concurrently
(`llmclient.ProbeAll` — zero or near-zero tokens per the kit's own
contract). A profile whose adapter doesn't implement `Ping` is reported
`not probeable`, never silently dropped from the output. Exits non-zero if
any probed lane is unhealthy or fails to construct.

## `fathom smoke` — one real completion, honestly priced

```sh
fathom smoke --profile work    # spends a handful of tokens
fathom smoke --all             # every configured profile — also spends
fathom smoke --all --json
```

Unlike `providers`, this **spends real tokens** — one tiny completion
(`"Reply with exactly: pong"`) per selected profile — which is why `--profile`
or `--all` is required explicitly; there is no implicit "smoke everything"
default. Reports model, wall-clock latency, token counts, and an estimated
cost (the same flat per-KTok rates every `compile` run's `cost_usd` uses, so
the numbers are directly comparable).

With **zero profiles configured**, `--all` falls back to the single
env-overlay lane (whatever `FATHOM_LLM_PROVIDER` et al. resolve to, shown as
`(env-overlay)` in the output) — the same fallback `providers` uses, so both
verbs stay useful before you've written a config file at all.

## The daemon: restart-to-apply

`serve-http` loads the same config at startup: `[daemon]` sets `store`,
`addr`, `call_log`, `auto_refresh` as flag *defaults* (a flag on the actual
invocation still wins), and `[daemon].default_profile` (falling back to the
top-level `default_profile`) selects which profile powers `compile_layer` —
with zero plist env required.

```toml
[daemon]
store           = "workspace/db/fleet.sqlite"
addr            = "127.0.0.1:7575"
auto_refresh    = true
default_profile = "work"
memory_limit    = "3GiB"
```

`memory_limit` (O108) is **superseded by `[tenancy].gc_hint`** (below). Set
alone — or as `FATHOM_MEMORY_LIMIT` — it keeps its meaning, the GC hint, and
serve-http names the migration at startup; set beside `[tenancy].gc_hint`,
`[tenancy]` wins and the conflict is named. It is a SOFT limit
(`runtime/debug.SetMemoryLimit`): the runtime GCs harder near it and never
stops the process. It is not protection — the daemon reached ~30 GB under a
3 GiB one (INC-23). The ladder below is the bound.

This is a **read-once** load: a change to `config.toml` reaches the daemon
only after a restart —
`fathom daemon status` (which names YOUR installed label) then
`launchctl kickstart -k gui/$(id -u)/<that label>` — the
same restart-to-apply rule that already governs a rebuilt `bin/fathom`
binary. Hot-reload was considered and deferred (see
`docs/plans/config-experience.md`'s Deferred section) — every other daemon
knob is read-once too, so this is consistent, not a regression.

**A malformed config file means the daemon refuses to start** (exit 1,
naming the file and the offending key) rather than running with whatever
defaults remain — silently ignoring a broken config is exactly the
misconfiguration class this file exists to kill. Under launchd that
manifests as a **crash-loop**, with the error on stderr in the launchd log.
Pre-flight before restarting: run `fathom config` (it loads the very same
layers and prints the same errors) — if it exits 0, the daemon will start.

**`FATHOM_LOG_LEVEL`** (env var, not a config key; diagnostics sweep DG-1)
sets the daemon's leveled `internal/dlog` output: `debug` / `info` / `warn`
/ `error`, case-insensitive. Unset or unrecognized falls back to `info` —
never a startup error, an operational tuning knob like `FATHOM_SCAVENGE_INTERVAL`
above. See `docs/guides/fleet-index.md`'s "Daemon logging" section for the
log format, rotation and dedupe-summary behavior this controls.

## The budget: `[tenancy]` (user scope only)

What the daemon may cost the machine it shares (ADR-047,
`docs/design/responsible-tenant.md` §1–§2). Budgets are machine contracts:
`[tenancy]` is read from `~/.fathom/config.toml` and env only. A `[tenancy]`
table in a repository's `.fathom/config.toml` validates but is **ignored**,
and serve-http names it at startup — a cloned repository cannot raise a
laptop's ceiling. Precedence: env > user > defaults.

```toml
# ~/.fathom/config.toml
[tenancy]
budget       = "auto"    # phys_footprint; auto = clamp(RAM/8, 1GiB, 4GiB)
shed_at      = 0.75      # × budget
refuse_at    = 1.00      # × budget; never below shed_at (a misordered ladder is refused at start)
exit_at      = 2.00      # × budget, capped at RAM/4, raised to the refuse mark
exit_enabled = true      # false keeps shed and refuse only (disclosed)
gc_hint      = "exit"    # SetMemoryLimit target: "exit" | "budget" | "off" | an IEC size (anything else is refused at start)
gomaxprocs   = "auto"    # managed daemons: max(2, NumCPU/2) unless GOMAXPROCS is in the env

[tenancy.host]
shed_on_pressure      = "warn"      # "warn" | "critical" | "off"
refuse_on_pressure    = "critical"
refuse_on_swap_growth = "2GiB"      # swap grew this much in the window while fathom ≥ the shed mark
swap_growth_window    = "5m"
min_swap_volume_free  = "5GiB"      # /System/Volumes/VM

[tenancy.unmanaged]
budget = "512MiB"                   # an unmanaged serve-http's own budget
```

Every 2 s the daemon samples its own `phys_footprint`, the footprint of its
child processes (`go list`, git, rg), host memory pressure, swap and volume
free space, and steps a ladder:

| Level | Entered when (two consecutive samples; a sample ≥ 6 s late counts twice) | What happens |
|---|---|---|
| `shed` | footprint + children ≥ `shed_at`, or host pressure warn | console and fleet memos dropped, console feeders and auto-refresh paused (reads answer from the index as it stands), memory returned to the OS |
| `refuse` | footprint + children ≥ `refuse_at`; pressure critical; swap grew > 2 GiB in 5 min while fathom ≥ the shed mark; swap volume < 5 GiB free | every cache dropped; every in-flight tree and fleet tool call cancelled, youngest first (`abandoned-by-governor`); whole-tree, fleet and ingest reads stop within one file or 1,000 rows; a heap profile captured |
| `exit` | own footprint ≥ the exit mark, or pressure critical for 60 s with footprint over budget | an in-flight ingest is halted first and its service **quarantined**; if the next sample is still over (or nothing was in flight), the child tree is killed, `heap.pb.gz` and `inflight.json` written under `~/.fathom/daemon/<id>/diag/`, an `exit` row (`reason: over_budget`) appended to `lifecycle.jsonl`, and the process exits **70** at once |

Step-down: 60 s below 0.6 × the current rung's mark with its host triggers
clear, one rung at a time. The swap-used fraction is never a trigger (macOS
grows swap on demand; 80% used is routine at normal pressure), and an
unreadable pressure or swap reading is disclosed, never a trigger; an
unreadable footprint turns the ladder off, disclosed. The startup line names
the budget and its derivation (`tenancy: budget 4 GiB (auto: clamp(RAM 32 GiB
/ 8, 1 GiB, 4 GiB)) · shed 3 GiB · refuse 4 GiB · exit 8 GiB`); `GET /metrics`
carries `fathom_tenancy_level`, `fathom_budget_bytes`,
`fathom_footprint_bytes`, `fathom_children_footprint_bytes`,
`fathom_host_pressure_level`, `fathom_swap_used_bytes`,
`fathom_gc_cpu_fraction` and `fathom_quarantined_services`.

**Quarantine.** A service whose ingest was halted at the exit mark (or was in
flight at an over-budget exit) is recorded in the daemon's lifecycle ledger;
the next start reads it, `/statusz` lists it (`quarantined: app-kit exceeded
budget during ingest — retry with the daemon's `ingest` tool (service
"app-kit"); a completed explicit ingest lifts it`), and auto-refresh never
re-ingests it. Only the daemon's `ingest` tool, called explicitly and
completing, lifts it (the response carries `quarantine_cleared: true`); a
hook's refresh (post-commit, edit) is refused with the quarantine named, a
failed or halted ingest leaves it, and a shell `fathom ingest` writes the
store directly and never reaches the daemon's quarantine.

**Unmanaged daemons** (no `--managed`: tests, scripts, benches) take
`[tenancy.unmanaged].budget` (512 MiB; exit mark 1 GiB), never the managed
budget. A scratch daemon that needs more names it:
`FATHOM_TENANCY_BUDGET=2GiB fathom serve-http …`.

Env: `FATHOM_TENANCY_BUDGET` (an IEC size; wins for managed and unmanaged
alike), `FATHOM_TENANCY_ADMISSION` (`observe` | `enforce` | `off`),
`FATHOM_TENANCY_EXIT=off` (disable the exit rung). An unparseable value is a
startup error naming the variable. Admission `off` (env or `[tenancy].admission`) is the
escape hatch: serve-http names it at startup and doctor's `tenancy_scope` WARNs on every
run, as it does when `[daemon].memory_limit` / `FATHOM_MEMORY_LIMIT` and
`[tenancy].gc_hint` are both set (`[tenancy]` wins).

## The decision port: `[decide]`

Off by default. When set, the daemon asks a typed decision model (vision 32,
ADR-042) three kinds of question about state it already holds — never for a fact —
and every answer is logged and disclosed:

```toml
# .fathom/config.toml
[decide]
provider          = "none"   # none | llm | typesafe — none makes no call and changes nothing
profile           = ""       # llm: the [llm] profile lane to use (default: the compile lane)
compile_threshold = 0.5      # a guide section whose "is this a binding rule" probability is below this is not compiled (counted, disclosed, still served raw)
route_confidence  = 0.9      # unchanged at this default (2026-09-24, ADR-042 amendment: route's own calibration reading failed at every cutoff); the prompt hook injects a HEDGED route line at or above the confirm floor (0.5) — never firm, whatever the confidence; nothing below 0.5
route_timeout_ms  = 3000     # POST /route's own decider budget, in ms (100-30000)
rank_within_tier  = false    # obligations re-orders rules only inside their governing-scope tier
rerank_timeout_ms = 1500     # obligations re-rank's own decider budget, in ms (100-60000)
sites = ["route", "compile_prefilter", "obligations_rerank", "certify_suggest", "assess"]  # the egress opt-in — this is the default
```

`rank_within_tier` defaults to **false** (amended 2026-09-23, ADR-042): the
decision-port A/B measured the LLM re-rank net negative on governed edits once
a real provider is configured (guards-jev-r1: paired mean -0.19, 3 tasks worse,
0 better — `docs/bench/readings/decision-port-ab-2026-09-23.md`), sometimes
pushing the one governing rule to the last visible slot on a later touch of
the same file. Routing and the compile pre-filter are unaffected and keep
their own measured (small positive / untested) results. Set it `true`
explicitly to opt back in.

`route_confidence` stays at its default **0.9** (amended 2026-09-24, ADR-042
decision 8/CV-M11): `docs/bench/readings/decision-port-calibration-2026-09-24.md`
checked route's proceed-band choice against 82 adjudicated route decisions and
found it right only 45.7% of the time at confidence ≥ 0.9 — well under this
ADR's 0.9-right-nine-times-in-ten bar — and sweeping every cutoff in `[0,1]`
never reaches that bar (the best any threshold achieves is 58.5%, and the
highest-confidence slice is measurably *worse* than the band below it).
Raising the threshold would not fix a miscalibration this shape, so it stays
put. Instead, the prompt hook's own route line is now **always hedged**: a
`proceed`-band and a `confirm`-band answer render identical wording (`cmd/
fathom/prompthook.go`'s `routeLine`), and nothing is ever injected below the
confirm floor (0.5). Routing still logs `applied: true` only for a proceed-band
answer at or above `route_confidence` (unchanged — that bookkeeping is
separate from what the line SAYS), and no other `[decide]` site's calibration
is affected.

`route_timeout_ms` (O109) bounds `POST /route`'s own decider call — the design
point is a **sub-second decision model** (Jev-class latency measured 70-500ms),
which is what the 3000ms default is generous for. An LLM-lane provider measured
live against OpenRouter answered in 1.5-2.8s warm (more cold) — comfortably over
half the old fixed budget — so a slower lane needs this raised; the prompt
hook's own shared deadline (`fathom prompt-hook`'s ~8s budget) still bounds the
line regardless of what this is set to, so raising it only trades "the route
line arrives" against "the hook's own deadline eats the wait instead."

`rerank_timeout_ms` (DP-25, O109) bounds how long the obligations re-rank
(`rerankWithinTiers`) may wait for the decider across EVERY chunk of one
`obligations` call — one shared budget for the whole call, not per chunk. The
design point is the same sub-second decision model (e.g. TypeSafe/Jev); an LLM
lane will usually exceed the default entirely — O109's own chunked live run
measured a single 20-question chunk at 8-18s against OpenRouter — and when the
budget is exceeded, the envelope discloses `rank_note` naming the budget
(`"decision budget <N>ms exceeded: <k> of <m> chunks unanswered"`) while rules
are served in their plain lexical order rather than waiting. The default
(1500ms) is deliberately **below** `route_timeout_ms`'s own default and below
even that sub-second design point's usual headroom, because `obligations`
(unlike `/route`) sits on the **PostToolUse edit hook's own critical path**:
`fetchObligations` (`cmd/fathom/edithook.go`) gives that hook's obligations RPC
a flat **2000ms HTTP client timeout**, and on any timeout or error the hook
renders **nothing at all** ("daemon down or error → silent") — a strictly worse
outcome than the rerank giving up early and serving the untouched lexical
order. A server that spent its entire budget on the rerank alone, with no
margin left for store I/O, JSON encode/decode and the network round trip,
risks losing that race outright; 1500ms leaves 500ms of margin under the
hook's 2000ms ceiling. Raise it only for offline/non-hook use (a terminal
`fathom obligations` call, a bench run) where nothing is racing a client-side
deadline.

`sites` (ADR-044 decision 8) is the egress opt-in for I14 ("questions carry
code off-host"): the site names this port is allowed to send a
`decision.Question` off-host from — `route` (`POST /route`), `compile_prefilter`
(the compile pre-filter, DP-9), `obligations_rerank` (the obligations
within-tier re-rank, DP-7/DP-8), `certify_suggest` (`fathom certify
suggest`, TC-24), and `assess` (`fathom assess`, ADR-046 — which also needs
its own default-off `assess` capability, so listing the site is never, by
itself, consent to spend). **A site absent from `sites` makes NO call at all**,
whatever `provider` is set to — this is a second, independent gate a real
provider must also clear, checked once per call site (`internal/decide.
SiteAllowed`) rather than re-derived at each one. Every failure discloses why:
`POST /route` answers its usual none-shaped body with `Error` naming the
blocked site; `fathom certify suggest` prints one line naming it and exits 0;
`fathom doctor`'s `decision_provider` row appends `(sites off: <names>)` to a
passing provider's own Reason whenever `sites` excludes any of them.
Unset (the default, shown above) means every current site, so an
unconfigured `[decide]` block behaves exactly as it did before `sites`
existed; `sites = []` disables every site outright — a fully quiet decision
port with a provider still configured. Closed to the five site names that
exist today (`fathom config` load refuses an unrecognized one by name, the
same discipline `provider`'s own enum already applies).

`provider = "typesafe"` routes through the kit's TypeSafe (Jev) adapter directly
— no `[llm]` profile needed — and lands 2026-09-22 (ADR-042 amendment; the
"parked until access" clause it supersedes is `docs/intent/futures.md` F21):

```toml
# .fathom/config.toml
[decide]
provider    = "typesafe"
endpoint    = "https://api.typesafe.ai"   # optional; this is the default
model       = "jev-latest"                # optional; this is the default
api_key_env = "TYPESAFE_API_KEY"          # or api_key_cmd = ["op", "read", "op://vault/typesafe/credential"]
```

The key is resolved once, when the daemon (or a direct `fathom compile --store`) opens the port at startup — never at config load, never written anywhere: `api_key_cmd` wins over
`api_key_env`, and when neither is set the well-known `TYPESAFE_API_KEY`
environment variable is read directly. A literal `api_key` in `[decide]` is not
representable — the schema is closed, same discipline as `[profiles.*]`.

**No TypeSafe key of your own?** OpenRouter serves the same model (Jev 1.13)
behind the same TypeSafe System One wire shape — `endpoint`/`model` are the
only two fields that change, since `TypeSafeConfig.BaseURL` is always suffixed
with `/v1/systemone`:

```toml
# .fathom/config.toml
[decide]
provider    = "typesafe"
endpoint    = "https://openrouter.ai/api"
model       = "typesafe/jev-1.13"       # pinned; "~typesafe/jev-latest" tracks the rolling alias instead
api_key_env = "OPENROUTER_API_KEY"
```

This is the recommended route for anyone without a direct TypeSafe API key —
one OpenRouter account covers this adapter and the `llm` adapter's own
OpenRouter lanes both. Priced (2026-09-22) at $0.042 per million input
tokens; output is free. First live run against the real model over this
route: O110 (`.fathom/observations.md`) — routing and the obligations
re-rank both worked and genuinely disclosed `ranked_by`/`relevance` on the
wire; the compile section pre-filter does not yet work against this adapter
on a repo whose obligations layer has grown past a small section count (see
O110 for the root cause — an unbounded batch size, not an OpenRouter or
Jev-specific limitation).

`fathom doctor` reports the provider under `decision_provider`; the obligations
envelope carries `ranked_by` (`none` | `llm` | `typesafe`), and `POST /route` on
the loopback daemon serves the prompt hook's route with the same `provider`
value.

Both `llm` and `typesafe` are **fail-open** (I14): no key, an unreachable
endpoint, or any other error opening the port at startup is one `warn:` line on
stderr and the port silently falls back to `none` — a misconfigured `[decide]`
never stops the daemon from starting or the CLI from compiling. `fathom doctor`
names exactly what failed (missing key, ping failure, unknown provider) under
`decision_provider` so the fallback is never silent to an operator running it.

Like every other `config.toml` block, `[decide]` is read once at daemon
startup — a change is **restart-to-apply**, never live-reloaded. `profile`
empty falls back to the daemon's own compile-lane profile (`[daemon].
default_profile`, else the top-level `default_profile`) — one lane
resolution chain shared by compile and the decision port, not two to keep
in sync (this fallback chain is `llm`-only; `typesafe` has no profile). Every
answered decision (the compile pre-filter, the obligations re-rank, and
`POST /route`) is appended to `decisions.jsonl`, written beside the configured
call log (`internal/decide.DefaultRecorderPath` — same directory as
`[daemon].call_log`/`--call-log`, default `logs/mcp-calls.jsonl` under the
repo's state directory); no call log configured means no decision log either.
A call that hard-fails (no answer at all) is logged too, as an `error` row
(DP-22/DP-23, O109) — so silence in `decisions.jsonl` always means "never
asked", never "asked and failed".

## The findings loop: `[findings]`

`fathom init` scaffolds `.fathom/config.toml` (project scope, sibling to
`guard.cue`/`modules.cue`/`concerns.cue`/`capabilities.cue`) with a fully
commented `[findings]` block — the review kit's projection outbox
(`docs/vision/30-findings-loop.md`, ADR-040) reaching a coding session
without a network hop or a daemon round trip:

```toml
# .fathom/config.toml
[findings]
subject   = "org/repo/component"                 # the provisioned review-kit subject this repo maps to
outbox    = ".reviewkit/outbox/projection.json"   # repo-relative; optional
producers = ["reviewkit.system-review"]           # producer ids this repo trusts
```

- `subject` binds this repository to one review-kit subject
  (`org/repo/component`). `fathom findings import` refuses any event whose
  own subject does not match exactly — no mapping, no import, and never a
  best-effort guess.
- `outbox` (optional) names a repo-relative projection file. When set, the
  SessionStart/SubagentStart hook runs `findings import` against it once
  per session, bounded to one attempt, IF the file's digest differs from
  the cursor recorded under fathom's state dir since the last import — a
  no-op session incurs no extra work. The session brief reports the
  receipt (`open findings: N (k producers; oldest …)`) or the failure
  reason, honestly, when the sync could not run (`docs/adoption/
  findings-loop.md` covers every failure shape: no subject provisioned, an
  unknown producer, a conflicting replay, a circular fathom-origin
  producer). Leaving `outbox` unset is a valid, permanent choice — a CI
  review that commits its own outbox, or an operator who only ever runs
  `fathom findings import` by hand, needs it never.
- `producers` is this repo's own allow-list against the producer catalog
  (`schema/semantic/producers.cue`) — an event from a producer not named
  here (or not in the catalog at all) is refused, never silently accepted
  at a lower tier.

This section is **read once per session** (the sync above), never
hot-reloaded mid-session — the same restart/re-read discipline every other
`.fathom/config.toml` section follows. `fathom findings status` and
`fathom findings candidates` read the store directly and need no config at
all beyond `--store`/`--service`; only the SESSION-HOOK sync depends on
`[findings]` being provisioned. See `docs/adoption/findings-loop.md` for
the end-to-end CI and local variants, and `docs/use-cases/change-review.md`
for the loop from a reviewer's own seat.

## Test evidence: `[tests]`

Vision 33 / ADR-043 stage 0's test-kit evidence ingest
(`docs/vision/33-test-certifications.md`): `fathom tests ingest
--results <file|dir>` always works explicitly; `[tests] results_dir` makes
a PLAIN `fathom ingest` also pick up attested `#TestResult` envelopes from
that repo-relative directory, without a separate `tests ingest` call.

```toml
# .fathom/config.toml
[tests]
results_dir   = "workspace/artifacts/test-results"   # repo-relative; optional
artifacts_dir = "workspace/artifacts/test-blobs"      # repo-relative; optional
```

- `results_dir` (optional): a repo-relative directory of attested
  `#TestResult` envelopes (`internal/testevidence.LoadEnvelopes`'s own
  shape — plain JSON, NDJSON, a JSON array, or DSSE/in-toto-wrapped).
  Unset means no auto-pickup on a plain `fathom ingest` — `fathom tests
  ingest --results` still works either way.
- `artifacts_dir` (optional): a repo-relative directory an envelope's
  artifact references (`name` + `sha256` digest) resolve against when the
  digest matches. Unset means artifact refs are recorded as facts but
  never resolved to bytes.

Both keys are read the same way every other `.fathom/config.toml` section
is (project layer over user layer, `fathom config --explain` for
provenance) — see `internal/config`'s `Tests` struct and
`schema/config/schema.cue`'s `#Tests`.

## Discovery: `[connector]`

Discovery skips the default cache directories (`.cache`, `.tools`, `target`,
`dist`, `build`, `__pycache__`, `.venv`, `.gradle`, `.next`, `.turbo`,
`coverage`) on top of the always-skipped names (RT-145, docs/design/
responsible-tenant.md §4.2). A corpus that commits source under one of them
re-includes it in **its own** `.fathom/config.toml` — the connector reads the
file at the root it discovers, so each service carries its own list:

```toml
# .fathom/config.toml
[connector]
include = ["coverage", "compute/.tools"]
```

A bare name re-includes every directory with that name; an entry with a slash
re-includes that one repo-relative path. `include` never re-includes
`vendor`, `.git`, `testdata` or the other always-skipped names. It is refused
in the user layer (`~/.fathom/config.toml`): a load error names the file,
since a user-wide list would change nothing. Measure the
effect first with `fathom ingest --path <root> --service <id> --dry-run
--json` (stat only; `default_ignored_dirs` names each pruned directory).

## Capability-inventory patterns: `[[surface.pattern]]`

ADR-044 §2.1 (`docs/design/refactor-conservation.md`): `internal/analysis/
inventory.Extract` derives most of a corpus's capability inventory from
built-in kinds — a Go exported declaration (`api:`), a JSON wire tag
(`json:`), an HTTP route (`route:`), an environment-variable read (`env:`),
a proto RPC (`rpc:`) — with no configuration at all. A repository whose own
surface a built-in kind cannot see (this repository's own MCP tool names
and CLI verbs: it dispatches its CLI through a plain switch, never cobra,
and registers MCP tools through its own struct literals, not a typed
registry) declares `[[surface.pattern]]` blocks instead, one per surface:

```toml
# .fathom/config.toml
[[surface.pattern]]
kind  = "tool"
glob  = "internal/mcp/*.go"
regex = 'registerTool\(toolDescriptor\{\s*\n\s*Name:\s*"([a-zA-Z_][a-zA-Z0-9_]*)"'

[[surface.pattern]]
kind  = "verb"
glob  = "cmd/fathom/verbs.go"
regex = '\{Name:\s*"([a-z][a-z0-9-]*)"'
```

- `kind` (required): the `"<kind>:"` prefix every match is emitted under —
  `fathom conserve`'s surface plane then treats `tool:find_callers` exactly
  like a built-in `api:`/`route:` key: removed or changed between base and
  head is a finding, excused by a plan row's `retires: "tool:find_callers"`
  or `surface_change:` declared BEFORE the change range (I45).
- `glob` (optional; unset matches every file): internal/filter's
  gitignore-style syntax (`**` included) scoping which files the pattern
  scans. Files ending `_test.go` are never scanned by any kind, built-in
  or configured — test fixtures are never part of the capability surface.
- `regex` (required): a Go regexp whose FIRST capturing group names the
  matched item; a pattern with no capturing group, or that fails to
  compile, yields no items rather than an error.

`fathom conserve --since <ref>` and (later) `fathom review --since <ref>`
read these via `internal/config.SurfacePatterns`; `fathom refactor plan`
(a later row) also reads them when generating a `gone:` row for a
configured-pattern surface item. This repository's own two entries above
declare its tool and verb surfaces (CV-26).

## Worked example

```toml
# ~/.fathom/config.toml
default_profile = "personal"

[profiles.personal]
provider = "claude-cli"          # subscription-backed, no API key at all

[profiles.ci]
provider    = "anthropic"
api_key_env = "ANTHROPIC_API_KEY"

[profiles.gov-bedrock]
provider = "bedrock"
region   = "us-gov-west-1"
model    = "anthropic.claude-3-5-sonnet-20241022-v2:0"

[daemon]
store           = "workspace/db/fleet.sqlite"
default_profile = "personal"
```

```sh
fathom config --explain                       # confirm what resolved and from where
fathom providers                              # confirm every lane's auth health, $0
fathom smoke --profile ci                     # confirm ci actually completes, small $
fathom compile --service myrepo --profile gov-bedrock --dry-run=false
```

## Scoped config: `~/.fathom/ignore` and `~/.fathom/guard.cue`

A second, separate `~/.fathom/` config surface — unrelated to LLM
provider profiles above — governs what fathom indexes and what an agent
sees before an edit. Design source: `docs/design/scoped-config.md`
(law 13: "user scope may add defaults; only project scope may govern").

### `~/.fathom/ignore` — additive-only, machine-wide excludes

Every repo's own `.fathomignore` (project scope) is UNIONed with
`~/.fathom/ignore` (user scope) when fathom resolves what to index —
`internal/connector.LoadIgnoreScoped` is the merge point, consulted by
both `Discover` (ingest) and every corpus-reading MCP tool (`read_source`).
This exists so machine-wide junk (`.DS_Store`, `*.wasm`, editor state) is
written once per machine, not re-derived by hand in every repo's own
`.fathomignore`.

**Additive only — this is the one rule that makes it safe.** A user-scope
pattern may only ever *exclude*; a gitignore-style re-include
(`!pattern`) at user scope is refused **loudly**:

```
connector: /Users/you/.fathom/ignore:2: "!keep-this.log" is a re-include
pattern (leading '!') — user-scope ignore files may only ADD exclusions,
never re-include (docs/design/scoped-config.md law 13); remove this line
or move it to the project's own .fathomignore
```

`Discover` (and therefore `fathom ingest`/`fathom module sources`) refuses
outright on a violating file — ingest simply does not proceed until it's
fixed. The read-time convenience `connector.LoadIgnore` (used by
`read_source`) degrades gracefully instead — a violating user-scope file
is dropped entirely (project-scope-only fallback) rather than breaking a
hot serving path; `fathom doctor`'s `user_scope_ignore` check is the loud
surface for that condition outside of ingest, and also warns when a
user-scope pattern (not a project one) is specifically what's hiding an
otherwise-indexable file — the "why can't fathom see my code" failure
mode, pre-empted.

Where both scopes exclude/re-include the same path, **project always
wins** (patterns are unioned user-then-project; the matcher is
last-match-wins) — a project's own `!pattern` can always override a
user-scope default.

Seed it with the machine-junk defaults (`.DS_Store`, `*.wasm`,
`.playwright-mcp/`, editor/tool state) via `fathom init`'s opt-in flag —
never written silently:

```sh
fathom init --seed-user-ignore   # prints exactly what it writes; write-if-absent
```

Disclosure: `fathom ingest` and `fathom module sources` print
`excluded: N pattern(s) (P project, U user-scope ~/.fathom/ignore)`
whenever either scope contributes a pattern.

### `~/.fathom/guard.cue` — notes and commands only, never governance

`~/.fathom/guard.cue` is the SAME `#Guards` CUE shape a project's own
`.fathom/guard.cue` uses (`docs/design/guarded-edits.md`), but loaded by a
separate, restricted reader — `guardspec.LoadUserGuards` — that permits
**only** `note` and `command_scopes`. Any governance field
(`scopes`, `max_rules`, `include_modules`, and — checked defensively even
though no such `#Guard` field exists today — `min_tier`) present on a
user-scope guard is a **strict validation error** naming the field:

```cue
guards: [
	{
		note: "confirm before an apply — this is MY danger list, not the repo's"
		command_scopes: ["terraform apply*", "kubectl apply*"]
	},
]
```

A user-scope guard can therefore only ever be a **command** guard
(`scopes` — file globs — is forbidden), matching the design's framing:
"the operator's own danger list," never a repo-governance surface. It is
consulted by the PreToolUse guard hook (`edit-hook --pre`) only when NO
project guard already matched the same command — project scope always
wins. Every rendered user-scope note is tagged `[user]` at load time
(`guardspec.LoadUserGuards` prefixes it), so the injected text is visibly
distinguishable from a project-authored rule. `fathom doctor`'s
`user_scope_guard` check names the file and is loud on a governance-field
violation.

`~/.fathom/modules.cue` (platform-module bindings) has **no** user-scope
variant, by design — binding is the trust boundary ("no org rule reaches
a repo that did not bind it"); a global binding would be the exact
inversion of that. It stays project-scope-only, always
(`<repo>/.fathom/modules.cue`, `docs/design/platform-modules.md`; doctor's
`binding_file` / `binding_modules_mounted` / `binding_module_readiness`
rows report it). The project-scope `.fathom/guard.cue` (`#Guards`: `scopes`
file globs, `command_scopes`, `note`, `max_rules`, `include_modules`) is the
PreToolUse guard hook's rule source — `docs/design/guarded-edits.md`.

### `capabilities.cue` — what is on, and the `budgets` block

Two scopes, same shape (`internal/capability/capability_schema.cue`;
design `docs/design/capability-registry.md`, law 13): `~/.fathom/capabilities.cue`
may set a **default**; `<repo>/.fathom/capabilities.cue` **governs**.

```cue
capabilities: {
    narrate: on: true          // fathom capabilities set narrate on
    assess:  on: true          // consent to the priced assess lane (default off; ADR-046)
}
// BA-9 (experimental): default answer budget per tool class, in TOKENS.
budgets: {
    list:      1500 // query_symbols, references
    composite: 4000 // pack_context
    map:       2500 // repo_map
}
```

The `budgets` block is closed over exactly those three classes — an unknown
key (`lists`) is refused with a did-you-mean, a `budgets?:` optional
declaration is refused, values must be whole positive token counts —
and resolves project → user → built-in. `find_symbol`, `find_callers`,
`symbol_context`, `outline` are not class-budgeted. `serve-http` honours
it; stdio `serve-mcp` still uses the built-ins (documented gap). `fathom
capabilities set` splices only the `capabilities` block and preserves the
rest of the file byte-for-byte (it refuses when the file cannot be
round-tripped); a user-scope file that sets a project-only capability is
refused with the key named. `fathom capabilities` / `explain <id>` read it;
`fathom doctor`'s `capabilities` row reds only when something switched on
is failing to deliver.

`assess` is the one capability that is **consent to spend** (ADR-046): it
defaults off, and `fathom assess` reads it failing CLOSED — a malformed
capabilities file refuses the run, where the safety capabilities fail open.
A live run also needs a `[decide]` provider and the `assess` site; each run
is metered on `~/.fathom/assess-ledger.jsonl` (`FATHOM_ASSESS_LEDGER`), never
on the session ledger (IR-4).

### `concerns.cue` — the config custodian's contracts

`~/.fathom/concerns.cue` (user defaults) and `<repo>/.fathom/concerns.cue`
(governs; a same-named entry replaces, `off: true` disables and is named).
Each concern is `{want, why?, check: {kind, …}}`; `fathom config audit`
reports `holds` / `violated (file:line)` / `unable to check` / `off`, and
doctor's `concerns` row and the session-brief attention line ride the same
model. Kinds: `forbidden_pattern` (`pattern`, `where: "agent-config"`),
`command_documented_and_exists` (`command`), `max_lines` (`file`, `lines`),
`pointers_resolve`, and — CC-14/CC-15, **experimental** —
`stanza_tools_served` (the one kind that asks the daemon; degrades to
unable-to-check) and `handoff_cites_instruments` (`days`) — plus (CC-12)
`required_pattern` (`pattern`, `where`, and over a YAML corpus `under`: only
the lines under one top-level key, such as a lefthook hook; a comment never
matches), `stanza_present` (`file`),
`max_bytes` (`file`, `bytes`) and `commands_resolve` (`where`: every
`task` target, local include and repo script the corpus invokes exists).
`where` names a corpus (CC-11): `agent-config` (default), `lefthook`,
`taskfile`, `ci`.

`packs: ["claude-code", "codex", "lefthook", "taskfile"]` (either scope; the
union is enabled) turns on concern sets fathom ships as CUE data; a same-id
entry in your file replaces a pack's wholesale and `<id>: {off: true}`
disables it (named `off` in the audit). An unknown pack, an unknown
top-level field (`pack:` for `packs:`), a `where` on a kind that reads no
corpus, a check field the kind does not read (`file:` on a
`required_pattern`), and an optional (`name?:`) concern are refused by name. Full model,
corpus and pack tables: `docs/design/config-custodian.md`; this repo's own
file is a worked example.

---

## Migrating the store from DuckDB to SQLite (historical — ADR-020/ADR-025)

**This runbook is historical.** The fleet migrated on 2026-07-23
(`workspace/logs/store-migration-stamp.txt` on the fleet checkout records
the exact binary + timestamp), the daemon ran a full day green on SQLite,
and SS-8 (`docs/plans/store-sqlite.md`) then deleted the DuckDB backend, the
`migrate-store` CLI command, and the CGO/deployment-target build
requirements it needed — outright. There is no DuckDB source to migrate
FROM in any build of fathom past that point; a `.duckdb` store path now
fails loudly with a typed error (`store.BackendForPath`) naming a pre-SS-8
build as the only way to read one. `scripts/migrate-fleet-store.sh` is kept,
also marked historical, as the executable record of exactly what the
migration ran.

The steps below are preserved for that historical record only — do not
follow them against a current build; `bin/fathom migrate-store` no longer
exists.

```sh
# 1. Stop the daemon (launchd label may differ; see your plist).
launchctl bootout gui/$(id -u)/"$(fathom daemon status | awk '/^launchd agent:/ {print $3}')"

# 2. Port the non-derivable tables (source is opened read-only and left
#    byte-identical; every ported table is hash-verified against the target).
bin/fathom migrate-store \
  --from workspace/db/fleet.duckdb \
  --to   workspace/db/fleet.sqlite

# 3. Repoint the daemon at the new store: either [daemon].store in
#    ~/.fathom/config.toml …
#      [daemon]
#      store = "workspace/db/fleet.sqlite"
#    … or the --store argument in the launchd plist, whichever you use.

# 4. Restart the daemon.
fathom daemon install --force   # or: launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/<label>.plist

# 5. Re-ingest each service to rebuild the derived tables (observations,
#    fingerprints) — migrate-store printed the exact per-service commands;
#    with the daemon up, the `ingest` MCP tool per service works too.

# 6. Once everything is green (services listed, symbols resolve, verify
#    passes), delete the old store — otherwise every store-directory scan
#    (PlatformStore, gen-platform-manifest) will keep warning about the
#    same-stem collision until you do.
rm workspace/db/fleet.duckdb   # plus any stray .duckdb.wal sidecar
```

**Sibling-repo hooks:** any repo whose post-commit hook refreshes into this
fathom store path (e.g. agentic-kit's) must be repointed to the new
`fleet.sqlite` path at the same time, or it will fail (or worse, recreate a
stale-path store) on its next commit.

**Rollback (historical):** the source `.duckdb` was left untouched by the
migration; rolling back meant repointing the daemon at it and deleting the
`.sqlite` (and its `-wal`/`-shm` sidecars). Post-SS-8 this is no longer a
live option — the DuckDB backend that would open the `.duckdb` file is
gone; a genuine rollback needs a pre-SS-8 build of fathom. The on-disk
`workspace/db/fleet.duckdb` rollback file itself is left for the operator to
delete manually once they're satisfied it's no longer needed — SS-8's
removal work does not touch it.
