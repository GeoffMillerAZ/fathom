# ADR-0017: The kit is multi-target and layered — it guides, it does not dictate

- **Status:** Accepted
- **Date:** 2026-08-07
- **Decision drivers:** the kit already serves two rendering targets and is
  already shipping code into the second one by copy, undetected; one
  maintainer, so any cross-framework shared code must earn its drift cost
  against a 7:1 consumer asymmetry; two capability gaps (motion, provenance)
  were closed by a consuming app before the kit, which is evidence about which
  way contributions travel; a kit whose payoff requires every app to comply has
  already failed

## Context

Geoff's directive, in his own words: *"app kit can be expanded to have support
for all of our projects and while it should guide design, it shouldn't dictate
it"*, and *"can we make sure that anything really cool like this we are doing we
are contributing back to the app kit."*

The kit's posture until now has been that it is for Go server-rendered apps and
nobody else. That posture is contradicted by a file already in production.
`geoffmiller-cloud/spa/router.js` and `appkit/ui/router/router.js` are
byte-identical — md5 `ad73ef48ea3726d06946595ce0acf296`, 179 lines each. A React
18 CDN site has been running kit code for months, vendored by copy, with nothing
detecting drift. ADR-0016 authorised that artifact for that surface; what it did
not do was name the surface as a target the kit serves, so the kit acquired a
consumer without acquiring a relationship with it.

The consumer facts, counted rather than assumed:

- **Seven Go modules** import `github.com/geoffmilleraz/app-kit`, every one via
  a local `replace` against a working tree, every one pinning `v0.0.0`. There is
  no released version of the kit.
- **Six of those repos** additionally drive their CSS through `appkit css` and
  their palettes through `appkit gen` from per-app CUE token sets. Per-app brand
  divergence is already free and already exercised: throughline authors whole
  new role families (`crit-critical/high/medium/low`, `line-soft`,
  `accent-ring`), `resume_as_code` ships a bespoke `neon-pink`. **Guide-not-
  dictate is already working at the token layer; this ADR extends that posture
  upward rather than inventing it.**
- **One browser-JS consumer**, the resume SPA: React 18 UMD from unpkg, JSX
  compiled ahead-of-time by `tools/jsxbuild` (esbuild's Go API), three `<link>`ed
  stylesheets, 4,598 lines of hand-authored CSS keyed on its own custom-property
  vocabulary, five root variant axes, and **no `package.json` anywhere in the
  repo**. It is simultaneously a React app and fully ADR-0002-compliant. The
  inference "React implies Node" is disproven by the motivating case itself.
- **Not consumers, on any current evidence:** `pumpkin`, `agentic-eng-harness/
  apps/studio`, `signet-console`. Different React majors, different component
  stacks, zero app-kit strings between them, and none has asked. `fathom-studio`
  is bubbletea/lipgloss with no HTML and could consume nothing but a palette.

Two capability gaps were identified and, at the time of writing, were closed in
the working tree by lifting technique out of the resume site:

- **Motion.** The kit shipped transitions or animations in ten files and had
  zero `prefers-reduced-motion` support anywhere — a live WCAG 2.2.2 defect, not
  a gap. `appkit/ui/motion` now ships the vocabulary with the reduced-motion
  cancel welded to the class that grants motion, asserted mechanically by
  `motion_test.go`. `appkit/ui/explainer` documents its exemption and carries
  its own escape.
- **Provenance.** `appkit/ui/viz` had no provenance concept — one comment in
  `gauge.go` and nothing structural. `appkit/ui/vizframe` is an unwired
  prototype whose package doc defers to this ADR; the full enforcement study is
  `docs/design/provenance-rule.md`.

Three facts about the kit's own shape decide the layering:

- `appkit/ui/viz` emits **zero** `class=` and resolves every colour through
  `cssColor(role)` to `var(--ak-color-<role>)`. A kit chart is already portable
  to any browser that has the custom properties; Tailwind is irrelevant to it.
- `appkit/ui/topology/tree.go` emits literal Tailwind utility strings nine
  times, and `appkit/ui/vizframe` emits role utilities throughout. Those require
  a consumer to run the Tailwind scanner over the kit's Go source.
- The kit therefore already contains a portable tier and a utility-class tier,
  and **nothing documents the split.** `get_kit_theming` even asserts "no inline
  color styles, use utilities only," which the kit's own `viz` violates on every
  element.

ADR-0002 §3 states that "JavaScript is never written as source and never built
by a toolchain." The kit ships four hand-authored browser JS files —
`chat.js` (194 lines), `overlay.js` (94), `router.js` (179), `motion.js`. The
clause is false as written, and a rule the kit breaks four times teaches nothing.
ADR-0002's own Boundaries paragraph already draws the line correctly: "This
decision does not forbid *consuming* artifacts that were originally built with
Node upstream (datastar.js itself is). It forbids us running that toolchain."

ADR-0016 carved out static clients for one artifact and ruled the Loopback
Invariant inapplicable to them. Its own revisit trigger anticipates a narrowing
ADR. That carve-out is now a category, not an exception.

## Decision

**The kit is multi-target and layered. It publishes artifacts, in layers, and a
consumer takes any prefix and stops. Stopping is a complete adoption, not a
partial one.**

### 1. The layers

| L | Name | Contract | Available to |
|---|---|---|---|
| L0 | Token source (CUE) | `#Tokens`; 23 required roles plus the open-role rule | the generator |
| L1 | Portable CSS (`tokens/theme.css`, `themes/*/theme.css`, `motion/motion.css`) | `--ak-*` custom properties and fixed class names; no literal colours; ≤ one class of specificity; no preflight or resets | anything with a `<link>` |
| L2 | Portable browser JS (`router`, `overlay`, `chat`, `motion`) | zero dependencies, no build step, one `window.*` global, `//go:embed` with `Script()`/`Handler()` | any `<script>` |
| L3a | Var-themed Go renderers (`appkit/ui/viz`) | `func(data, Options) string`; colour only via `var(--ak-color-*)`; **no Tailwind scanner required** | any Go program, including a build-time tool inside a non-Go repo |
| L3b | Utility-themed Go renderers (the other `appkit/ui/*`) | emits Tailwind role utilities; the consumer must run `appkit css` with an `@source` glob over the kit | Go apps running the kit CSS build |
| L4 | templ components | D4, D5 | Go server-rendered kit apps only |
| L5 | Discovery (`cmd/appkit-mcp`, `docs/`) | knowledge queryable mid-task (D15) | agents |

Adopting layer *N* obliges a consumer to nothing at *N+1*. **A consumer that
takes tokens and CSS and no components has adopted the kit fully, and the kit
measures its own reach by layer-weighted adoption — already six of eight
consumers at L1 — never by component-import counts.** The L3a/L3b split is named
here because it already exists undocumented and a consumer must be told which
tier it is taking.

### 2. Guide, do not dictate — stated as constraints on the kit

The principle is enforceable only if it binds the kit rather than exhorting
apps. Three invariants, all testable:

- **The kit must never require a consumer to run Node, and must never emit an
  artifact only a bundler can consume.** Emitted CSS is valid in a `<link>`.
  Emitted JS is a classic script registering one global. Emitted data is JSON.
  Emitted markup is a string.
- **No kit artifact emits a literal colour.** Every colour reaches the page as
  `var(--ak-color-<role>)` or a role utility. This is what lets a consuming app
  alias the kit's vocabulary onto its own without the kit knowing, and it is
  already asserted by `vizframe`'s `TestRender_NoLiteralColors` and by
  `tokens/roles_test.go`.
- **No kit CSS rule exceeds one class of specificity, uses `!important`
  outside an accessibility floor, or ships preflight.** An app overrides any
  kit rule with one class in a later stylesheet.

**What this forbids the kit from doing**, concretely:

- Shipping an artifact whose only consumption path is a component. If a
  capability is adoptable only by importing `appkit/ui/x`, it has failed.
- Enforcing an editorial rule inside a shared renderer. A guard inside
  `viz.Bars` would be a silent runtime disappearance, invisible in a diff and
  impossible to decline. Opinions live in wrappers a consumer can refuse.
- Requiring provenance, or any other field, as a positional constructor
  parameter. Go has no non-zero-able field, so "required" means a signature
  change that breaks all 41 in-repo call sites and taxes the legitimately
  unsourced case — the Gallery's own specimens, a decorative sparkline in a KPI
  tile — with no escape that is not a lie. `Source: "TODO"` type-checks.
- Growing a variant axis, a token category, or an emitter mode to serve one
  consumer's theming when that consumer can alias in a file it owns.
- Treating a documented divergence as a failure. `platform-engineering/portal`
  imports kit `callout` six times and ships its own second callout because CSP
  `script-src 'none'` and the air-gap realm make the corpus mechanism unusable.
  That is the system working. Divergences carry a `DIVERGENCE:` header and a row
  in `docs/divergence.md`; two divergences with the same reason are a kit bug
  with a queue position.

### 3. What "no Node" now means, and what it no longer means

ADR-0002 conflated four rules under one slogan. They are separated permanently:

- **Kit build hygiene — stands verbatim, restated from ADR-0002 §4.** CSS is
  produced by the Tailwind v4.3.0 standalone binary, pinned by SHA-256 per
  platform in `internal/tailwind/manifest.cue`, cached under `os.UserCacheDir()`,
  **re-verified on every invocation**, deleted and hard-failed on mismatch. No
  npm, no `node_modules`, no lockfile, no PostCSS, no `tailwind.config.js`. This
  is what the rule protects: a clean-machine build that produces the same bits
  on any day. It is untouched by this ADR and is strengthened by routing
  `resume_as_code/cmd/serve/web/build-css.sh` back through `appkit css` instead
  of its hand-rolled binary path.
- **No `package.json` in a kit repo — stands, with the undefined term now
  defined.** "A kit repo" means **the app-kit monorepo, and any repo whose
  primary artifact is a Go binary or an `http.Handler` tree.** It does not bind
  a consumer repo that is not a Go app.
- **ADR-0002 §3's "JavaScript is never written as source" — retired,
  because it is already false.** Replaced by: *kit-authored browser JavaScript
  is permitted at rung L1 of the escape-hatch ladder
  (`docs/design/ui-architecture-patterns.md` §3) — purely ephemeral view
  manipulation, no state outliving the element, deleting it loses polish and not
  correctness. It must be dependency-free, a classic script registering exactly
  one global, require no build step, be `//go:embed`-ed with a `Script()`/
  `Handler()` pair, and appear in `docs/ui-components.md`.* **Third-party
  JavaScript still enters only as a vendored, version-pinned, SHA-256-recorded
  artifact** — that half of §3 was always the real rule and it stands unchanged.
- **What the rule never meant: nothing in it says a Node-built application may
  not consume the kit's outputs.** ADR-0002's own Boundaries paragraph settles
  it. A React/Vite app importing `theme.css` and `motion.css` is consuming
  generated artifacts, exactly as permitted, and is compliant.

### 4. Propagation: ADR-0013 is unamended and already sufficient

ADR-0013 says apps consume the kit by import, never by copy, and that adoption
is `go get -u`. A repo whose shipped site is not Go still consumes by import,
through **a Go build tool inside that repo**. `geoffmiller-cloud` already
contains `tools/jsxbuild` — its own `go.mod`, `GOWORK=off go run .`, with a
`-check` mode that already gates staleness. A sibling tool that imports the kit
and materialises its L1/L2 artifacts makes `go get -u` the upgrade path for a
target with no `go.mod` in its site. **ADR-0013 does not need widening; it
needed noticing.**

The kit adds `appkit vendor -out DIR --include … [--check]`, which writes the
requested L0/L1/L2 artifacts plus a `kit.lock` recording the module version and
a SHA-256 per file, and exits 1 on drift. This mirrors ADR-0002's own sanctioned
treatment of `datastar.js`. `spa/router.js` — today a byte-identical copy with
nothing detecting drift — becomes a checksummed dependency with a CI gate.

`appkit vendor` defaults to `--dark-variant none`. `EmitThemeCSS` emits
`@custom-variant dark (&:where([data-theme="dark"], …))` unconditionally, which
re-keys Tailwind's built-in `dark:` from `prefers-color-scheme` to an attribute a
consuming app may not set — silently removing that app's dark mode with no
error. `appkit gen` keeps `attribute` as its default, so all seven Go consumers
are byte-unchanged.

Prerequisite: **cut v0.1.0.** There is no released version; every consumer pins
`v0.0.0` through a `replace`. A lockfile with nothing honest to record is
theatre. `replace` directives stay for local development.

### 5. The promotion bar scales with the layer; D13 is preserved intact

D13's "if two apps need it, it lives in the kit" was written when the kit had one
layer, and it is the right bar for **an API with options and a permanent support
obligation**. It is the wrong bar for a colour role, a CSS class, a hazard
paragraph, or a pure function — all additive, namespaced, individually
ignorable, and cheap to delete. The bar is therefore indexed to the layer, which
is the correct generalisation of the rule D13 already states rather than a
compromise with it:

| Layer | Bar |
|---|---|
| Hazard or rule (`docs/theming-hazards.md`) | one occurrence, with its measured evidence |
| L0 token | one consumer; must be a *role*, not a value |
| L1 CSS / L2 JS | one consumer, plus a Gallery specimen and a legible no-motion end state |
| L3a var-themed renderer | one consumer, plus a golden, plus passing the no-literal-colour test |
| **L3b / L4 component** | **two consumers — D13 unchanged — plus a Gallery page, an upgrade note, and an a11y contract** |

**Promote at the lowest layer that carries it.** A colour decision is a token. A
lesson is a hazard entry. An interaction is CSS plus a class vocabulary. A
calculation is a Go function. Only a whole rendered thing with options is a
component. A promotion that leaves the original in place has not promoted
anything; it has forked. An artifact with zero consumers for two releases is
demoted back to the app that wanted it, or deleted, and the removal is recorded.

### 6. Provenance is adopted as a sibling package, with three strengths

`appkit/ui/vizframe` is adopted **as it stands** — a separate package that
imports nothing from `appkit/ui/viz` and is imported by nothing. `viz` changes
by zero lines; 0 of 41 call sites are touched. One predicate, three reactions,
and the consumer picks:

```go
func (o Options) Validate() error              // inspect — wire into your own test
func Render(body string, o Options) string     // guard   — returns "" rather than lie
func MustRender(body string, o Options) string // panic   — dev builds and golden tests
```

The three questions `docs/design/provenance-rule.md` §4.5 left open are decided
here rather than inherited:

- **`Render` returns `""`, not a loud "UNSOURCED" band.** The blank is honest —
  the figure genuinely should not be seen — and cannot be mistaken for a real
  chart. The residual worry that a blank region is invisible to review is
  answered by the coverage assertion below, not by putting an ugly artefact on a
  production page. `MustRender` is the development-time loudness.
- **The figure/mechanism split stays.** `Options{}` (zero value) is
  `CarriesFigure` and strict; `CarriesMechanism` waives the rule and is visible
  in a diff, in a grep, and in the DOM as `data-viz-carries="mechanism"`.
  Demanding a source line for a box-and-arrow drawing of your own architecture
  produces a ritual line, and a ritual line is worse than no line because it
  trains the reader to skip the source row — destroying the rule exactly where
  it was needed. **Silence is strict; the escape hatch is one you have to write
  down.** The boundary is genuinely fuzzy and this ADR says so.
- **Kind labels stay non-caller-overridable, and the frame stays English-only.**
  If an app could relabel `Estimate` as `Verified` the rule would be decorative.

**Adding `Source` to the fifteen `viz.*Options` structs is rejected**, and is
named here because it is the change that will be proposed as the small one. It
is additive at compile time and a breaking change in effect: the guard would
then live inside each render function, silently blanking 41 call sites on the
version bump. It also multiplies one rule by fifteen and collides with the
figure/mechanism split — `stat.go:157`'s three-pixel trend sparkline and
`hitl/approval.go:94`'s confidence pips would acquire a field they can never
meaningfully fill.

**The kit holds itself to the rule first.** ADR-0012 already requires a Gallery
page and a baseline; a sibling assertion — every `<figure class="ak-vizframe">`
on every Gallery page carries a source line — gates every chart the kit itself
authors. That is a property of a page rather than of a call, and only a test can
check it.

### 7. What this ADR supersedes, and what it deliberately does not

**Supersedes ADR-0002.** Its §3 first clause is retired; §3's third-party
vendoring rule, §4's pinned-Tailwind pipeline, and §5's package.json ban are
restated above and remain binding. The rest of ADR-0002 — Go as the only
general-purpose language, CUE as the only schema language, the determinism
payoff, the boundaries — is unchanged and continues to govern.

**Supersedes ADR-0016.** Its one-off carve-out becomes a named target class.
Its hard boundary is restated verbatim and remains in force: *"this router
exists ONLY for static-client surfaces with no server router. It MUST NOT be
used in kit apps that have a real HTTP server."* So does its ruling that the
Loopback Invariant governs kit apps and a static CDN-deployed site is not one.

**Amends nothing else, and the declines are deliberate:**

- **ADR-0012 is not amended.** The Gallery is a Go app that serves HTML; a page
  rendering a static specimen of `motion.css`, or a `vizframe` figure, *is* a
  Gallery page. The gate stands as written and both new packages must pass it.
- **ADR-0013 is not amended.** See §4.
- **ADR-0011 is not amended.** `cmd/appkit-mcp/kitdocs.go:71` returns
  `app-kit/CLAUDE.md` verbatim as `get_kit_guidance`. Correcting that file
  corrects the MCP. The hardcoded `D1–D15` footer at `kitdocs.go:79` is a bug
  fix, not an architecture decision, and `kitdocs.go`'s ADR auto-discovery means
  this document becomes queryable with no registration step.
- **ADR-0003, 0004, 0005 and 0007 are not amended.** They bind what the kit
  *builds*. A React app importing a generated stylesheet is not a kit UI; it is
  a consumer of an artifact. `motion.css` is hand-authored rather than generated
  and does not violate D7, because it derives from no token set and contains no
  values — every quantity in it is a `var(--ak-*)` reference.

## Alternatives considered

- **Publish an npm package of kit components.** Attractive: it is what a
  multi-target design system normally means, and it would let a React app
  `import { Callout }`. Lost because it is a whole new distribution channel, a
  new release process, and a semver contract against React majors, built for
  zero demand — the three npm-native React repos run different React majors and
  different component stacks and contain zero app-kit strings between them. The
  honest count is two targets at 7:1, and the second target is one site.
- **Shared component code across frameworks — an IR, an AST, a component
  schema, or a Go→JSX emitter.** Attractive: propagation would reach both
  targets automatically, which is the one thing this decision gives up. Lost
  because a component satisfying both Go string-building and React
  reconciliation converges on the worst of both — an intermediate nobody reads,
  an escape hatch for every real case, and two test suites — for one consumer,
  maintained by one person. `dangerouslySetInnerHTML` around a self-contained
  kit SVG already works today and costs one line.
- **A neutral Scene / geometry-IR JSON layer.** Attractive, and the most
  seductive option here: geometry computed once in Go, serialised, rendered by
  any target, with drift structurally impossible. Lost on three grounds. The
  consumer never computes — both states of these visuals are in the DOM at all
  times and CSS interpolates between them — so an IR buys nothing that a
  prerendered SVG string does not. It is the most *dictating* artifact available:
  an app wanting a slightly different mark must fork the geometry or fight the
  schema, which is the failure this ADR exists to prevent. And a golden fixture
  over it would assert on float formatting rather than mathematics —
  `GW/DAYS = 41.428571428571431` renders as `41.43` from Go's `fmtNum` and at
  full precision from JS, same pixels, different bytes. **Runtime geometry in
  JavaScript is declared out of scope**: the moment a visual needs a third
  computed state or a live client-side filter, it has become an application and
  belongs behind a Go server.
- **Generalise `EmitThemeCSS` to arbitrary selector-keyed variant axes**, so the
  kit could express the resume site's `data-aesthetic` / `data-density` /
  `data-accent`. Attractive: the SPA's five axes would then be first-class.
  Lost because it touches the schema, the CSS emitter, the DTCG shape and the Go
  names emitter simultaneously; throughline, synthwave and the Gallery all
  regenerate through it against `task check`'s byte-identity gate; and it would
  be a breaking change to shared code serving one consumer whose own cascade
  already resolves those axes. A ~25-line alias file **owned by that app**,
  pointing kit roles at the app's existing variables, inherits all five axes for
  free and is deleted in one keystroke.
- **Generate that alias file from a CUE alias token set (`appkit gen
  -aliases`).** Attractive: a new required kit role would then fail the
  consumer's build loudly instead of rendering transparent. Lost because it puts
  a file containing no values into CUE — the one thing D7 exists to hold — adds
  an emitter mode, and makes an app's divergence into a kit feature, which is
  backwards. The loud-failure property is bought instead by `appkit vendor
  -check` and the kit's existing `TestNoDanglingRoleReferences`.
- **Make provenance a required constructor parameter.** Attractive, and the only
  option that catches a consumer nobody will ever review — a genuinely stronger
  guarantee than anything adopted here. Lost because it makes the kit's
  editorial opinion an unappealable condition of using the kit's charts at all,
  breaks 41 in-repo call sites plus every downstream app, contradicts the viz
  package's own promise that `BarsOptions{}` is always a valid call, and buys
  syntax rather than honesty — a codebase that types `"TODO"` to satisfy a
  compiler has *worse* provenance than one that left the field empty, because
  the empty case is detectable and the lie is not. The steelman is real and is
  recorded as a revisit trigger below.
- **Do nothing; let each target solve it.** Attractive: zero scope creep, and
  the SPA is demonstrably capable of solving its own problems — it solved motion
  and provenance before the kit did. Lost because that is exactly how
  `spa/router.js` became an undetected copy, and ADR-0013 names copied code as
  the failure the kit was founded against.

## Consequences

- **Easier:** a React app, a static CDN site, and a Go server-rendered app all
  theme and animate identically, from one generated stylesheet, with no shared
  component code and no framework binding layer.
- **Easier:** partial adoption stops being a compliance question. Six of eight
  consumers are at L1 today and that is now the headline number rather than an
  embarrassment.
- **Easier:** contributions travel to the kit at the layer that fits them. The
  motion vocabulary and the provenance rule entered as CSS, a class vocabulary
  and a wrapper package — none of which needed a second consumer to justify.
- **Easier:** `spa/router.js` becomes a checksummed dependency with a CI gate,
  and `resume_as_code`'s CSS build stops requiring a sibling checkout and stops
  bypassing the Tailwind checksum verification.
- **Harder:** the kit now has two documented tiers of Go renderer (L3a var-themed,
  L3b utility-themed) and a consumer must be told which it is taking. Mitigation:
  the no-literal-colour test plus a row per component in `docs/ui-components.md`,
  so a component cannot pick a side by accident.
- **Harder:** artifacts must now be produced for two channels from one tree, and
  `appkit vendor -check` must be wired into each consumer's CI or the vendored
  copy drifts silently — the exact failure that has been running in production.
- **Risk: `appkit vendor` becomes a second distribution channel that rots.**
  Tripwire: any consumer's `-check` is red for more than one release, or a
  vendored file is edited in place → make the check a pre-push hook in that
  repo, or withdraw the layer from it.
- **Risk: "guide, do not dictate" is read as "anything goes", and L0–L3a
  accretes artifacts nobody uses.** Tripwire: two consecutive releases in which
  no consumer imports something promoted at the one-consumer bar → demote it
  (§5) and record the removal.
- **Risk: `Render` returning `""` hides an unsourced figure from review.** This
  is the failure mode by which the source site's two worst bugs survived two
  review rounds. Tripwire: any figure ships blank to production → switch the
  default to a loud band, which is a ten-line change.
- **Risk: the layer-indexed bar erodes D13 at L3b/L4 by precedent creep.**
  Tripwire: a component enters the kit with one consumer → it comes back out,
  and the ADR is amended rather than the rule quietly bent.
- **Risk: a React consumer takes L1 and gets none of the Go-side guardrails.**
  Nothing stops it setting `--ak-motion-dim: 0` and deleting the evidence a
  departure left behind. L1 consumption is guidance-only by construction and
  this ADR does not pretend otherwise; the hazard list is the mitigation and it
  is a document, not a mechanism.

## Revisit when

- **The consumer set stops being Geoff's apps** — a team, a second human
  contributor, or any public release. At that point the "catches consumers you
  will never review" property of a required provenance parameter becomes the
  dominant consideration and §6's trade must be re-opened, along with D13's
  pre-1.0 breaking-change freedom.
- **A second non-Go consumer appears with a real ask** — at which point npm
  distribution and per-framework bindings stop being built for nobody and
  deserve an honest re-costing.
- **A kit visual needs a third computed state or a live client-side filter.**
  That is the tripwire for runtime geometry, and the answer is still not an IR:
  it is that the visual has become an application and belongs behind a Go server.
- **The "kit app" and "static client" categories converge** (a kit app deployed
  as a static export) — ADR-0016's original trigger, inherited here.
- **A third distinct provenance shape appears.** Two exist today with genuinely
  different geometry — conjure's inline templ chip and a page-scale two-lane
  diagram — and that argues for two small things, not one abstraction that fits
  neither. A third is evidence the abstraction was real after all.
- **Tailwind discontinues or feature-freezes the standalone binary**, or an
  emitted artifact needs a v4.1+ feature (`@source inline(...)`) that a consumer
  pinning an older v4 minor cannot parse.
- **The kit needs to render in a language other than English**, which is when
  the non-overridable provenance kind labels have to be re-solved.
