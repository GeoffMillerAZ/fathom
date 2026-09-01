# ADR-0002: Go and CUE are the only languages; no Node, ever

- **Status:** Superseded in part by [ADR-0017](0017-multi-target-layered-kit.md)
  (2026-08-07) — §3's first clause only, struck in place below. Everything
  else in this ADR stands and continues to govern.
- **Date:** 2026-06-10
- **Decision drivers:** deterministic clean-machine builds; compile-time
  provability over runtime discovery; one owner maintaining everything for
  decades; agent legibility (one language for agents to be expert in)

## Context

Every kit app is built and maintained by one person plus AI agents, on a
multi-decade horizon. Every language in the stack is a permanent tax: a
second toolchain to pin, a second dependency graph to audit, a second
ecosystem whose churn we absorb, a second mental model agents must hold.
The enterprise default — TypeScript frontend, Node toolchain, npm dependency
graph — optimizes for team scale and hiring, neither of which exists here.

The facts that make a no-Node stack practical in June 2026, where it was
painful in 2020:

- **CSS without Node.** Tailwind CSS v4.3.0 (released 2026-05-08) ships a
  standalone, self-contained per-platform executable that bundles Lightning
  CSS — no Node, no npm
  (https://github.com/tailwindlabs/tailwindcss/releases). Configuration
  moved into the input CSS file itself: `@import "tailwindcss"; @source
  "./**/*.templ";`, then `./tailwindcss -i input.css -o output.css
  --minify` (https://tailwindcss.com/docs/installation/tailwind-cli).
  Plugins like daisyUI v5 load as CSS `@plugin` directives and work with
  the standalone binary (https://daisyui.com/pages/tailwind-css-without-node/).
- **Reactivity without a JS build.** Datastar v1.0.2 (2026-06-02) is a
  single ~34KB JS artifact (11.76 KiB brotli, measured from the official
  v1.0.2 bundle; https://github.com/starfederation/datastar/releases) with
  a declarative attribute contract. It is consumed, never compiled. See
  [0003-hypermedia-first-datastar.md](0003-hypermedia-first-datastar.md).
- **Schemas without YAML/JSON sprawl.** CUE v0.16.1 (2026-04-08) embeds
  directly in Go via `cuelang.org/go`, and the project is funded — CUE Labs
  emerged from stealth 2025-10-30 with $10M+ led by Sequoia
  (https://cue.dev/blog/announcing-cue-labs/). See
  [0007-cue-single-source-of-truth.md](0007-cue-single-source-of-truth.md).
- **Templates without a runtime.** templ compiles to plain Go; see
  [0004-templ-templates.md](0004-templ-templates.md).

One constraint flows back into Go itself: `cuelang.org/go` v0.16.x requires
Go 1.25+ and tracks the two latest Go majors, so embedding CUE commits us to
staying current on Go toolchains. We are on Go ≥1.26 and accept that.

One known hazard: Tailwind v4 point releases (v4.2.0 2026-02-18, v4.3.0
2026-05-08) have added and changed utilities quickly, so an unpinned
`tailwindcss` binary makes builds non-reproducible.

## Decision

**All logic is written in Go. All schemas are written in CUE. Nothing else
is a source language in this repo or any kit app.**

Precisely:

1. **Go is the only general-purpose language.** Application logic, build
   tools, codegen drivers, the data pipeline, the Assistant, CI helpers,
   scripts that would conventionally be bash-heavy — all Go. Where a shell
   one-liner suffices it may live in a Makefile/Taskfile, but anything with
   logic, parsing, or error handling becomes a `cmd/` Go program.
2. **CUE is the only schema/config language.** App manifests (`app.cue`),
   config schemas, design tokens, component metadata, subject hierarchies —
   per [0007-cue-single-source-of-truth.md](0007-cue-single-source-of-truth.md).
   CUE upgrades are treated like compiler upgrades: pinned exactly
   (v0.16.1 today), bumped deliberately with release notes read.
3. ~~**JavaScript is never written as source and never built by a
   toolchain.**~~ **Retired in place by
   [ADR-0017](0017-multi-target-layered-kit.md) §3 — the first clause was
   already false when written down.** The replacement rule, the half that
   still stands, and the third-party half are stated below under
   [§3 amendment — kit-authored browser JavaScript (ADR-0017)](#3-amendment--kit-authored-browser-javascript-adr-0017).
4. **CSS is produced by the Tailwind v4.3 standalone binary, pinned by
   SHA-256 per platform.** A Go tool (`cmd/appkit tools`) downloads the
   per-platform executables and verifies them against checksums committed
   in the repo; CI fails on mismatch. The binary is invoked directly —
   there is no PostCSS, no `tailwind.config.js`, no Node resolution of
   plugins. daisyUI-style CSS `@plugin`s are permitted because they are
   CSS files, not npm packages.
5. **No Node anywhere means anywhere:** not in dev tooling, not in CI, not
   in docs generation, not in test harnesses, not in "just this once"
   scripts. `package.json` appearing in a kit repo is a build failure by
   policy. The single sanctioned exception: Playwright, inside the pinned
   visual-regression CI container per
   [0012](0012-gallery-living-style-guide.md); its config lives only under
   `apps/gallery/visreg/`, and the `package.json` build-failure check
   excludes exactly that path — nothing else, nowhere else.

**The determinism payoff this buys:** a kit app builds on a clean machine
with two commands — fetch-and-verify pinned tools, then `go build` — and
produces the same bits. Everything that runs is either the Go toolchain
(versioned in `go.mod`), a `go.mod` tool-directive binary, or a
SHA-256-pinned executable. There is no `npm install` step whose output
depends on the day you ran it, no lockfile drift, no transitive
`node_modules` graph to audit. Compile-time provability follows: with all
logic in Go and templates compiled to Go
([0004](0004-templ-templates.md)), "does the UI reference a field that
exists" is a compiler question, not a runtime discovery.

**Boundaries.** This decision does not forbid *consuming* artifacts that
were originally built with Node upstream (datastar.js itself is). It
forbids us running that toolchain. It also does not cover Rust/C artifacts
we never touch (Lightning CSS lives inside the Tailwind binary; SQLite
arrives as WASM via wazero per [0009](0009-sqlite-default-store.md)) —
those are pinned binaries/artifacts, same policy as the Tailwind CLI.

## §3 amendment — kit-authored browser JavaScript (ADR-0017)

Filed at the same level as the Decision it edits, so a reader who skims the
headings cannot miss that §3 no longer says what it used to. Same shape as
ADR-0016's supersession notice.

**What was struck.** ~~*JavaScript is never written as source*~~ — the first
clause of §3 only. It was already false the day it was written: the kit
hand-authors four browser JS files (`chat.js` 194 lines, `overlay.js` 94,
`router.js` 179, `motion.js`), and a rule the kit breaks four times teaches
nothing. A rule nobody can follow is not a standard, it is a place people learn
to stop reading.

**What replaces it.** Kit-authored browser JavaScript is permitted at rung L1
of the escape-hatch ladder in
[../design/ui-architecture-patterns.md](../design/ui-architecture-patterns.md)
§3 — purely ephemeral view manipulation, no state outliving the element,
deleting it loses polish and not correctness. It must be dependency-free, a
classic script registering exactly one global, require no build step, be
`go:embed`-ed with a `Script()`/`Handler()` pair, and appear in
`docs/ui-components.md`.

**What is untouched.** The **"never built by a toolchain"** half is what the
rule always actually protected, and it stands: no npm, no bundler, no build
step, nothing whose output depends on the day you ran it. This is why the
amendment costs nothing — every file it legalises was already shipping under
exactly these terms.

**The third-party half also stands unchanged.** Third-party JS enters the repo
only as vendored, version-pinned, `go:embed`-ed artifacts: `datastar.js` v1.0.2
(~34KB) and the rare approved islands defined by the escape-hatch ladder in
[../design/ui-architecture-patterns.md](../design/ui-architecture-patterns.md).
Each vendored artifact records upstream version, source URL, and SHA-256
alongside the file. No bundler exists to run, because there is nothing to
bundle.

## Alternatives considered

- **TypeScript islands with a bundler (Vite/esbuild).** Attractive: the
  richest UI ecosystem on earth, typed client code, escape hatch for any
  interactivity ceiling. Lost because it reintroduces everything this ADR
  exists to exclude: a second language for agents and future-Geoff, a
  second dependency graph with npm's supply-chain surface, a build step
  whose output is not bit-reproducible without heroic pinning, and a
  standing temptation to move logic client-side — which violates the
  backend-owns-state model of [0003](0003-hypermedia-first-datastar.md).
  The actual need it serves is covered by the island contract: vendored,
  pinned web components bridged to Datastar signals.
- **"Tasteful npm" — Node for dev tooling only (Tailwind, a linter, a
  bundler), never shipped.** Attractive: it's the common compromise, and
  "it's only devDependencies" sounds harmless. Lost because dev-toolchain
  determinism *is* the product here: clean-machine builds and agent-driven
  CI cannot depend on a 200MB `node_modules` tree that resolves differently
  across months. And the compromise is unstable — once Node exists for
  Tailwind, the next "just one package" has no principled stopping point.
  The Tailwind standalone binary (v4.3, 2026-05-08) removed the last real
  justification.
- **Rust for CLI tools and the data pipeline.** Attractive: speed,
  single-binary distribution, strong correctness culture. Lost because Go
  already gives single-binary distribution and adequate speed for
  everything the kit does (the heaviest job — the TailwindPlus pipeline,
  [0010](0010-component-data-pipeline.md) — is IO-bound), while Rust would
  split the codebase into two languages with different build systems,
  doubling what agents must master and halving code reuse with the Kit
  library itself. Performance has never been the bottleneck; context is.

**Real costs of the winner, accepted with eyes open:** the Go UI ecosystem
is a fraction of npm's — there is no Storybook (we build the Gallery,
[0012](0012-gallery-living-style-guide.md)), component libraries are young
(templUI v1.12.0, DatastarUI ~146 stars as of 2026-06), and some problems
with off-the-shelf npm answers get solved in-house here. Rich client
widgets (canvas editors, advanced charts) require either an approved
vendored island or doing without. We pay that tax knowingly; the
alternative taxes compound worse over decades.

## Consequences

- **Easier:** clean-machine reproducibility (two commands, same bits);
  one `go.mod` to audit; agents operate in one language end-to-end; every
  tool in the repo is debuggable with the same debugger; security review
  surface shrinks to the Go module graph plus a handful of pinned blobs.
- **Harder:** UI capabilities arrive at the speed we vendor or build them,
  not the speed npm ships them. Mitigation: the island contract for true
  client-side needs, the TailwindPlus corpus + Gallery for component
  breadth, and the Assistant making what exists findable.
- **Risk: a future need (rich editor, collaborative canvas) exceeds what
  vendored islands can deliver.** Tripwire: the second time we reject a
  product feature *solely* because the island contract can't express it,
  we write a superseding ADR scoping a real exception rather than
  cheating quietly.
- **Risk: Tailwind drops or degrades the standalone binary.** Tripwire: a
  Tailwind release whose standalone CLI lags the npm package in features
  we use. Fallback: pin the last good binary indefinitely (it is
  self-contained) while evaluating CSS alternatives.
- **Risk: CUE pre-1.0 churn taxes the schema layer.** Owned by
  [0007](0007-cue-single-source-of-truth.md); this ADR only fixes *that*
  schemas are CUE, not how upgrades are absorbed.

## Revisit when

- The island contract fails its second real product need (see tripwire).
- Tailwind's standalone binary is discontinued or feature-frozen behind
  the npm distribution.
- Go or CUE stops being maintained at a level a solo owner can bet on —
  for CUE, watch CUE Labs' trajectory past its 2025 funding.
- A kit app genuinely requires a client runtime no vendored artifact can
  provide (e.g., local-first sync logic that must run in-browser), forcing
  a scoped exception ADR.
