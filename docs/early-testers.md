# Early testers

fathom v0.10.0 is early-tester grade: usable every day on a repository you
own, gated hard on its own repository, and not yet a production service.
This page is the one place a first-time tester needs. It says what to
install, how to wire a repository, what works per language and tool format
today, what does not, and how to tell us when something breaks. Every fact
here is checked against the code or the document it links; when they
disagree, the code wins and the page is wrong, so please report it.

## Who it is for

- You use **Claude Code** or **Codex** on a repository you own, and you want
  the agent to get symbols, callers, blast radius, the rules that govern a
  file and plan verification from an index instead of from grep.
- You can build a Go program from source with CGO, and you are on **macOS**
  (the supported daemon) or **Linux** (the daemon runs in the foreground).
  Windows is untested.
- You are willing to file friction when something is wrong or confusing
  (see [How to report problems](#how-to-report-problems)).

It is not yet for gating other people's work in production, for a
fleet you cannot rebuild from source, or for anyone who needs prebuilt
binaries.

## Install from source

### What you need

| Need | Version | Why |
|---|---|---|
| Go | 1.26.2, go.mod's `go` line | Go 1.21 or later fetches it for you through `proxy.golang.org` under `GOTOOLCHAIN=auto` (the default) |
| A C compiler and `CGO_ENABLED=1` | Xcode Command Line Tools on macOS; gcc or clang on Linux | the tree-sitter extractor bindings use CGO (ADR-013); the store is pure Go |
| `MACOSX_DEPLOYMENT_TARGET=14.0` | macOS only | matches the tree-sitter objects (ADR-013); Linux ignores it |
| git | any recent | ingest and verification read git history |

Only to run fathom's own gate (contributors, not users of the binary), at the
pins in `scripts/cloud-tools.env`:

| Tool | Pin | Install |
|---|---|---|
| go-task | v3.52.0 | `go install github.com/go-task/task/v3/cmd/task@v3.52.0` |
| cue | v0.16.1 | `go install cuelang.org/go/cmd/cue@v0.16.1` |
| lefthook | v1.13.6 | `go install github.com/evilmartians/lefthook@v1.13.6` |
| rsync | any | `task stranger-check`, a step of `task validate` |

### Build

```sh
git clone <the fathom repository> fathom && cd fathom

# macOS
CGO_ENABLED=1 MACOSX_DEPLOYMENT_TARGET=14.0 go build -o bin/fathom ./cmd/fathom
# Linux
CGO_ENABLED=1 go build -o bin/fathom ./cmd/fathom

bin/fathom version    # fathom v0.10.0 @ <the commit you built>
```

`task build` runs the same build with the same environment (`Taskfile.yml`
sets `CGO_ENABLED=1` and `MACOSX_DEPLOYMENT_TARGET=14.0`). Build to
`bin/fathom`, the path the adoption guides and `task deploy` use. The
commands below call `fathom`; put the binary you built on your `PATH`
(`fathom init` writes the path of the binary that ran it into the hooks).

**No access to agentic-kit? Nothing to do.** fathom depends on a private
module, and `go.mod`'s `replace` falls back to the committed mirror in
`third_party/agentic-kit` when no `go.work` and no sibling checkout exist,
which is the case in a fresh clone. See
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

### macOS: launchd, through `task deploy`

The daemon runs an installed, signed copy at
`~/.local/libexec/fathom/fathom`, never `bin/fathom` itself, so a rebuild
cannot pull the binary out from under it.

1. **Once: create a code-signing identity named `fathom-dev`** in your login
   keychain (Keychain Access, Certificate Assistant, Create a Certificate,
   type Code Signing). `task deploy` refuses to install an unsigned binary,
   because an unsigned copy loses the daemon's macOS file-access grant on
   every rebuild.
2. **`task deploy`** builds `bin/fathom`, signs it and installs it as
   `~/.local/libexec/fathom/fathom`. The first time, no launchd agent exists
   yet: it prints `no launchd agent installed — run: fathom daemon install`
   and its health check then fails. That is expected.
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

### Linux: a foreground `fathom serve-http`

```sh
mkdir -p ~/.fathom/db
fathom serve-http --store ~/.fathom/db/fathom.sqlite \
  --idle-exit=0 --ttl=0 --auto-refresh=true      # 127.0.0.1:7575; keep the terminal open
```

Without a supervisor the daemon is unmanaged and bounded by default: it exits
within 10 seconds of its parent's death, after 20 minutes with no request in
flight, or after 8 hours, does not re-ingest on its own, and holds a 512 MiB
memory budget. The flags above lift the idle and lifetime bounds and turn
re-ingest on, as the cloud session script does; raise the budget with
`FATHOM_TENANCY_BUDGET` (for example `FATHOM_TENANCY_BUDGET=2GiB`) for a large
repository. **There is no systemd unit yet** (RT-D8, deferred until a Linux
adopter runs the managed daemon; `docs/plans/responsible-tenant.md`).

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

`fathom doctor` is the answer to "is it working": MCP registration, daemon
health, binary freshness, hooks, stanza, skills, config, obligations and
traffic, each with a one-command remedy. A state it cannot check is `SKIP`
with a reason, never a hopeful `PASS`.

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

The levels below are the generated excellence board in
[docs/languages.md](languages.md#excellence-board-generated), **as of v0.10.0
(commit a5448438)**. The live board moves with every reading; read it there.

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
fathom's own formats. 89 more entries sit outside the excellence target set;
the board's "Support levels" section lists each.

### T1: languages

At target, 7 of 20:

- **Python**: L4, target L4 (L1 excellent, L2 excellent, L3 excellent, L4 good).
- **SQL**: L2, target L2 (L1 good, L2 good).
- **Shell / Bash**: L2, target L2 (L1 good, L2 good).
- **PHP**: L4, target L4 (L1 excellent, L2 good, L3 excellent, L4 excellent).
- **Rust**: L4, target L4 (L1 excellent, L2 excellent, L3 good, L4 good).
- **Ruby**: L3, target L3 (L1 excellent, L2 good, L3 good).
- **Dart**: L4, target L4 (L1 excellent, L2 excellent, L3 excellent, L4 excellent); excellent through its target.

Below target, with the next gap:

- **TypeScript**: L3 (L1 excellent, L2 excellent, L3 good), target L4. L4 not met: def_mapping 0.2706 < 0.85.
- **JavaScript**: L3 (L1 good, L2 excellent, L3 good), target L4. L4 not met: def_mapping 0.3591 < 0.85.
- **HTML**: L1 (L1 good), target L3. L2 not met: call_recall not measured.
- **CSS**: none (measured), target L3. L1 not met: sym_recall 0.1423 < 0.85.
- **Java**: L3 (L1 excellent, L2 excellent, L3 excellent), target L4. L4 not met: typed_recall 0.669 < 0.85.
- **C#**: L1 (L1 excellent), target L4. L2 not met: call_precision 0.794 < 0.85.
- **C**: L3 (L1 excellent, L2 excellent, L3 good), target L4. L4 not met: def_mapping 0.3425 < 0.85.
- **C++**: L2 (L1 good, L2 excellent), target L4. L3 not met: file_edge_precision 0.8639 < 0.9.
- **Go**: L2 (L1 excellent, L2 excellent), target L4. L3 not met: file_edge_precision 0.5625 < 0.9.
- **Kotlin**: L2 (L1 excellent, L2 good), target L4. L3 not met: file_edge_precision 0.7993 < 0.9.

No level claimed:

- **Swift**: refused; re-plan (LB-Q-swift-rest refused).
- **SCSS / Sass**: unsupported; no dialect (LB-Q-scss).
- **Scala**: tolerated; no reading (LB-Q-scala).

### T2: build, package and CI

At target, 13 of 23:

- **GitHub Actions workflows and composite actions**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 good).
- **npm/pnpm/yarn package manifests**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **Python packaging**, tool format: L2, target L2 (L1 good, L2 excellent).
- **Makefile / GNU make**, tool format: L2, target L2 (L1 excellent, L2 excellent); excellent through its target.
- **GitLab CI**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **Maven POMs**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **Cargo manifests**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **Go modules**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **MSBuild projects**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **Protocol Buffers**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **Taskfile**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **vcpkg manifests**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **Clang compilation databases**, tool format: L2, target L2 (L1 excellent, L2 excellent); excellent through its target.

Below target, with the next gap:

- **CMake**, tool format: none (measured), target L3. L1 not met: tenancy floor.
- **Conan recipes and lockfiles**, tool format: L2 (L1 excellent, L2 excellent), target L3. L3 not met: xref_precision not measured.
- **Meson builds**, tool format: L2 (L1 excellent, L2 excellent), target L3. L3 not met: xref_precision not measured.

No level claimed:

- **Composer manifests**, tool format: unsupported; no dialect (LB-Q-composer).
- **Bazel / Starlark**, tool format: unsupported; no dialect (LB-Q-bazel).
- **GraphQL schemas and operations**, tool format: tolerated; no reading (LB-Q-graphql).
- **JSON Schema documents**, tool format: unsupported; no dialect (LB-Q-json-schema).
- **Jenkins pipelines**, tool format: unsupported; no dialect (LB-Q-jenkinsfile).
- **Azure Pipelines**, tool format: unsupported; no dialect (LB-Q-azure-pipelines).
- **CircleCI**, tool format: unsupported; no dialect (LB-Q-circleci).

### T3: infrastructure as code

At target, 14 of 16:

- **Kubernetes manifests**, tool format: L3, target L3 (L1 excellent, L2 good, L3 good).
- **Helm charts**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 good).
- **Kustomize**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **Argo CD Applications and Argo Workflows**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **Ansible playbooks, roles and inventories**, tool format: L3, target L3 (L1 good, L2 good, L3 good).
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

- **AWS CloudFormation / SAM templates**, tool format: L2 (L1 excellent, L2 excellent), target L3. L3 not met: xref_precision not measured.
- **Timoni modules and bundles**, tool format: L1 (L1 good), target L3. L2 not met: ref_precision 0.1611 < 0.9.

### T4: policy as code

At target, 6 of 10:

- **Rego / OPA policy**, language: L3, target L3 (L1 excellent, L2 excellent, L3 good).
- **Kyverno policies**, tool format: L3, target L2 (L1 excellent, L2 excellent, L3 good); excellent through its target.
- **OPA Gatekeeper ConstraintTemplates and constraints**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **AWS IAM policy documents**, tool format: L2, target L2 (L1 excellent, L2 excellent); excellent through its target.
- **Azure Policy definitions and initiatives**, tool format: L2, target L2 (L1 good, L2 good).
- **Cloud Custodian policies**, tool format: L2, target L2 (L1 excellent, L2 excellent); excellent through its target.

No level claimed:

- **Conftest policies**, tool format: unsupported; no dialect (LB-Q-conftest).
- **Cedar policies and schemas**, language: refused; re-plan (LB-Q-cedar-rest refused).
- **Checkov custom policies**, tool format: unsupported; no dialect (LB-Q-checkov-policies).
- **HashiCorp Sentinel policies**, language: unsupported; no dialect (LB-Q-sentinel).

### T5: proxies and gateways

At target, 5 of 8:

- **nginx configuration**, tool format: L3, target L3 (L1 excellent, L2 good, L3 good).
- **HAProxy configuration**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **Caddyfile**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 excellent); excellent through its target.
- **Envoy proxy configuration**, tool format: L3, target L3 (L1 excellent, L2 excellent, L3 good).
- **Kong declarative configuration**, tool format: L3, target L3 (L1 excellent, L2 good, L3 excellent).

Below target, with the next gap:

- **Traefik static and file-provider configuration**, tool format: L2 (L1 excellent, L2 excellent), target L3. L3 not met: xref_precision 0.7241 < 0.9.

No level claimed:

- **PgBouncer configuration**, tool format: unsupported; no dialect (LB-Q-pgbouncer).
- **ProxySQL configuration**, tool format: unsupported; no dialect (LB-Q-proxysql).

### T6: fathom's own formats

At target, 2 of 5:

- **fathom configuration**, tool format: L2, target L2 (L1 good, L2 good).
- **fathom plan grammar v2 rows**, tool format: L3, target L3 (L1 good, L2 good, L3 good).

Below target, with the next gap:

- **CUE**, language: L2 (L1 excellent, L2 excellent), target L4. L3 not met: import_resolution not measured.

No level claimed:

- **fathom concerns**, tool format: unsupported; no dialect (LB-Q-fathom-concerns).
- **fathom layer packs**, tool format: unsupported; no dialect (LB-Q-fathom-layer-packs).

## Known limits

- **The daemon deploys on macOS only.** `task deploy` needs codesign and
  launchd; on Linux the daemon runs in the foreground, with no systemd unit
  yet (RT-D8).
- **Cloud sessions are unproven in a real cloud VM.** The three owner
  readings, a real `claude --cloud` session, `task validate` inside one, and
  a Codex cloud task (CS-M1..CS-M3, `docs/plans/cloud-sessions.md`), are open;
  what is measured is an `ubuntu:24.04` container of the VM's shape.
- **No prebuilt binaries.** You build from source; the repository has no CI
  workflow that publishes a release.
- **Model calls are opt-in, and so is their cost.** With the decision port
  off (`[decide] provider = "none"`, the default) fathom makes no model call,
  and everything on this page works that way. The semantic-layer compile
  (`fathom compile`) only estimates its cost unless you pass
  `--dry-run=false` with a configured provider profile.
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

Every push to `main` runs the pre-push gate. `fathom verify --strict`
re-proves every claimed plan row against a fresh index of the pushed tree;
`task stranger-check` copies the tracked tree, builds it with `GOWORK=off`
and runs `fathom init` and `fathom doctor` against a throwaway daemon and a
temp HOME (`scripts/stranger-check.sh`); and `task validate` runs the full
`go test -race` suite. That makes fathom dependable on its own repository and
early-tester grade elsewhere: expect rough edges on a repository unlike
ours, and expect the levels above to move. It is not production software.
