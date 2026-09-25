# Design-source conversion and verification

Accepted direction: [ADR-0023](../adr/0023-profile-specific-conversion-and-client-compute.md), with [ADR-0069's core-first/JIT scope](just-in-time-conversion.md) and [converted-core-v1](converted-component-standard.md). Imported originals remain unchanged; the long tail is not a current completion prerequisite.
Current implementation: private source inventory, bounded CLI/MCP/Gallery reads and a [syntax-only HTML-to-templ translator](html-translation.md). The twelve [pilot inputs](../assessments/2026-09-13-conversion-pilot.md) are frozen. Private syntax attempts persist with original-linked CLI/MCP discovery. A [typed P02 adapter](../assessments/2026-09-14-pricing-typed-adapter.md)
adds private faithful/enhanced Go output with [explicit immutable export/replay](../adr/0034-source-specific-private-component-delivery.md);
general behavior adapters, repair queue and target certification remain open.
The [P02 delivery rehearsal](../assessments/2026-09-14-pricing-private-delivery.md) proves two isolated synthetic builds, not actual app adoption.
A second [P04 directory adapter](../assessments/2026-09-14-directory-enhanced-adapter.md) adds typed repeated rows/labels, native navigation and an opt-in usability companion; original failures remain. Its five/eight-file export/check shares only filesystem assurance under [ADR-0036](../adr/0036-share-private-delivery-assurance.md). A [synthetic native journey](../assessments/2026-09-16-directory-journey.md) adds app-owned filter/page/detail/Back/CSV, independent values and partial/error recovery with 1,203 browser checks. JSON defaults stay unchanged; neither adapter is certified or proven adopted by this fixture.
[P05 email-field adaptation](form-validation-adapter.md) adds typed error/neutral states, private delivery and a native recovery fixture. [Runtime selection](runtime-selection.md) defines fit; none of these candidates is certified.
[P06 page navigation](page-navigation-adapter.md) adds a fourth private typed candidate:
source-reference, native GET and static-link variants, with standalone replay and
scoped browser evidence. [P03 FAQ](faq-adapter.md) adds explicit faithful Elements and
JavaScript-free native variants, repeated-instance content, standalone replay and scoped
browser/200% zoom proof. [P01 hero](hero-adapter.md) adds typed copy, explicit image
semantics, six app-owned URLs and immutable export/replay. [P07 menu](menu-adapter.md)
adds typed links and an explicit app-owned POST/CSRF action beside the unchanged
Elements reference. [P08 profile drawer](profile-drawer-adapter.md) adds explicit
identity/image/details and optional navigation beside its unchanged nested-menu reference.
[P09 responsive navbar](responsive-navbar-adapter.md) shares one typed route model across desktop/mobile, with optional app-owned GET destinations. All nine typed tools are experimental CLI-only catalog/MCP
references; metadata does not certify web components.
[P11 task mapping](matdash-analytics-adapter.md) separates MatDash's native composition
from a synthetic product-receipts workflow. Its integer-cent projection and consolidated
CSV now run in [the receipts reference](../../apps/receipts/README.md), with scoped
signed-chart/native-download/browser evidence. The full source-faithful port remains open.
The private [P12 client-owned editor](client-editor-island.md) preserves React/Tiptap
inside a Go page with typed ownership/lifecycle, repeat builds and scoped browser proof;
it is not a templ editor conversion, public API or completed P12 acceptance.
The [Elements source-native reference](../assessments/2026-09-14-elements-source-reference.md)
pins the four P03/P07/P08/P09 originals and runtime with deterministic private replay
and 404 scoped browser assertions. A separate [opt-in Elements rule](elements-translation.md)
now preserves those sources through compiler/DOM replay and [paired browser proof](../assessments/2026-09-14-elements-conversion.md):
842 scoped assertions and 34 byte-identical screenshot pairs. Source gaps stay visible;
the legacy translator, saved attempts and certification counts are unchanged.

## Product promise

Pay a shared engineering cost once, preserve the purchased design, and make subsequent
apps and vendor upgrades cheaper to integrate. The deliverable is readable source plus
working behavior, provenance and reproducible proof. It is not a screenshot generator,
a universal transpiler, or a mechanism that turns red tests green at any cost.

## Pipeline and ownership

1. **Inventory:** retain source archive/member hashes, vendor/framework/Tailwind axes,
   license pointers, dependencies and complete-page relationships. Classify every source
   family cheaply; missing classifications remain unknown, never silently skipped.
   [All 657 official HTML sources](../assessments/2026-09-13-html-conversion-coverage.md) have inventory; 439 pass [syntax](../assessments/2026-09-13-html-syntax-audit.md) and separate [compiler/DOM checks](../assessments/2026-09-13-html-compiler-dom-audit.md), without publication or target certification.
2. **Choose:** select target, task, visual reference, fixtures and budgets before editing.
   Freeze a source-native baseline and its actual fonts/assets/runtime. Without a runnable
   reference, mark fidelity unverified and do not claim equivalent behavior.
3. **Translate:** parse syntax; apply named, versioned transformations with a bounded
   supported grammar. Preserve markup and classes initially. Emit readable Go/templ or
   static HTML, source maps/lineage and explicit unsupported-node diagnostics.
4. **Connect behavior:** select an existing proven native browser/kit/upstream interaction
   implementation or a reviewed family adapter. App-owned data, auth and mutations stay
   explicit parameters/handlers, not inferred from demo names. Avoid raw HTML escape hatches.
5. **Verify:** run the matrix below against source and target with the same fixture.
   Distinguish conversion failure, behavioral defect, environment failure and visual delta.
6. **Repair:** a coding agent receives the smallest failing case, contract, logs, diff and
   bounded source context. It proposes a general rule when several cases justify it,
   otherwise an explicit source-specific patch. Independent review reruns the full family.
7. **Publish privately:** expose only earned target results in the shared catalog; retain
   source/output/rule/runtime/test hashes, unresolved gaps and integration instructions.
8. **Upgrade:** source-hash changes invalidate affected receipts. Replay transforms and
   patches; conflicts are failures. Compare old/new vendor behavior and adapted results.

Keep the original archive, syntax translation and semantic design adaptation separate.
Contextual token mapping is a reviewed pass, not a palette-name substitution. A private
reference may retain vendor colors; a promoted kit renderer follows semantic role rules.
Neither stage silently removes dark/hover/disabled styles, interaction code or ARIA.

## Conversion families

| Source / behavior | First approach | Escalate when |
|---|---|---|
| Static HTML sections, typography, lists | parse into templ; build-time HTML or SSR | parser ambiguity, scoped CSS/assets or unsupported syntax |
| HTML with Elements interactions | preserve pinned runtime; test lifecycle with Datastar | focus, teardown, morphing, CSP or browser support fails |
| Form/filter/detail tasks | typed Go view data and handlers; optional Datastar projection | latency/offline/input ownership needs local behavior |
| Catalyst / MatDash React compositions | inspect native source; port selected complete pages through reviewed adapters | hooks/context/portals/virtualization need client-native ownership |
| Rich editor, canvas, large interactive grid | keep a client-owned subtree or full client app | measured benefit supports an isolated alternative |
| Pure compute-heavy transform | benchmark worker JS against browser Wasm | startup, memory, copy cost or feature support defeats benefit |

Do not turn Headless UI/Flowbite state machines into HTML tags and call that a port.
Prefer the upstream HTML variant where supplied; React-only templates are a different
problem. Stable syntax can be automated; selecting semantics requires a contract.
Use the smallest parsing representation needed for one transformation, not a generic
cross-framework intermediate language or a second hand-maintained design system.

## Verification matrix

| Gate | Required evidence / refusal |
|---|---|
| Reproducibility | exact input, generator, config, patch and output hashes; clean replay without LLM/network |
| Structure | parser/compiler checks, valid HTML, unique IDs, labels/landmarks, safe URL/data escaping |
| Visual fidelity | source/target at identical fixture, viewport, theme, density, fonts and reduced motion; reviewed deltas |
| Interaction | keyboard/pointer/touch, focus return, nested Escape, selection, form errors, long labels and zoom |
| Dynamic lifecycle | mount/patch/remove/remount, no duplicate listeners, focus/selection preserved, cancellation/reconnect |
| Security | XSS/expression canaries, CSP, auth/CSRF/origin boundaries, actual native POST under served referrer policy ([INC-16](../intent/incidents.md)), secrets and licensed-output checks |
| Data truth | fixed expected values, missing/zero/stale/partial, sort/filter/page/detail/export agreement |
| Performance | payload, first usable content, input/update latency, marks/rows, heap, stream fan-out under named workload |
| Integration | clean build, one real task, upstream fix replaces fork, second comparable consumer before permanent L3b/L4 promotion |

Screenshots alone do not certify accessibility or behavior; compiling alone does not
certify compatibility. Assertions use seeded failures, including a deleted focus rule,
lost dark variant, changed metric, stale receipt and disabled check, to prove detection.
Asset checks must cover the actual page's request URLs, not only stored file paths:
query-bearing CSS/font URLs need exact, byte-bound delivery mappings. Direct asset
probes cannot substitute for browser request-graph checks ([P10 cache gap](../assessments/2026-09-15-catalyst-cached-static.md)).
Native adapters must prove focus is painted: `:focus-visible` alone can match while retained framework styles draw nothing (INC-29). Inspect the outline against its local backdrop and clipping, with form-owned accessible-name/error evidence.
Font subsets must preserve whole-cluster rendering with an original-font fallback. `FontFaceSet.check()` alone is not glyph proof: bind actual rendered fonts, shaping and paired pixels to delivered bytes, including mixed-script and missing-font controls ([P01 fixture](../assessments/fixtures/hero-adapter/FONT.md)).
For P01 alone, [ADR-0067](../adr/0067-prove-isolated-warm-cache-lineage.md) admits a separately frozen v2 cache-lineage method under exact isolated-origin/representation/rendering controls. Its [first full acquisition](../assessments/2026-09-16-hero-cache-lineage.md) fails warm header/event evidence despite passing numeric budgets; v1 direct-body failures remain failed. Other families do not inherit applicability or a pass.
Textarea conversions must test actual form transport and HTML parsing: CRLF wire values,
LF API counts and the parser's ignored first newline are distinct representations.
Prove limits/counters and retained leading/trailing lines through native POST (INC-30).
Do not normalize away meaningful classes, ordering, whitespace, animation or DOM changes.
Allow only named non-semantic normalization rules; keep original bytes for comparison.

Pixel rendering varies by browser/OS. Deterministic means pinned controlled conditions
and repeatable structural tests, not an assertion that every host produces equal pixels.
Nonblank capture pixels alone do not establish coverage: bind PNG dimensions to measured
viewport/capture geometry and include open popovers and required targets (INC-28).
If paint screening uses a task region, record its bounds inside the complete capture;
do not crop away missing content or dilute sparse UI with unrelated empty-page area.
Choose visual thresholds before conversion. Layout, text/contrast and interaction
regressions require review even when a global image-difference threshold passes.

## Repair discipline

An LLM is a development collaborator, never a required build-time/runtime service.
Review code and oracle changes separately. No deleting assertions, expanding tolerances,
updating goldens or masking regions to satisfy a failed gate without an explicit
contract decision and independent review. A flaky environment is not a product pass.
Bound each repair attempt by case count and effort; unresolved work stays quarantined.
Promote repairs into fixtures/rules, not repeated unrecorded agent improvisation.
Retain exact executed harness/configuration bytes with each run before editing them;
hashes alone cannot recover an overwritten oracle ([INC-43](../intent/incidents.md)).
Save private snapshots before browser work, bind them in the receipt, and preserve
failed runs. A missing historical snapshot remains an explicit evidence limitation.

## Historical pilot and current scale decision

The owner admitted this pilot under G1/G4; its retained research covers these
12 source cases with exact IDs: static hero, pricing,
content/FAQ, table/list, form validation, tabs, menu, dialog with nested menu, mobile
navigation, a Catalyst settings page, a MatDash analytical composition and one
interaction-heavy client-owned specimen. Keep a denominator for every case and target.

ADR-0069 now selects five complete starter areas instead of requiring full certification
of all twelve before this milestone. Keep every old failure; select deferred inputs only
for a consuming task or bounded owner batch. For a selected case, prove applicable
static HTML and Go/templ targets; evaluate Datastar plus
Elements only where interaction needs it. The client specimen tests DOM ownership and
local response, not a promise to port an editor. Add missing/error/long-label fixtures.
Inspect ordinary content too: no-overflow checks must not hide compressed columns or
vertical-letter labels; keep visual findings as explicit regression checks.
Bulk conversion starts only after representative families pass, with explicit exclusions
for non-fitting targets. Inventory coverage, conversion coverage, verified behavior and
consumer adoption are four different measures. Broader coverage is demand-led or an
explicit batch, not guaranteed completion of unused variants or a component-count goal.

Record setup/rule-authoring time, manual exceptions, repair attempts, bytes/runtime costs,
task failures and per-consumer integration time. Compare against using source directly
and today's manual adaptation. Replay the already observed UI-block vendor delta as
an upgrade rehearsal. A family advances when its critical gates and layer promotion
bar pass; overall investment payback needs two comparable integrations with reduced
effort and no worse task quality. Two consumers remain mandatory for permanent L3b/L4,
not every lower-layer artifact. Revise or stop families that fail to repay their cost.
No arbitrary pass-rate target can hide an essential failure.

## Catalog and delivery contract to implement

The syntax-attempt store is implemented separately; it cannot carry compatibility passes.
The separate [target-evidence checker and private catalog](target-evidence.md) bind
canonical contracts, observations and retained bytes. CLI/MCP/Gallery share historical
records under [ADR-0043](../adr/0043-private-historical-evidence-catalog.md); search shows
recorded metadata, exact get rechecks historical integrity and original membership.
All results remain not-verified. Semantic certification and promotion remain unimplemented.
A separate [P06 semantic task checker](../assessments/2026-09-14-navigation-semantic-proof.md)
derives scoped input/focus/contrast results from prospectively collected facts. It does
not authenticate reports, complete full gates or promote compatibility.
Keep source identity separate from target artifact identity. A target receipt names source
members, transformer/config/patch/runtime hashes, output hash, evidence fixtures/toolchain,
checks with passed/failed/not-run/not-applicable, exclusions/reasons and consumer history.
Freeze applicable gates before repair. N/A needs a reviewed reason; changing applicability
or target exclusions requires independent contract review, just like changing a baseline.
Any missing applicable gate prevents verified-target discovery.
Use CUE to validate receipts and Go for generation/checks; do not infer proof from text
commands. Search remains bounded; exact get supplies full evidence. Gallery and MCP
must agree on IDs, revisions and status; stale proof never advertises compatibility.
Existing first-party profile IDs are not automatically broadened by this design.

## Originals and adaptations coexist

Always offer the exact vendor original alongside linked target artifacts. Discovery
can prefer a verified fitting adaptation without hiding the source-native choice.
Show source revision, available targets, preserved/changed behavior, dependencies,
evidence gaps and the reason a target is unsupported. Conversion failure never deletes
the original, fabricates compatibility or silently falls back to another renderer.
For client-heavy or non-fitting tasks, the original can be the primary implementation,
not merely a last-resort fallback. Selecting React/Vue is a deliberate consumer-profile
decision, not failure of the kit. An original still needs integration proof; it is not
dependency-free or directly compatible with Go-only apps. Snippet tools and archive IDs stay available.
