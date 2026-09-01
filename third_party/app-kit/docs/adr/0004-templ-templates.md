# ADR-0004: templ is the template layer

- **Status:** Accepted
- **Date:** 2026-06-10
- **Decision drivers:** compile-time provability (D2); Datastar ecosystem
  gravity (D3); codegen discipline must be mechanically enforceable, not
  aspirational; agents author most UI code and need the compiler as reviewer

## Context

Every kit app renders HTML on the server ([0003-hypermedia-first-datastar.md](0003-hypermedia-first-datastar.md),
[0005-loopback-invariant.md](0005-loopback-invariant.md)). The template
layer is therefore the single largest surface agents write against, and
[0002-go-and-cue-only-no-node.md](0002-go-and-cue-only-no-node.md) demands
it be Go. The Go options as of 2026-06-10:

- **templ** (a-h/templ, MIT, ~10.3k stars): v0.3.1020 released 2026-05-10
  on a steady ~6–8-week cadence (v0.3.1001 2026-02-28, v0.3.977
  2025-12-31); repo pushed 2026-06-05
  (https://github.com/a-h/templ/releases). `.templ` files code-generate to
  plain Go (`*_templ.go`), so component signatures, parameter types, and
  composition are checked by the Go compiler, with context-aware escaping
  (HTML text, attributes, URLs via `templ.SafeURL`, scripts via
  `templ.JSFuncCall`) layered on top (https://templ.guide/). v0.3.1020
  added a `-check` flag (validate without generating — built for CI),
  concurrent component rendering, and Range/position metadata completing a
  campaign to fix LSP diagnostic precision
  (https://github.com/a-h/templ/releases/tag/v0.3.1020). v0.3.1001 moved
  the toolchain to Go 1.26 and added TLS to the live-reload proxy. Still
  pre-1.0: no API stability promise, and commits on main after v0.3.1020
  are loosening the URL-attribute escaping API (plain strings without
  `templ.SafeURL`) — security-relevant surface still moves.
- **gomponents** (maragu.dev/gomponents, MIT): HTML as pure Go functions,
  zero codegen, zero build step. v1.3.0 released 2026-04-14, post-1.0
  semver-stable since Oct 2024, 100% test coverage, pushed 2026-06-08,
  with first-party Datastar bindings (maragudk/gomponents-datastar)
  (https://github.com/maragudk/gomponents).
- **html/template** (stdlib): robust contextual auto-escaping, but
  templates parse and resolve field references at runtime — a typo'd field
  name fails at request time, not compile time. Community benchmarks put
  templ and gomponents both ~5–10x faster
  (https://ewen.quimerch.com/articles/14-templ-vs-gomponents/, dated
  relative to 2025–2026 versions).
- **gostar** (delaneyj/gostar, Apache-2.0): the Datastar creator's fluent
  HTML builder; ~150 stars, last pushed 2025-11-12 — effectively
  unmaintained (https://github.com/delaneyj/gostar).

The Datastar ecosystem has converged on templ: templ.guide carries an
official Datastar page authored by Datastar's creator (PR #774;
https://templ.guide/server-side-rendering/datastar/), the official
datastar-go SDK ships Templ helpers, and both DatastarUI (pushed
2026-06-01) and templUI v1.12.0 (2026-06-01) — the component corpora the
kit vendors from — are built on templ.

templ's known sharp edge is version skew: the CLI and the
`github.com/a-h/templ` module version must match or `*_templ.go` output is
silently stale or broken (discussion #592;
https://github.com/a-h/templ/discussions/592). Go 1.24+ `go.mod` tool
directives make this mechanically enforceable.

## Decision

**templ (≥ v0.3.1020, pinned exactly) is the template layer for the Kit,
the Gallery, and every kit app. Components are typed Go functions; the
compiler is the first reviewer of every screen.**

The codegen discipline, all three legs mandatory:

1. **Pinning.** The templ CLI is pinned via the `go.mod` tool directive
   (`tool github.com/a-h/templ/cmd/templ`) and invoked only as
   `go tool templ ...`. CLI/module skew is structurally impossible; a
   templ upgrade is one `go get` line in one commit.
2. **CI verification.** CI runs `go tool templ generate -check` (falling
   back to generate + `git diff --exit-code` if `-check` regresses) and
   `go tool templ fmt -fail` on every push. Stale or hand-edited generated
   code cannot merge.
3. **Committed, marked generated.** `*_templ.go` files are committed —
   the kit must be `go build`-able and `go install`-able with no build
   step beyond the Go toolchain (D2's two-command clean-machine build) —
   and marked `*_templ.go linguist-generated=true` in `.gitattributes` so
   they collapse in review. Agents are instructed (shipped `CLAUDE.md`
   pack, [0015-ai-first-surface.md](0015-ai-first-surface.md)) to never
   edit them.

**Dev loop.** The standard inner loop is templ's own watcher with proxy
auto-reload, supervised by wgo (v0.6.4, 2026-05) rather than air — fewer
moving parts, no TOML config:

```
wgo -file=.go -file=.templ -xfile=_templ.go \
    go tool templ generate :: go run ./cmd/app
```

or, for HTML-heavy sessions, `go tool templ generate --watch
--proxy=http://127.0.0.1:<port> --cmd='go run ./cmd/app'`, where dev mode
serves text changes from `_templ.txt` without restarting the Go process
and the proxy injects the reload script
(https://templ.guide/developer-tools/live-reload/). The `-xfile=_templ.go`
exclusion is load-bearing: watching generated output loops forever. The
proxy fronts the loopback listener of [0005](0005-loopback-invariant.md);
dev-only, never shipped.

**Escaping policy.** We rely on templ's context-aware escaping and treat
its API as security surface: the in-flight URL-escaping loosening on main
means every templ version bump gets its release notes read before merge,
same posture as CUE upgrades (D2).

**The documented fallback is gomponents v1.3.0**, and it is specific, not
decorative. gomponents is used instead of templ when:

- **A tool is too small for codegen.** One-file utilities and throwaway
  internal pages where a generate step is disproportionate — the same
  opt-out spirit as the channel-backed bus in
  [0008-embedded-nats-spine.md](0008-embedded-nats-spine.md).
- **Go code is emitting UI.** Generators driven from CUE (e.g., Gallery
  knob scaffolds, [0012-gallery-living-style-guide.md](0012-gallery-living-style-guide.md))
  emit gomponents calls directly — plain Go output — rather than emitting
  `.templ` source that needs a second codegen pass.
- **templ fails the kit.** If templ goes unmaintained (no release or
  maintainer activity for ~9 months) or ships a breaking change we cannot
  absorb, gomponents is the migration target: post-1.0 stable, one honest
  author, first-party Datastar bindings, same Tailwind pipeline.
  Migration is mechanical but manual — both end as Go functions returning
  renderable components, so call sites and tests largely survive.

Boundaries: this ADR covers HTML templating only. CSS is Tailwind's
standalone binary (D2); client reactivity is Datastar (D3); component
*content* (which components exist) is [0010](0010-component-data-pipeline.md)
/ [0012](0012-gallery-living-style-guide.md) territory.

## Alternatives considered

- **gomponents as the primary, not the fallback.** Attractive: zero
  codegen, post-1.0 stability templ can't offer (a community comparison
  templ's own maintainer engaged with concedes templ is "not quite there
  yet" on that axis — https://github.com/a-h/templ/discussions/948), no
  generated files in git. Lost as primary because HTML-as-nested-Go-function-calls
  is materially worse for design-heavy work: no HTML-shaped source for
  designers'-eye review, no `templ fmt`, no LSP inside markup, and — the
  deciding weight — the component corpora the kit lives on (templUI,
  DatastarUI, TailwindPlus HTML) translate to `.templ` nearly 1:1 but to
  gomponents only through tedious transliteration. It remains the named
  fallback precisely because it is good.
- **html/template.** Attractive: stdlib, zero dependencies, battle-tested
  escaping. Lost on the one axis this kit refuses to trade: field
  references resolve at runtime, so a renamed struct field becomes a
  request-time error an agent discovers only if a test happens to render
  that path. Runtime field resolution kills it; ~5–10x slower rendering is
  just the confirmation.
- **gostar.** Attractive: from the Datastar creator, generated from the
  HTML living standard, benchmarks faster than html/template. Lost
  because it is a side project — last push 2025-11-12, ~150 stars — and
  the kit does not build its largest code surface on an unmaintained
  foundation, author pedigree notwithstanding.

**Real costs of the winner:** a codegen step exists forever; generated
files inflate diffs (mitigated by linguist-generated and a single-owner
repo — merge-conflict pain is a team disease we mostly don't have); templ
is pre-1.0 with a moving, security-relevant escaping API; the LSP is a
gopls proxy with residual flakiness despite the 2025–2026 Range-metadata
work; and the project carries single-maintainer gravity around Adrian
Hesketh. We accept all five, with tripwires below.

## Consequences

- **Easier:** UI typos become compile errors — for agent-authored code
  this converts review burden into build failures; refactors propagate
  through templates via ordinary Go tooling (`gopls` rename touches
  `.templ`-generated call sites); kit components expose typed signatures
  the Gallery turns into knob controls ([0012](0012-gallery-living-style-guide.md))
  and the Assistant serves as machine-readable metadata
  ([0011](0011-mcp-design-assistant.md)).
- **Harder:** every contributor-agent must know the generate step exists;
  mitigated by the tool directive (no separate install), wgo recipe in the
  scaffold, and CI catching every lapse.
- **Risk: templ pre-1.0 breaking change in escaping.** Tripwire: any
  release note touching `templ.SafeURL`/escaping semantics triggers a
  review of kit components before the bump merges.
- **Risk: maintainer abandonment.** Tripwire: ~9 months without release
  or maintainer activity → execute the gomponents migration while both
  ecosystems still overlap.
- **Risk: `-check` or watch-mode regressions on the bleeding edge** (watch
  gzip corruption was only fixed in v0.3.1020). Tripwire: a broken dev
  loop after a bump → revert the pin same-day; the pin makes this cheap.

## Revisit when

- templ ships v1.0 with a stability promise (re-evaluate the pin cadence
  and relax release-note paranoia).
- templ hits the 9-month inactivity tripwire (execute fallback).
- The gomponents-vs-templ ergonomics balance flips — e.g., gomponents
  gains an HTML-source toolchain, or templ's codegen friction starts
  consuming real weekly time (track honestly; today it consumes none).
- Generated-file churn becomes measurable agent-context pollution despite
  linguist-generated marking ([0015](0015-ai-first-surface.md) pressure).
