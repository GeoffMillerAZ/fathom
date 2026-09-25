# The Gallery — design

Historical implementation sketch. [ADR-0020](../adr/0020-shared-catalog-and-adoption-evidence.md)
now governs: the shipped Gallery evolves independently of optional NATS/desktop
services, with shared catalog contracts in [experience-system.md](experience-system.md).
The registry, knobs and gates described below are not all implemented; see
[current scope](../intent/scope.md) before planning work.

Decision: `../adr/0012-gallery-living-style-guide.md` (D12). This document is
the build plan: information architecture, the component registry, knob
wiring, theming, pattern demos, service exercisers, visual regression, the
TailwindPlus browser, and the shell story. Versions cited were verified
2026-06-10.

The Gallery is a kit app like any other: one `http.Handler` tree on a
loopback listener (D5), an `app.cue` manifest (D7), embedded NATS JetStream
as its state spine (D8), templ + Datastar UI (D3/D4). It earns its keep by
being dogfood #1 — the first consumer of every kit package and the proof
that the platform contract holds.

## 0. Scope ladder (honest, one developer)

- **v1:** Components section (registry, knobs, source, variants), Tokens
  section, theme/density switching, 2–3 pattern demos, visual-regression
  manifest + CI gate, runs as web app and in the Wails shell.
- **v1.x:** remaining pattern demos, service exercisers, Home dashboard,
  Playbook section.
- **Later:** TailwindPlus browser tab, "adapt into kit component" workflow,
  wire-view panels on every demo.

Everything below is designed so the later rungs accrete without rework, but
only the v1 rung is committed.

## 1. Repo placement and file layout

Monorepo, single module (D13):

```
apps/gallery/
  main.go              // kit scaffold shape: appkit.Run(gallery.New)
  app.cue              // the Gallery's own app manifest (go:embed-ed; read by appkit.Run)
  ui/                  // gallery chrome: shell layout, nav, knob panel
  sections/
    home/
    components/        // catalog pages (generic — driven by the registry)
    tokens/
    patterns/          // demo mini-apps, one package per pattern
    services/          // capability exercisers
    playbook/          // ADR/design-doc rendering
    twplus/            // TailwindPlus browser (later)
  visreg/
    manifest.go        // /api/visreg/manifest generator
    playwright/        // spec + pinned container config (CI-only Node, see §9)
    baselines/         // committed PNG baselines
appkit/ui/catalog/     // the registry — lives in the KIT, not the Gallery
appkit/ui/<component>/ // each kit component package self-registers (§4)
```

The registry deliberately lives in `appkit/ui/catalog`, not in the Gallery:
components register themselves where they are defined, the Gallery merely
renders the registry, and the Assistant (D11) reads the same registry for
component search. One metadata source, three consumers.

## 2. Information architecture

Top-level nav, in order:

| Route | Section | What it is |
|---|---|---|
| `/` | Home | Kit version, what changed since last tag, spine health (NATS/SQLite/shell), catalog counts |
| `/components` | Components | Category-grouped grid of every registered component |
| `/components/{slug}` | Component page | Live render + knobs + source + usage + variants (anatomy below) |
| `/tokens` | Tokens | Color, type scale, spacing, radii, shadows, motion — rendered from CUE truth |
| `/patterns` | Patterns | Index of runnable pattern demos |
| `/patterns/{slug}` | Pattern demo | A working mini-app (§7) |
| `/services` | Services | Capability matrix + exercisers (§8) |
| `/playbook` | Playbook | ADRs and design docs rendered in-app (embedded from `docs/`) |
| `/twplus` | TailwindPlus | D10 index browser (§10, later) |
| `/stream` | — | The per-tab SSE stream (kit default: one long-lived GET per tab, D3) |
| `/api/visreg/manifest` | — | Machine-readable snapshot manifest (§9) |

**Per-component page anatomy** (`/components/button`):

- **Preview pane** — the component rendered live inside a neutral stage,
  re-rendered server-side on every knob change (§5). A "stage" wrapper
  provides padding, optional width constraint, and a checkered backdrop
  toggle for transparency work.
- **Knob panel** — controls generated from the component's typed parameters:
  enums become segmented controls, bools become switches, strings/ints become
  inputs, token-typed params become token pickers. Driven by Datastar
  signals (§5).
- **Source tab** — the actual `.templ` source (embedded at build time via
  `go:embed`) plus the component's CUE metadata. Never a copy; the file
  itself.
- **Usage tab** — prose notes (when to use, when not to), a copyable call
  snippet reflecting the *current knob values* (re-rendered with the
  preview), and links to patterns that employ the component.
- **Variants tab** — the named presets from CUE metadata rendered as a grid;
  these are exactly the visual-regression snapshot set (§9).
- **Isolated render** — `/components/{slug}/preview?variant=&theme=&density=`:
  chrome-free, deterministic, used by the snapshot runner and embeddable
  anywhere.

## 3. App shape and wire flow

The Gallery follows the Tao-of-Datastar kit default (D3) without exception,
because demonstrating the default is its job:

- One long-lived `GET /stream` SSE connection per tab, opened by the layout.
  It renders idempotent fat-morph snapshots of the page region that changed.
- All interaction is short POST commands carrying Datastar signals.
- All durable-ish state (knob values, theme, density, demo state) lives
  server-side in JetStream KV (D8), keyed per tab or per session. The
  browser holds no state worth keeping.

Canonical loop, which every knob, theme toggle, and demo reuses:

```
input change → Datastar signal patch → @post(command URL)
  → handler ReadSignals → validate → KV put (gallery.tab.<tab>.…)
  → KV watch (held by the /stream goroutine) fires
  → re-render affected templ fragment → sse.PatchElementTempl → morph
```

This is D8's "KV watch → re-render → SSE patch" canonical loop, running in
anger. The Gallery is the reference implementation other apps copy.

## 4. The registry: `appkit/ui/catalog`

### Types

```go
package catalog

type KnobKind int

const (
    KnobBool KnobKind = iota
    KnobString
    KnobInt
    KnobEnum
    KnobToken // value constrained to a token group, e.g. "color.accent.*"
)

type Knob struct {
    Name    string   // parameter name, matches CUE + templ param
    Label   string
    Kind    KnobKind
    Default any
    Options []string // KnobEnum: allowed values; KnobToken: token group prefix
    Min, Max int     // KnobInt
}

type Variant struct {
    Name   string            // "primary-loading"
    Values map[string]any    // knob preset
}

type ComponentSpec struct {
    Slug     string // "button" — stable ID, used in routes and baselines
    Name     string // "Button"
    Category string // "actions", "forms", "layout", "feedback", "data"
    Doc      string // one-paragraph usage summary (agent-facing too)
    Knobs    []Knob
    Variants []Variant
    Source   string // go:embed-ed .templ source
    MetaCUE  string // go:embed-ed .cue metadata
    Render   func(ctx context.Context, v Values) templ.Component
}

type Values map[string]any // helpers: v.Bool("x"), v.Enum("kind"), v.Int("n")…

func Register(s ComponentSpec)        // called from generated init()
func All() []ComponentSpec            // sorted: category, then name
func Get(slug string) (ComponentSpec, bool)
```

### Metadata is authored in CUE, next to the component (D7)

```cue
// appkit/ui/button/button.cue
component: button: {
    name:     "Button"
    category: "actions"
    doc:      "Primary action trigger. One primary per view; prefer ghost for secondary actions."
    params: {
        label:    {kind: "string", default: "Save"}
        variant:  {kind: "enum", options: ["primary", "secondary", "ghost", "danger"], default: "primary"}
        size:     {kind: "enum", options: ["sm", "md", "lg"], default: "md"}
        disabled: {kind: "bool", default: false}
        loading:  {kind: "bool", default: false}
    }
    variants: [
        {name: "primary", values: {}},
        {name: "danger", values: {variant: "danger", label: "Delete"}},
        {name: "loading", values: {loading: true}},
        {name: "disabled", values: {disabled: true}},
    ]
}
```

Authored in the Go-representable CUE subset (D7: no load-bearing
disjunctions), validated against a kit-owned `#Component` schema.

### Codegen, and why drift is a compile error

A subcommand of the kit's CUE codegen driver (the small Go program on
`cuelang.org/go` mandated by D7 — not `cue cmd`) walks `appkit/ui/*/`,
validates each `*.cue` against `#Component`, and emits one
`catalog_gen.go` per component package:

```go
// Code generated by appkit gen catalog. DO NOT EDIT.
package button

import (
    "context"
    _ "embed"

    "github.com/geoffmilleraz/app-kit/appkit/ui/catalog"
    "github.com/a-h/templ"
)

//go:embed button.templ
var source string

//go:embed button.cue
var metaCUE string

func init() {
    catalog.Register(catalog.ComponentSpec{
        Slug: "button", Name: "Button", Category: "actions",
        Doc:   "Primary action trigger. …",
        Knobs: []catalog.Knob{ /* from CUE params */ },
        Variants: []catalog.Variant{ /* from CUE variants */ },
        Source: source, MetaCUE: metaCUE,
        Render: func(ctx context.Context, v catalog.Values) templ.Component {
            return Button(Props{
                Label:    v.String("label"),
                Variant:  Variant(v.Enum("variant")),
                Size:     Size(v.Enum("size")),
                Disabled: v.Bool("disabled"),
                Loading:  v.Bool("loading"),
            })
        },
    })
}
```

The generated `Render` shim calls the real templ component with typed
arguments. If the CUE metadata and the templ signature disagree — a renamed
param, a removed enum value — the generated shim **fails to compile**. That
is the whole trick: metadata can't drift from implementation because the Go
compiler arbitrates. Generated files are committed and CI checks
regeneration is clean, same regime as `*_templ.go` (D4) and `cue exp
gengotypes` output (D7).

The Gallery binary imports `appkit/ui/...` (all component packages), so
`init()` registration populates the catalog; `catalog.All()` drives the
Components section, the visreg manifest, and — via the Assistant — kit
component search. One source, no second bookkeeping.

## 5. Knobs: Datastar signals driving server re-render

Knob state is **server-owned per tab**, like everything else. The knob panel
binds inputs to signals; any change POSTs all signals; the server persists
and the stream re-renders. No client-side preview logic exists.

```templ
// knob panel fragment (generated generically from spec.Knobs)
<div data-signals={ knobSignalsJSON(spec, current) }>
    <select data-bind-knobs.variant
            data-on-change="@post('/components/button/knobs')">…</select>
    <input type="checkbox" data-bind-knobs.loading
           data-on-change="@post('/components/button/knobs')"/>
    …
</div>
```

```go
func handleKnobs(kv jetstream.KeyValue) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var sig struct {
            Tab   string         `json:"tab"`
            Knobs map[string]any `json:"knobs"`
        }
        if err := datastar.ReadSignals(r, &sig); err != nil { /* 400 */ }
        spec, _ := catalog.Get(r.PathValue("slug"))
        vals := spec.CoerceAndValidate(sig.Knobs) // clamp ints, check enums
        b, _ := json.Marshal(vals)
        kv.Put(r.Context(), "tab."+sig.Tab+".knobs."+spec.Slug, b)
        w.WriteHeader(http.StatusNoContent) // render happens on the stream
    }
}
```

The `/stream` goroutine for that tab watches `tab.<tab>.>` and, on the knob
key changing, re-renders the preview pane and the usage snippet with
`sse.PatchElementTempl` (datastar-go v1.2.2 ships templ helpers, 2026-06-02,
https://github.com/starfederation/datastar-go). Round-trip over loopback is
local; per the Tao, no optimistic UI
(https://data-star.dev/guide/the_tao_of_datastar, verified 2026-06).

This is deliberate dogfooding: the knob panel is the simplest possible
instance of the kit's default interaction loop. If it ever feels slow or
awkward, the *kit default* has a problem, and the Gallery is where we find
out first.

## 6. Tokens and live theme switching

Design tokens are authored once in CUE and emitted as DTCG 2025.10 JSON and
Tailwind `@theme` CSS (D7; Tailwind v4.3.0 standalone binary, 2026-05-08,
https://tailwindcss.com/docs/installation/tailwind-cli — config lives in the
input CSS, `@source "./**/*.templ"`). `@theme` yields CSS custom properties,
which makes runtime switching a pure attribute swap:

- **Theme:** `light | dark | system`, as `data-theme` on `<html>`. Dark
  overrides redefine the semantic color custom properties.
- **Density:** `comfortable | compact`, as `data-density` on `<html>`.
  Compact redefines the handful of spacing/type-scale custom properties the
  token schema marks density-sensitive.

Switching is server-owned session state: the toggle POSTs
`{theme: "dark"}`, the handler writes session KV, every open tab's stream
sees the watch fire and re-renders its page snapshot with the new `<html>`
attributes — multi-tab convergence falls out for free, which is itself a
demo of the kit's claim.

**Token visualization pages** iterate the *generated Go token structs*
(gengotypes output from the CUE token source, D7) — never a hand-copied
list, so the page cannot drift from the truth:

- Color: swatch grid per group, resolved value in current theme, custom
  property name, contrast hints against surface tokens.
- Type: rendered specimen per step of the scale.
- Spacing/radii/shadows/motion: rendered exemplars with names and values.

Every catalog preview honors the current theme + density, which is what
makes the snapshot matrix in §9 meaningful.

## 7. Pattern demos: working mini-apps

Each scenario in `design/ui-architecture-patterns.md` gets a runnable demo —
not a description, a mini-app exercising the real wiring. Demos register
like components:

```go
type PatternSpec struct {
    Slug, Name, Doc string
    PatternRef      string // anchor into design/ui-architecture-patterns.md
    Mount           func(deps Deps) http.Handler // gets the real bus/KV/store
}
```

Each demo owns a namespaced KV bucket / subject prefix
(`gallery.demo.<slug>.>`) on the embedded NATS spine (nats-server v2.14.x,
D8) and mounts under `/patterns/{slug}/`. v1 ships two or three; the set
grows to cover every scenario the patterns doc defines. Expected early
demos, named by their pattern:

- **List + detail with live updates** — KV bucket as the backing store, KV
  watch → snapshot re-render; mutate from two tabs and watch them converge.
- **Long-running job with progress** — command POST starts a job, progress
  published on a subject, stream re-renders the progress UI; includes
  cancel.
- **Form command + server validation** — signals → POST → validation errors
  rendered from the server, no client validation logic.
- **Reconnect/self-heal** — a "kill my stream" button; Datastar's built-in
  retry (datastar v1.0.2 defaults: retryInterval 1000ms, scaler 2, max 10
  retries, https://data-star.dev/reference/actions, verified 2026-06)
  reconnects and the idempotent snapshot heals the page.

Each demo page carries a **wire view** panel (v1.x): a live tail of the
demo's NATS subjects and outgoing SSE events, rendered through the same
stream. Seeing the wire is how the pattern teaches itself — to Geoff after
six months away, and to an agent reading the page through the Assistant.

## 8. Service exercisers and the capability matrix

`/services` renders the platform contract: a matrix of every capability in
the kit's shell interface (owned by `../adr/0005-loopback-invariant.md` /
`../adr/0006-wails-v3-desktop-shell.md`) × the currently running shell, each
cell Native / Degraded / Unavailable as reported by the live shell adapter —
not a hardcoded table. Run the Gallery in a browser tab and in the Wails
shell side by side and the matrix *is* the degradation documentation.

Per-capability exerciser pages:

- **Dialogs** — open/save/message dialogs with parameter knobs; shows the
  returned value.
- **Notifications** — fire a notification, observe per-shell behavior.
- **Tray** — toggle tray icon/menu items (Wails v3 systray, D6); page shows
  current tray state; in a browser tab the exerciser renders its
  Unavailable state honestly.
- **Updater dry-run** — drives the Wails v3 built-in `app.Updater` (GitHub
  Releases provider + SHA256SUMS, D6; updater is a state machine emitting
  events per transition,
  https://github.com/wailsapp/wails/blob/master/docs/src/content/docs/guides/updater.mdx,
  verified 2026-06) in check-and-verify mode with the swap step disabled;
  the event log of every state transition renders live via the stream.
- **Secrets** — round-trip a throwaway value through the keychain capability
  (D14), confirming the capability path without displaying anything real.

Exercisers are the acceptance tests for shell work: a capability isn't done
until its exerciser behaves correctly in both shells.

## 9. Visual regression

**Matrix:** every registered component × its named variants × theme
{light, dark} × density {comfortable, compact} — snapshots of the isolated
render route (`/components/{slug}/preview?...`). Variants only, never the
full knob space. Components whose CUE metadata sets `densityInvariant: true`
skip the density axis. Pattern demos get a small set of hand-picked stable
states, not full coverage.

**Determinism rules**, enforced by the preview route: fixed viewport,
embedded fonts only, animations and transitions disabled
(reduced-motion forced), fixture data from the registry (no clocks, no
randomness — `Values` fixtures are part of the variant definition).

**The manifest is generated, so coverage is automatic:**
`/api/visreg/manifest` emits JSON enumerating every snapshot URL + stable
baseline ID, derived from `catalog.All()`. Registering a component *is*
opting into the CI gate; there is no second list to forget. The manifest is
also agent-legible (D15) — "what does the kit look like" is one GET.

**Runner:** Playwright, executed only inside a pinned CI container image
that also runs locally via the same container for baseline updates. Node
exists nowhere else — not in the build, not in the dev loop, not in any
shipped artifact; this is the scoped D2 exception accepted in
`../adr/0012-gallery-living-style-guide.md`. Baselines are committed PNGs under
`apps/gallery/visreg/baselines/`; CI fails on diff; updating baselines is an
explicit task that re-renders inside the same container. If the exception
ever grates, the manifest-driven design makes the runner swappable — the
spec is "walk the manifest, screenshot, compare," nothing Playwright-shaped.

**The cheaper sibling gate** runs without any browser: boot the Gallery
binary, walk every route in the manifest plus every section index, assert
HTTP 200 and parseable HTML. This is the smoke half of the acceptance
harness and runs on every commit; the snapshot half can run on a slower
cadence if CI time demands.

## 10. The TailwindPlus browser (later rung)

A `/twplus` section backed directly by the D10 SQLite index (657 components
× ~12 variants, `Section.Group.Category.Name` taxonomy; contentless FTS5
BM25 + sqlite-vec 768-dim embeddings, graceful degradation to BM25 when no
embedder is reachable — all per D10):

- **Search** — query box bound to a signal, POST on change, results
  re-rendered via the stream; facet filters by section/group/category. Same
  hybrid query path the Assistant uses (D11) — one index, two front ends.
- **Preview** — the stored component HTML (html framework, Tailwind v4,
  system mode) rendered inside a sandboxed iframe, isolated from kit CSS.
- **Adapt into kit component** — the workflow that turns reference material
  into kit material:
  1. POST `adapt` with the component's full name and a chosen slug/category.
  2. The handler scaffolds `appkit/ui/<slug>/`: `<slug>.templ` seeded with
     the TailwindPlus markup, a `<slug>.cue` metadata stub, and a TODO brief
     (map hardcoded classes to kit tokens, excise any JS in favor of
     Datastar or an approved island, define knobs and variants).
  3. The developer+agent pair finishes the adaptation; `appkit gen catalog`
     registers it; the component appears in `/components` and enters the
     visreg gate. Vendor-once-into-the-kit, per D13.

Licensing fence: the TailwindPlus corpus is licensed material and the
Gallery is private (VISION non-goal: parts may be opened later, this data
never). If any part of the Gallery is ever opened, `/twplus` and the D10
index stay behind a build tag that excludes them entirely.

## 11. Shells: the Gallery exercises the platform contract

The Gallery ships and runs two ways from one binary, because proving that is
its job (D5):

- **Web app:** `gallery serve` → loopback listener, open a browser tab. The
  permanently supported zero-install tier; also the dev loop
  (`templ generate --watch --proxy` + wgo, D4).
- **Wails shell:** the same binary under the Wails v3 wrapper
  (≥ v3.0.0-alpha.99, 2026-06-10, D6), webview pointed at the loopback URL,
  SSE over real TCP — never the asset transport (the Windows WebView2
  buffering fact behind D5,
  https://github.com/MicrosoftEdge/WebView2Feedback/issues/3519).

The Gallery is the standing answer to "does the platform contract hold":
identical behavior in both shells except where the capability matrix (§8)
says otherwise, on all three desktop OSes. It is also the reference output
of the kit scaffold — `apps/gallery/main.go` stays boring on purpose,
because new apps are stamped from the same shape.

## 12. Build order

1. `appkit/ui/catalog` types + `appkit gen catalog` codegen against the
   first 3–4 real components (button, input, card, dialog-trigger).
2. Gallery skeleton: layout, nav, `/stream`, tab KV plumbing.
3. Component pages: preview + knobs (the §5 loop), source/usage/variants.
4. Tokens section + theme/density switching.
5. Smoke gate (manifest walk) in CI; then the Playwright container and
   first baselines.
6. Two pattern demos (list + detail with live updates, long-running job).
7. — v1 line —
8. Service exercisers, Home, Playbook, remaining demos, wire view.
9. TailwindPlus browser + adapt workflow.

Steps 1–6 are deliberately small: the hand-rolled gallery is the prevailing
ecosystem pattern (templUI and goship.it each maintain one as a side
artifact of their library, https://github.com/templui/templui,
https://github.com/haatos/goshipit, verified 2026-06), but the research
also warns this is a build-it-yourself area — budget real effort. The basic
catalog is the small part; the typed-knob registry, the visreg manifest,
and the platform-contract proof are the deliberate extra cost — the three
things nobody else's gallery does and the kit cannot live without.
