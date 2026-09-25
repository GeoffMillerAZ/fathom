# Local app-owned theme review

Experimental implementation: `apps/styleguide`; contract:
[ADR-0031](../adr/0031-local-app-owned-theme-review.md).
It complements [named themes](theme-workbench.md), not application settings.
RAC now has an owner-authorized opt-in command using its real synthetic renderers,
selected first-party vendor closure and pinned CSS freshness gate: [consumer proof](../assessments/2026-09-15-resume-theme-guide.md).
Its [105-check normal and62-check actual200% journeys](../assessments/2026-09-15-resume-theme-contrast-and-zoom.md) replay on an independently built binary;
full visual/accessibility/font proof and two-consumer widget promotion remain open.
Observed checks and live identities: [assessment](../assessments/2026-09-13-app-owned-theme-review.md).
The [Chrome follow-up](../assessments/2026-09-14-theme-lab-browser.md) adds native edit/export and scoped usability proof.
The [narrow-preview/zoom follow-up](../assessments/2026-09-15-theme-preview-reflow.md) keeps all17 Gallery SVG boxes inside their figures and proves scoped native200% keyboard repair/reset; original chart bytes and code scrollers remain.
The [request-cost workload](../assessments/2026-09-16-theme-guide-request-cost.md) meets in-process budgets. The [corrected browser follow-up](../assessments/2026-09-16-theme-guide-transport.md) meets120 ordinary and60 degraded task/timing/body checks using INC-52's complete-asset endpoint, preserving historical failures and zero cache hits. Cooperative Go cancellation remains distinct from browser stop/retry; the [memory investigation](../assessments/2026-09-17-theme-guide-memory.md) now retains failed RSS/growth measurements and a blank-Chrome control above the absolute ceiling. Metric review, preview cache and browser cancellation keep full performance open; no historical budget is relaxed.

The separate [Stop/retry acquisition](../assessments/2026-09-17-theme-guide-cancellation.md)
now has bounded owner cleanup, unchanged-retry/export/reset observations and exact
Target-command lineage. Intermittent guard failures and the full matrix remain open;
one CDP-command diagnostic pass is not physical keyboard or complete lifecycle proof.
The [ordinary Preview comparison](../assessments/2026-09-17-theme-guide-target-controls.md)
also reproduces the guard failure without Stop; paused-target metadata does not justify
an exclusion or application-policy change. Full task/containment acceptance stays open.

## Try the Gallery adapter

```sh
go run ./apps/gallery/cmd/gallery -addr 127.0.0.1:18766 \
  -theme-source cue/tokens/geoff-synthwave/geoff-synthwave.cue
```

Use an unused port if a Gallery is already listening. Open `/theme-lab`, or use the
link from `/themes` when the lab is enabled. Ordinary Gallery startup does not mount
the lab. Selecting another preset in `/themes` does not change the lab's source.
The startup source file is read once (maximum64KiB), never rewritten. Restart with
another source to change the baseline. The lab records the actual local executable
SHA-256, not merely a HEAD that omits working-tree changes or a claimed release pin.

## Review loop

1. Choose the app specimen, light/dark branch and available/narrow preview width.
2. Leave fields blank to retain source values. Override colors with opaque `#rgb` or
   `#rrggbb`; choose an approved font ID; use nonnegative px/rem radii. Display type
   can inherit sans. Color branches stay independent, even for a dark-only source.
3. Preview candidate. Inspect the actual app fragment, sorted token diff and the
   app's selected contrast checks. Show source compares the immutable baseline while
   keeping the candidate form; Reset discards edits. Neither saves preferences.
4. Resolve failed/unknown checks. Preview again after editing, then export the
   acknowledged candidate ZIP. A stale digest is refused, not silently accepted.
5. Review and apply the change explicitly in the app. Rebuild its utilities, run its
   real keyboard/narrow/theme/state tasks and review the result before adoption.

The preview loads no font/image assets and runs no scripts or forms. Declared font
families can fall back to installed system fonts. It is a static app-owned fixture,
not a working settings page. Higher-specificity app styles can mask token changes;
selected token contrast checks do not prove their rendered use or whole-screen access.
Narrow preview is at most390px and can shrink with the available page width.
The editor starts compact; edited sections reopen after submission. Native Edit tokens,
Review preview and Export candidate links transfer keyboard focus. Invalid overrides
receive associated error text and a focused summary with a direct repair link.

## Mount in another Go app

Use `styleguide.NewEngine(EngineOptions{...})` with:

- `Source`: trusted app-authored CUE bytes, including expressions/comments to preserve.
- `SourceLabel`, `KitRevision`: explicit source and selected implementation identity.
  A label is supplied evidence, not a verifier of the claimed module pin.
- `Fonts`: approved ID-to-literal stacks; no implicit font download or installation.
- `Checks`: nonempty unique IDs, actual foreground/background roles, light/dark scheme
  and finite minimum ratio. Pick pairs used by your app, not a decorative default list.
- `DarkVariant`: `none` (default) or `attribute`, matching the app's compiled utilities.

Inventory rendered pairings, not just token names. RAC's action used `accent-ink`
on `accent`, while its intent badge used `accent` on `accent-soft`: darkening only
the accent improved one and made the other unreadable. Check both schemes and
bind each pairing to actual renderer classes/computed paint. Repair companion
tokens together where necessary; passing a selected list is not whole-theme approval.

Create `styleguide.New(Options{...})` with that engine, a canonical `BasePath`, exact
loopback `Origin` including port, title, optional same-origin CSS paths and fixed
`Specimen` callbacks. A callback is `func(context.Context, io.Writer) error`; render
an actual app-owned synthetic fragment, not a copied Gallery page or a full document.
Mount the base and its descendants without `StripPrefix`:

```go
mux.Handle("/dev/theme", guide)
mux.Handle("/dev/theme/", guide)
```

Protect all these routes and stylesheet assets with the app's authentication wrapper
if required. Local-origin/peer checks are not identity authentication. HTTPS requires
the actual request TLS state; forwarded headers do not grant trust. Public/remote
editor serving is deliberately unsupported. No dependency on Gallery, paid corpora,
MCP, an LLM, Node or browser JavaScript is needed for this loop.
Keep same-origin Referrer-Policy on form-bearing pages: no-referrer can make Chrome
generate Origin:null and break native POSTs. Never fix that by accepting null origins.
Read-only previews/assets/exports retain no-referrer and the opaque preview sandbox.

Callbacks and startup source/styles are trusted application code. Keep fixtures
deterministic, synthetic and side-effect-free: the handler does not sandbox arbitrary
Go I/O or CPU. Output is capped while writing at512KiB; rendering failure emits no
partial specimen. Candidate CSS follows app stylesheets and remains inside its iframe.
Rebuild older utilities for runtime font/radius bindings before expecting them to switch.

## Export and identity

Static app CSS may have byte-derived validators and mandatory revalidation; generated
editor/preview/candidate responses must retain `no-store`. Gallery's
[CSS delivery evidence](../assessments/2026-09-17-gallery-css-revalidation.md) proves
ordinary-page304 reuse, but not reuse across its opaque preview frames. Do not add
`allow-same-origin` or cache private candidate documents to obtain a cache hit.

The deterministic ZIP contains `original.cue`, `candidate.cue`, `candidate.css`,
`diff.json` and `receipt.json`. The original is included whole: do not select source
files containing secrets or unrelated private content that should not be exported.
The candidate is a resolved JSON-as-CUE snapshot, not a patch preserving original
comments, aliases, expressions or layout. Preserve the authored source through review.

The review digest binds raw original bytes, candidate/CSS/diff hashes, source label,
kit identity, dark strategy and ordered check definitions/results. Export recomputes
the candidate and checks; mutated client metadata cannot replace engine authority.
Acknowledgement is not authenticated human approval. ZIP entries use fixed metadata,
0600 modes and deterministic order; identical inputs yield identical bytes.

## Verification and remaining work

Run `go test -race ./apps/styleguide ./apps/gallery ./apps/gallery/cmd/gallery`.
Pure engine checks cover replay, immutable inputs, bounded refusals, unknown contrast,
stale review and exact original/candidate/export agreement. HTTP integration exercises
an app-owned stat specimen beside an ordinary app route, export/reset and access/error
boundaries. These are synthetic app-mount evidence, not a deployed consumer migration.
Opt-in [browser probes](../../apps/styleguide/browser/README.md) now exercise Gallery's
native review/export, selected computed token uses, narrow layout and focused recovery;
the preview canary also runs with JavaScript disabled. Full keyboard/zoom/screen-reader,
actual fonts, performance budgets and broader downstream acceptance remain separate required
evidence. Local-font delivery and interactive specimen execution need
a named requirement and a separately reviewed boundary; they are not enabled today.
