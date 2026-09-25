# ADR-0012: Build the Gallery — a kit app that IS the living style guide

- **Status:** Superseded by ADR-0020 (shared catalog and independent Gallery)
- **Date:** 2026-06-10
- **Decision drivers:**
  - No mature Storybook equivalent exists for server-rendered Go; the gap is real and verified (2026-06).
  - Taste rots without a catalog: components, tokens, and patterns that aren't rendered somewhere drift into folklore.
  - The kit needs an acceptance harness and a visual-regression baseline that exercise the *real* stack (templ + Datastar + NATS + the Loopback Invariant), not mocks of it.
  - Agents are first-class users (D15): "what does the kit have and how does it behave" must be a URL, not an archaeology project.

## Context

The kit's UI layer is templ v0.3.1020 (2026-05-10) rendering Datastar v1.0.2
(2026-06-02) hypermedia over SSE, styled by the Tailwind v4.3.0 standalone
binary (2026-05-08), with state on embedded NATS JetStream (D3, D4, D8). Every
app is an HTTP server behind the Loopback Invariant (D5). The component corpus
is kit-owned templ components — vendored and adapted from templUI v1.12.0
(2026-06-01, MIT core) and DatastarUI (MIT, pushed 2026-06-01), informed by the
licensed TailwindPlus corpus indexed under D10 — unified under CUE-defined
design tokens (D7).

A component library without a living catalog degrades in three ways at once:
visual drift goes unnoticed (no regression baseline), API drift goes
undocumented (usage examples rot in markdown), and design drift goes
unchecked (each new screen reinvents a pattern instead of citing one). The
JS world solved this with Storybook. The server-rendered-Go world has not
solved it:

- **storytempl** (gitlab.com/joeslazaro/storytempl, verified 2026-06) bridges
  templ components into Storybook's server renderer by wrapping them in HTTP
  handlers — but it runs the actual Storybook toolchain, i.e. Node, which D2
  bans from every kit toolchain, forever.
- The bridge idea itself dates to Adrian Hesketh's 2021 pattern of pointing
  Storybook's server renderer at Go HTTP handlers
  (https://adrianhesketh.com/2021/10/23/using-storybook-with-go-frontends/) —
  it has not matured into a maintained Go-native tool since.
- The prevailing pattern in the ecosystem is the **hand-rolled gallery**:
  templUI's docs site and goship.it are themselves Go apps rendering their own
  components (https://github.com/templui/templui,
  https://github.com/haatos/goshipit, both active 2026-06). Nobody installs a
  style-guide tool; everybody builds a small one.

Separately, the kit makes claims that need a resident proof: that the
Tao-of-Datastar loop (signal POST → KV write → KV watch → re-render → SSE
patch, D3/D8) is the default and it works; that one binary runs in a browser
tab and in the Wails v3 shell (D5/D6); that capability degradation across
shells is explicit, not accidental. VISION.md's acceptance tests need a
harness. The Assistant (D11) needs demonstrable ground truth to point at.

## Decision

We build **the Gallery**: a kit app, in this monorepo (D13), that *is* the
living style guide. It is dogfood #1 — the first consumer of every kit
package — and it is load-bearing in three roles:

1. **The catalog.** Every kit component renders live with knob controls
   generated from its typed parameters (CUE component metadata → generated
   registry entries, D7), alongside its templ source, usage notes, and named
   variants. Every design token is visualized from the CUE source of truth.
   Every UI/UX pattern in `../design/ui-architecture-patterns.md` runs as a
   working mini-app exercising real NATS/SSE wiring. Every shell service
   (dialogs, notifications, tray, updater) has an exerciser page showing its
   per-shell degradation state.
2. **The visual-regression baseline.** Headless-browser snapshots per
   component × theme × density, driven by a registry-generated manifest, gate
   CI. A component is not "in the kit" until it has a Gallery page and a
   baseline.
3. **The acceptance harness.** "The kit works" means: the Gallery binary
   boots, serves over loopback, runs identically as a web app and inside the
   Wails v3 shell, and every catalog route renders. New kit ideas land in the
   Gallery first; kit releases are gated on it.

Boundaries, deliberately drawn:

- The Gallery is **not a public docs site** (VISION non-goal). It is private,
  which is also what permits the TailwindPlus browser tab over the licensed
  D10 index.
- The Gallery does **not replace unit tests**. templ render tests and package
  tests live with their packages; the Gallery covers what only a running
  app can: visual truth, wiring truth, shell truth.
- v1 scope is honest for one developer: components + tokens + a few pattern
  demos + visual regression. Service exercisers, the TailwindPlus tab, and
  the Playbook section accrete after. Full design:
  `../design/gallery-style-guide.md`.

## Alternatives considered

**Storybook via storytempl.** Attractive: Storybook is the mature category
king — knobs, addon ecosystem, snapshot tooling, familiar UX, and storytempl
proves templ components can plug into its server renderer. Lost on four
counts: (1) it reintroduces the Node toolchain, a direct D2 violation with no
quarantine possible — Storybook *is* the harness, not a peripheral; (2) knob
definitions would live in Storybook's JS config, severed from our typed templ
parameters and CUE metadata, exactly the drift we're trying to kill; (3)
storytempl is a thin, low-activity bridge — a bus-factor bet on glue; (4)
Storybook can render our components but cannot exercise our platform: no
NATS loop, no SSE patterns, no shell capability matrix, no Loopback
Invariant proof. It would catalog the least interesting layer of the kit.

**Static docs site.** Attractive: near-zero runtime cost — generate HTML
pages with screenshots and code samples, host anywhere, done in a day. Lost
because it is not *living*: screenshots rot the day a token changes, code
samples drift from real signatures with no compiler to object, and a static
page can't host knob controls, an SSE demo, or an updater dry-run. It can't
be a regression baseline (it *is* unverified renderings) and can't be an
acceptance harness (nothing runs). It is documentation about the kit rather
than a demonstration of it — the exact failure mode VISION.md calls cargo
cult.

**No gallery.** Attractive: zero cost; every hour goes into shipping
Throughline and the next app, and components are "documented" by their call
sites. Lost because taste rots without a catalog: with no single place where
everything renders, visual drift is invisible until it embarrasses, pattern
knowledge lives only in past code and Geoff's head (and is therefore opaque
to agents, failing the ambient-knowledge test), and there is no
visual-regression baseline at all. The kit's claims would be untestable as a
whole. This is the cheapest option this quarter and the most expensive one
over a decade.

**Adopt templUI's or goship's gallery code wholesale.** Attractive: existing,
working Go gallery code under MIT. Lost because those sites are bespoke to
their own component sets — no typed-knob registry, no CUE token integration,
no pattern/service sections — and adapting a finished docs site is more work
than building our small one on kit primitives. They remain reference
material (the pattern's existence proof), not a base.

**Real costs of the winner, stated plainly:** we own another application
forever, including its registry codegen (bespoke, nobody else maintains it);
visual regression pulls a headless-browser toolchain (Playwright, which means
Node) into CI — quarantined to a CI container, never in any build or dev
toolchain, but it is a scoped exception to D2's spirit that we accept and
document; and Gallery hours compete with product hours, mitigated only by the
fact that building the Gallery *is* building the kit.

## Consequences

- **Easier:** judging any UI change (open the page, look); landing new
  components (scaffold → register → page + baseline appear); agent
  development (the Assistant cites Gallery routes as ground truth — D11's
  pattern guidance gets a "see it running" URL); kit releases (the harness
  gate is mechanical); onboarding future-Geoff after six months away.
- **Harder:** every component now carries metadata + registration overhead
  (mitigated: generated from CUE, drift is a compile error); CI grows a
  browser matrix (mitigated: snapshot scope capped per
  `../design/gallery-style-guide.md` §9); the Gallery must itself stay upgraded
  as the kit churns pre-1.0 (mitigated: it's dogfood #1 — that *is* the job).
- **Risk: scope creep.** The Gallery is fun to build and infinite in
  principle. Tripwire: Gallery-only work blocking app-feature work for more
  than a sprint-equivalent → cut Gallery scope back to the v1 ladder.
- **Risk: Node creep via Playwright.** Tripwire: Node appearing anywhere
  outside the pinned CI container image → replace the runner (pure-Go CDP
  driver) or drop to render-hash regression until it's replaced.
- **Risk: registry codegen outgrows its value.** If the knob-codegen layer
  becomes harder than hand-writing registration, it has failed. Tripwire:
  codegen source exceeding the size of the generated code it replaces →
  simplify to hand-written specs with a CUE validation check.

## Revisit when

- A maintained, Node-free living-style-guide tool for server-rendered Go
  components reaches maturity (none exists as of 2026-06-10).
- templ ships first-party preview/gallery tooling that overlaps the
  registry's job.
- The component count or snapshot matrix makes the hand-rolled harness or CI
  gate creak (build times, baseline churn) — revisit snapshot scope and
  registry architecture.
- A pure-Go browser-automation driver proves equivalent to Playwright for
  snapshot work — retire the CI Node exception.
- The Gallery's registry and the Assistant's component index (D10/D11)
  diverge into double bookkeeping — unify on one metadata source.
