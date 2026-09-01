# The provenance rule for kit charts — specification and enforcement study

**Status: input to ADR-0017, which is in flight. Nothing here is adopted.**
`appkit/ui/viz` is untouched by this work. The prototype lives beside it, in
`appkit/ui/vizframe/`, imports nothing from it, and is imported by nothing.

The rule comes from Geoff's résumé site, where it is load-bearing and where it
was arrived at by getting it wrong first: the first chart the site shipped was a
smooth spend-down curve against a smooth workload-up curve, "which is exactly
what a made-up chart looks like, and it read as made up"
(`geoffmiller-cloud/spa/components/Viz.jsx:34-43`). Two enforcement shapes exist
there and the split between them is the interesting part:

| Site component | Shape | Behaviour |
|---|---|---|
| `Specializations.jsx:168-201` `SpecMetric` | **structural** | a metric with no `source` **returns null and `console.warn`s**. "Not a lint rule, not a convention — the component has no code path that emits a number without a provenance line under it." |
| `Viz.jsx:120-131` `VizFrame` | **advisory** | `{source && <p className="vizchart-src">{source}</p>}` — renders the line if given one, says nothing if not. |

`Specializations.jsx:57-58` states the split as a deliberate choice: *"VizFrame
states this rule for charts and leaves it to review; here it is structural."*
`SpecSignature` (`:224-241`) then re-imposes the strict rule on the frame from
the outside, by refusing to call `VizFrame` without a source. That is the
prototype's shape: **the strictness lives in the wrapper, not in the renderer.**

How to use this document: §1–§3 are the specification ADR-0017 needs to adopt or
reject. §4 is the enforcement argument with a recommendation and its costs. §5
is the migration. §6 is the shared-file work an adoption would require, which
this run deliberately did not do.

---

## 1. What provenance is, and what qualifies

**A chart is a claim.** An unsourced claim is worth less than no claim, because
it invites the reader to wonder what else on the page was drawn rather than
measured. Provenance is therefore not attribution etiquette; it is the thing
that makes the chart worth drawing.

**Provenance is a sentence a reader can check the claim against, printed on the
page.** Three consequences fall straight out of that definition and all three
are enforceable:

1. **On the page, not in a tooltip.** The site's rule: "the renderers put that
   text on the page rather than in a tooltip". A source that requires a hover is
   a source that does not exist in print, in a screenshot, or for a keyboard
   reader who never hovers.
2. **A sentence, not a URL.** A bare link asserts nothing about what is behind
   it and rots. In the prototype `Href` is an *optional decoration on* `Text`,
   never a substitute for it, and `Render` drops an unsafe href while keeping
   the sentence (`TestRender_UnsafeHrefDropped`).
3. **Checkable, which means specific.** "Internal data" is not provenance. The
   per-kind precision tests are in §1.1.

### 1.1 The three kinds

The vocabulary is closed at three, and the claim is that these **exhaust** the
ways a number on a chart can be true. Either somebody else already put it in
public, or you can show your arithmetic, or you are asserting it on your own
authority. A fourth kind would not be a fourth way of being true; it would be a
way of not answering the question.

| Kind | Means | Passes when | Fails when |
|---|---|---|---|
| `KindPublished` | Someone other than the author put this number in public. | The text names the publication precisely enough to find it again — publisher, document, table/page, date. | "Industry reports." "Gartner." A vendor page that no longer exists. |
| `KindDerived` | Arithmetic the reader could redo. | The text names the **inputs** and where the working lives, not just the conclusion. | "Calculated from our data." A conclusion restated as if it were a method. |
| `KindEstimate` | The author's own figure — unpublished, underived. | The text says what the estimate is based on, and ideally what would change it. | Anything, if the frame does not print the word *Estimate*. |

**The estimate kind is the load-bearing one, and it must be cheap.** A rule with
only the first two kinds is unusable — every real deck contains numbers that are
somebody's considered judgement — and an unusable rule does not produce fewer
figures, it produces figures filed under `derived` that are not derived. Making
the honest option one call (`vizframe.Estimate("…")`) is what keeps the other
two honest. This is the same reasoning the site applies to its per-row
attribution, which prefixes the author's own figures with `"Geoff's own figure —
"` in the row itself (`Viz.jsx:232`, `:275`).

**The kind word is printed by the frame, not by the caller** (`kindLabel`, and
it is not caller-overridable). If an app could relabel `Estimate` as `Verified`
the rule would be decorative. The cost is that the frame is currently
English-only; see §4.5.

### 1.2 What explicitly does not qualify

- The chart's own axis label or title. Restating the quantity is not sourcing it.
- The commit message, the ADR, the alt text, the tooltip, the changelog.
- A page-level banner covering figures that individually differ. The site is
  emphatic here and it is worth porting verbatim as a principle: *"A banner that
  disclaims a real number is exactly as dishonest as a number with no source"*
  (`Specializations.jsx:118-120`) — which is why its banner lede is overridable
  per record. Provenance is **per figure**.

---

## 2. Draft as a render state

**Draft answers a different question from provenance, and the frame asks both.**
Provenance answers *where did this come from*. Draft answers *has it been
checked yet*. They are orthogonal: a published figure can be awaiting sign-off,
and an estimate can be final. So in the prototype `Draft` is a `bool` on
`Source`, not a fourth `Kind`.

**Draft never excuses provenance.** `Source{Draft: true}` with no kind and no
text does not render — `TestRender_DraftDoesNotExcuseProvenance`. There is always
a truthful sentence available for a genuinely unverified figure, and it is one
call: `vizframe.Estimate("Author's own estimate from three sampled months; not
yet reconciled.").Drafted()`.

**It is a render state, not a data flag.** A `placeholder: true` that only the
data file knows about is worth nothing. The frame changes what the reader sees:

- a dashed `caution` border on the whole figure,
- a `Draft` badge in the caption — **a word, plus a shape, plus a colour**.
  Colour alone fails for colour-blind readers, in greyscale print, and under any
  theme that remaps the palette. The site uses the same triple (amber + dashed +
  uppercase mono, `components.css:3103-3117`),
- `data-viz-draft="true"` on the `<figure>`, so an app can restyle or audit the
  state from CSS or the DOM without forking the Go. (Precedent:
  `chat/citation.go`'s `data-cite-kind`, and `Citation.Unresolved`, which is the
  closest thing the kit already has to this idea — an unresolved citation renders
  with a warning style and is "never silently dropped — the anti-fabrication
  signal".)

**Open question for ADR-0017.** Draft-ness is currently declared per figure. The
site also carries a page-level banner. A kit-level banner belongs in
`appkit/ui/callout`, not here, and its lede must be per-page-overridable for the
reason in §1.2 — otherwise the banner itself becomes a lie.

---

## 3. The distinction that turned out to be load-bearing: figure vs mechanism

The résumé site enforces the rule strictly on a **metric** (`SpecMetric`) and
leaves it advisory on a **frame** that might be wrapping a hand-drawn diagram
(`VizFrame`). That split is not an oversight; it is the only reason the strict
half survives.

**A chart carrying a quantity invites comparison with the world.** The reader
has no way to check it except your say-so, so your say-so has to be inspectable.

**A mechanism diagram — a flow, a topology, a state machine — asserts only "this
is how the thing is wired."** It is answerable by reading the thing. Demanding a
source line for a box-and-arrow drawing of your own architecture produces a
ritual line ("Source: the author"), and a ritual line is worse than no line: it
trains the reader to skip the source row, which destroys the rule exactly where
it was needed.

So the prototype makes the classification an explicit, greppable declaration:

```go
Options{Carries: vizframe.CarriesMechanism}   // no quantity; source optional
Options{}                                      // zero value = CarriesFigure = strict
```

Three deliberate properties:

- **Silence is strict.** The zero value is the strict case
  (`TestOptions_ZeroValueDefaultsToStrict`). A rule whose default is the lenient
  branch leaks.
- **The escape hatch is visible.** Waiving the rule requires typing
  `CarriesMechanism`, which shows up in a diff, in a grep, and in the rendered
  DOM as `data-viz-carries="mechanism"`. Compare the alternative — waiving it by
  leaving a field empty — which shows up nowhere.
- **It can still be abused,** and this is the honest weakness. `CarriesMechanism`
  on a chart with numbers on it is a lie the type system cannot catch. The
  mitigation is that it is a *lie you have to write down*, not an omission.

**The boundary is genuinely fuzzy** and ADR-0017 should say so rather than
pretend otherwise. A topology diagram captioned "216 nodes" is a figure. A funnel
with no printed numbers still encodes proportions by area. In today's kit,
14 of the 15 `appkit/ui/viz` render functions are quantity-carrying by
construction; the plausible mechanism cases are `viz.Steps` (a state sequence),
`appkit/ui/topology`, and `appkit/ui/explainer`. The two kit-internal viz call
sites — `stat.go:157` (`viz.Sparkline` as a 3-pixel trend inside a KPI tile) and
`hitl/approval.go:94` (`viz.Rating` as a confidence pip row) — are a third case
that is neither: **decorative micro-charts whose quantity is stated in words
right next to them.** Forcing a source line onto those would be absurd, and §5
treats that as evidence about *where* the rule can live.

---

## 4. Three enforcement mechanisms

For each: what it catches, what it misses, what it costs a consumer, and whether
it is compatible with *"app-kit should guide design, it shouldn't dictate it."*

### 4.1 A required struct field — omission is a compile error

**First, a correction that changes the argument: Go cannot express a required
struct field.** `Options{}` always compiles. Unexported fields plus a
constructor do not help — `viz.Source{}` is still a legal zero value; you get a
detectable zero, not a compile error. The only construction that genuinely
fails to build on omission is a **required positional parameter**:

```go
func Bars(bars []Bar, src Source, o BarsOptions) string
```

So "required struct field" honestly means "signature change", and that is what
must be costed.

**Catches:** omission. Completely, at build time, in every consumer — including
consumers the kit maintainer will never review. Nothing else on this list can
say that. It is the strongest guarantee available and it is not close.

**Misses:** everything about quality. `viz.Bars(rows, viz.Published("."), o)`
compiles. It catches the empty slot and never the empty gesture — and the empty
gesture is the actual observed failure mode. The site's real failure was not a
missing field; it was a plausible curve nobody had measured. Two rounds of human
review on that codebase also missed 1.30:1 and 1.66:1 contrast on marks carrying
the whole argument. A compile error is a guarantee about *shape*, and every
shipped provenance failure so far has been about *substance*.

**Cost to a consumer:** every existing call site breaks — 41 in this repo (39 in
`apps/gallery/render.go`, 1 in `appkit/ui/stat`, 1 in `appkit/ui/hitl`), plus
every downstream app, on the version bump. It also contradicts the viz package's
own stated house rule — data first, `Options` last, by value, "so `BarsOptions{}`
is always a valid call" — which is a promise the package makes on every one of
its 15 render functions.

**Compatible with "guide, do not dictate"? No.** This is the one clear verdict in
the section. It makes the kit's editorial opinion an unappealable condition of
using the kit's charts *at all*. An app that wants a decorative sparkline in a
table cell — which is `stat.Stat.Spark`, shipped, today — is now required to
invent provenance for a trend line whose value is printed two millimetres away.
"The kit makes the good choice the easy one, not the only one" is precisely the
sentence this mechanism violates.

**The steelman, which is real.** If app-kit ever ships outside Geoff's apps — to
a team, to the public — the "catches consumers you will never review" property
becomes the dominant consideration and this argument flips. The recommendation
below is contingent on the current consumer set, and ADR-0017 should record that
as its revisit trigger.

### 4.2 A runtime guard — a source-less chart refuses to render

**Catches:** a figure reaching a reader without a source. In every consumer,
including those that never run the kit's tests and never read its docs, at the
only moment that actually matters. It also catches the case a compile error
cannot: a source string that is present but empty, whitespace, or filed under a
kind that does not exist (`TestValidate_Cases`).

**Misses:** build time — you find out by looking. It misses quality for the same
reason as §4.1. And it can be routed around by not using the frame, which is a
miss and a feature at once (below).

**Cost to a consumer:** one field on an opt-in wrapper. Zero for a consumer that
does not adopt it, because it lives in a separate package that existing code does
not import.

**Compatible with "guide, do not dictate"? Yes — but only because of *where* it
lives, and this is the whole crux.**

- A guard **inside `viz.Bars`** (return `""` when `Source` is empty) would be
  *more* dictatorial than the compile error, not less: a silent runtime
  disappearance, invisible in a diff, unopt-out-able, and it would blank 41
  existing call sites the moment they upgraded. Strictly worse than 4.1 on every
  axis except the version number.
- A guard **in a wrapper you can decline** is the opposite. `viz.Bars` keeps
  returning an SVG forever. `vizframe.Render` is the thing with the opinion. Take
  the chart without the frame, or the frame without the chart — the frame takes a
  `string` body and knows nothing about viz. That is "an app must be able to take
  the CSS without the components", applied to an editorial rule.

The site's own split is the evidence: it put the structural guard exactly where
the component owned the number (`SpecMetric`) and left it advisory where the
component owned only the chrome (`VizFrame`).

**The residual worry, stated plainly: `return ""` is invisible to review.** A
blank region in a page is exactly the failure mode that got past two review
rounds on the source site. See §4.5, where this is an open question rather than
a settled one.

### 4.3 A lint or golden test — CI catches it

**First, the ground truth: the kit has no linter.** No `.golangci.yml`, no
golangci-lint. "Lint" in this repo means `go vet ./...` inside `task check`. So
this mechanism is really two different proposals:

- **A `go/analysis` pass.** New tooling, a new pin, a new gate, and it can only
  see call sites it can resolve statically — a `Source` built at runtime from a
  database row defeats it. High cost, partial coverage. Not recommended.
- **A test.** Near-free, and the kit already has the harness and the precedent:
  ADR-0012 states flatly that *"a component is not 'in the kit' until it has a
  Gallery page and a baseline"*, and `apps/gallery/gallery_test.go:122`
  `TestComponentsPageRendersEveryViz` already asserts a property over the whole
  rendered catalogue. A sibling assertion — *every `<figure class="ak-vizframe">`
  on every Gallery page has a source line* — is roughly fifteen lines and would
  gate every chart the kit itself authors.

**Catches:** kit-authored charts, at CI, with a message that names the file. It
also catches something neither other mechanism can express, because it is a
property of a **page** rather than of a call: *coverage*. "Every figure in this
catalogue is sourced" is a sentence only a test can check.

**Misses:** consumer apps entirely, unless each one writes its own version.
Nothing at runtime. And — the sharpest limitation — a test only catches what it
asserts, and "has a non-empty source string" is trivially satisfiable by a
period.

**Cost to a consumer:** zero. It binds the kit's output and nobody else's. The
cost lands on the kit maintainer, as one more test to keep true.

**Compatible with "guide, do not dictate"? Most cleanly of the three.** The kit
holds itself to the rule in public, in the Gallery, where ADR-0012 says agents
and humans both go to learn what the kit does. An app that wants the discipline
copies the test. Nothing is imposed on anyone. It is also, by exactly the same
property, the weakest guarantee on offer.

### 4.4 Position

**Adopt 4.2 and 4.3. Reject 4.1.** Concretely, that is what
`appkit/ui/vizframe` already is:

```go
func (o Options) Validate() error            // the predicate — wire it into your own test
func Render(body string, o Options) string   // the guard — returns "" rather than lie
func MustRender(body string, o Options) string // panic — dev builds, golden tests
```

**The argument that matters is that these are not three alternatives.** They are
three *reactions to one predicate*, and which reaction is right depends on facts
the kit does not have: whether this app's charts face the public or a dashboard,
whether a blank panel or a crashed page is the worse outcome, whether the team
has CI. So the kit should ship the predicate once and expose all three
strengths, and let the consumer choose the strength. That is the most complete
answer to "guide, do not dictate" available here: the *rule* is the kit's, the
*enforcement* is the app's.

**What this position costs, said honestly.** It gives up the only guarantee that
reaches a consumer nobody reviews. A team that imports `appkit/ui/viz`, never
imports `vizframe`, and never reads this document will ship unsourced charts and
nothing will stop them. That is a real loss and it is the price of not breaking
41 call sites and not forcing ritual provenance onto mechanism diagrams and
decorative sparklines. If the consumer set ever stops being "Geoff's apps", this
trade should be re-opened.

### 4.5 Where the evidence is genuinely balanced

Three things this study did **not** settle. ADR-0017 should decide them, not
inherit them.

1. **`Render` returning `""` versus rendering a loud "UNSOURCED" band.** `""`
   matches the site, is honest (the figure genuinely should not be seen), and
   cannot be mistaken for a real chart. But a blank region is *invisible to
   review* — which is the precise mechanism by which the source site's two worst
   bugs survived two review rounds. A loud band would be caught by a screenshot
   diff against the Gallery's visreg baseline, at the cost of putting an ugly
   artefact on a production page. I lean slightly toward `""` for production plus
   `MustRender` in development, but I do not think the argument is decided, and
   in the prototype it is a ten-line change either way.
2. **Whether the figure/mechanism split should exist at all.** The collapse
   argument is decent: one rule is easier to hold than two, "Diagram of this
   repository's own ingest path, drawn from `internal/ingest`" is a *true and
   useful* sentence, and the ritual-line worry may be imaginary. The split
   survives here only because the site kept it deliberately and appears to have
   benefited. That is one data point.
3. **Whether non-overridable kind labels are worth English-only rendering.** The
   labels are fixed so that `Estimate` cannot be renamed to `Verified`. A
   localisable variant would need a mechanism that translates without permitting
   re-editorialising, and I do not have a good one.

---

## 5. Migration sketch — if ADR-0017 adopts this

### Option A — sibling package (what the prototype is; recommended)

`appkit/ui/vizframe` stays where it is. It imports nothing from `viz`; `viz`
imports nothing from it. A consumer that wants the rule wraps:

```go
svg := viz.Bars(rows, viz.BarsOptions{AriaLabel: "spend by team"})
html := vizframe.Render(svg, vizframe.Options{
    Title:  "Spend by team",
    Source: vizframe.Derived("Sum of the FY25 billing export by tag; working in docs/spend.md."),
})
```

- **Changes in `appkit/ui/viz`: none.** Not one line.
- **Call sites forced to change: 0 of 41.**
- **Breaks consumers: no.** A new package is a pure addition; under D13's
  monorepo versioning it is a minor bump.
- **The trade:** opt-out by omission. A consumer who never wraps never gets the
  rule. §4.4 already accepted that.

### Option B — fold into `viz` as `viz.Frame`

Identical code, moved to `appkit/ui/viz/frame.go`, exported as
`viz.Frame(body string, o viz.FrameOptions) string`. One import less for the
consumer, and it puts the frame where someone browsing the chart package will
find it — a real discovery gain given §6.

- **Changes in `viz`: two new files, plus one edit to the package doc.**
  `viz.go:2` currently promises *"All functions return a plain string of SVG
  markup"*, which `Frame` would falsify. That is a shared-file edit and this run
  did not make it (see §6).
- **Call sites forced to change: 0 of 41.** Still purely additive.
- **The cost:** `viz` stops being one thing. Today it is a clean SVG generator
  with a single-sentence contract; afterwards it is a generator *and* an HTML
  chrome package with an editorial opinion. Options A and B are close, and the
  deciding question is discovery (§6) versus package coherence.

### Option C — a `Source` field on all 15 `*Options` structs (rejected)

Adding `Source Source` to `BarsOptions`, `GaugeOptions`, and so on is additive
at compile time — every existing `BarsOptions{}` literal still builds — which
makes it *look* like the cheapest option. It is not:

- The guard would then have to live **inside each render function**, so
  `viz.Bars` returning `""` for a missing source would silently blank all 41
  existing call sites on the version bump. **That is a breaking change wearing an
  additive disguise,** and ADR-0017 should be warned about this shape
  specifically, because it is the one that will be proposed as "the small change".
- Alternatively the field is added and *not* enforced, which is a documentation
  comment with extra steps.
- It also multiplies the rule by 15 instead of stating it once, and it collides
  head-on with §3: `stat.go:157`'s sparkline and `hitl/approval.go:94`'s rating
  pips would both acquire a provenance field they can never meaningfully fill.

### Adoption sequencing under A or B

1. Land the package (done, unwired) and an ADR-0017 that picks a shape.
2. Add a Gallery page — per ADR-0012, a component is not in the kit without one,
   and it is where the rule gets demonstrated rather than asserted.
3. Add the CI assertion of §4.3 over the Gallery's own charts. The kit holds
   itself to the rule first.
4. Wrap Gallery chart demos one at a time. Each is independent; none is forced.
5. Leave `appkit/ui/viz` alone throughout.

---

## 6. Shared-file changes an adoption would need (NOT made by this run)

Listed for whoever adopts. Each is a file owned elsewhere.

1. **`apps/gallery/ui/input.css`** — add `@source "../../../appkit/ui/vizframe/*.go";`
   to the glob block (precedent: lines 18–31). Without it Tailwind never sees the
   frame's utility classes and the frame renders unstyled, silently — the
   failure mode `cmd/appkit-mcp/setup.go:54-55` already calls "the most common
   'my `bg-accent` does nothing' cause". Then `task gallery:css` and **commit
   `apps/gallery/ui/app.css`**, which `task check` does not gate.
2. **`docs/ui-components.md`** — one table row for the frame. It is the kit's
   per-component catalogue; a component absent from it is undiscoverable.
3. **`apps/gallery/gallery.go` + `render.go`** — a `/provenance` route and a
   `build*` page (ADR-0012:66-67).
4. **`docs/adr/0017-*.md`** — the decision. `kitdocs.go:31-54` auto-discovers
   `docs/adr/NNNN-*.md`, so it becomes queryable through `list_kit_decisions` /
   `get_kit_decision` with no registration step.
5. **`cmd/appkit-mcp/setup.go`** or **`theming.go`** — one paragraph, if the rule
   is to reach agents. An agent reaching the kit through `get_kit_guidance` /
   `get_kit_css_setup` / `get_kit_theming` will not find a `docs/design/` file on
   its own, and this document is a `docs/design/` file.
6. **`appkit/ui/viz/viz.go:2`** — package doc edit, **only under Option B**.

---

## 7. The prototype

`appkit/ui/vizframe/` — `vizframe.go`, `vizframe_test.go`, four goldens in
`testdata/*.html.golden` (same `UPDATE_GOLDEN=1` contract as
`appkit/ui/viz/viz_test.go`, different extension because a frame is HTML).

Conformance with the viz house style: exactly one concept per file; `Options`
last and by value; zero value means default; `withDefaults()` on a pointer
receiver returning a value copy; every field's default in its doc comment; no
literal colours, asserted by `TestRender_NoLiteralColors`; every caller string
escaped at the point of use; deterministic, asserted; goldens in the kit's
convention.

What the tests prove, beyond the goldens:

- a figure with no source renders **nothing** (`…FigureWithoutSource_RendersNothing`);
- a mechanism diagram with no source renders **normally** (`…MechanismWithoutSource_Renders`);
- draft **does not** excuse provenance (`…DraftDoesNotExcuseProvenance`);
- the kind word is always printed and always matches the kind (`…KindLabels`, `…EstimateIsLabelled`);
- unsafe hrefs are dropped **while keeping the sentence** (`…UnsafeHrefDropped`);
- the viz body passes through byte-for-byte with its `role="img"`/`<title>`
  intact (`…BodyPassesThroughUntouched`) — the proof that adoption requires no
  change to any existing component.

`task check` passes with the package present.
