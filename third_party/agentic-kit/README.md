# agentic-kit

[![CI](https://github.com/GeoffMillerAZ/agentic-kit/actions/workflows/ci.yml/badge.svg)](https://github.com/GeoffMillerAZ/agentic-kit/actions/workflows/ci.yml)

A layered, org-agnostic platform for **governed agentic work** — developed
almost entirely by AI agents. CUE owns the *shape* of data; Go owns
*behavior* — drift between them is a bug. The tree is **205 Go packages, zero
external dependencies** (`go.mod` has no `require` block) — a generated count,
not a hand-maintained one: see [`docs/catalog.md`](docs/catalog.md) (`task
docs-index` regenerates it, `constitution-lint`'s `catalog-fresh` check
fails the gate if the catalog drifts from source, and its `readme-count`
check fails the gate if this very sentence drifts from the catalog). The count includes the
js/wasm-only `examples/governance-wasm`, which host `go list` cannot see
(±1 vs a go-list count); TRACKING.md's progress-summary number is lower for
a different reason — it is a dated snapshot ("as of Wave 4, v0.6.0", as it
labels itself), not a live count. The whole module is gated
by `go test -race`. Not a framework: narrow foundations compose upward
into kits, runtimes, surfaces, and toolchains — anything org-specific
(sensitivity catalogs, gateways, taxonomies) is CUE config, never hard-coded.

## Layout

```
foundations/   Narrow, stdlib-only packages — the durable nouns (28 categories)
kits/          High-order workflows composed from foundations (agent-harness, mcp-server, endpoint-guard, …)
runtimes/      Policy enforcement + credential-resolution substrates
surfaces/      Human- and developer-facing interfaces (forms/cueform, editor)
toolchains/    Canonical CUE schemas, generators, materialization
internal/      Private implementation helpers shared by strict kit/command paths
cmd/           Runnable binaries (endpoint-guard sidecar, kit-catalog generator)
examples/      Small end-to-end programs (agent-cli, governance-wasm)
scenarios/     Seeded end-to-end blueprints (governed-investigation)
specs/         Docs-as-spec acceptance contracts
docs/          Explainers + the generated package catalog
workspace/     Runtime/state artifacts (untracked; wasm builds land here)
TRACKING.md    Single source of truth for build/test status
```

## What can it do today

- **Real Anthropic client** (`foundations/llmclient/anthropic`) — Messages API
  with SSE streaming — plus an exported **conformance suite**
  (`foundations/llmclient/llm-test`) that any `LLMClient` implementation runs.
- **Governed agent loop** (`kits/agent-harness`) — one `Run(ctx, input)` call:
  model turn → tool dispatch → governance screens → repeat, until a stop
  condition. `examples/agent-cli` is the runnable hello-world (mock / replay /
  live modes, same composition in all three).
- **MCP server kit** (`kits/mcp-server`) — exposes a `tool-registry` Registry
  to any MCP client (Claude Code, Claude Desktop, …) over stdio JSON-RPC 2.0.
- **Governance gauntlet** (`kits/endpoint-guard` + the `cmd/endpoint-guard`
  sidecar) — cheapest-first, fail-closed screening for a public LLM endpoint:
  circuit breaker, hashed identity, strike ledger, rate limits, topic gate,
  proof-of-work, budget caps, answer cache. Rejects cost $0 in model spend.
  See [`docs/governance.md`](docs/governance.md).
- **Record/replay testing** (`foundations/llmclient/replay`) — JSONL cassettes:
  record any live client, replay deterministically with no key and no network.
- **Evidence & provenance spine** — evidence graph, grading, spans, conflict
  and contradiction detection (`foundations/evidence`, 13 packages); hash-chained
  event store, anchors, AI provenance (`foundations/provenance`).
- **Cost + telemetry** — config-driven cost accounting
  (`foundations/cost/cost-tracker`), drop-in client instrumentation
  (`foundations/llmclient/instrument`), metrics/events/sinks
  (`foundations/telemetry`).
- **Evidence-linked assessments and prompt screening** — required criteria cannot
  be outweighed by soft scores; optional small-model screening applies configurable
  refusal standards before a protected continuation. [Offline example](examples/engineering-assessments/README.md),
  [screening configuration](kits/guardrails-pipeline/README.md); live quality is unmeasured.

## Start here

| Doc | What it is |
|-----|------------|
| [`docs/first-agent.md`](docs/first-agent.md) | **Start here:** zero to a governed agent in ~10 minutes, then one-line switch to the real Anthropic client |
| [`VISION.md`](VISION.md) | Why the kit exists — beliefs, scope, the junk gate |
| [`docs/practices.md`](docs/practices.md) | Agentic anti-patterns → the packages that prevent them |
| [`docs/orchestrating-agents.md`](docs/orchestrating-agents.md) | Running many agents: triage before dispatch, shared exploration maps (and maps as documentation), a gate that survives loaded workers, durable snapshots, stop-edit-resume, done means wired, user-level ledgers, research that ends in records, periodic consolidation of branches and worktrees |
| [`docs/serving-rules.md`](docs/serving-rules.md) | Serving rules to agents and knowing they were kept: where to put a deterministic check, facts with locations and no bypass, the pinned test as the bridge, what to refuse, replay before building |
| [`docs/measuring-tools.md`](docs/measuring-tools.md) | Grading a tool against a reference: audit the oracle, tier it and cap the grade, disclose blind spots without inflating the score, record the instrument |
| [`docs/incident-readiness.md`](docs/incident-readiness.md) | Preparing for incidents: pre-shaped declarations, "what changed" first, facets that say "not computed", links and typed parameters instead of connectors, nothing executed or mitigated, a replay before any build |
| [`docs/fast-judgment.md`](docs/fast-judgment.md) | Using cheap typed-judgment models (Jev and its kind) safely: primitives, calibration-first method, cost, privacy, anti-patterns; [use-case catalog](docs/fast-judgment-catalog.md) |
| [`docs/adopting.md`](docs/adopting.md) | Build a new project on the kit |
| [`docs/roadmap.md`](docs/roadmap.md) | Pre-thought patterns awaiting a subscriber — check here when starting a workload |
| [`docs/catalog.md`](docs/catalog.md) | Every package, one line (generated — `task docs-index`) |
| [`docs/agentic.md`](docs/agentic.md) | The agentic stack |
| [`docs/why-cue.md`](docs/why-cue.md) | CUE for engineers who've never seen CUE |
| [`docs/governance.md`](docs/governance.md) | Governing a public LLM endpoint |
| [`docs/compat.md`](docs/compat.md) | API stability classification + the compatibility promise (pre-1.0 and at 1.0) |
| [`docs/threat-model.md`](docs/threat-model.md) | Honest threat model for kit-as-dependency: what's mitigated, what's explicitly not guaranteed |
| [`AGENTS.md`](AGENTS.md) | Contributor + agent guide (hard rules, package shape, the gate); the one instruction file |
| [`TRACKING.md`](TRACKING.md) | Exhaustive build/test/validation checklist |

## Develop

```sh
task            # list tasks
task validate   # full gate: fmt-check + vet + cue-vet + go test -race
task doctor     # toolchain + workspace health
task docs-index # regenerate docs/catalog.md from package doc comments
lefthook install
```

Two-layer git hooks (see `lefthook.yml`): pre-commit runs fast changed-file
checks (gofmt, `cue vet`); pre-push runs the full `task validate`.
Never bypass the hooks or full gate (`LEFTHOOK=0` / `--no-verify`). If a hook fails,
repair the cause within the [autonomy policy](docs/intent/autonomy.md); gate edits need approval.

Also useful: `task smoke-live` (end-to-end live agent smoke; needs
`ANTHROPIC_API_KEY`, skips cleanly without it) and `task downstream-check`
(builds + tests all four real consumers — fathom (`../fathom`, or
pre-rename `../sextant` — either resolves automatically), `../throughline`,
`../growing-gardens-bi`, and `../resume_as_code`, which it re-pins and
re-vendors to kit HEAD — against kit HEAD; each skips cleanly if that sibling
isn't checked out). Run it before tagging or whenever an exported API
changes.

Requires: Go 1.26+, [CUE](https://cuelang.org) 0.16+, [go-task](https://taskfile.dev), [lefthook](https://github.com/evilmartians/lefthook).

## Status

[`TRACKING.md`](TRACKING.md) is the single source of truth: every capability
has a checkbox, and `[x]` means tests pass **and** the full gate
(`task validate`) is green.
