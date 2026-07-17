# Configuration — profiles, `~/.fathom/config.toml`, health, smoke

Canonical guide for fathom's LLM provider configuration: `config.toml` file
format, precedence, named profiles (including AWS GovCloud and Azure
Government), the secrets doctrine, and the three verbs that read it —
`fathom config`, `fathom providers`, `fathom smoke`. Design source:
`docs/plans/config-experience.md`.

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
store           = "workspace/db/fleet.duckdb"
addr            = "127.0.0.1:7575"
auto_refresh    = true
default_profile = "work"
```

This is a **read-once** load: a change to `config.toml` reaches the daemon
only after a restart —
`launchctl kickstart -k gui/$(id -u)/com.geoffmilleraz.fathom-http` — the
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
store           = "workspace/db/fleet.duckdb"
default_profile = "personal"
```

```sh
fathom config --explain                       # confirm what resolved and from where
fathom providers                              # confirm every lane's auth health, $0
fathom smoke --profile ci                     # confirm ci actually completes, small $
fathom compile --service myrepo --profile gov-bedrock --dry-run=false
```
