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
- **Project**: `<repo>/.fathom/config.toml` — the repository's own
  behaviour: which provider and model, thresholds, include lists,
  repository-relative paths. It never names an executable, a credential, a
  key host, a path outside the repository or the daemon's own settings:
  those fields are user-only and are ignored there, by name (see "Scope"
  below).

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

## Scope: what a repository's config may set

A repository's `.fathom/config.toml` may choose **behaviour**: a provider, a
model, thresholds, include lists and repository-relative paths. It may never
name an executable fathom runs, name a credential, name a path outside the
repository, or switch on automatic execution (SEC-2, ADR-042 amendment
2026-10-09). Every field below has one of three classes, and
`internal/config`'s registry is the source: a test walks the schema against
it, so a new field cannot land unclassified, and another pins this table to
it.

- **behaviour**: either file may set it; the project file wins.
- **repo-path**: a path inside the repository. An absolute value, a `..`
  escape or a symlink that resolves outside the repository is refused where
  the path is used, by name (`config.RepoPath`), in whichever file it is set.
- **user-only**: read from `~/.fathom/config.toml` only. A repository that
  sets one has it dropped at load; `fathom doctor`'s `key_source_scope` row
  and `fathom config` (text and `--json`) name the file, the field and why,
  never the value, and a daemon started inside the repository logs the same
  line once. The row also names a repo-path value that leaves the
  repository.

`*` stands for a name you choose (a profile, a zone), `[]` for one entry of
an array of tables, and `tenancy.**` for every key under `[tenancy]`.

| Field | Scope | Why |
|---|---|---|
| `connector.include` | repo-path | directory names or repo-relative paths discovery re-includes (internal/connector/defaultignore.go) |
| `connector.respect_gitignore` | behaviour | discovery reads `git ls-files` (fixed argv, a read) |
| `console.conserve_max_files` | behaviour | a bound on the console's conserve range |
| `daemon.addr` | user-only | configures this machine's daemon, not one repository: the address serve-http listens on (servehttp_config.go) |
| `daemon.auto_refresh` | user-only | switches on automatic execution (re-ingest runs the language toolchain in every repository) |
| `daemon.call_log` | user-only | configures this machine's daemon, not one repository: a path the daemon writes, anywhere on disk |
| `daemon.default_profile` | behaviour | chooses a profile by name; a profile's executable and key host are user-only |
| `daemon.memory_limit` | user-only | configures this machine's daemon, not one repository: a memory ceiling, like [tenancy] (P13) |
| `daemon.state_dir` | behaviour | the one value, "workspace", keeps runtime state inside the repository (statedir.Resolve) |
| `daemon.store` | user-only | configures this machine's daemon, not one repository: the store path fathom writes (the session hook's findings import passes it to --store) |
| `decide.api_key_cmd` | user-only | names a credential or the command that prints one |
| `decide.api_key_env` | user-only | names a credential or the command that prints one |
| `decide.api_key_file` | user-only | names a credential or the command that prints one |
| `decide.base_url` | behaviour | a key goes only to the provider's own host, a loopback one, or one a user-layer base_url names (internal/decide checkKeyDestination) |
| `decide.compile_threshold` | behaviour | a threshold |
| `decide.endpoint` | behaviour | base_url's older name, under the same key-host rule |
| `decide.model` | behaviour | chooses a model |
| `decide.profile` | behaviour | chooses a profile by name |
| `decide.provider` | behaviour | chooses a provider |
| `decide.rank_within_tier` | behaviour | a ranking choice |
| `decide.rerank_timeout_ms` | behaviour | a threshold |
| `decide.route_confidence` | behaviour | a threshold |
| `decide.route_timeout_ms` | behaviour | a threshold |
| `decide.sites` | behaviour | the sites a decision may come from; an opt-in site only from the repository (JF-4) |
| `decide.transport` | behaviour | a key stays with the transport it was named for (DP-39) |
| `default_profile` | behaviour | chooses a profile by name |
| `findings.outbox` | repo-path | a repo-relative file the session hook imports (findings_outbox.go) |
| `findings.producers` | behaviour | an allow-list of producer ids |
| `findings.subject` | behaviour | the review-kit subject this repository maps to |
| `guards.enforce` | behaviour | turns the repository's own guards advisory |
| `hooks.pin_check` | behaviour | the commit pin line's output mode; the check itself is on by default |
| `hooks.test_run` | behaviour | the test-run hook runs nothing: it reads the output of a command the harness ran (TR-X2, testrunhook.go) |
| `profiles.*.api_key_cmd` | user-only | names a credential or the command that prints one |
| `profiles.*.api_key_env` | user-only | names a credential or the command that prints one |
| `profiles.*.api_version` | behaviour | an Azure API version |
| `profiles.*.bin` | user-only | names an executable fathom runs (the claude-cli and codex-cli lanes exec it; codex-cli at construction) |
| `profiles.*.deployment` | behaviour | an Azure deployment name |
| `profiles.*.endpoint` | user-only | names the host a profile sends your API key to (the openai lane defaults its key to OPENAI_API_KEY) |
| `profiles.*.model` | behaviour | chooses a model |
| `profiles.*.provider` | behaviour | chooses a provider |
| `profiles.*.region` | behaviour | an AWS region |
| `profiles.*.timeout` | behaviour | a bound on one model call |
| `surface.pattern[].glob` | behaviour | a glob over the repository's files |
| `surface.pattern[].kind` | behaviour | a label |
| `surface.pattern[].regex` | behaviour | an RE2 pattern over the repository's files |
| `tenancy.**` | user-only | configures this machine's daemon, not one repository: the budget is a machine contract (RT-30, P13) |
| `tests.artifacts_dir` | repo-path | a repo-relative directory of test artifacts (read by no code yet) |
| `tests.commands` | behaviour | declared test commands the test-run hook matches; no automatic path runs them |
| `tests.results_dir` | repo-path | a repo-relative directory a plain ingest reads envelopes from (ingest.go, conserve/coverageload.go) |
| `work.ci_summaries[].commit` | behaviour | a commit id |
| `work.ci_summaries[].path` | repo-path | a repo-relative CI summary file the console reads (console_work_signals.go) |
| `zones.*` | behaviour | repo-relative path globs |

**Which commands may run a repository's own commands.** Only an explicit
verb you type with its flag: `fathom doctor --stranger` runs the
repository's `scripts/stranger-check.sh`, and `fathom receipt verify
--run-tests` runs `go test` for the test a receipt names. No hook, no daemon path, no ingest and no
prompt hook runs a command a repository names: the test-run hook matches
`[tests] commands` against a command the harness already ran and reads its
output; nothing executes them.

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
| `timeout` | `claude-cli` only: a Go duration (`"20m"`, `"90s"`) bounding ONE model call's subprocess; mirrors `FATHOM_CLAUDE_TIMEOUT`. Omitted keeps the kit's 5 minute default. A section whose call exceeds it counts as failed and is named in `section_errors`; the compile continues. |
| `api_key_env` | Name of an environment variable to read at construction time. |
| `api_key_cmd` | Argv (no shell) executed at construction time; trimmed stdout is the key. |

`bin`, `endpoint`, `api_key_env` and `api_key_cmd` are **user-only**: a
repository's `.fathom/config.toml` may define a profile and choose its
provider, model, region, deployment and timeout, but the binary fathom
executes and the host your key is sent to come from `~/.fathom/config.toml`
(the same profile name there merges field by field). A repository's own
`bin = "./script"` would otherwise run when a daemon or `serve-mcp` starts
inside it, and its `endpoint` would receive your `OPENAI_API_KEY`.

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
logged or embedded in an error message. The decision port's remote providers
(`[decide]`, below) also take **`api_key_file`**, a path to a `chmod 600` file
the adapter reads at first use — the one source the launchd daemon can see.

**Where the key fields are honored.** `api_key_cmd`, `api_key_file` and
`api_key_env` (in `[decide]` and every `[profiles.*]`) are read from your user
config, `~/.fathom/config.toml`, and nowhere else (plus the transport's default
variable when none is named); since SEC-2 the same holds for every user-only
field in the scope table above. A repository's `.fathom/config.toml` can choose a
provider, model and thresholds, but a credential, and the command that fetches
it, are yours: a repository could otherwise make fathom run any command when it
opens that repository. `fathom` drops such a field at load, `fathom doctor`'s
`key_source_scope` row names the file and field (never the value), and `fathom
config` lists it as ignored. A successful `api_key_cmd` runs at most once per
process and its result is kept in memory only (SEC-1, ADR-042 amendment
2026-10-09).

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
with zero plist env required. `store`, `addr`, `call_log`, `auto_refresh`
and `memory_limit` configure this machine's daemon, so they are read from
`~/.fathom/config.toml` only: a daemon started inside a repository ignores
that repository's values and logs one line naming each (SEC-2).
`state_dir = "workspace"` and `default_profile` stay repository choices.

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

**`FATHOM_GATE_LOCK`** and **`FATHOM_GATE_NICE`** (env vars, not config keys; RT-129,
design §12.3) tune the machine-wide gate lease `task validate` runs under
(`scripts/gate-lease.sh`). `FATHOM_GATE_LOCK` is the lease's flock file (default
`~/.fathom/locks/gate.lock`): every gate on the machine that names the same file queues
on it, so point concurrent checkouts at one file, not one each. `FATHOM_GATE_NICE=off`
runs the gate at normal priority instead of `nice -n 10`; `FATHOM_GATE_NICE=background` runs it
under macOS's `taskpolicy -b` (efficiency cores, throttled I/O — measured ~3x slower).
`task validate:unleased` is the same gate without the lease. See `docs/usage.md`.

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
admission    = "observe" # "observe" (default) | "enforce" | "off"; env FATHOM_TENANCY_ADMISSION

[tenancy.host]
shed_on_pressure      = "warn"      # "warn" | "critical" | "off"
refuse_on_pressure    = "critical"
refuse_on_swap_growth = "2GiB"      # swap grew this much in the window while fathom ≥ the shed mark
swap_growth_window    = "5m"
min_swap_volume_free  = "5GiB"      # /System/Volumes/VM
min_store_volume_free = "2GiB"      # below it the store is in distress: writes and ingest refused

[tenancy.breaker]
safe_after  = 3                     # counted exits in safe_window → next start in safe mode
safe_window = "30m"
stop_after  = 5                     # counted exits in stop_window → disabled.json {reason: breaker}, exit 0
stop_window = "60m"
warmup      = "120s"                # after any start: one slot per heavy class (refused past it unless admission = "off"), no inline ingest

[tenancy.unmanaged]
budget = "512MiB"                   # an unmanaged serve-http's own budget

[tenancy.caches]
fraction  = 0.25                    # × budget: the memo registry's byte budget (graphs, snapshots, derived caches)
shed_trim = 0.50                    # at shed the registry trims by LRU to this × its budget, and stays under it while shed
idle_drop = "30m"                   # no in-flight call this long: every memo cache dropped, memory returned to the OS
```

Every derived in-memory cache — each service's symbol snapshot and call
graphs (one per service and store watermark, shared by the MCP tools and
the console), the layer-coverage paths — is a group in one registry
(`internal/memo`, RT-72..RT-75, RT-163): one build per key however many ask,
waiters that leave at their own deadline, every entry's bytes counted under
`caches.fraction`, a service's entries evicted the moment its watermark
moves. `shed` trims the registry to `shed_trim` of its budget and admits one
rebuild at a time while shed; `refuse` drops it and cancels the builds in
flight; `idle_drop` drops it. `fathom serve-mcp` bounds the same registry by
the cache fraction of the unmanaged budget (`[tenancy.unmanaged].budget`, or
`FATHOM_TENANCY_BUDGET`).

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
`fathom_gc_cpu_fraction`, `fathom_quarantined_services` and the disk-write
gauges below. Every scavenge — the 15-minute tick and each ladder event (shed
entry, every 30 s while shed, refuse entry, the exit-mark halt) — logs one
line: `scavenge (<event>): released X — total−released A → B, phys_footprint
C → D` (runtime/metrics and the process footprint; `heap_sys`, a high-water
mark that never falls, is not the instrument).

**Auto-refresh deferral.** Auto-refresh runs no inline ingest at `shed` and
above, while the daemon is paused (safe mode), during the managed daemon's
warm-up after a start (`[tenancy.breaker].warmup`, 120 s) or for a
quarantined service. The read answers from the index as it stands and its
response envelope says so: `"refresh": {"<service>": "deferred (shed) —
served from the index as it stands; freshness not checked …"}`.

**Interrupted ingests.** An ingest writes `ingest_state = in_progress` in the
store before its first write and records the watermark (with `complete`)
only at its final commit, so a killed or halted ingest never looks fresh.
Auto-refresh re-ingests a service another process left `in_progress` once
that process is gone (a running `fathom ingest` into the same store is left
alone; a pid reused by a later process counts as gone), even at the same
commit and even for a first ingest that never recorded a watermark — the row
names the corpus root — unless the service is quarantined. An ingest that
fails before its first write (Discover, fetch, or the IT-15 refusal) puts the
previous row back.

**Refresh queue (RT-97..RT-100).** Every whole-service ingest the daemon runs
— auto-refresh after drift, the `ingest` tool, the post-commit hook's `POST
/refresh-request` — is a request on one queue: one ingest per service (a request
while it runs becomes exactly one follow-up), one at a time, the most recently
queried service first. After a start, background requests wait 60 s and, during
the warm-up, start one per 30 s; they pause at `shed` and are refused at
`refuse`, in store distress, in safe mode and for a quarantined service (an
explicit `ingest` passes safe mode and a quarantine). A read never waits for an
ingest: it answers from the index and says `refresh: queued (index_stale) …`.
No setting; the knobs are the ladder's.

**Incremental ingest (RT-102..RT-108).** An ingest stats every file and reads
only those whose size, mtime or inode moved since the daemon's last ingest of the
service (`fathom ingest --rehash` reads everything), through a 64 MiB in-flight
window of file content. The typed Go pass runs only when a
`.go`, `go.mod`, `go.sum` or `go.work` file changed since its last successful
pass, the CUE pass only for `.cue`, and the TS/JS/Python import resolver only
for those files (reusing each unchanged file's recorded imports from the store's
`file_imports` table and recording a parse fault against the bytes, disclosed on
every ingest); each pass's success is recorded as a `typed-pass-state:*` row, so a
halted pass runs again. The typed pass's `go` children run with `GOFLAGS=… -p=2`,
`GOMAXPROCS=2`, `nice -n 10` and, on macOS, `taskpolicy -b`.

**Disk writes.** The sampler reads the daemon's cumulative bytes written
(rusage) and each live child's (`go list -export`, git, rg — rusage has no
children's disk-I/O field, so a child is read while it lives: its last
2 s, or a child shorter than one sample, are not seen, and the children's
figure is a floor). Both are kept per minute, split by whether an explicit
`ingest`/`onboard_service` call was in flight. The contract is ≤ 2 GiB a day
outside explicit ingest (§12.4); it is disclosed, not enforced: `/statusz`'s
tenancy block carries `disk_writes` (and `disk_writes_over`,
`children_disk_written`), `/metrics` carries
`fathom_bytes_written_total{subsystem="self"|"children"}`,
`fathom_bytes_written_24h_outside_ingest`,
`fathom_bytes_written_24h_explicit_ingest`,
`fathom_bytes_written_window_seconds` and `fathom_disk_write_budget_bytes`,
and doctor's `disk_writes` row WARNs over budget. The per-minute ring is in
memory (the resource ring keeps only the process counter), so after a start the "24h" figures cover
only `fathom_bytes_written_window_seconds`; the line and doctor name that
window ("within budget so far"). An unreadable counter is unknown, never 0.

**Admission** (`admission`, design §3; RT-53..RT-65). Every MCP tool declares a
cost class (`docs/mcp-tools.md`, "Cost classes"), and the gate admits each
`tools/call` in its class before the handler runs. Slots are derived from the
budget, not configured: at 4 GiB, `point` 64, `index` max(2, min(NumCPU−2, 8)),
`tree` 2, `fleet` 1, `ingest` 1, `egress` 2 (the startup line prints them). A
waiter queues for at most its origin's wait — a hook 100 ms for `point`/`index`
and never for `tree`/`fleet`; the console 2 s (never for `fleet`); an agent
5 s (20 s for `fleet`); a CLI 30 s — and leaves when its request ends. A
request whose remaining `X-Fathom-Deadline` is below its tool's median over the
last 15 minutes (ok, partial and abandoned calls; none until eight) is refused
at once. Each call runs under its request's context and the class deadline
(`point` 10 s, `index` 30 s, `tree` 60 s, `fleet` 120 s), so a client that
disconnects cancels it (counted abandoned), and a caller already gone is never
run (refused `caller_gone`, in every mode but `off`); an ingest runs detached,
free of the route's write deadline. A refusal's `X-Fathom-Busy-Scope` is
`class` (slots full, deadline, caller gone: the hooks' breaker stays closed) or
`daemon` (the refuse rung, safe mode: hooks back off for `Retry-After`). The refuse rung refuses `tree`, `fleet`, `ingest` and `egress`
(decision-port calls: `/route`, the obligations re-rank); safe mode refuses
`tree`, `fleet` and `egress`.

| Mode | What it does |
|---|---|
| `observe` (default until RT-96) | runs the whole decision, logs every would-refuse (`admission (observe): would refuse …`) and counts it (`fathom_would_refuse_total{class,reason,origin}`), and admits; the per-call byte meter records but never stops a producer |
| `enforce` | refuses with HTTP 503, `Retry-After` and JSON-RPC `-32050` (`fathom busy: <reason>`); a producer stopped at its per-call ceiling answers `budget: {partial: true, elided, bytes_read, reason}` (elided a lower bound), in a `batch` entry too |
| `off` | admits without deciding (class deadlines still apply); a loud startup line and a `fathom doctor` `admission_mode` WARN on every run (the escape hatch: `FATHOM_TENANCY_ADMISSION=off`) |

`GET /metrics` adds `fathom_inflight{class,origin}`, `fathom_queue{class}`,
`fathom_slots{class}`, `fathom_rejected_total{class,reason,origin}`,
`fathom_calls_total{class,outcome}`, `fathom_admission_mode` and
`fathom_test_callers_rejected_total`. The daemon's own in-process console
reads are admitted as `console-inprocess`. serve-http's HTTP server has a 5 s
header timeout, 120 s idle timeout, 64 KiB header cap and a 150 s write
deadline per route (none for `/events`), and every route requires a loopback
`Host` (and a same-origin `Origin` when sent); the hosted-modules exception
(`--modules-token-env`) was removed by SR-17. The managed daemon answers a test binary
(`X-Fathom-Caller: …;test=1`) 409.

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

**Lifecycle and the crash-loop breaker** (§2.3, §8.4, §8.5). Every start and
exit is a row in `~/.fathom/daemon/<id>/lifecycle.jsonl`; a start whose
predecessor left no exit row names it as unclean (with a jetsam or
resource-report file when one is found). Counted exits — over budget, unclean,
a port held by something else — trip the breaker: `safe_after` in
`safe_window` starts the daemon in **safe mode** (tree and fleet refused with a
named 503 `-32050`; point and index served; auto-refresh, feeders, jobs and the
decision port off), and `stop_after` in `stop_window`, or an over-budget exit
while already in safe mode, writes `disabled.json {reason: breaker}` and exits
0. An adoption, `fathom daemon stop`, a store wait, a SIGTERM drain and a
`task deploy` kickstart (its `deploy` intent row) are not counted; `fathom
daemon start` resets the window. An unmanaged daemon gets a persistent ledger
only with `serve-http --lifecycle-id <id>`. Store distress (SQLITE_FULL,
IOERR, the store volume under `min_store_volume_free`) refuses writes and
ingest and names a failed read, never a zero.

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
provider          = "none"   # none | llm | typesafe | openai — none makes no call and changes nothing
profile           = ""       # llm: the [llm] profile lane to use (default: the compile lane)
compile_threshold = 0.5      # a guide section whose "is this a binding rule" probability is below this is not compiled (counted, disclosed, still served raw)
route_confidence  = 0.9      # unchanged at this default (2026-09-24, ADR-042 amendment: route's own calibration reading failed at every cutoff); the prompt hook injects a HEDGED route line at or above the confirm floor (0.5) — never firm, whatever the confidence; nothing below 0.5
route_timeout_ms  = 3000     # POST /route's own decider budget, in ms (100-30000)
rank_within_tier  = false    # obligations re-orders rules only inside their governing-scope tier
rerank_timeout_ms = 1500     # obligations re-rank's own decider budget, in ms (100-60000)
sites = ["compile_prefilter", "obligations_rerank", "certify_suggest", "assess", "preflight"]  # the egress opt-in — this is the default (route is off: add "route" to turn it on)
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

**SR-44 (F-25, `docs/plans/surface-reckoning.md` §5) adds a deterministic
pre-check ahead of this whole calibration story.** `routeLineFor`
(`cmd/fathom/prompthook.go`) recognizes three lexically unambiguous issue
shapes — a stack trace, a failing test name, an error string
(`routeIssueShapedPrompt`) — and answers `localize` directly, with an
UNHEDGED line and no `POST /route` call at all: these shapes need no LLM
judgment, and the 45.7%-right calibration above never applies to them.
Everything else still goes through `POST /route` exactly as before.

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
suggest`, TC-24), `assess` (`fathom assess`, ADR-046 — which also needs
its own default-off `assess` capability, so listing the site is never, by
itself, consent to spend), and `preflight` (task preflight's Score, WI-18 —
asked only when `fathom plans context --preflight --score` requests it, over
measured numbers alone; never the row's text, symbol or paths). **A site absent from `sites` makes NO call at all**,
whatever `provider` is set to — this is a second, independent gate a real
provider must also clear, checked once per call site (`internal/decide.
SiteAllowed`) rather than re-derived at each one. Every failure discloses why:
`POST /route` answers its usual none-shaped body with `Error` naming the
blocked site; `fathom certify suggest` prints one line naming it and exits 0;
`fathom doctor`'s `decision_provider` row appends `(sites off: <names>)` to a
passing provider's own Reason whenever `sites` excludes any of them.
Unset (the default, shown above) means every current site except `route`:
`route` is off by default since 2026-10-01 (JF-7, judgment-foundations §6: it
failed calibration), its code and the deterministic precheck are kept, and
listing `"route"` in `sites` turns it back on. The default list is frozen: it
never grows. `sites = []` disables every site outright — a fully quiet decision
port with a provider still configured. Closed to the six site names that
exist today plus the **opt-in** names (JF-4, R2) — `triage_failure`, `ask`,
`explore`, `compact_advice` and one `judge_<name>` per judge site — for
judgments not built yet (`fathom config` load refuses any other name, the same
discipline `provider`'s own enum already applies). An opt-in site is refused
unless the repository's **own** `.fathom/config.toml` lists it: the default
never does, and a user-global `~/.fathom/config.toml` that lists it does not
opt a repository in, whatever `provider` is set to.

When a configured provider fails a call (a timeout, a 429 or 529, refuse mode
or safe mode in the daemon), the port prints exactly one line, `warn: decision
port degraded: ...`, and the asking site answers on its deterministic path, as
if no provider were set; a missing key prints one `warn: decision port
disabled: ...` line at startup and the port stays off (JF-6).

### Remote decision models: `typesafe` (Jev) and `openai`

Decision models are **optional**: `provider = "none"` stays the default, and with no
`[decide]` block every surface prints exactly what it printed before any provider
existed (I42, held by `TestI42UnconfiguredDecideOutputIsByteIdentical`). Two remote
providers sit behind the same port, each the agentic-kit's own adapter
(`foundations/decision/typesafe`, `foundations/decision/openai`):

| `provider` | `transport` | Requests go to | Default `model` | Key variable when none is named |
|---|---|---|---|---|
| `typesafe` | `typesafe` (default) | `https://api.typesafe.ai/v1/systemone` | `jev-latest` | `TYPESAFE_API_KEY` |
| `typesafe` | `openrouter` | `https://openrouter.ai/api/alpha/decisions` | `typesafe/jev-1.13` | `OPENROUTER_API_KEY` |
| `openai` | `openai` (the only one) | `https://api.openai.com/v1/decisions` | `gpt-6-luna` | `OPENAI_API_KEY` |

OpenAI's Decisions API (public beta):

```toml
# .fathom/config.toml
[decide]
provider     = "openai"
model        = "gpt-6-luna"                    # optional; the default and, today, the only model
api_key_file = "~/secrets/<provider>/token"    # a chmod 600 file holding the key; or api_key_env = "OPENAI_API_KEY"
```

Jev (TypeSafe's System One model) through OpenRouter's Decisions router, the route
for anyone without a direct TypeSafe key — one OpenRouter account covers it and the
`llm` adapter's OpenRouter lanes both:

```toml
# .fathom/config.toml
[decide]
provider     = "typesafe"
transport    = "openrouter"
model        = "typesafe/jev-1.13"             # optional; the default (a reply names the dated build that ran)
api_key_file = "~/secrets/<provider>/token"    # or api_key_env = "OPENROUTER_API_KEY"
```

`provider = "typesafe"` with no `transport` calls TypeSafe's own API exactly as it
did when it landed (2026-09-22, ADR-042 amendment; `docs/intent/futures.md` F21).
`base_url` overrides the origin for either provider (`endpoint` is its older name,
still read when `base_url` is unset); `transport` must fit the provider, and the
schema refuses `openai` over `openrouter` or `typesafe` over `openai` at load.

**The key.** Name where it is, never what it is. A literal `api_key` in `[decide]`
is refused at load with a message pointing at `api_key_file`. Precedence:
`api_key_cmd` (an argv credential helper, as in `[profiles.*]`), then
`api_key_file`, then `api_key_env`, then the transport's well-known variable in the
table above. The three named sources are one setting: the config file that names any
of them replaces all three from a lower layer, within the user config (a repository's
`.fathom/config.toml` never names them: they are ignored there, see "Where the key fields
are honored" above). `api_key_file` must be
an absolute or `~/` path to a file that grants no access to group or other users
(`chmod 600`); the adapter reads it at its first
call and never copies it anywhere, and a too-open file is refused with the
`chmod 600 <path>` that fixes it. `api_key_env` must be a variable's name. Neither
shape check repeats the value it refused, so a key pasted into the wrong field does
not reach a log line. **The launchd daemon does not see your shell's environment**:
an exported `OPENAI_API_KEY` works for a `fathom` verb run from that shell and not
for `serve-http` started by launchd — give the daemon `api_key_file`.

**Where the key may go.** A key is sent only to the provider's own host (the table
above), a loopback host (a local fake or proxy), or a host your own
`~/.fathom/config.toml` names in `base_url`. A repository's `.fathom/config.toml`
is the repository's to write, so a `base_url` there that names any other host turns
the port off with one `warn: decision port disabled:` line naming the rule, and
`fathom doctor` fails the row the same way. A non-loopback `base_url` must be
`https`, and a `base_url` carrying credentials (`https://user:secret@host`) is
refused without being repeated: name the key with `api_key_file` instead. A named
key also stays with the provider it was named for: if the file that names
`api_key_cmd`, `api_key_file` or `api_key_env` sets one provider and transport (or
none) and a later layer moves the port to another without naming its own key, the
port turns off rather than send, say, an OpenRouter key to OpenAI. Name each
provider's key in the same file as its `provider` and `transport`; the transport's
well-known variable is bound to it by name and always fits.

**Replaceable at the port, not interchangeable without calibration.** Every site
asks the same typed questions whichever provider answers, so switching providers is a
config change, not a code change. It is not a free swap: a provider's probabilities are
its own, `compile_threshold` and `route_confidence` were set against one provider's
answers, and neither OpenAI (which documents its probabilities as uncalibrated and asks
you to set thresholds from your own labelled examples) nor Jev is calibrated on your
repository until a reading says so (`docs/bench/readings/decision-port-calibration-*.md`,
ADR-056 R3). `fathom doctor`'s `decision_provider` row says so for every configured
provider.

**Cost.** Both are priced per input token with no output charge: Jev over OpenRouter
at US$0.042 per million input tokens (OpenRouter reports each call's cost in its reply;
the 2026-10-09 smoke spent US$0.00017 on eight calls, 4,074 input tokens), OpenAI's
Decisions API at US$0.10 per million input tokens (no answered call measured yet: the
smoke's account had no credit). A batch of questions shares one state, so batching
the questions a site asks about one state is cheaper than asking them one by one.
Live shapes, latency and the side-by-side answers:
`docs/bench/readings/decide/provider-smoke-2026-10-09.md`.

`fathom doctor` reports the provider under `decision_provider`: provider, model,
transport, endpoint and the key's source and state (`key file "<path>" (present, mode
0600)`, `environment variable "OPENAI_API_KEY" (set)`), never the key, after a
one-question ping through the adapter. The obligations envelope carries `ranked_by`
(`none` | `llm` | `typesafe` | `openai`), and `POST /route` on the loopback daemon
serves the prompt hook's route with the same `provider` value.

When a remote provider answers nothing — a timeout, a 429 rate limit, a quota or
credit exhausted (OpenAI's `insufficient_quota`), a 5xx, a refused key — or refuses a
question (an OpenAI `refusal` answer), the port writes one `warn: decision port
degraded (<class>): ...` line and the site answers on its deterministic path for the
whole request: a refusal is never read as an answer, a false or a zero.

Every provider is **fail-open** (I14): no key, a key file others can read, a
`base_url` the rule above refuses, or any other error opening the port at startup is
one `warn:` line on stderr and the port falls back to `none` — a misconfigured
`[decide]` never stops the daemon from starting or the CLI from compiling. `fathom
doctor` names exactly what failed (the key source, the file's mode, ping failure,
unknown provider) under `decision_provider` so the fallback is never silent to an
operator running it.

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
asked", never "asked and failed". A yes/no answer's `kind` is `predicate`
(`schema_version` 2, from 0.10.0); a row written before says `noul`, and every
reader (`fathom decide log`, the console) reads it as `predicate`
(`docs/upgrades/0.10.0-decision-kind-predicate.md`).

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
- `outbox` (optional) names a repo-relative projection file, inside the
  repository: an absolute path, a `..` escape or a symlink out is refused by
  name in the brief line. The import writes to the store `[daemon] store`
  names in `~/.fathom/config.toml` (a repository's own is ignored). When set,
  the SessionStart/SubagentStart hook runs `findings import` against it once
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

- `results_dir` (optional): a repo-relative directory (inside the
  repository: an absolute path, a `..` escape or a symlink out is refused by
  name, and the ingest warns) of attested
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

A third key, `commands`, lists the repository's own declared test commands
for the test-run hook (next section):

```toml
[tests]
commands = ["go test", "bin/itest"]   # optional; the layer that sets it replaces the list
```

## The test-run hook: `[hooks] test_run`

The test-run hook (ADR-056 decision 6, `docs/design/test-run-hook.md`) is
fathom's level 10, tiers 0 and 1, provider none. It is **off by default**
until the level 10 bench (TR-M1) reads PASS; a repository opts in:

```toml
# .fathom/config.toml
[hooks]
test_run = true   # default false
```

What it does, once on:

- It rides two Claude Code hook rows `fathom init` writes:
  `PostToolUse(Bash)` → `fathom session-hook --event test-run` and
  `PostToolUseFailure(Bash)` → `fathom session-hook --event failed-test-run`.
  A piped test run (`go test ./... | tail -30`) arrives on the first with
  no exit code; an unpiped run that exited non-zero on the second, with
  the exit code in its `error` text
  (`docs/bench/readings/test-run-hook/hook-facts.md`). A repository needs
  both: `fathom doctor` (hooks_installed) and `fathom fleet reconcile`
  check each row by its own command, so one that wires the failure row
  but not the `PostToolUse(Bash)` one, where `fathom edit-hook` shares the
  matcher, is named, and `reconcile --apply` adds it (FA-7, O175).
- It acts only on a Bash command that runs one of the repository's
  **declared** test commands: a Taskfile task, Makefile target or
  package.json script that is test-named or runs a test runner (directly,
  through a nested task or a dependency), the runner such a task invokes
  (so `go test ./one/pkg -run TestX` matches a Taskfile whose test task
  runs `go test ./...`), and every `[tests] commands` entry. Any other Bash
  call exits before a daemon call.
- Tier 0 parses plain `go test`, `go test -json` and JUnit XML into failing
  test ids, panics, file:line frames and build errors, and reads a
  three-way result: **pass** only when the runner's own summary covers
  every package the output shows and the output was neither piped through
  a filter nor cut by the harness; **fail** on any failure in the output or
  a carried non-zero exit; **unknown** otherwise. The output decides a
  failure whatever the exit status, so a piped run (exit 0, a tail of the
  output) fails from its `--- FAIL:`, `FAIL<tab>pkg` or `panic:` lines, and
  a panic whose `--- FAIL:` line fell outside the tail is named by its
  stack's test frame (TR-13).
- On a fail, tier 1 names the changed symbols each failing test reaches:
  `test_impact` over the Go functions `git diff HEAD` touches, intersected
  with the failing tests, and `localize` from the panic's stack or the
  failing test with its hops capped at 4.
- It injects **one line** (at most 700 bytes, a cut disclosed) and nothing
  on a pass or an unknown result, for example
  `🧪 fathom test run: 2 failed: TestRound (money_test.go:7), TestTotal (cart_test.go:9); both reach money.Round (changed in working tree, money/money.go:41) — full output: workspace/tmp/test-run/<hash>.log`.
  The full output is saved at that path, named by its content hash
  (`workspace/` should be gitignored in the repository, I31).
- It logs **one row per matched run** to the existing hook log
  (`~/.fathom/hooks.jsonl`, channel `test-run-hook`): the declared command,
  the event, runner, result, failing test ids (at most 10) and counts, the
  exit code when carried, and whether a line was emitted — never the
  output's text, and marked `attested: false`: a piped output is not an
  attested run and never counts as certificate evidence (I43).

What it does not do: it never rewrites the tool output
(`updatedToolOutput`), never blocks, never asks for a permission decision
and never runs a command — the harness ran the test under its own
permissions, and fathom only reads what came back (plus `git diff HEAD`, a
read that never writes the repository). It stays inside its hook's
deadline (5 s less the I53 margin): tier 1 past the deadline is dropped
silently and the row says `deadline: missed`. No model is asked anything
(provider none): the line is byte-identical whatever `[decide]` says.

## The commit pin line: `[hooks] pin_check`

The commit pin line (ADR-057 decision 4, `docs/design/served-not-kept.md`
section 3) checks each `git commit` an agent runs through its Bash tool for
a **lost pin**: a test that a governing source names (an invariant's
enforcement column in `docs/intent/invariants.md`, a `tests/*.cue`
manifest entry, a `docs/intent/scope.md` evidence line, a `// certifies:`
marker, or a guard note's `done needs:`) and that the commit deleted,
renamed, weakened, skipped, changed the golden data of, or moved the
marker off. It runs in **record** mode by default until PR-M1 (30 days of
record mode on owner traffic) passes:

```toml
# .fathom/config.toml (or ~/.fathom/config.toml)
[hooks]
pin_check = "record"   # "off" | "record" (default) | "feedback"
```

- `off`: no check and no beat.
- `record` (the default when no layer sets the key): each commit is
  checked and leaves one `pin` beat in the hook log (`~/.fathom/hooks.jsonl`,
  channel `edit-hook`): the session, the commit, the status (`kept`,
  `lost`, or `skipped` with its reason), the loss kinds, the lost test ids,
  the naming sources as `path:line`, and the stage's milliseconds. Never
  source text. The hook's output is byte-identical to `off`.
- `feedback`: `record`, plus one line per lost pin in the agent's context,
  in the same payload as anything else the call earned (Claude Code only).

What the line does:

- It rides the existing `PostToolUse(Bash)` → `fathom edit-hook` row. Only
  a command that runs `git commit` reaches it (after `cd`, `pushd` or
  `git -C`); every other Bash call still forks nothing.
- It reads the commit's sha from git's own summary line (`[main 3f9a1c2]
  …`) in the command's output and checks the commit's first parent to the
  commit: no store, no daemon call, no reach (a pin is named by its source,
  never inferred from what a test reaches). It runs whether or not the rule
  was served: a lost pin is a fact about the diff.
- One line per lost pin, at most three per commit and then `+N more at
  pre-push`, each under 240 bytes: the loss kind, the test, the source that
  names it and where to look at head, for example
  `fathom pin renamed · TestRoundHalfUp (named by invariant_enforcement docs/intent/invariants.md:5) · now TestRoundHalfUpAgain (money/half_test.go:5) in 3f9a1c2 · a -run on this name now matches no test`.
  When the same commit also edits the source that names the pin, the line
  adds `(its source was edited in this commit)`: a deliberate retirement
  stays visible without being called a violation. A test whose body was
  only `edited` is silent, a test moved unchanged to another file is kept,
  and nothing is printed when no pin is lost.
- It reads every blob through one `git cat-file --batch` process, the
  changed files through one `git diff-tree` and the parent's file list (for
  its `tests/*.cue` manifests) through one `git ls-tree`, inside a 250 ms
  sub-budget of the hook's deadline (I53). Over budget, past the deadline or on a read
  error it prints nothing and its beat says `skipped` with the reason,
  never `kept`.

What it does not do: it never blocks the commit, never offers a waiver or a
bypass, never repeats the rule's text and never describes the detector; it
opens no store and keeps no state file. A commit that prints no summary
line (`git commit -q`) is not checked: its beat says `skipped` with reason
`no_summary`, so the skip rate is measured. Harnesses other than Claude
Code stay record-only until their post-shell input and added context are
confirmed (PR-V1). The backstop is pre-push: `fathom conserve
--strict-plane integrity` blocks a silent pin loss whatever this line said
(PR-9).

## The console: `[console]`

The Observatory console bounds its own background work
(`docs/design/responsible-tenant.md` §14, RT-117/RT-118): plan verification
and `/refactors`' conserve run in one pool of two workers with a queue of
32, and each git read a page makes is stopped after 10 s.

```toml
# ~/.fathom/config.toml (or the daemon's working repo's .fathom/config.toml)
[console]
conserve_max_files = 500   # default; an integer ≥ 1
```

- `conserve_max_files` (optional, default 500): the widest range, in
  changed files, `/refactors` computes conserve for. A wider range is
  refused by name ("not computed here: the range since … has N changed
  files, over [console] conserve_max_files = …") with the verb that still
  computes it — `fathom conserve --since <ref> --service <svc>` — never a
  partial report. Read by `serve-http` at start (restart to apply).

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
re-includes that one repo-relative path. An absolute entry, a `..` escape or a
symlink out of the corpus is refused (`fathom config` names it). `include` never re-includes
`vendor`, `.git`, `testdata` or the other always-skipped names. It is refused
in the user layer (`~/.fathom/config.toml`): a load error names the file,
since a user-wide list would change nothing. Measure the
effect first with `fathom ingest --path <root> --service <id> --dry-run
--json` (stat only; `default_ignored_dirs` names each pruned directory).

`respect_gitignore = true` (RT-68, default **off** until RT-M11 has measured
every registered service) discovers a git work tree through `git ls-files -co
--exclude-standard` instead of walking it: what `.gitignore` excludes never
enters the corpus. The always-skipped names, the default cache list and the
ignore files still apply to every path git returns (a committed `vendor/` or
`testdata/` stays out), `include` also re-includes a gitignored directory,
`/refresh-file` refuses a gitignored file, and the ingest summary counts
`gitignored` entries. Compare `fathom ingest --dry-run --stats` with and
without `--respect-gitignore` before turning it on.

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
`task` target, local include and repo script the corpus invokes exists) —
plus (ADR-059) `agents_md_reaches_claude` (optional `allow_import` or `import_only`): AGENTS.md
exists and either no root `CLAUDE.md`, `.claude/CLAUDE.md` or `CLAUDE.local.md`
exists (Claude Code 2.1.277 and later loads AGENTS.md natively) or the first
present one imports `@AGENTS.md`; `allow_import: false` refuses every CLAUDE
file outside `testdata/`; `import_only: true` (ADR-059 amendment 2026-10-02)
admits a `CLAUDE.md` only when its whole content is `@AGENTS.md` beside an
AGENTS.md, requires one beside every nested AGENTS.md once a root `CLAUDE.md`
exists (Claude Code's default mode then reads CLAUDE files only), and refuses
`.claude/CLAUDE.md` and `CLAUDE.local.md` — the two switches contradict and
the loader refuses both together. The claude-code pack's
`claude_code_stanza_present` is this kind.
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
