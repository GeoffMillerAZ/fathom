# ADR-0013: One repo, one Go module, tagged semver — propagation is `go get -u`

- **Status:** Superseded by ADR-0045
- **Date:** 2026-06-10
- **Decision drivers:** the propagation test (VISION.md §North-star #3 — kit
  improvement reaches every app via a version bump and a sub-hour guided
  upgrade); one maintainer with one brain, so version bookkeeping must round to
  zero; the kit as a churn firewall around deliberately young dependencies;
  agents do the upgrade labor, so upgrades must be legible artifacts, not oral
  tradition

## Context

The kit is heterogeneous by design: an importable library (`appkit`), two
binaries (`cmd/appkit`, `cmd/appkit-mcp`), a full application (the Gallery,
D12), a data pipeline (D10), CUE schemas (D7), and this docs corpus. A change
frequently cuts across all of them at once — a design-token rename touches the
CUE schema, generated Go, components, Gallery pages, and an upgrade note.

VISION.md §3 is explicit: "implement once, adopt everywhere" is a *versioning*
problem, not a copying problem. Whatever we choose here is the mechanism by
which every future app receives every future improvement.

Go modules make one arrangement nearly free and all others expensive. A single
module at the repo root is tagged `vX.Y.Z` and consumed with one `require`
line. A multi-module repo requires per-module prefixed tags
(`appkit/v0.4.0`, `apps/gallery/v0.2.1`, ...), explicit cross-module version bumps
on every internal API change, and workspace/replace juggling during
development — machinery built for organizations shipping independently
versioned products to strangers.

The dependency facts that shape this (all dated, all pre-1.0 or
freshly stable):

- templ is v0.3.1020 (2026-05-10), pre-1.0, with a documented CLI/module
  version-skew hazard solved by Go `tool` directives plus a CI `-check`
  (https://github.com/a-h/templ/releases,
  https://github.com/a-h/templ/discussions/592).
- CUE is v0.16.1 (2026-04-08) with roughly quarterly minors that each carry
  flagged breaking changes; `gengotypes` and the JSON Schema encoder are
  explicitly experimental (https://github.com/cue-lang/cue/releases).
- The MCP go-sdk is v1.6.1 (2026-05-22) with a spec-tracking bump expected
  after the 2026-07-28 revision finalizes
  (https://github.com/modelcontextprotocol/go-sdk/releases).
- Wails v3 is a pinned alpha (≥ alpha.99, 2026-06-10) with breaking changes
  still landing (D6).

Every one of those upgrades is a deliberate, tested event. The question is
*where* that event happens: once, in the kit — or N times, in N apps.

Third-party UI material has a settled entry path in the ecosystem: templUI
(v1.12.0, 2026-06-01, MIT core) installs component *source* into your repo via
a shadcn-style CLI — "own your code", updates become manual merges
(https://github.com/templui/templui). TailwindPlus is licensed reference
material — 657 components reachable through the D10 corpus and the Assistant —
whose license forbids redistribution; it can never be imported as a code
dependency.

The kit is private — no public hosting, no outside consumers — and pre-1.0
with exactly one customer.

## Decision

**This repo is a monorepo containing one Go module, tagged semver from the
root.** The module path is the private repo path
(`github.com/geoffmilleraz/app-kit`); every dev machine and CI environment
sets `GOPRIVATE=github.com/geoffmilleraz/*`. No proxy, no checksum database,
no public availability.

Everything lives in the module: the `appkit` library packages, `cmd/appkit`,
`cmd/appkit-mcp`, the Gallery, the pipeline, the CUE schemas, the docs. The
full target layout is [../design/repo-layout.md](../design/repo-layout.md).

**Apps consume the kit by import, never by copy.** A downstream app's `go.mod`
requires the kit at an exact tag; adoption of an improvement is
`go get -u github.com/geoffmilleraz/app-kit && go mod tidy` plus the upgrade
note. The scaffold (`appkit new`) emits app-specific code only — pages, domain
logic, `app.cue` — and imports everything else. If a scaffolded file would need
editing when the kit improves, it belongs in the kit, not the scaffold.

**Third-party material is vendored into the kit exactly once, then owned:**

- templUI components enter via its CLI into per-component packages
  `appkit/ui/<component>/` (e.g. `appkit/ui/button/`), get
  adapted to kit tokens (D7) and kit conventions, and from then on are ours —
  upstream divergence is accepted, upstream improvements are cherry-picked
  manually if ever.
- TailwindPlus is never code. It is reference material reached through the
  corpus and the Assistant (D10, D11); designs are *adapted* into kit
  components by hand or by agent, not imported.

**Tag discipline.** Tags are cut only from green CI. Pre-1.0 semantics:
`v0.MINOR.PATCH`, where a minor may contain features *and* breaking changes,
and a patch is fixes only. Every tag gets a `CHANGELOG.md` entry. Every minor
gets an upgrade note at `docs/upgrades/vX.Y.md` — even if its entire body is
"no app action required" — containing: summary, breaking changes, mechanical
steps, verification steps, and a small structured header that CI validates.
The Assistant's `upgrade_guide` tool serves these notes, so the propagation
loop is: land in kit → tag → `go get -u` in the app → agent walks the note
([../design/mcp-design-assistant.md](../design/mcp-design-assistant.md)).

**Pre-1.0 policy:** breaking changes are allowed and undeprecated — one
customer, an upgrade assistant, and backwards-compatibility theater is a
non-goal (VISION.md §Non-goals). The non-negotiable: a breaking change ships
in the same tag as the upgrade note that names it.

Boundaries: this ADR fixes the repo/module/version topology and the
propagation mechanism. It does not fix internal directory layout (that is
[../design/repo-layout.md](../design/repo-layout.md)), does not decide the
post-1.0 `/v2` import-path strategy (deferred until v1.0 is actually in
sight), and does not version the data artifacts — those carry their own
content-addressed manifest (D10).

## Alternatives considered

- **Multi-module monorepo** (separate modules for `appkit`, the Gallery, the
  pipeline, the binaries). Attractive: apps depending on `appkit` would not
  carry the pipeline's or the Gallery's dependency graph, and each part could
  version independently. Lost because independent versioning is precisely the
  cost: per-module prefixed tags, cross-module `require` bumps for every
  internal refactor, and replace/workspace ceremony during development —
  version skew hell administered by one person who is also the only consumer.
  The real cost of the single-module winner: every app's module graph carries
  the kit's full `require` list (NATS server, cuelang.org/go, Wails,
  parquet-go), inflating `go.sum` and first-download time. We accept it
  because the Go linker prunes unimported packages from binaries — the bloat
  is bookkeeping weight, not shipped bytes — and because D8/D9 already accept
  ~16MB of NATS as the default spine anyway.
- **Polyrepo** (kit, CLI, Assistant, Gallery, pipeline each in its own repo).
  Attractive: small clones, clean ownership boundaries, independent release
  cadence. Lost because cross-repo changes are propagation friction — the
  exact disease the kit exists to kill (VISION.md §3). A token rename becomes
  four PRs, four tags, and a dependency-bump dance; atomic
  schema+code+docs+Gallery commits become impossible by construction; and
  agents lose the single-checkout property where everything they need is
  already in the working tree.
- **Copy-template / scaffold-and-own** (the shadcn/templUI model applied to
  kit→app: `appkit new` copies kit source into each app). Attractive: zero
  version coupling, apps can never be broken by an upgrade, and the model is
  proven upstream — templUI v1.12.0 ships exactly this (2026-06-01). Lost
  because templates rot — VISION.md names this as the failure the kit was
  founded against. Copied code receives no fixes; the propagation test
  (north-star #3) becomes unpassable by construction; twelve apps means twelve
  manual retrofits. The model is *correct* at the third-party→kit boundary
  (we vendor templUI once, into one place, with one owner) and wrong at the
  kit→apps boundary, where there will be many consumers over decades.
- **Single module, no tags — apps pin commit SHAs.** Attractive: zero release
  ceremony, `go get` handles pseudo-versions fine. Lost because SHAs carry no
  compatibility signal and give upgrade notes nothing to anchor to: the
  Assistant's `upgrade_guide` needs "you are on v0.4, here is the path to
  v0.7" semantics, and `docs/upgrades/<sha>.md` is not a system. Tagging
  discipline costs minutes per release and buys the entire propagation story.

## Consequences

- Easier: atomic cross-cutting change — one commit can move a CUE schema, its
  generated Go, the component, the Gallery page, the ADR, and the upgrade
  note together, and CI gates them as a unit.
- Easier: the kit is a churn firewall. templ, CUE, go-sdk, and Wails upgrades
  (each a deliberate event per D4/D7/D11/D6) happen once in the kit; apps
  only ever see kit minors with upgrade notes.
- Easier: one version number, one CHANGELOG, one CI pipeline, one bisect
  space; agents always operate in a checkout where every referenced artifact
  exists.
- Harder: apps inherit the kit's full dependency bookkeeping (`go.sum`
  weight, occasional `go mod tidy` churn). Accepted; revisit only if a truly
  hostile dependency appears (see risks).
- Harder: one version couples unrelated components — a Gallery-only release
  still bumps the version apps see. Mitigated by upgrade notes that say "no
  app action required", which the Assistant relays in seconds.
- Risk: the pipeline someday wants a dependency that contaminates all
  consumers (cgo, enormous transitive graphs — e.g. anything DuckDB-shaped,
  which D10 already exiles from shipped binaries). Tripwire: any cgo or
  multi-hundred-MB dependency is proposed for the module → carve *that
  consumer* (likely `internal/twplus/`) into a nested module as the one sanctioned
  exception, with its own prefixed tags.
- Risk: pre-1.0 breaking-change freedom becomes a permanent habit. Tripwire:
  a second human contributor, or any part of the kit being opened publicly →
  cut v1.0.0 and write the `/v2` import-path ADR before, not after.
- Risk: GOPRIVATE setup is forgotten on a new machine or CI runner and
  `go get` fails confusingly. Mitigated: the scaffold's CLAUDE.md pack (D15)
  states the env var and the failure signature, so the agent self-heals it.

## Revisit when

- v1.0.0 comes into sight — semantic import versioning (`/v2` suffixes) and
  compatibility promises need their own ADR before the tag.
- Any component needs a release cadence the single version stream cannot
  serve (e.g. the Assistant must ship daily while the library is frozen).
- The pipeline or any tool demands cgo or a dependency that materially
  poisons downstream `go.mod` graphs — nested-module carve-out time.
- A second human contributor joins, or any kit code is published publicly.
- Full CI on the monorepo exceeds ~10 minutes — split CI jobs first, the
  module only if that fails.
