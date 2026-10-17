# Early testers

fathom is a beta (current: the [newest release](https://github.com/GeoffMillerAZ/fathom/releases)) for early adopters: usable every day on a repository
you own, gated hard on its own repository, and not yet a production service.
This page is the one place a first-time tester needs. It says what to
install, how to wire a repository, what works per language and tool format
today, what does not, and how to tell us when something breaks. Every fact
here is checked against the code or the document it links; when they
disagree, the code wins and the page is wrong, so please report it.

## Who it is for

- You use **Claude Code** or **Codex** on a repository you own, and you want
  the agent to get symbols, callers, blast radius, the rules that govern a
  file and plan verification from an index instead of from grep.
- You can build a Go program from source, and you are on **macOS** or
  **Linux**. A foreground daemon works on both; the managed daemon (launchd)
  is macOS only. Windows is untested.
- You are willing to file friction when something is wrong or confusing
  (see [How to report problems](#how-to-report-problems)).

It is not yet for gating other people's work in production, for a
fleet you cannot rebuild from source, or for anyone who needs signed or
notarized binaries.

## Install

Three ways, in this order.

### 1. Download a release archive

Each [GitHub release](https://github.com/GeoffMillerAZ/fathom/releases) carries
`fathom_<version>_<os>_<arch>.tar.gz` for darwin and linux on arm64 and amd64 (the binary,
`LICENSE.md`, `README.md`) and a `checksums.txt` (sha256). GoReleaser builds them on one
macOS runner (`.goreleaser.yaml`, `.github/workflows/release.yml`) with the same environment
as the source build below: darwin with cgo and `MACOSX_DEPLOYMENT_TARGET=14.0`, Linux pure Go.

```sh
V=$(curl -fsSL 'https://api.github.com/repos/GeoffMillerAZ/fathom/releases?per_page=1' | sed -n 's/.*"tag_name": *"v\([^"]*\)".*/\1/p' | head -n 1)   # newest, prereleases included
OS=$(uname -s | tr A-Z a-z); ARCH=$(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/')
curl -fsSLO "https://github.com/GeoffMillerAZ/fathom/releases/download/v$V/fathom_${V}_${OS}_${ARCH}.tar.gz"
curl -fsSL "https://github.com/GeoffMillerAZ/fathom/releases/download/v$V/checksums.txt" | grep "fathom_${V}_${OS}_${ARCH}.tar.gz" | shasum -a 256 -c -
mkdir -p ~/.local/bin && tar -xzf "fathom_${V}_${OS}_${ARCH}.tar.gz" -C ~/.local/bin fathom
fathom version    # fathom v0.13.1 @ <commit>
```

The binaries are **not signed or notarized**. A file downloaded through a macOS browser is
quarantined; clear it with `xattr -d com.apple.quarantine fathom`. (`curl` does not set the flag.)
On Linux the binary is static. Put it on your `PATH`: `fathom init` writes the path of the
binary that ran it into the hooks.

### 2. `go install`

```sh
go install github.com/geoffmilleraz/fathom/cmd/fathom@latest
```

macOS needs the Xcode Command Line Tools (cgo); Linux needs Go alone. It needs no
credentials, clone or `replace`: from a clean module cache it was read against v0.13.0
through the public proxy, with `go get` of `pkg/client` from a scratch module
(`docs/bench/readings/distribution/install-paths.md`).

### 3. Install from source

#### What you need

| Need | Version | Why |
|---|---|---|
| Go | 1.26.2, go.mod's `go` line | Go 1.21 or later fetches it for you through `proxy.golang.org` under `GOTOOLCHAIN=auto` (the default) |
| A C compiler and `CGO_ENABLED=1` | macOS only: Xcode Command Line Tools | the daemon's two memory readers use cgo; Linux builds pure Go with `CGO_ENABLED=0` and needs no C compiler (ADR-066) |
| `MACOSX_DEPLOYMENT_TARGET=14.0` | macOS only | the binary's minimum macOS; unset, the cgo link stamps the build machine's version |
| git | any recent | ingest and verification read git history |

Only to run fathom's own gate (contributors, not users of the binary), at the
pins in `scripts/cloud-tools.env`:

| Tool | Pin | Install |
|---|---|---|
| go-task | v3.52.0 | `go install github.com/go-task/task/v3/cmd/task@v3.52.0` |
| cue | v0.16.1 | `go install cuelang.org/go/cmd/cue@v0.16.1` |
| lefthook | v1.13.6 | `go install github.com/evilmartians/lefthook@v1.13.6` |
| rsync | any | `task stranger-check`, a step of `task validate` |
| gcc or clang | any | Linux only: `task test` runs `-race`, which needs cgo there |

#### Build

```sh
git clone https://github.com/GeoffMillerAZ/fathom.git fathom && cd fathom

# macOS
CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0 go build -o bin/fathom ./cmd/fathom
# Linux (pure Go, no C compiler)
CGO_ENABLED=0 go build -o bin/fathom ./cmd/fathom

bin/fathom version    # fathom v0.13.1 @ <the commit you built>
```

`task build` runs the same build with the same environment (`Taskfile.yml`
sets `CGO_ENABLED` to 1 on macOS and 0 on Linux, and
`MACOSX_DEPLOYMENT_TARGET=14.0`). Build to
`bin/fathom`, the path the adoption guides and `task deploy` use. The
commands below call `fathom`; put the binary you built on your `PATH`
(`fathom init` writes the path of the binary that ran it into the hooks).

**No access to the kits? Nothing to do.** fathom depends on two private
modules, agentic-kit and app-kit. `go.mod`'s `replace` lines point at the
committed mirrors in `third_party/agentic-kit` and `third_party/app-kit`, so
a fresh clone builds with no other checkout. See
[docs/distribution.md](distribution.md).

To work on fathom itself, install the gate tools above, then:

```sh
lefthook install      # pre-commit: fmt and cue; pre-push: validate and verify
task validate         # the full gate
```

## The daemon

Agents and hooks talk to one long-running `fathom serve-http` on
`127.0.0.1:7575`. Start it before you wire a repository: `fathom init`
ingests through it.

### First run, macOS or Linux: a foreground daemon

```sh
mkdir -p ~/.fathom/db
FATHOM_TENANCY_BUDGET=2GiB fathom serve-http --store ~/.fathom/db/fathom.sqlite \
  --idle-exit=0 --ttl=0 --auto-refresh=true      # 127.0.0.1:7575; keep the terminal open
```

This is the quickest way to evaluate fathom on either OS, and it needs no
signing identity. A daemon started this way is unmanaged, and its defaults
are bounded:

- **It exits within 10 seconds of its parent's death.** Run it in its own
  terminal. Started with `&` or `nohup`, it exits soon after the shell that
  started it does. To run it in the background, add
  `--exit-with-parent=false`.
- **It exits after 20 minutes idle or 8 hours** unless you pass
  `--idle-exit=0 --ttl=0`. It re-ingests on its own only with
  `--auto-refresh=true`.
- **It holds a 512 MiB memory budget** unless `FATHOM_TENANCY_BUDGET` says
  otherwise. A repository of about 10,000 files (fathom's own tree) needs
  `2GiB`. Over budget, `fathom init` exits 2 and the error says the
  typed-edge pass halted. Restart the daemon with a larger budget and run
  `init` again.

On Linux this is the supported daemon; there is no systemd unit yet.

### macOS daily use: launchd, through `task deploy`

The managed daemon runs an installed, signed copy at
`~/.local/libexec/fathom/fathom`, never `bin/fathom` itself, so a rebuild
cannot pull the binary out from under it. launchd keeps it running, and it
restarts itself when a new build is installed.

1. **Once: create a code-signing identity named `fathom-dev`** in your login
   keychain (Keychain Access, Certificate Assistant, Create a Certificate,
   type Code Signing). `task deploy` refuses to install an unsigned binary,
   because an unsigned copy loses the daemon's macOS file-access grant on
   every rebuild.
2. **`task deploy`** builds `bin/fathom`, signs it and installs it as
   `~/.local/libexec/fathom/fathom`. The first time, no launchd agent exists
   yet, so it tells you to run `fathom daemon install` and its health check
   fails. That is expected.
3. **`~/.local/libexec/fathom/fathom daemon install`** writes
   `~/Library/LaunchAgents/dev.fathom.serve-http.plist` for that installed
   copy (store `~/.fathom/db/fathom.sqlite`) and starts it.
   `fathom daemon status` must say reachable.
4. **After every later rebuild, `task deploy`, never a raw `cp`.** `cp`
   truncates a running executable, and an unsigned copy loses the grant.
   `task deploy` ends by asserting the daemon serves the commit you built.
   Run it from your own terminal, not from an agent's shell.

`fathom daemon stop` is the off switch (launchd does not relaunch it), and
`fathom resources` shows what the daemon takes even while it is down. See
[docs/usage/daemon.md](usage/daemon.md).

## Wire a repository: `fathom init`

With the daemon up, from the root of the repository you want indexed:

```sh
claude mcp add --transport http --scope user fathom http://127.0.0.1:7575/rpc/agent   # once, Claude Code
fathom init --skills
fathom doctor
```

`fathom init` is one idempotent, additive command
([docs/usage/init.md](usage/init.md)). It ingests the repository through the
daemon and confirms it indexed, wires the hooks, stamps the operating-contract
stanza into `AGENTS.md` (a root `CLAUDE.md` that exists gets one
`@AGENTS.md` import; init never creates one) and scaffolds
`~/.fathom/config.toml` if absent. It exits 0 when the daemon confirms the
repository is indexed and 2 when the wiring is done but the daemon is
unreachable or the index is empty. It never runs the interactive MCP
registration: it prints your harness's own `mcp add` line instead.

`--skills` installs the ten shipped agent skills (fathom-activate, custodian,
doctor, feedback, findings, impact, onboard, plan, rewire and verify):

| Value | Writes |
|---|---|
| bare `--skills`, or `all` | `.claude/skills`, and `.agents/skills` as a link to it |
| `claude` | `.claude/skills` only |
| `codex`, `agents` | `.claude/skills`, and `.agents/skills` as a link to it |
| `codex-prompts` | the deprecated user-global `~/.codex/prompts` twin, only when named |

`--harness` picks the agent: `claude-code` (the default), `codex`,
`gemini-cli`, `agents` or `none`. Write `--skills=claude`, not
`--skills claude`; init refuses the second form rather than guess.

**Reading init's output.** The line that matters is
`ok ingest (service "<name>" indexed: <n> symbols, <language>)`. A
TypeScript repository reports `javascript`, because one dialect reads both.
A `note:` line names a SCIP indexer that is not on your `PATH` (see
[Languages](#languages-and-tool-formats)). The `MANUAL:` lines after it are
optional, except the `claude mcp add` line if you have not registered fathom
yet:

- The lefthook snippets call `fathom` from your `PATH`. If the binary is not
  on it, write its absolute path instead.
- The provider and guard steps matter only once you want compiled rules or
  guards.

`fathom doctor` is the answer to "is it working": MCP registration, daemon
health, binary freshness, hooks, stanza, skills, config, obligations and
traffic, each with a one-command remedy. A state it cannot check is `SKIP`
with a reason, never a hopeful `PASS`. On a fresh repository it may exit 1
with these rows, none of which affects the index or the MCP tools:

| Row | Why it fails on a fresh repository | When to act |
|---|---|---|
| `lefthook_gates` | the repository has no `lefthook.yml` | only if you want the verify gate on push |
| `obligations_served` | no rules are compiled yet; compiling needs a model provider | only if you want compiled rules at the edit |
| `surface_traffic` | no tool call has reached the daemon yet | it passes after the first call |

## Cloud sessions: Claude Code on the web and Codex cloud

[docs/adoption/cloud-sessions.md](adoption/cloud-sessions.md) gets fathom
working inside a cloud session's VM. You paste a short locator script into
the environment's setup; the committed `scripts/cloud-setup.sh` provisions the
toolchain and tools at the pinned versions, builds `bin/fathom` and pre-builds
the index; and each session starts its own daemon, which fathom's MCP tools
reach through a stdio front, `fathom serve-mcp --url
http://127.0.0.1:7575/rpc/agent`. Today the setup provisions and indexes the
fathom repository itself, one repository per environment, and the readings
in a real cloud VM are still open (see [Known limits](#known-limits)).

## What works today

### Languages and tool formats

The levels below are the generated excellence board in
[docs/languages.md](languages.md#excellence-board-generated), **as of v0.11.0
(the board last regenerated at commit e32430b5), with PowerShell and Jenkins pipelines
updated for v0.12.0**. The live board moves with
every reading; read it there.

In short: **TypeScript, JavaScript, Python, Java, C#, C, C++, PHP, Rust and
Dart read L4** (typed edges), and **HTML, CSS, Bash, Ruby, Kotlin, Go, React,
Vue and Astro read L3** (file edges). Go and Kotlin read L3 against an L4
target.

**L4 outside Go needs the language's own SCIP indexer.** For those ten
languages, L4 measures how faithfully fathom imports a SCIP index:
scip-typescript (TypeScript and JavaScript), scip-python, scip-java,
scip-dotnet, scip-clang (C and C++), scip-php, rust-analyzer or scip-dart.
Run the indexer in your repository, then import its output with
`fathom ingest --scip <index.scip>`, or leave it as `index.scip` at the root
or under `.fathom/scip/`, where ingest finds it. Without an index, that
language's call edges are file-resolved heuristics at tier `pre-scan`. They
are labelled as heuristic in every answer, and some are wrong: on
psf/requests, one points `merge_setting` at `RequestsCookieJar.update` when
the call is a plain `dict.update`. Go's typed edges come from `go/types` with
no extra tool.

A level is claimed only when a committed reading proves it against an
independent oracle (ctags, a SCIP indexer, or the tool's own code): **L1**
symbols, **L2** syntax call edges, **L3** imports and file-resolved edges,
**L4** typed edges. Each met level is graded good or excellent; a reading
against an oracle below tier A (the language's or tool's own compiler,
indexer or evaluation) is capped at good. **At target** is a level met at or
above the item's target. **Refused** is built
and measured but not dispatched; **tolerated** is registered with no reading
yet; **unsupported** has no dialect and is named per service by
`list_services`, never silently skipped. Tiers come from the language
breadth program (ADR-048): T1 languages, T2 build, package and CI, T3
infrastructure as code, T4 policy as code, T5 proxies and gateways, T6
fathom's own formats, T7 observability as code. 80 more entries sit outside
the excellence target set; the board's "Support levels" section lists each.

#### T1: languages

At target, 18 of 34:

- **TypeScript**: L4, target L4 (L1 good, L2 excellent, L3 excellent, L4 excellent).
- **JavaScript**: L4, target L4 (L1 excellent, L2 excellent, L3 excellent, L4 good).
- **React components, props, hooks and React Router routes**: L3, target L3 (L1 good, L2 good, L3 good).
- **Python**: L4, target L4 (L1 excellent, L2 excellent, L3 excellent, L4 excellent); excellent through its target.
- **HTML**: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **CSS**: L3, target L3 (L1 good, L2 excellent, L3 excellent).
- **SQL**: L2, target L2 (L1 excellent, L2 good).
- **Shell / Bash**: L3, target L3 (L1 excellent, L2 good, L3 good).
- **Java**: L4, target L4 (L1 excellent, L2 excellent, L3 excellent, L4 excellent); excellent through its target.
- **C#**: L4, target L4 (L1 excellent, L2 excellent, L3 excellent, L4 excellent); excellent through its target.
- **C**: L4, target L4 (L1 excellent, L2 excellent, L3 excellent, L4 excellent); excellent through its target.
- **C++**: L4, target L4 (L1 excellent, L2 excellent, L3 good, L4 good).
- **PHP**: L4, target L4 (L1 excellent, L2 good, L3 excellent, L4 excellent).
- **Rust**: L4, target L4 (L1 excellent, L2 excellent, L3 excellent, L4 excellent); excellent through its target.
- **Ruby**: L3, target L3 (L1 excellent, L2 good, L3 good).
- **Dart**: L4, target L4 (L1 excellent, L2 excellent, L3 excellent, L4 excellent); excellent through its target.
- **Astro components**: L3, target L3 (L1 good, L2 excellent, L3 excellent).
- **Vue single-file components**: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.

Below target, with the next gap:

- **Go**: L3 (L1 excellent, L2 excellent, L3 excellent), target L4. L4 not met: def_mapping 0.5891 < 0.85.
- **PowerShell**: L3, target L3 (L1 excellent, L2 good, L3 excellent).
- **Kotlin**: L3 (L1 excellent, L2 good, L3 good), target L4. L4 not met: def_mapping not measured.
- **Lua**: L2 (L1 excellent, L2 good), target L3. L3 not met: import_resolution not measured.
- **SCSS**: L1 (L1 good), target L3. L2 not met: call_recall not measured.
- **Scala**: L2 (L1 good, L2 good), target L4. L3 not met: file_edge_recall 0.0112 < 0.25.
- **Svelte components**: none (measured), target L3. L1 not met: sym_precision 0.3816 < 0.9.

No level claimed:

- **Angular components and templates**: refused at the grammar gate; to be re-planned.
- **Swift**: refused at the grammar gate; to be re-planned.
- **Sass (indented syntax)**: unsupported; no dialect yet.
- **Tailwind CSS v4 at-rules and custom utilities**: unsupported; no dialect yet.
- **Less**: unsupported; no dialect yet.
- **Perl**: unsupported; no dialect yet.
- **Jupyter notebooks**, tool format: unsupported; no dialect yet.
- **Laravel Blade templates**: unsupported; no dialect yet.
- **Jinja2 templates**: unsupported; no dialect yet.

#### T2: build, package and CI

At target, 14 of 23:

- **GitHub Actions workflows and composite actions**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 good).
- **npm/pnpm/yarn package manifests**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **Python packaging**, tool format: L2, target L2 (L1 good, L2 excellent).
- **Makefile / GNU make**, tool format: L2, target L2 (L1 excellent, L2 excellent); excellent through its target.
- **GitLab CI**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **Maven POMs**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **Cargo manifests**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **Go modules**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **MSBuild projects**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **Composer manifests**, tool format: L2, target L2 (L1 excellent, L2 excellent); excellent through its target.
- **Protocol Buffers**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **Taskfile**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **vcpkg manifests**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **Clang compilation databases**, tool format: L2, target L2 (L1 excellent, L2 excellent); excellent through its target.

Below target, with the next gap:

- **CMake**, tool format: none (measured), target L3. L1 not met: tenancy floor.
- **Jenkins pipelines**, tool format: L3, target L3 (L1 good, L2 good, L3 good).
- **Conan recipes and lockfiles**, tool format: L2 (L1 excellent, L2 excellent), target L3. L3 not met: xref_precision not measured.
- **Meson builds**, tool format: L2 (L1 excellent, L2 excellent), target L3. L3 not met: xref_precision not measured.

No level claimed:

- **Bazel / Starlark**, tool format: unsupported; no dialect yet.
- **GraphQL schemas and operations**, tool format: tolerated; no reading yet.
- **JSON Schema documents**, tool format: unsupported; no dialect yet.
- **Azure Pipelines**, tool format: unsupported; no dialect yet.
- **CircleCI**, tool format: unsupported; no dialect yet.

#### T3: infrastructure as code

At target, 15 of 16:

- **Kubernetes manifests**, tool format: L3, target L3 (L1 excellent, L2 good, L3 good).
- **Helm charts**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 good).
- **Kustomize**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **Argo CD Applications and Argo Workflows**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **Ansible playbooks, roles and inventories**, tool format: L3, target L3 (L1 good, L2 good, L3 good).
- **AWS CloudFormation / SAM templates**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 good).
- **Terragrunt**, tool format: L3, target L3 (L1 good, L2 good, L3 excellent).
- **HashiCorp HCL tool configs**, tool format: L2, target L2 (L1 good, L2 good).
- **Pulumi YAML programs and project files**, tool format: L2, target L2 (L1 excellent, L2 excellent); excellent through its target.
- **Crossplane compositions and XRDs**, tool format: L3, target L3 (L1 excellent, L2 good, L3 good).
- **Serverless Framework**, tool format: L2, target L2 (L1 excellent, L2 excellent); excellent through its target.
- **Azure Bicep**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **Azure Resource Manager templates**, tool format: L3, target L3 (L1 excellent, L2 good, L3 excellent).
- **AWS CDK apps**, tool format: L2, target L2 (L1 excellent, L2 good).
- **Flux CD resources**, tool format: L3, target L3 (L1 excellent, L2 good, L3 good).

Below target, with the next gap:

- **Timoni modules and bundles**, tool format: L1 (L1 good), target L3. L2 not met: ref_precision 0.1611 < 0.9.

#### T4: policy as code

At target, 6 of 10:

- **Rego / OPA policy**, language: L3, target L3 (L1 excellent, L2 excellent, L3 good).
- **Kyverno policies**, tool format: L3, target L2 (L1 excellent, L2 excellent, L3 good); excellent through its target.
- **OPA Gatekeeper ConstraintTemplates and constraints**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **AWS IAM policy documents**, tool format: L2, target L2 (L1 excellent, L2 excellent); excellent through its target.
- **Azure Policy definitions and initiatives**, tool format: L2, target L2 (L1 good, L2 good).
- **Cloud Custodian policies**, tool format: L2, target L2 (L1 excellent, L2 excellent); excellent through its target.

No level claimed:

- **Conftest policies**, tool format: unsupported; no dialect yet.
- **Cedar policies and schemas**, language: refused at the grammar gate; to be re-planned.
- **Checkov custom policies**, tool format: unsupported; no dialect yet.
- **HashiCorp Sentinel policies**, language: unsupported; no dialect yet.

#### T5: proxies and gateways

At target, 5 of 8:

- **nginx configuration**, tool format: L3, target L3 (L1 excellent, L2 good, L3 good).
- **HAProxy configuration**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **Caddyfile**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **Envoy proxy configuration**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 good).
- **Kong declarative configuration**, tool format: L3, target L3 (L1 excellent, L2 good, L3 excellent).

Below target, with the next gap:

- **Traefik static and file-provider configuration**, tool format: L2 (L1 excellent, L2 excellent), target L3. L3 not met: xref_precision 0.7241 < 0.9.

No level claimed:

- **PgBouncer configuration**, tool format: unsupported; no dialect yet.
- **ProxySQL configuration**, tool format: unsupported; no dialect yet.

#### T6: fathom's own formats

At target, 2 of 5:

- **fathom configuration**, tool format: L2, target L2 (L1 good, L2 good).
- **fathom plan grammar v2 rows**, tool format: L3, target L3 (L1 good, L2 good, L3 good).

Below target, with the next gap:

- **CUE**, language: L2 (L1 excellent, L2 excellent), target L4. L3 not met: import_resolution not measured.

No level claimed:

- **fathom concerns**, tool format: unsupported; no dialect yet.
- **fathom layer packs**, tool format: unsupported; no dialect yet.

#### T7: observability as code

At target, 4 of 4:

- **Prometheus rules, configuration and promtool tests**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **Alertmanager configuration**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **Grafana dashboards and provisioning**, tool format: L3, target L3 (L1 good, L2 good, L3 good).
- **OpenTelemetry Collector configuration**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 good).

### Verification and receipts

- **`fathom verify`** proves a plan's claimed rows against the index and
  exits 0, 1 or 2 ([docs/usage/verify.md](usage/verify.md)). From v0.11.0,
  `reachable_from` and `wired` pass only over typed call edges: a row reached
  only through a heuristic edge fails and names that edge
  ([migration](upgrades/0.11.0-typed-only-reach.md)).
- **Receipts.** Every SARIF fathom writes (`verify --format sarif`,
  `obligations`, `review --receipt`, `intent audit` and more) names the
  commit it was produced at and fingerprints each cited excerpt, so a reader
  checks it with git alone. `fathom receipt verify` re-derives a saved
  answer from a bare checkout, offline
  ([docs/usage/receipt.md](usage/receipt.md)).
- **`localize`** takes a pasted stack trace, error payload or log line to the
  code that wrote it, in Python, Java, JavaScript, TypeScript, PHP and Go.

### Cloud sessions

Cloud sessions work as [described above](#cloud-sessions-claude-code-on-the-web-and-codex-cloud):
`scripts/cloud-setup.sh` provisions the tools, builds `bin/fathom` and
pre-builds the index, and each session starts its own daemon. Linux builds
are pure Go (`CGO_ENABLED=0`). The readings in a real cloud VM are still
open.

## Known limits

- **A repository can open a paid decision provider on your key** (O1113,
  fixed by `docs/plans/decision-port.md` KS-1 to KS-7, not yet built). A
  committed `.fathom/config.toml` that sets `[decide] provider` wins over
  your own config today, and with no key source named fathom reads the
  transport's well-known variable (`TYPESAFE_API_KEY`, `OPENROUTER_API_KEY`
  or `OPENAI_API_KEY`) from your environment. Until those rows land, read
  the `[decide]` block of a cloned repository's `.fathom/config.toml` before
  running fathom there, or do not export those variables in that shell;
  `fathom doctor`'s `decision_provider` row names the provider that is open.
- **The managed daemon is macOS only.** `task deploy` needs codesign, a local
  signing identity and launchd. On Linux the daemon runs in the foreground,
  and there is no systemd unit yet.
- **Cloud sessions are unproven in a real cloud VM.** Three readings are
  still open: a real `claude --cloud` session, `task validate` inside one,
  and a Codex cloud task. What has been measured is an `ubuntu:24.04`
  container of the VM's shape, and the setup indexes only the fathom
  repository itself.
- **The prebuilt binaries are not signed or notarized** (see
  [Install](#1-download-a-release-archive) for the quarantine flag).
- **Typed edges outside Go need a SCIP index** (see
  [Languages and tool formats](#languages-and-tool-formats)). Without one,
  callers and blast radius for that language are heuristic and labelled so.
- **`fathom verify` checks reach and wiring through Go's typed pass only.**
  A row in another language proves its reach with `test_reaches`, or stays
  unchecked ([migration](upgrades/0.11.0-typed-only-reach.md)).
- **The MCP server reports version `1.0.0`** in its `serverInfo`, not the
  binary's version. `fathom version` is authoritative.
- **Model calls are opt-in, and so is their cost.** With the decision port
  off (`[decide] provider = "none"`, the default) fathom makes no model call,
  and everything on this page works that way. The semantic-layer compile
  (`fathom compile`) only estimates its cost unless you pass
  `--dry-run=false` with a configured provider profile.
- **Some web and shell languages are below target or unread.** Svelte
  components read no level (L1 unmet: a component's `<script>` is read as
  TypeScript with `lang="ts"`, else JavaScript, and either dialect names a
  `let` binding such as `$state` or `$props` a constant). Angular templates are
  refused at the grammar gate and stay HTML. zsh scripts are not read,
  because no zsh grammar parses them well enough.
- **The daemon's seven-day footprint reading is not yet in.**
  `fathom resources --days 7` prints its verdict on your machine. On the
  author's machine, the first full window closes 2026-10-19.
- **Language levels are readings, not coverage.** A level is what a committed
  reading measured on its named subjects; your repository may read lower.

## How to report problems

- **`fathom observe`** files a structured friction entry in the repository
  you ran it in, as its own file under `.fathom/observations/`:

  ```sh
  fathom observe --symptom "what went wrong" --expected "what you expected" \
    --actual "what happened" --repro "the exact command" --tool <tool or verb>
  ```

  Flags: [docs/usage/observe.md](usage/observe.md). Send us the entry, or
  commit it if you are working in fathom's own repository.
- **The repository's issue tracker** for anything you would rather discuss:
  include `fathom version`, your OS, and `fathom doctor`'s output.

## Upgrading

```sh
git pull && task build          # or the go build line above
task deploy                     # macOS: install the new binary and assert the daemon serves it
                                # Linux: stop the foreground serve-http (Ctrl-C) and start it again
fathom init --skills            # in each wired repository: refreshes skills behind the shipped source
fathom doctor                   # names stale hooks, skills and wiring, each with its remedy
```

`fathom init` is idempotent: run again, it rewrites only what drifted from the
shipped source and leaves a region you edited by hand untouched (it reports
the file as blocked). Read the `### Breaking` lines of `CHANGELOG.md` and
their migration notes under `docs/upgrades/` before you upgrade across a
release.

## How much to trust it

Every push the maintainer makes runs the full gate as a lefthook pre-push
hook, and CI runs the same `task validate` steps on Linux for every push and
pull request (`.github/workflows/ci.yml`). `fathom verify --strict`
re-proves every claimed plan row against a fresh index of the pushed tree;
`task stranger-check` copies the tracked tree, builds it with `GOWORK=off`
and runs `fathom init` and `fathom doctor` against a throwaway daemon and a
temp HOME (`scripts/stranger-check.sh`); and `task validate` runs the full
`go test -race` suite. That makes fathom dependable on its own repository and
beta grade elsewhere: expect rough edges on a repository unlike
fathom's, and expect the levels above to move. It is not production software.
