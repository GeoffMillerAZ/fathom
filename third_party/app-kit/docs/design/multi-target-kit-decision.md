# Multi-target kit — the decision record

> **Provenance.** Decided 2026-08-07, in the app-kit multi-target architecture session, against the working tree as it stood that day; adjudicating three competing designs — `design:core-bindings` (won the spine), `design:layered`, `design:patterns-first`.
> **What became binding.** This document's DELIVERABLE 2 was ratified verbatim as [ADR-0017](../adr/0017-multi-target-layered-kit.md); its DELIVERABLE 3 replaced the "UI/UX stack" section of the global `CLAUDE.md` (in-repo copy: [global-claude-ui-section.md](global-claude-ui-section.md)). Only the ADR is binding — this file is the reasoning behind it.
> **Why it is kept.** §2 (where the three designs disagree, and why the loser is wrong) and §7 (what to contribute first, what waits, what never) are not in the ADR and cannot be reconstructed from the code. Everything below §0 is verbatim as decided; the ADR text itself is not duplicated here.

---

I read the working tree before deciding — and it has moved since the proposals were written. That changes the decision materially, so the verdict is stated against reality, not against the recon.

---

# DELIVERABLE 1 — THE DECISION

## 0. Ground truth the three designers did not have

Verified in the working tree on 2026-08-07, all uncommitted:

| Path | State | Consequence |
|---|---|---|
| `/Volumes/m2ext/code/geoffmilleraz/app-kit/appkit/ui/motion/{motion.css,motion.go,motion.js,motion_test.go}` | untracked, written 13:16–13:18 today | **The motion gap is already closed**, with a better design than any of the three proposed |
| `/Volumes/m2ext/code/geoffmilleraz/app-kit/appkit/ui/vizframe/` + 4 goldens | untracked, 13:15–13:16 | **The provenance gap is already prototyped**, unwired, and its package doc says *"ADR-0017 decides whether and how it is adopted"* |
| `/Volumes/m2ext/code/geoffmilleraz/app-kit/docs/design/provenance-rule.md` | untracked | A full enforcement study that hands ADR-0017 three explicit open questions |
| `/Volumes/m2ext/code/geoffmilleraz/app-kit/appkit/ui/tokens/{contrast.go,contrast_test.go,roles_test.go}` | untracked | WCAG measurement in Go, no browser, no Node; `TestNoDanglingRoleReferences` already exists |
| `/Volumes/m2ext/code/geoffmilleraz/app-kit/appkit/ui/explainer/explainer.go` | modified | Reduced-motion escape added at line 288; documented exemption from `ui/motion` in the package doc |
| `/Volumes/m2ext/code/geoffmilleraz/geoffmiller-cloud/spa/components/Spec*.jsx` (7 files) + `Specializations.jsx` | **all untracked**, `specializations` feature flag **default off**, `SpecFinOpsRate.jsx` last written 13:19 today | **The motivating visuals are not signed off, not shipped, and under active authorship this hour** |
| `spa/router.js` vs `appkit/ui/router/router.js` | md5 `ad73ef48ea3726d06946595ce0acf296` both | Still a byte-identical untracked copy |
| synthwave `:root` vs `[data-theme="dark"]` `--ak-color-*` sets | md5 `35349abafe6c6d6e2586070993af70fe` both | The `@custom-variant dark` bug is **real** |
| `docs/theming-hazards.md` | **does not exist**, referenced by `contrast.go:11` | A hazard doc is already load-bearing on committed-to-be code |
| `cmd/appkit-mcp/kitdocs.go:71-79` | `guidance()` returns **`app-kit/CLAUDE.md` verbatim** + a `D1–D15` footer | Fixing the global CLAUDE.md alone is provably insufficient; the MCP's guidance *is* `app-kit/CLAUDE.md:102-107` |

**This reorders the brief's own priority 3.** The resume site's *visuals* cannot go into the kit yet — they are a day old, flag-gated, and being edited hourly. What can go in, today, is the **technique** the visuals produced, and two-thirds of that is already sitting in the working tree waiting for an ADR to ratify it. The architecture's job is therefore to make that ratification correct and to build the channel the *pictures* will travel down later — not to plan a port of files nobody has approved.

---

## 1. The verdict

**Spine: `design:core-bindings`.** It is the only proposal that leads with the mechanism decision and defends it: *Go is the single implementation; its output crosses every target boundary as a build-time artifact — CSS, SVG, JS — never as a runtime.* That one sentence is what makes a multi-target kit maintainable by one person, and everything else in the design follows from it. It also names the split the kit already has by accident (var-themed `viz` vs utility-themed everything-else), which is worth more than any layer anyone could invent.

**Grafted in:**

- From **`design:layered`**: the alias direction and the fact that **`kit-alias.css` is hand-authored and owned by the consuming app**; the framing that a Go build tool in a non-Go repo *is* ADR-0013's mechanism rather than an exception to it; the "promote at the lowest layer that carries it" formulation; the no-literal-color test as a load-bearing invariant rather than tidiness.
- From **`design:patterns-first`**: **the hazard list** — the single best idea in any of the three, and the only one that is pure guidance with zero dictation surface. It now has a mandatory home: `contrast.go` already cites `docs/theming-hazards.md`, which does not exist. Also `geometry: "none"` as a legitimate, recorded classification (the reach chart), and the insistence that a rule the MCP cannot serve does not exist (D15).

**Rejected outright:** an npm package; a JSX emitter; a Go→React codegen path; a neutral Scene/IR JSON layer; WASM; parallel Go+JS geometry with a golden referee; generalizing `EmitThemeCSS` to N variant axes; renaming the SPA's 4,598 lines of CSS; a terminal target; measuring adoption by component-import count.

---

## 2. Where they disagree — decided, with why the loser is wrong

### 2.1 The Scene / geometry-IR layer — **NO.** `layered` loses.

`layered` makes `appkit/ui/scene` + `appkit/ui/geom` two of its five layers and budgets 7 of 23 days for them. It is wrong for three reasons, and its own document concedes the first two.

1. **There is nothing for it to prevent.** `layered` justifies the Scene as the thing that stops drift between a Go and a JS implementation. But both states of every one of these visuals are in the DOM at all times and CSS interpolates between them — the browser computes no geometry, ever. With one implementation running at build time there is no second copy to drift. The Scene is a solution to a problem the architecture already deleted.
2. **It is the most dictating artifact proposed anywhere in the three documents**, which fails priority 1 head-on. An app that wants a slightly different mark must fork the geometry or fight a schema. `layered` sees this ("the layer most at risk of *dictating*") and answers with an additivity rule — a mitigation, where the right answer is not to build the layer.
3. **`layered` itself says to gate it.** Its §9 marks S5–S6 as "the part most likely to overrun and the part with the least current demand," to be gated on S2 proving useful. A layer you would gate on evidence is not a layer; it is a maybe. Ship the maybe as a build-time SVG string (§4.3 below) — same benefit, no format, no schema, no versioning surface, no second renderer.

`patterns-first` gets closest to right here by demoting `scene` to a per-pattern opt-in rather than a layer. That is the correct *disposition* — but its `appkit/patterns/` scaffolding, `pattern.cue` schema, and six-section `PATTERN.md` template are 4 days of new taxonomy for the same content. The hazard list is the part of that layer worth building; the taxonomy is not.

### 2.2 The token alias — **hand-authored in the consuming app.** `core-bindings` loses.

`core-bindings` proposes `appkit gen -aliases` generating `spa/ak-aliases.css` from a `spa/ak-aliases.cue` alias token set. `patterns-first` proposes the same via a `spa/tokens.cue` whose every value is `var(--accent)`. Both are wrong for the same reason: they put a file containing **no values** into CUE, which is the one thing D7 exists to hold, and they make the app's divergence into a kit feature — a new emitter mode, a new generated artifact, and a kit that now has an opinion about how an app aliases.

`layered`'s version is 25 lines of hand-written CSS in `spa/kit-alias.css` that the site owns, that aliases one direction only (kit role → site variable), that is additive, reversible by deletion, and adoptable per-component. It inherits all five of the SPA's variant axes for free because it points at *variables*, not values. That is what "divergence must be cheap and legible, not a fight with the framework" looks like in a file.

The one thing `core-bindings` is right about — that a new required kit role should fail the consumer's build loudly rather than render transparent — is bought instead by `appkit vendor -check` plus the kit's existing `TestNoDanglingRoleReferences`, at zero new machinery.

### 2.3 Provenance as a `Source` field on all 15 `viz.*Options` — **NO.** All three lose.

All three proposals independently recommend adding `Source string` to the fifteen `*Options` structs and call it "additive, zero breakage." The working tree's `docs/design/provenance-rule.md` §5 Option C already refuted this, and it is right:

> The guard would then have to live **inside each render function**, so `viz.Bars` returning `""` for a missing source would silently blank all 41 existing call sites on the version bump. **That is a breaking change wearing an additive disguise** — and ADR-0017 should be warned about this shape specifically, because it is the one that will be proposed as "the small change."

It is also the shape that fails "guide, do not dictate" at the worst point: `stat.go:157`'s three-pixel sparkline inside a KPI tile and `hitl/approval.go:94`'s confidence pip row would both acquire a provenance field they can never meaningfully fill. Ritual provenance is worse than none, because it trains the reader to skip the source row.

**Adopt the sibling-package shape already in the tree** (`appkit/ui/vizframe`, Option A): `viz` unchanged, 0 of 41 call sites touched, one predicate exposed at three strengths — `Validate() error` (inspect), `Render() string` (guard, returns `""`), `MustRender() string` (panic). The kit ships the rule; the app picks the enforcement.

### 2.4 The motion vocabulary — **ratify the tree; all three proposals lose.**

All three designed `ak-m-*` with a reduced-motion block keyed on `[class*="ak-m-"]`, and two of them clamped the stagger with CSS `min()`. The implementation in the tree beats both on the two things that matter:

- **The cancel is welded to the grant.** `motion.css` scopes every `transition-*` declaration to `.ak-motion`, and `.ak-motion` is the exact selector the reduced-motion block cancels — *including transitions the consumer wrote themselves inside it*. `motion_test.go` fails `task check` if a future rule declares a transition outside that scope. You cannot opt into kit motion without opting into the cancel. A `[class*=]` attribute match is a broad selector that also catches an app's own `ak-m-`-prefixed classes and proves nothing.
- **`MaxStep = 8` makes an unbounded stagger inexpressible, not clamped.** A `min(calc(...), cap)` clamp lets a caller pass index 216 and quietly truncates. A `Step(i)` that tops out at 8 makes "648ms of tail across 216 nodes" a thing you cannot write. Prefer inexpressible to clamped.

Also ratify `explainer.go`'s documented exemption (single-file contract, decorative infinite loops, carries its own escape) — it is the right shape for an exception: named, reasoned, and in the package doc where the next editor will hit it.

### 2.5 Which prior ADRs are touched — **fewer than any of the three claimed.**

All three proposals asserted amendments to ADR-0011, ADR-0012 and ADR-0013. All three are wrong, and inventing those amendments would be the exact failure the brief warns against.

- **ADR-0012 needs no amendment.** All three read "a component is not in the kit until it has a Gallery page and a baseline" as blocking a non-Go contribution. The Gallery is a Go app that serves HTML. A page rendering a static specimen of `motion.css`, or a `vizframe` figure, *is* a Gallery page. Nothing is blocked. The gate stands as written and `ui/motion` and `ui/vizframe` must each pass it.
- **ADR-0013 needs no amendment.** 0013 says apps consume by import and propagation is `go get -u`. `geoffmiller-cloud` already contains `tools/jsxbuild` — a Go program with its own `go.mod` that builds the site's JS. A sibling Go tool in the same repo running `go get -u github.com/geoffmilleraz/app-kit` **is 0013's mechanism, executed**, in a repo whose shipped site is not Go. 0013 does not need widening; it needed noticing.
- **ADR-0011 needs no amendment.** `cmd/appkit-mcp/kitdocs.go:71-79` shows `get_kit_guidance` returns `app-kit/CLAUDE.md` verbatim. Fixing `app-kit/CLAUDE.md:102-107` fixes the MCP. The `D1–D15` footer at `kitdocs.go:79` is a hardcoded string that is now two behind — a bug fix, not an architecture decision.
- **ADR-0003 needs no amendment.** It binds what the kit *builds*. A React app importing `theme.css` is not a kit UI; it is a consumer of a generated artifact. ADR-0016 already established that the invariants govern kit apps.

**Real supersessions, both of them:** **ADR-0002** (its §3 "JavaScript is never written as source" is contradicted by four hand-authored kit JS files, `motion.js` now among them — retiring a stated clause is a partial reversal, and this repo's README rules partial reversals to be supersessions) and **ADR-0016** (0017 generalizes its one-off static-client carve-out into a named target class, and restates its hard boundary verbatim).

---

## 3. The architecture

**One sentence:** *The kit is a stack of offers, produced entirely in Go and CUE, published as build-time artifacts. A consumer takes any prefix and stops. Stopping is a complete adoption.*

Two constraints make that true rather than aspirational, and both are constraints on **the kit**, not on apps — which is what "guide, do not dictate" means when written as something testable:

> **The kit must never require a consumer to run Node, and must never emit an artifact only a bundler can consume.** Emitted CSS is valid in a `<link>`. Emitted JS is a classic script registering one global. Emitted data is JSON. Emitted markup is a string.

> **No kit artifact emits a literal color.** Every color reaches the page as `var(--ak-color-<role>)` or a role utility. This is what makes aliasing (§4.2) work at all, and it is already asserted twice in the tree (`vizframe`'s `TestRender_NoLiteralColors`, `tokens/roles_test.go`).

### 3.1 The layer map, real paths

| L | Name | Real paths | Language | Contract | Who can take it |
|---|---|---|---|---|---|
| **L0** | Token source | `cue/tokens/kit.cue`, `cue/tokens/synthwave/synthwave.cue`, `appkit/ui/tokens/schema.cue` | CUE | `#Tokens`: 23 required roles + the open-role rule `[=~"^[a-z][a-z0-9-]*$"]: #ColorPair` | the generator only |
| **L1** | Portable CSS | `appkit/ui/tokens/theme.css`, `appkit/ui/themes/*/theme.css`, `appkit/ui/motion/motion.css` | generated CSS + hand-authored CSS | `--ak-*` custom properties under documented selectors; fixed class names; **no literal colors**; ≤ one class of specificity; no preflight, no resets | anything with a `<link>` |
| **L2** | Portable browser JS | `appkit/ui/{router,overlay,chat,motion}/*.js` | hand-written ES2017 classic script | zero deps, no build step, one `window.*` global, `//go:embed` + `Script()`/`Handler()` | any `<script>` |
| **L3a** | Var-themed Go renderers | `appkit/ui/viz/*` | Go | `func(data, Options) string`; **zero `class=` in the package**; color only via `cssColor(role)` → `var(--ak-color-*)`; **needs no Tailwind scanner** | any Go program — including a build-time tool inside a non-Go repo |
| **L3b** | Utility-themed Go renderers | `appkit/ui/{callout,card,chrome,dashboard,explainer,export,form,hitl,kanban,stat,table,topology,vizframe}` | Go | emits Tailwind role utilities; consumer **must** run `appkit css` with an `@source` glob over the kit's Go source | Go apps that run the kit CSS build |
| **L4** | templ components | `*.templ` / `*_templ.go` in the L3b packages | templ | D4, D5 | Go server-rendered kit apps only |
| **L5** | Discovery | `cmd/appkit-mcp/*`, `docs/ui-components.md`, `docs/theming-hazards.md` (new), `docs/design/*`, `docs/adr/*` | Go + markdown | `get_kit_guidance` returns `app-kit/CLAUDE.md` verbatim (`kitdocs.go:71`) | agents, mid-task |

**The L3a/L3b split is the most useful thing in this map and it already exists undocumented.** `appkit/ui/viz` has zero `class=` and themes purely through custom properties, so a kit chart drops into a page with no Tailwind and no build. `appkit/ui/topology/tree.go` emits nine literal utility strings and `vizframe` emits `rounded-lg border border-line bg-surface-raised p-4`. Today the docs imply everything is L3b, which is why the first React adopter would conclude the kit is broken. Name it; make the no-literal-color test the thing that stops a component picking a side by coin-flip.

### 3.2 New things this decision creates

```
cmd/appkit/vendor.go            appkit vendor -out DIR --include … [--check]  + kit.lock
appkit/ui/themes/synthwave/synthwave.go   6 lines of //go:embed so themes ride in `go mod vendor`
docs/theming-hazards.md         the hazard list  (already cited by tokens/contrast.go:11)
appkit/ui/motion/count.js       the useCountTo tween  (currently duplicated byte-for-byte across two site files)
```

That is the whole build. Everything else is ratification, wiring, and documentation.

---

## 4. How each consumer shape wires in

### 4.1 Go server-rendered apps — throughline, `racli serve`, portal, conjure, car-assistant, growing-gardens-bi, studio

**Nothing breaks. No import path moves. No `appkit/ui/*` API changes. Zero required edits on day one.** Three things become available and one gets fixed:

- `@import ".../appkit/ui/motion/motion.css";` after the theme import; markup writes `class={ motion.RootClass(collapsed) }`, `class="ak-when-a ak-tint"`, `style={ motion.DelayStyle(d) }`.
- `vizframe.Render(svg, vizframe.Options{...})` around any chart that carries a quantity.
- `tokens.Contrast` / `ContrastOver` / `ContrastRoles` — measurement in `go test`, no browser, no Node.
- **Fixed:** `/Volumes/m2ext/code/geoffmilleraz/resume_as_code/cmd/serve/web/build-css.sh` currently hardcodes `$HOME/Library/Caches/app-kit/tailwindcss/v4.3.0/tailwindcss-macos-arm64`, bypassing `tailwind.Resolve()`'s per-call SHA-256 re-verification, **and** hard-fails without a sibling `../../../../app-kit` checkout because `appkit/ui/themes/synthwave/` contains only `theme.css` and no `.go` file, so `go mod vendor` cannot carry it. Both die at once: add `synthwave.go` (6 lines, `//go:embed theme.css`), point `input.css` at the vendored tree, and replace the script body with `go run github.com/geoffmilleraz/app-kit/cmd/appkit css -i input.css -o app.css --minify`. Thirty lines deleted, a platform pin deleted, a checksum restored.

### 4.2 The resume SPA — React 18 UMD, no bundler, no `package.json` anywhere

Takes **L0 + L1 + L2** and stops. Never touches L3b or L4.

```
spa/kit/theme.vars.css     vendored — the portable half of the synthwave palette
spa/kit/motion.css         vendored — the motion vocabulary
spa/kit/motion.js          vendored
spa/kit/count.js           vendored
spa/kit/router.js          vendored — spa/router.js is DELETED; the copy becomes a dependency
spa/kit/kit.lock           module version + per-file sha256
spa/kit-alias.css          HAND-AUTHORED, OWNED BY THE SITE. ~25 lines. Not generated. Not the kit's.
```

`kit-alias.css` declares the kit's vocabulary in terms of the site's, one direction only:

```css
:root {
  --ak-color-surface:        var(--bg);
  --ak-color-ink:            var(--text);
  --ak-color-ink-muted:      var(--text-mute);   /* NOT --text-faint: 2.16:1 */
  --ak-color-accent:         var(--neon-pink);
  --ak-color-line:           var(--line);
  /* … */
}
```

Because the aliases point at **variables**, all five of the SPA's root axes (`data-theme`, `data-aesthetic`, `data-accent`, `data-density`, `data-grid`) come along at use time for free, with **zero kit changes**. The kit's single hardcoded variant axis — correctly flagged in recon as the architectural blocker — simply never comes up, because the kit is not the one doing the theming. Delete the file and every kit artifact stops being themed; nothing of the site's changes.

**Propagation:** `tools/kitsync/` — a sibling to the existing `tools/jsxbuild/`, its own `go.mod`, `GOWORK=off go run .`, `-check` mode exiting 1 on drift, run from `spa:check` in CI. Upgrading is `go get -u github.com/geoffmilleraz/app-kit` **in that directory** — ADR-0013's mechanism, unamended, in a repo whose shipped site is not Go.

**One line of code changes in the build.** `tools/jsxbuild/main.go`'s asset regex matches flat paths or `components/` only; widening the path segment to `(?:[A-Za-z0-9_.-]+/)*` puts every vendored kit file inside the existing `?v=` digest and under the existing `spa:check` staleness gate. The SPA's own freshness discipline then guards the kit dependency for free.

### 4.3 pumpkin — React 19 + Vite 8 + Tailwind ^4.2.2

Takes **L1** and stops. Keeps Recharts, Headless UI, three.js, its own components. Gains one thing: it themes and animates identically to every Go app.

```bash
go run github.com/geoffmilleraz/app-kit/cmd/appkit vendor \
    -out ui/src/kit --include tokens,motion --dark-variant none
```

`ui/src/index.css` — today exactly `@import "tailwindcss";` — becomes three lines. `npm run build` unchanged. `package.json` untouched; that is pumpkin's business and always was.

**The one real hazard, and why the default protects against it.** `EmitThemeCSS` (`tokens.go:163`) unconditionally emits `@custom-variant dark (&:where([data-theme="dark"], [data-theme="dark"] *))`, re-keying Tailwind's built-in `dark:` from `prefers-color-scheme` to an attribute nothing in pumpkin sets. Every existing `dark:` utility silently stops responding to the OS, with no error. Hence `--dark-variant` defaults to `none` for `vendor` (and stays `attribute` for `gen`, so all seven Go consumers are byte-unchanged). Two footnotes: `ui/tailwind.config.cjs` is dead code — Tailwind v4 does not auto-load it without an explicit `@config` and pumpkin has none — and the kit pins v4.3.0 while pumpkin resolves ^4.2.2, which is fine for `@theme inline`/`@custom-variant` (v4.0) but means **vendored output must never use `@source inline(...)`** (v4.1+). That is a constraint on the emitter, written into its doc comment.

**If pumpkin ever wants a kit visual:** it injects a prerendered SVG string emitted at build time by a Go tool. `dangerouslySetInnerHTML` around a self-contained SVG is the entire binding. No IR, no format, no runtime math, no second implementation.

---

## 5. The promotion path and its bar

### 5.1 The reconciliation

D13's "two consumers" and Geoff's "anything really cool" are not in tension once you notice **D13 was written when the kit had one layer.** Two consumers is the right bar for *an API you will support forever*. It is the wrong bar for a color role, a CSS class, a hazard paragraph, or a pure function — all of which are additive, namespaced, individually ignorable, and cheap to delete. **The bar scales with the layer.** That is not a compromise with D13; it is the correct generalization of the rule D13 already states.

| Layer | Bar to promote | Why that bar |
|---|---|---|
| **Hazard / rule** (`docs/theming-hazards.md`) | **one occurrence + the measured evidence** | Cost is one paragraph. Losing the reasoning is the expensive outcome, and it cannot be re-derived from code. |
| **L0** token role | one consumer; must be a *role*, not a value | Open-role schema already permits it. Precedent: throughline's `crit-*`/`line-soft`/`accent-ring`, resume_as_code's `neon-pink`. |
| **L1** CSS class / motion rule | one consumer + a Gallery specimen + a legible no-motion end state | Unused CSS costs bytes, not API surface. |
| **L2** browser JS | one consumer + `Script()`/`Handler()` + a `kit.lock` entry + listed in `docs/ui-components.md` | Additive, but it is code with a lifetime. |
| **L3a** var-themed renderer | one consumer + a golden + passes no-literal-color | Still just a string function. |
| **L3b / L4** component | **two consumers — D13 unchanged** — + a Gallery page + an upgrade note + an a11y contract | An API with options and a permanent support obligation. This is exactly what D13 was protecting, and it still is. |

**And: promote at the lowest layer that carries it.** A color decision becomes a token. A lesson becomes a hazard. An interaction becomes CSS plus a class vocabulary. A calculation becomes a Go function. Only a whole rendered thing with options becomes a component.

### 5.2 The seven steps

1. **Name the layer.** If you cannot, it is L4 and you need a second consumer.
2. **Strip what does not generalize** — measured constants, per-state prose, app-specific gating. They stay in the app.
3. **Write the artifact at that layer**, obeying that layer's prohibitions (no literal colors; ≤ one class of specificity; no preflight; classic script, one global).
4. **Add the fixture** — golden SVG/HTML for L3, a `motion_test.go`-style structural assertion for L1.
5. **Add the Gallery page** (ADR-0012, unamended).
6. **Delete the app's copy and consume the kit's.** Not optional. A promotion where the app keeps its version is a fork with extra steps — and this is the step that would have caught `spa/router.js` fourteen months ago.
7. **Record it** — a row in `docs/ui-components.md`; an upgrade note for L3b/L4; a `CHANGELOG` line for L0–L3a.

### 5.3 Demotion, because a kit needs one

A kit artifact with **zero consumers for two releases** moves back down to the app that wanted it, or is deleted, and the removal is recorded. Demotion at L0–L3a is nearly free, which is *why* the bar there can be one consumer.

### 5.4 Divergence is the system working, not failing

`platform-engineering/portal` imports kit `callout` six times **and** ships its own second callout, with the reason in the file header: the corpus mechanism needs a CDN script, dead twice over under `script-src 'none'` (ADR-0039) and the ADR-0027 air-gap realm. It calls this "the standing move for this corpus." `car-assistant` has ~29 local `statusChip`/`dealerPill`/`baitBadge` functions despite being a heavy kit consumer — that is the kit's heaviest user filing a feature request for a badge primitive, not disobedience.

Formalize: a divergent component carries a `DIVERGENCE:` header naming the kit thing it declined and why; `docs/divergence.md` collects them. **Two divergences with the same reason is a kit bug with a queue position.**

**Adoption is measured by L1 reach — already 6 of 8 consumers — never by component-import counts.** Counting imports scores the SPA at zero and portal's correct divergence as a defect. Measuring by compliance is precisely the failure the brief names.

---

## 6. The staged migration

Single maintainer, conservative days. **S0–S2 is the minimum viable slice** and each stage is independently useful on the day it lands.

| # | Stage | Days | Contents | Gate |
|---|---|---|---|---|
| **S0** | **Ratify what exists** | **2.0** | Commit `ui/motion`, `ui/vizframe`, `tokens/contrast.go`, `tokens/roles_test.go`, `docs/design/provenance-rule.md`, the `explainer.go` escape. Gallery pages for motion + vizframe (ADR-0012, unamended). Add `@source "../../../appkit/ui/vizframe/*.go"` to `apps/gallery/ui/input.css`, rebuild + commit `apps/gallery/ui/app.css`. Rows in `docs/ui-components.md`. The §4.3 coverage test: every `<figure class="ak-vizframe">` in the Gallery has a source line. | `task check` green; Gallery renders both; coverage test fails on a deliberately unsourced figure |
| **S1** | **Governance** | **1.0** | ADR-0017. `docs/adr/0002` + `0016` status lines → `Superseded by ADR-0017`; ADR index rows; `docs/DECISIONS.md` D2/D16 rows + a D17 row. **`app-kit/CLAUDE.md:102-107`** (the MCP's guidance source). `kitdocs.go:79` `D1–D15` → `D1–D17`. **Geoff's global CLAUDE.md.** | `get_kit_guidance` returns the new text; `get_kit_decision("0017")` resolves via `kitdocs.go`'s auto-discovery with no registration |
| **S2** | **Distribution** | **2.0** | Tag **v0.1.0** (there is no released version; all seven consumers pin `v0.0.0` via `replace`, so a lockfile has nothing honest to record). `appkit vendor` + `kit.lock` + `--check` + `--dark-variant`. `themes/synthwave/synthwave.go` embed. | `vendor --check` round-trips; a deliberate edit exits 1 |
| **S3** | **Go consumers de-risked** | **1.5** | `racli serve`: vendor the synthwave theme, delete the four-level sibling-checkout `@import`, replace `build-css.sh`'s hand-rolled binary resolution with `appkit css`. Same `@source` treatment for throughline. Prove the channel on the low-risk target first. | `racli serve` rebuilds CSS with no sibling checkout present |
| **S4** | **SPA takes L1 + L2** | **2.0** | `tools/kitsync`; `spa/kit/*`; hand-written `spa/kit-alias.css`; delete `spa/router.js`; widen the `jsxbuild` asset regex (1 line); `spa:vendor:check` joins `spa:check`. | Site renders pixel-unchanged; `kit:check` fails on a deliberate edit to a vendored file |
| **S5** | **pumpkin takes L1** | **0.5** | Vendor into `ui/src/kit`; two `@import` lines; delete the dead `tailwind.config.cjs`; `vendor -check` in the lint script. | pumpkin's existing `dark:` utilities still follow the OS under `--dark-variant none` |
| **S6** | **Hazards + the first real contribution** | **2.0** | `docs/theming-hazards.md` — H-001…H-005 with measured evidence (it is already cited by `contrast.go:11`). `appkit/ui/motion/count.js` (the `useCountTo` tween), promoted and the site's two copies deleted. `--ak-polarity` emitted per branch, **derived by the generator from the token set's own surfaces**, plus the synthwave `dark:` fix. | Both SPA copies of `useCountTo` gone; a synthwave page with no `data-theme` no longer paints dark with `dark:` utilities off |
| **S7** | *(gated)* **First visual's geometry** | **2.0** | Only after Geoff signs off on the visual and the file stops moving. The endpoint mesh is the smallest: three constants, four coordinate functions, two class names. Lands as a Go function + a build-time SVG string, never as a Scene format. | Site's mesh pixel-equivalent; exactly one implementation of the arithmetic exists |
| | **Core total (S0–S6)** | **11.0** | | |

**Sequencing constraints, not preferences:**

- **S0 before S1.** An ADR that ratifies uncommitted files is an ADR about a rumour.
- **S1 must land `app-kit/CLAUDE.md` and the global CLAUDE.md in the same commit.** `kitdocs.go:71` returns `app-kit/CLAUDE.md` verbatim as `get_kit_guidance`, and the guidance instructs agents to read it first. Fixing one file silently regresses.
- **S2 before S4.** The SPA must not acquire a *second* untracked copy.
- **S6's `--ak-polarity` work is a coordinated commit.** `task check` gates on `git diff --exit-code -- appkit/ui/tokens`, so the CUE change plus every regenerated artifact across both token sets must land together. It cannot be split.
- **S7 is gated on Geoff, not on engineering.** See §7.

---

## 7. What to contribute from the resume site FIRST, and what waits

Every `Spec*.jsx` is **untracked**, the `specializations` feature flag defaults **off** (`spa/data.js`), and `SpecFinOpsRate.jsx` was written at 13:19 today. `SPECIALIZATIONS-CONTRACT.md` shows five pages under parallel authorship. These are drafts, not decisions.

### Contribute now, in this order — none of it depends on a visual surviving review

1. **The reduced-motion contract and the "both states legible without motion" invariant.** Already `appkit/ui/motion/`. It is technique, not picture. The three things worth naming in its docs: kill interpolation never the end state; *suppress unrequested state changes entirely* rather than merely un-animating them (an instantaneous unrequested jump is worse for a vestibular reader, not better); and departure is demotion, not deletion — the after state must still show the hole the removal made.
2. **`docs/theming-hazards.md`.** Five entries, each with its measured evidence, none palette-specific *as a rule*: **H-001** never gate a contrast decision on `data-theme` (a variant attribute declared later at equal specificity remaps surfaces back; measured consequence was ground at 8.51:1 against figure at 3.39:1 — the idle background brighter than the lit foreground — and it shipped twice); **H-002** `@custom-variant dark` silently disables an app's existing `dark:` utilities; **H-003** departure is demotion, not deletion; **H-004** a caveat set quieter than the claim it qualifies is dishonest — `ink-faint` is banned for provenance lines; **H-005** total stagger is bounded, never proportional to n. `contrast.go:11` already cites this file. It does not exist. That is the cheapest highest-value contribution available.
3. **The provenance rule, ratified.** ADR-0017 adopts `vizframe` Option A, decides the three open questions, and the Gallery coverage test makes the kit hold itself to the rule in public before it asks anyone else to.
4. **`useCountTo` → `appkit/ui/motion/count.js`.** Byte-identical duplicate across `SpecFinOps.jsx` and `SpecFinOpsUtil.jsx`, whose own comment admits the duplication. No React, no DOM assumptions, 23 lines. The single cheapest correct promotion in the whole survey, and it is L2 where the bar is one consumer.
5. **`--dark-variant` + `--ak-polarity` + the synthwave fix.** Driven by H-001/H-002, not by any visual.
6. **The colour *discipline*, never the colour values.** "A colour is measured, not judged" ships as `tokens.Contrast`/`ContrastOver`/`ContrastRoles` plus the `MeaningFloor`/`TextFloor` constants — already written. Add the `ink-faint` ban to the `vizframe` package doc.

### Wait for Geoff to review

- **Every `Spec*` geometry** — the 216→18 endpoint mesh, the sprint-ledger `colX` lane arithmetic, the reach chart, the two-lane provenance diagram. All four files are untracked, flag-gated, and three of them changed within four hours of this writing. Extracting geometry from files under active authorship collides on exactly the regions being moved.
- **All measured contrast constants** — `color-mix(in oklab, var(--neon-cyan) 58%, #000)` was solved against a `#df65b3` ground (3.14:1 at 58%, 2.54:1 at 66%). Those numbers are properties of *that palette on those surfaces*. They carry measurement comments, which makes them read as authoritative and makes them a trap. Port the discipline; re-measure with `tokens.Contrast`.
- **The `data-aesthetic` axis**, and any generalization of `EmitThemeCSS` to N variant axes. `kit-alias.css` solves it in 25 lines the app owns.
- **Any `Spec*` CSS block as a kit stylesheet.** The rules are contributable now; the stylesheets are drafts.
- **The page-level draft banner** (`callout.Draft`). The provenance study flags that its lede must be per-record overridable, because "a banner that disclaims a real number is exactly as dishonest as a number with no source" — and the site's own banner is still being tuned.

### Do not contribute at all

- **`SpecAgentic.jsx`'s diagram.** Zero computed geometry — `<DgxBox x={14} y={155} w={138} h={64}>` and `<path d="M152 171 C174 171 174 102 192 102">` are hand-placed literals throughout. A faithful Go port of a hand-placed bezier diagram is a strictly worse artifact than what exists. Record it in the hazard doc as `geometry: none — and why`, so nobody spends a weekend rediscovering it.
- **The reach chart as SVG.** Its bar length is `gridColumn: "1 / span " + 2*(i+1)` — a CSS-grid span, chosen so labels stay selectable and a narrow viewport restacks instead of scaling into illegibility. Emitting coordinates would produce a different, worse chart. It is the counterexample that keeps the geometry question honest.
