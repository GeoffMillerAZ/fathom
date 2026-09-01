# ADR-0007: Author every schema once, in CUE

- **Status:** Accepted
- **Date:** 2026-06-10
- **Decision drivers:** one source of truth for every contract the kit
  ships; machine-legible schemas for agents (D15); multi-target emission
  (Go types, DTCG tokens, Tailwind CSS, JSON Schema) without Node or a
  second schema language (D2); validation at build time *and* runtime.

## Context

The kit is contracts before code (see `../../VISION.md`, "What is the unit
of reuse?"). Those contracts are numerous and heterogeneous: the app
manifest (`app.cue`), config schemas, design tokens, component and pattern
metadata for the Assistant (D10, D11), JetStream KV bucket schemas and NATS
subject hierarchies (D8). Authoring each in its native ad-hoc format —
JSON Schema here, Go structs there, a tokens JSON file somewhere else —
guarantees drift, and drift is fatal to a platform whose primary readers
are AI agents validating their own output against the kit's contracts.

D2 already commits us to Go + CUE as the only languages. This ADR decides
*how far* CUE goes and *how* it connects to Go.

The state of CUE as of 2026-06-10 (all facts dated; re-verify on upgrade):

- Latest stable is **v0.16.1** (2026-04-08); v0.17.0-alpha.2 is in flight
  (2026-06-03). Cadence is roughly quarterly minors, each carrying flagged
  breaking changes (https://github.com/cue-lang/cue/releases).
- The multi-year evaluator rewrite is **done**: evalv3 default since
  v0.13.0 (May 2025), evalv2 deleted in v0.15.0 (2025-11-06), with large
  perf wins through v0.16 (up to 10x closedness wall-time, 80% faster
  typo-checking, up to 60% memory reduction on large projects)
  (https://github.com/cue-lang/cue/releases/tag/v0.16.0).
- Embedding via `cuelang.org/go` (cuecontext / `cue.Value`) is the most
  mature surface in the project; v0.16.0 improved concurrent use of
  `cue.Value` and added `cue.Unmarshaler` and `cue.IsIncomplete`
  (https://pkg.go.dev/cuelang.org/go). It requires Go 1.25+ and tracks the
  two latest Go majors — compatible with D2's Go ≥ 1.26.
- CUE→Go type generation exists as `cue exp gengotypes` (since v0.12.0,
  Jan 2025): types only, no validators; generated types accept anything
  the CUE accepts but may be more general — e.g. `string | int` becomes
  `any`. Explicitly experimental: "may be changed or removed at any time"
  (https://cuelang.org/docs/howto/generate-go-types-from-cue-definitions/,
  https://cuelang.org/docs/reference/command/cue-help-exp/).
- The `cue cmd` scripting layer's v2 redesign is an open, unresolved issue
  (#1325, last activity Oct 2025)
  (https://github.com/cue-lang/cue/issues/1325).
- The Central Registry (registry.cue.works) is still **beta** as of June
  2026, operated by CUE Labs
  (https://cuelang.org/docs/tutorial/working-with-the-central-registry/).
- Longevity signal: CUE Labs emerged from stealth 2025-10-30 with $10M+
  led by Sequoia Capital, founded by CUE's creator Marcel van Lohuizen
  (https://cue.dev/blog/announcing-cue-labs/). The project is not a
  one-maintainer hobby; it has multi-year funded velocity.
- The top community DX complaint is error diagnostics — repetitive,
  obscure "unify" messages, slow root-cause analysis (Discussion #3723,
  https://github.com/cue-lang/cue/discussions/3723).
- The W3C DTCG Design Tokens Format reached its first stable version
  2025-10-28 — plain JSON, trivially emitted from CUE
  (https://www.w3.org/community/design-tokens/2025/10/28/design-tokens-specification-reaches-first-stable-version/).
  Grafana's `cog` proves the CUE-schema→multi-language-SDK pattern at
  production scale (https://github.com/grafana/cog); nobody occupies the
  CUE-for-design-tokens niche yet.

## Decision

**Every schema in the kit is authored exactly once, in CUE.** The
inventory this binds today:

- the app manifest, `app.cue` — identity, capabilities, nav, theme;
- config schemas for the kit's services and for each app;
- design tokens, emitting DTCG 2025.10 JSON and Tailwind v4 `@theme` CSS
  (consumed by the checksum-pinned standalone binary, D2);
- component and pattern metadata for the Gallery and the Assistant
  (`0010-component-data-pipeline.md`, `0011-mcp-design-assistant.md`);
- JetStream KV bucket schemas and NATS subject hierarchies
  (`0008-embedded-nats-spine.md`).

**We write CUE in a Go-representable subset.** Rule: any definition that
gengotypes consumes must collapse to a concrete named Go type. No
load-bearing disjunctions — gengotypes degrades them to `any`, which
silently deletes type safety where we most want it. Disjunctions are
permitted only as validation detail that runtime checking enforces;
structure that Go code navigates must be plain structs, lists, and
scalars, shaped with `@go()` attributes where naming demands it.

**Codegen is a Go program, not `cue cmd`.** A small generator in the
monorepo (run via `go generate`, exposed as `appkit gen`) drives
everything through `cuelang.org/go` directly: token emission (DTCG JSON +
`@theme` CSS), JSON Schema snapshots where an external tool wants them,
manifest validation at scaffold time. Go types come from
`cue exp gengotypes`, with the `cue` binary pinned by the `go.mod` tool
directive (it is a Go program; pinning is free). We do **not** build on
`cue cmd` / `_tool.cue`: its v2 redesign is unsettled (#1325), and a Go
driver is more deterministic, debuggable, and agent-legible anyway.

**Generated output is committed.** CI regenerates and fails on diff. A
clean checkout builds without running CUE; CUE runs when contracts change.

**Runtime validation is cuecontext, embedded.** Schemas travel inside the
`appkit` Go module via `go:embed`; apps validate config and manifests at
startup with `cuecontext` (stable and concurrency-safe as of v0.16). The
generated Go types are the ergonomic surface; the CUE evaluator is the
authority. This two-layer scheme is what makes the experimental status of
gengotypes survivable: generated types can only be *too loose*, never
wrong, and the runtime check catches what the types do not.

**`cuelang.org/go` is pinned exactly — v0.16.1 today — and upgraded like
a compiler.** Read the release notes, run the full codegen pipeline, diff
every generated artifact, run the validation test suite. Never `go get -u`.

**No build dependency on the Central Registry.** CUE module dependencies,
should we ever take any, are vendored in-repo. Today the goal is zero
external CUE deps.

Boundaries: this ADR does not make CUE a runtime data format for hot-path
payloads (request bodies, bus messages decode through generated Go types);
it does not promise OpenAPI documents (CUE's encoding/openapi emits
component schemas only — issue #3133 — and we have no API consumers who
need paths); and it does not govern the SQL schema of the durable store
(`0009-sqlite-default-store.md` owns migrations).

## Alternatives considered

- **JSON Schema first.** Attractive: the universal lingua franca, every
  validator speaks it, every LLM knows it cold. Lost because it is a
  validation format, not an authoring language: no unification (the kit's
  base manifest + per-app overlay composition is exactly CUE unification),
  no computed values, no defaults-with-constraints, and no path from one
  source to Go types + tokens + CSS without writing the schema several
  times. We get JSON Schema anyway when a tool wants it — exported from
  CUE (`encoding/jsonschema.Generate`, landed v0.15.0, still experimental;
  output naming changes in v0.17, so exports are snapshotted and
  re-baselined on upgrade).
- **Go structs first + reflection/struct tags.** Attractive: zero new
  language, zero codegen, the compiler is the schema. Lost because struct
  tags cannot express real constraints (ranges, patterns, cross-field
  rules), and Go source cannot be consumed by anything that is not Go —
  design tokens must become CSS and DTCG JSON, manifests must be readable
  by agents and scaffolds without compiling the app. The direction of
  generation matters: constraints flow CUE→Go losslessly-but-loosely;
  Go→anything flows structure only.
- **Protobuf (+ protovalidate).** Attractive: industrial-grade codegen,
  decades of multi-language tooling. Lost because it is the wrong grain:
  proto is a wire-message IDL, hostile to human-edited config with
  comments, defaults, and rich constraints; validation is a bolt-on; and
  it drags in the protoc toolchain for zero wire-protocol benefit — kit
  apps speak hypermedia over loopback (D3, D5), not RPC.

The real costs of the winner, accepted with eyes open: gengotypes and the
JSON Schema encoder are experimental and will churn; quarterly minors
carry breaking changes; error diagnostics are genuinely weak and will tax
a solo developer debugging a failed unification that blocks all codegen;
CUE-for-design-tokens has no prior art, so we are first and alone there.

## Consequences

- Easier: one edit to a contract propagates to Go types, tokens, CSS, and
  validation in one `appkit gen` run; agents validate their output against
  the same schemas the runtime enforces (D15); the Assistant serves schema
  knowledge from the same files the build consumes.
- Harder: CUE upgrades are deliberate events with a regeneration-and-diff
  ritual; we budget a half-day per quarterly minor. Error-message
  archaeology is the standing tax; we mitigate with small, well-named
  definitions and `cue vet` in the dev loop, and we track Discussion
  #3723 for diagnostics improvements.
- Risk: gengotypes is removed or breaks incompatibly. Tripwire: any
  release note touching `exp gengotypes`. Committed output means we are
  never blocked — worst case we freeze the generator version and migrate
  deliberately.
- Risk: the Go-representable subset discipline erodes as schemas grow.
  Tripwire: any `any` appearing in generated output — CI greps for it and
  fails the build.
- Risk: CUE Labs' open-core trajectory degrades the OSS project.
  Tripwire: stable-release cadence stops or core features move behind CUE
  Hub. Vendored modules and a pinned compiler mean we degrade gracefully,
  not suddenly.

## Revisit when

- gengotypes graduates from `cue exp` (adopt the stable form) or is
  removed (freeze and re-plan).
- `cue cmd` v2 (#1325) ships and is demonstrably better than our Go
  driver for some pipeline step.
- CUE v1.0 lands — re-read this ADR top to bottom.
- The Central Registry leaves beta with terms acceptable for a private,
  vendor-everything monorepo.
- Error diagnostics improve enough (or worsen enough) to change the tax
  math recorded here.
