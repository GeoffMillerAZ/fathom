# multi-target-kit — ratify ADR-0017 and build the two-target channel

Turns the decision at `docs/design/multi-target-kit-decision.md` (task A-1 puts it
there) into work. The decision's own §6 staging is preserved; the stage letters
below map to it: **B**=S1 governance, **C/D**=naming the tiers, **E**=S2
distribution, **F**=S3/S4 consumers, **G**=S6 hazards, **H**=S0 gallery for the
ADR-gated half.

Gate: `fathom verify --service app-kit --plan docs/plans/multi-target-kit.md`
Run **without** `--strict`: section F is cross-repo and app-kit's index cannot
see it (each F task carries its own proving command instead).

---

## Findings this plan is built on

Each was verified in the tree on 2026-08-07, not inferred. If one of these turns
out false, the task that rests on it is void — that is the point of listing them.

1. **`geoffmiller-cloud/spa/router.js` and `appkit/ui/router/router.js` are
   byte-identical** — md5 `ad73ef48ea3726d06946595ce0acf296`, both. A React 18
   CDN site has been running kit code by copy for months with nothing detecting
   drift. This is the single fact that makes the kit multi-target *already*,
   rather than aspirationally. Re-check: `md5 …/spa/router.js …/router/router.js`.
2. **Seven Go modules import the kit via a local `replace` at `v0.0.0`; there is
   no released version.** `throughline`, `resume_as_code`, `conjure`,
   `car-assistant`, `growing-gardens-bi`, `platform-engineering/portal`,
   `geoffmiller-cloud/studio`. A `kit.lock` therefore has nothing honest to
   record until E-5 cuts a tag — which is why E-5 gates F.
3. **`pumpkin`, `agentic-eng-harness/apps/studio` and `signet-console` are NOT
   consumers on current evidence.** Zero app-kit strings, different React
   majors, no ask. **This is a two-target kit, not a five-target one**, and every
   npm/IR/codegen option is costed against that count.
4. **The kit already has a portable tier and a utility tier and documents
   neither.** `appkit/ui/viz` emits zero `class=` and resolves colour only
   through `cssColor(role)` → `var(--ak-color-*)`, so a kit chart needs no
   Tailwind and no build. `appkit/ui/topology/tree.go` emits literal Tailwind
   nine times, and `appkit/ui/vizframe` emits role utilities throughout. The
   split is real, load-bearing, and invisible. Tasks D-1/D-2 name it.
5. **`get_kit_theming` currently asserts the opposite of the kit's own
   behaviour.** `cmd/appkit-mcp/theming.go:249` writes *"No Node, no inline color
   styles. Use utilities only (D2)"* — which `appkit/ui/viz` violates on every
   element it emits. An agent that obeys the guidance cannot ship an L3a chart.
   Task C-1.
6. **ADR-0002 §3 is false as written.** *"JavaScript is never written as source
   and never built by a toolchain"* — the kit hand-authors four browser JS files
   (`chat.js` 194 lines, `overlay.js` 94, `router.js` 179, `motion.js`). A rule
   the kit breaks four times teaches nothing. Task B-2.

---

## Decisions taken

Judgement calls made on Geoff's behalf. Each names the one edit that reverses it.

1. **ADR-0017 is written verbatim from the decision doc; its content is not
   re-litigated here.** *Reverse:* edit `docs/adr/0017-multi-target-layered-kit.md`
   directly — it is a normal ADR once written.
2. **The decision doc is copied into the repo (A-1) before anything else.** Its
   §2 ("where they disagree, and why the loser is wrong") is not in the ADR and
   would otherwise die with the scratchpad. *Reverse:* delete
   `docs/design/multi-target-kit-decision.md`.
3. **No commit, no push, no tag in this plan.** E-5 (cut v0.1.0) is therefore
   `[-] deferred` and F is gated behind it. *Reverse:* Geoff tags, then flips E-5
   and starts F.
4. **`appkit vendor` is a subcommand of the existing `cmd/appkit` binary, not a
   new binary.** Consumers already `go run …/cmd/appkit css`; a second binary
   would be a second install story. *Reverse:* move `cmd/appkit/vendor.go` to
   `cmd/appkit-vendor/main.go`.
5. **`--dark-variant` is plumbed via a new `EmitThemeCSSOpts`, with
   `EmitThemeCSS` kept as a wrapper.** Changing the existing signature would
   ripple through `appkit gen` and trip `task check`'s byte-identity gate for no
   gain. `vendor` defaults to `none`, `gen` stays `attribute`, so all seven Go
   consumers are byte-unchanged. *Reverse:* one constant in `cmd/appkit/vendor.go`.
6. **The `get_kit_theming` contradiction is fixed by amending rule 5, not
   deleting it.** The no-Node half is correct and load-bearing; only the
   "utilities only" half is wrong. *Reverse:* revert one `WriteString` in
   `cmd/appkit-mcp/theming.go`.
7. **ADR-0002 gets `Superseded in part by ADR-0017` and keeps its text**, with
   the retired clause struck in place rather than deleted. A reader who arrives
   via an old citation must still find the sentence that was retired and why.
   *Reverse:* rewrite the status line and delete the strike.
8. **Doc tasks are gated on the heading existing in the index** (`min_tier:
   doc-claimed`), and the drift-prone ones additionally get a Go test. Prose with
   no assertion behind it is the failure mode that produced the `D1–D15` footer
   two decisions out of date. *Reverse:* drop the test, drop the annotation.
9. **pumpkin is not migrated (F-3 is `[-] deferred`).** Finding 3 says it is not
   a consumer; migrating it would be building a channel for nobody, which is the
   exact failure the ADR argues against. *Reverse:* flip F-3 to `[ ]` — the work
   is one `appkit vendor` invocation and two `@import` lines.
10. **The resume site's *visuals* are out of scope entirely.** Every `Spec*.jsx`
    is untracked, flag-gated off, and was edited within hours of the decision.
    The *technique* they produced is in scope (G-1, and the whole of
    `motion-provenance.md`). *Reverse:* add an S7 section once Geoff signs off
    and the files stop moving.
11. **No spec IDs on tasks.** The repo has no spec corpus; bracketed IDs that
    resolve to nothing would fail `--strict` for a bookkeeping reason.
    *Reverse:* add `[ADR-0017]` to each line once ADRs are registered as specs.

---

## Judgement calls made while implementing (2026-08-07)

Places where the plan or the decision doc was underdetermined, ambiguous, or
wrong about the tree. Each says what was done, why, and the one edit that
reverses it. Nothing here needed a question first; everything here is cheap to
undo.

1. **`H-004` → `H6`.** The plan and the decision doc cite hazard `H-004` ("a
   caveat set quieter than the claim it qualifies is dishonest"). The shipped
   `docs/theming-hazards.md` numbers its hazards **H1–H5** with entirely
   different content — `H4` there is "a token that does not exist fails
   silently". Citing `H-004` would have been a dangling reference in a doc whose
   whole point is that citations resolve. Added the hazard as **H6**, with its
   measured evidence, and cited that. *Reverse:* renumber in
   `docs/theming-hazards.md` and fix the two citations in
   `appkit/ui/vizframe/vizframe.go` and `vizframe_test.go`.

2. **`docs/adr/README.md`'s supersession process gained one exception.** Step 2
   said "edit **only** the status line… the body stays untouched", which
   decision 7 above directly violates. Rather than leave the repo contradicting
   itself, the process now permits an in-place strike for a *partial* reversal,
   with the reason. *Reverse:* delete that paragraph from
   `docs/adr/README.md`.

3. **ADR-0002's status reads `Superseded in part by ADR-0017`,** per decision 7
   — not the decision doc's own closing checklist, which said "status line only
   → `Superseded by ADR-0017`". The plan is the later and more specific
   instruction, and "in part" is the true statement: §4 and §5 of ADR-0002 are
   restated and still binding. *Reverse:* one line in
   `docs/adr/0002-go-and-cue-only-no-node.md`.

4. **`appkit vendor --dark-variant` strips the `@custom-variant` statement from
   the embedded `theme.css` rather than re-running the CUE pipeline.** A
   consumer running `go run …/cmd/appkit vendor` has no `cue/tokens/` checkout,
   so `EmitThemeCSSOpts` has no token set to emit from; `tokens.ThemeCSS()` (new
   `//go:embed`) plus `tokens.StripDarkVariant` is what actually works outside
   this repo. The two paths are proved equivalent by
   `TestStripDarkVariantEqualsNoneEmission`, so they cannot drift into two
   different notions of "none". *Reverse:* embed the CUE token sets inside
   `appkit/ui/tokens` and call `EmitThemeCSSOpts` from `vendorSets`.

5. **`--include` offers exactly the five groups the plan names** (tokens,
   motion, router, overlay, chat). `synthwave` is deliberately not vendorable:
   E-1 makes it ride in `go mod vendor`, which is the channel F-1 actually
   needs. *Reverse:* one entry in `vendorSets` in `cmd/appkit/vendor.go`.

6. **G-2 does more than emit `--ak-polarity`.** A custom property cannot be
   selected on in CSS, so emitting one alone could not deliver G-2's own
   acceptance criterion ("a synthwave page with no `data-theme` no longer paints
   dark with `dark:` utilities off"). `EmitThemeCSSOpts` therefore also **widens
   the `dark:` custom variant** to match an un-forced `:root` when the token
   set's *default* branch is dark-valued — derived from the same measurement, so
   the kit token set is unaffected. That is the line that fixes the bug;
   `--ak-polarity` and the per-branch `color-scheme` are what make it legible.
   *Reverse:* drop the `lightPolarity == "dark"` arm in
   `appkit/ui/tokens/tokens.go` and re-run `task gen` + `task gen:synthwave`.

7. **`explainer` and `export` are tiered L3b with a footnote.** Both are
   self-contained by contract — a single file carrying its own inline CSS/JS —
   so neither is really utility-themed. L3b is the strictest thing a consumer
   might assume, and the footnote states what is actually true. *Reverse:* the
   footnote line in `docs/ui-components.md`.

8. **B-5's D-range assertion cross-checks the register's `### DN` body sections,
   not its table.** Deriving the footer from the table and then asserting
   against the table would have been a tautology that passes on an
   inconsistent register. *Reverse:* the regexp in
   `TestDecisionsIndexCoversEveryADR`.

9. **`task check` fails at its final step, by construction.** Its last command
   is `git diff --exit-code -- appkit/ui/tokens`, and G-2's coordinated change
   (CUE emitter + both regenerated `theme.css` files) is exactly what that gate
   is designed to force into one commit — while the standing instruction for
   this pass was to commit nothing. Everything before it passes: `go vet ./...`,
   `go test ./...`, both `cue vet`s, and `task gen`. Regeneration was separately
   verified byte-stable across two runs. *Resolve:* commit the tokens change;
   the gate goes green with no code edit.

---

## A. Preserve the reasoning

- [x] Copy the multi-target decision into the repo verbatim under a 3-line provenance banner naming its date, author-session and the three designs it adjudicates; keep §0 ground truth through §7 contribute/wait/never intact {id: A-1, symbol: Multi-target kit — the decision record, package: docs/design, min_tier: doc-claimed, skip_test: true, skip: "markdown design record; the ADR carries the binding half and B-1 gates that"}
      → **proves it:** `find_symbol` resolves the h1 in `docs/design/multi-target-kit-decision.md`, and §2.1–2.5 (the five rejected alternatives with their refutations) are present — those are the paragraphs the ADR compresses away.

## B. Governance — ADR-0017 and the documents that must move with it

- [x] Write ADR-0017 verbatim from the decision's DELIVERABLE 2 into `docs/adr/0017-multi-target-layered-kit.md`; no edits, no re-litigation {id: B-1, symbol: "ADR-0017: The kit is multi-target and layered — it guides, it does not dictate", package: docs/adr, min_tier: doc-claimed, skip_test: true, skip: "ADR prose; B-3 and B-4 assert its wiring in Go"}
      → **proves it:** the h1 resolves at `docs/adr/0017-multi-target-layered-kit.md:1`; `get_kit_decision("0017")` resolves through `kitDocs.findDecision`'s auto-discovery with no registration step.

- [x] Retire ADR-0002 §3's first clause in place: strike *"JavaScript is never written as source and never built by a toolchain"*, replace with the L1-escape-ladder rule from ADR-0017 §3, keep §3's third-party-vendoring half unchanged, and set the status line to `Superseded in part by ADR-0017` {id: B-2, symbol: "§3 amendment — kit-authored browser JavaScript (ADR-0017)", package: docs/adr, min_tier: doc-claimed, skip_test: true, skip: "ADR prose; the heading only exists after the retirement is written, and B-3's test asserts the struck sentence is gone"}
      → **proves it:** B-3's `TestADRStatusLinesAreHonest` fails if the retired sentence survives anywhere in `docs/adr/0002-*.md` outside a strikethrough, or if the status line does not name 0017.

- [x] Add `TestADRStatusLinesAreHonest` to `cmd/appkit-mcp/kitdocs_test.go`: every ADR whose status says Superseded names a successor that exists on disk; ADR-0016's status names 0017; ADR-0002's retired clause appears only struck {id: B-3, symbol: TestADRStatusLinesAreHonest, package: cmd/appkit-mcp, skip_test: true, skip: "the deliverable IS the assertion"}
      → **proves it:** `go test ./cmd/appkit-mcp/` fails when ADR-0016's status line is reverted to bare `Accepted`.

- [x] Set ADR-0016's status to `Superseded by ADR-0017` (its one-off static-client carve-out becomes a named target class; its hard boundary is restated verbatim in 0017 §7 and stays in force) and add index rows for 0002/0016/0017 to `docs/adr/README.md` {id: B-4, symbol: "Superseded by ADR-0017 — the carve-out became a target class", package: docs/adr, min_tier: doc-claimed, skip_test: true, skip: "status-line + successor section; B-3 and B-5 assert the wiring"}
      → **proves it:** B-3 passes; B-5's test finds a README row for 0017.

- [x] Add `TestDecisionsIndexCoversEveryADR` to `cmd/appkit-mcp/kitdocs_test.go`: every `docs/adr/NNNN-*.md` has a row in `docs/DECISIONS.md` and in `docs/adr/README.md`, and `kitDocs.guidance`'s D-range footer matches the highest D-row — killing the hardcoded `D1–D15` string at `kitdocs.go:79` {id: B-5, symbol: TestDecisionsIndexCoversEveryADR, package: cmd/appkit-mcp, skip_test: true, skip: "the deliverable IS the assertion"}
      → **proves it:** `go test ./cmd/appkit-mcp/` currently fails on this test (footer says D1–D15, DECISIONS.md has D16) and passes after B-6.

- [x] Rewrite the D2 row and body in `docs/DECISIONS.md` to the four-clause split (pinned-Tailwind build hygiene: stands · no `package.json` in a kit repo, term now defined: stands · kit-authored browser JS at ladder rung L1: amended by 0017 · *"React apps can't use the kit"*: deleted, never true), repoint D16 at 0017, add a D17 row {id: B-6, symbol: D17 — Multi-target layered kit, package: docs, min_tier: doc-claimed, skip_test: true, skip: "decision-index prose; B-5's test gates its completeness"}
      → **proves it:** B-5 goes green; the heading resolves in `docs/DECISIONS.md`.

- [x] Replace `app-kit/CLAUDE.md`'s Path A/Path B section with the layered version (L0/L1 framework-neutral · L2 browser JS · L3a var-themed · L3b/L4 Go-only), derive the D-range footer in `kitDocs.guidance` from `docs/DECISIONS.md` instead of hardcoding it, and add `TestGuidanceIsCurrent` asserting the returned text names ADR-0017, `L3a`, and no stale D-range {id: B-7, symbol: TestGuidanceIsCurrent, package: cmd/appkit-mcp, skip_test: true, skip: "the deliverable IS the assertion; kitDocs.guidance already exists, so anchoring on it would pass before the work"}
      → **proves it:** `get_kit_guidance` returns text containing "ADR-0017" and "L3a" after `task mcp:build` + an MCP client restart. **`app-kit/CLAUDE.md` is returned verbatim by `kitdocs.go:71` — fixing only the global CLAUDE.md silently regresses on the next session's first MCP call.**

- [x] Replace the "UI/UX stack (all projects)" section of `/Users/geoff/.claude/CLAUDE.md` with the decision's DELIVERABLE 3 text, and keep a verbatim copy at `docs/design/global-claude-ui-section.md` so the only copy is not on an unversioned path {id: B-8, symbol: Global CLAUDE.md — UI stack section (verbatim copy), package: docs/design, min_tier: doc-claimed, skip_test: true, skip: "the live file is outside every repo; the in-repo copy is what the index can see"}
      → **proves it:** the in-repo copy's h1 resolves, and `diff <(sed -n '/^## UI/,/^## /p' ~/.claude/CLAUDE.md) docs/design/global-claude-ui-section.md` is empty modulo the copy's banner. **Land with B-7.**

## C. The contradiction the MCP is currently serving

- [x] Amend `themingGuide`'s rule 5 (`theming.go:249`) from *"No Node, no inline color styles. Use utilities only (D2)"* to the tier-aware form: utilities are the L3b path; `appkit/ui/viz` (L3a) emits presentation attributes carrying `var(--ak-color-*)` and that is correct, not a violation; what is banned in both tiers is a **literal** colour {id: C-1, symbol: TestThemingGuideDoesNotBanInlineColor, package: cmd/appkit-mcp, skip_test: true, skip: "the deliverable IS the assertion; themingGuide already exists, so anchoring on it would pass before the work"}
      → **proves it:** new `TestThemingGuideDoesNotBanInlineColor` in `cmd/appkit-mcp/theming_test.go` asserts the guide contains neither "utilities only" nor "no inline color styles", and does contain "L3a" and "var(--ak-color-". Without this an agent obeying the kit's own guidance cannot ship an L3a chart.

## D. Name the tier split that already exists

- [x] Add a **Tier** column (L1/L2/L3a/L3b/L4) to `docs/ui-components.md` and a row per package, including new rows for `motion` (L1+L2) and `vizframe` (L3b); state the L3a contract inline — zero `class=`, colour only via `var(--ak-color-*)`, no `@source` glob needed {id: D-1, symbol: TestUIComponentsDocCoversEveryPackage, package: cmd/appkit-mcp, skip_test: true, skip: "the deliverable IS the assertion"}
      → **proves it:** the new test walks `appkit/ui/*/` and fails if any directory lacks a row with a tier, so a future package cannot enter the kit tierless.

- [x] Add `TestVizEmitsNoClassAttribute` to `appkit/ui/viz`: render every exported `viz.*` function with its zero-value options and assert the output contains no `class=` and no literal colour (`#`, `rgb(`, `oklch(` outside a `var(` fallback) {id: D-2, symbol: TestVizEmitsNoClassAttribute, package: appkit/ui/viz, skip_test: true, skip: "the deliverable IS the assertion"}
      → **proves it:** `go test ./appkit/ui/viz/` fails if a chart is given a Tailwind class — the mechanism that stops a component picking a tier by coin-flip.

## E. Distribution

- [x] Add `appkit/ui/themes/synthwave/synthwave.go` — package `synthwave`, `//go:embed theme.css`, `func CSS() string` — so the theme rides in `go mod vendor` and a consumer stops needing a sibling checkout at a fixed relative depth {id: E-1, symbol: CSS, package: appkit/ui/themes/synthwave}
      → **proves it:** a test in the package asserts `CSS()` is non-empty and contains `--ak-color-surface`; `go mod vendor` in `resume_as_code` then carries `themes/synthwave/`, which today it cannot (a directory with no `.go` file is dropped).

- [x] Add `cmd/appkit/vendor.go`: `appkit vendor -out DIR --include tokens,motion,router,overlay,chat [--dark-variant none|attribute] [--check]`, writing the requested L0/L1/L2 artifacts only — never L3b, never templ {id: E-2, symbol: vendorCmd, package: cmd/appkit}
      → **proves it:** `TestVendorWritesRequestedArtifactsOnly` renders to a temp dir and asserts the file set equals the `--include` set and that no emitted CSS contains a literal colour.

- [x] `appkit vendor` writes `kit.lock` (module version + sha256 per file) and `--check` re-hashes and exits 1 on drift {id: E-3, symbol: checkKitLock, package: cmd/appkit}
      → **proves it:** `TestVendorCheckDetectsDrift` vendors, mutates one byte of one vendored file, and asserts a non-nil error naming that file. This is the mechanism that would have caught `spa/router.js` fourteen months ago (finding 1).

- [x] Add `EmitThemeCSSOpts(t Tokens, o EmitOptions) string` with `DarkVariant` ∈ {`attribute`,`none`}; keep `EmitThemeCSS` as a wrapper passing `attribute` so `appkit gen` output is byte-identical and `task check`'s regeneration gate stays green {id: E-4, symbol: EmitThemeCSSOpts, package: appkit/ui/tokens}
      → **proves it:** `TestEmitThemeCSSOpts_NoneOmitsCustomVariant` asserts `@custom-variant dark` is absent under `none` and present under `attribute`, and `TestEmitThemeCSS_WrapperIsByteIdentical` asserts the wrapper equals the old output. Rationale: `@custom-variant dark` unconditionally re-keys Tailwind's built-in `dark:` from `prefers-color-scheme` to an attribute a consuming app may not set — silently deleting that app's dark mode with no error.

- [-] Tag `v0.1.0` {id: E-5, disposition: deferred, reason: "this plan does not commit, push or tag; and a kit.lock has nothing honest to record while all seven consumers pin v0.0.0 via replace", trigger: "Geoff tags v0.1.0 in app-kit; `replace` directives stay for local development"}
      → **proves it:** `git tag --list v0.1.0` is non-empty. **Section F does not start before this.**

## F. Consumers — cross-repo, not gated by app-kit's index

Each carries its own proving command. All three are `deferred` until E-5, which
is a real sequencing constraint (S2 before S3/S4), not a hedge.

- [-] `resume_as_code/cmd/serve/web`: point `input.css` at the vendored `themes/synthwave/theme.css` instead of `../../../../app-kit/…`, and replace `build-css.sh`'s hand-rolled `$HOME/Library/Caches/app-kit/tailwindcss/v4.3.0/tailwindcss-macos-arm64` path with `go run github.com/geoffmilleraz/app-kit/cmd/appkit css -i input.css -o app.css --minify` {id: F-1, disposition: deferred, reason: "needs E-1 (synthwave.go) so go mod vendor carries the theme, and E-5 so the version is real", trigger: "E-1 and E-5 both done"}
      → **proves it:** `build-css.sh` succeeds with no sibling `../../../../app-kit` checkout present, and the hardcoded platform path and version pin are gone from the file. Restores the per-call SHA-256 re-verification that the hardcoded path bypasses. **Does not touch resume-core or the PDF renderers.**

- [-] `geoffmiller-cloud`: add `tools/kitsync/` (own `go.mod`, `GOWORK=off go run .`, `-check`) alongside the existing `tools/jsxbuild/`; vendor L1+L2 into `spa/kit/`; hand-author `spa/kit-alias.css` (~25 lines, site-owned, kit-role → site-variable, one direction); **delete `spa/router.js`**; widen `jsxbuild`'s `assetRE` path segment from `(?:components/)?` to `(?:[A-Za-z0-9_.-]+/)*` so vendored files join the existing `?v=` digest {id: F-2, disposition: deferred, reason: "needs E-2/E-3/E-5; and the site must not acquire a second untracked copy before vendor -check exists", trigger: "E-5 done"}
      → **proves it:** site renders pixel-unchanged; `kitsync -check` exits 1 after a deliberate edit to a vendored file; `md5 spa/kit/router.js` equals the kit's; `spa/router.js` no longer exists. `kit-alias.css` points at *variables*, so all five root axes (`data-theme`, `data-aesthetic`, `data-accent`, `data-density`, `data-grid`) come along for free with zero kit changes — and deleting the file unthemes every kit artifact and changes nothing of the site's.

- [-] pumpkin takes L1 {id: F-3, disposition: deferred, reason: "finding 3 — pumpkin is not a consumer on any current evidence and has not asked; building the channel for nobody is the failure ADR-0017 argues against", trigger: "pumpkin asks, or a second non-Go consumer appears — then it is one appkit vendor call with --dark-variant none plus two @import lines"}

## G. Hazards and the first real promotions

- [x] Promote the `useCountTo` tween to `appkit/ui/motion/count.js` (23 lines, no React, no DOM assumptions) with `//go:embed`, `CountScript() string` and `CountHandler() http.Handler`; the L2 bar is one consumer {id: G-1, symbol: CountScript, package: appkit/ui/motion}
      → **proves it:** a test asserts the script registers exactly one global and contains no `import`/`export`; and — the step that makes it a promotion rather than a fork — **both byte-identical copies in `SpecFinOps.jsx` and `SpecFinOpsUtil.jsx` are deleted** (`grep -c useCountTo geoffmiller-cloud/spa/components/*.jsx` returns 0). Deletion of the donor is deferred with F-2.

- [x] Emit `--ak-polarity` per theme branch, **derived by the generator from the token set's own surface luminance** (not authored), and fix synthwave's `:root` block being byte-identical to its `[data-theme="dark"]` block {id: G-2, symbol: polarityFor, package: appkit/ui/tokens}
      → **proves it:** `TestPolarityDerivedFromSurfaces` asserts light branches emit `--ak-polarity: light`; a synthwave page with no `data-theme` no longer paints dark with `dark:` utilities off. **Coordinated change:** `task check` gates on `git diff --exit-code -- appkit/ui/tokens`, so the CUE edit plus every regenerated artifact across both token sets must land in one commit — it cannot be split.

- [x] Ban `ink-faint` for provenance lines in `vizframe`: state it in the package doc citing hazard H-004 (*a caveat set quieter than the claim it qualifies is dishonest*), and assert it {id: G-3, symbol: TestSourceLineNeverUsesInkFaint, package: appkit/ui/vizframe, skip_test: true, skip: "the deliverable IS the assertion; sourceText already exists"}
      → **proves it:** `TestSourceLineNeverUsesInkFaint` fails if the source line's role is changed to `ink-faint` (measured 3.78:1 light / 4.00:1 dark against `ink`'s 16.97/15.80).

## H. Gallery — ADR-0017's own compliance (ADR-0012, unamended)

ADR-0012 needs no amendment: the Gallery is a Go app that serves HTML, so a page
rendering a `vizframe` figure *is* a Gallery page. The gate stands as written and
`vizframe` must pass it. (The `/motion` page is `motion-provenance.md` V-2.)

- [x] Add `/provenance` to the Gallery — `buildProvenance` specimens for all three strengths (`Validate`/`Render`/`MustRender`), all four `Kind` labels, the draft flag, and the `CarriesMechanism` waiver — and add `@source "../../../appkit/ui/vizframe/*.go"` to `apps/gallery/ui/input.css`, then rebuild and commit `apps/gallery/ui/app.css` {id: H-1, symbol: buildProvenance, package: apps/gallery}
      → **proves it:** `TestPagesServe` covers `/provenance`; the rendered page shows role utilities actually resolving, which is the check that the `@source` line took effect. `vizframe` emits role utilities, so **the `@source` entry is required** — unlike `motion/*`, which is plain class selectors and needs none.

- [x] Add `TestEveryGalleryFigureCarriesASource` to `apps/gallery`: fetch every route, parse for `<figure class="ak-vizframe"`, and fail if any lacks a source line {id: H-2, symbol: TestEveryGalleryFigureCarriesASource, package: apps/gallery, skip_test: true, skip: "the deliverable IS the assertion"}
      → **proves it:** the test fails when a deliberately unsourced figure is added to any page. This is the kit holding itself to the provenance rule in public before it asks anyone else to — and it is a property of a *page*, not of a call, so only a test can check it.
