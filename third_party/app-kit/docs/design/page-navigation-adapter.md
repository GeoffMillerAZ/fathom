# P06 responsive page navigation

Status: private candidate implemented with [bounded evidence](../assessments/2026-09-14-navigation-typed-adapter.md); not a certified target.
Extends the frozen [P06 pilot](../assessments/2026-09-13-conversion-pilot.md),
[conversion policy](conversion-pipeline.md), ADR-0023 and ADR-0036. Originals remain
independently available. No new runtime, public exporter or generic navigation IR.

## Task and ownership

Move between sibling app pages while retaining a truthful current-page indication
at wide and narrow widths. This is page navigation, not an ARIA tab-panel widget.
The source has desktop placeholder links and a mobile select with an explanatory
comment requesting an onChange handler; no handler is included. A source-faithful
render alone therefore cannot prove usable navigation.

The app owns routes, authorization and the current key. The adapter owns bounded,
escaped presentation and consistent option/link order. Neither constructor discovers
routes or invents a backend. Generated code depends on templ, not app-kit at runtime.

## Bounded model and explicit variants

`PageNavigation` has ordered `Items` (Key, Label, URL), CurrentKey, NavLabel and
SelectLabel. Require 2–12 choices, unique conservative ASCII keys, one existing
CurrentKey, nonblank UTF-8 labels (at most 256 bytes) and no control characters.
Snapshot caller-owned items before rendering. Operational URLs are distinct canonical
local ASCII paths (at most 512 bytes); no scheme, host, query, fragment, encoded path,
backslash, repeated slash or dot segments. Query-bearing destinations need a separately
reviewed expansion; do not silently strip context from URLs.

| Constructor | Narrow behavior | Evidence boundary |
|---|---|---|
| `NewPageNavigationReference` | Original unbound select | Source-style specimen only; permits original `#` placeholders; not a usable-navigation claim |
| `NewPageNavigationGET` | Native select plus explicit submit button | App-owned GET action maps stable choice keys to allowlisted routes; no automatic navigation on selection |
| `NewPageNavigationLinks` | Ordinary wrapping page links | Static-site adaptation; no JS, action endpoint or client runtime |

The GET constructor takes Action, FieldName and SubmitLabel. Action uses the same
local-path grammar; FieldName is a conservative identifier. It never takes a URL from
the browser as a redirect destination. The app must validate request cardinality and
map the submitted key to a known item. This adapter does not preserve arbitrary hidden
query state: app routes must explicitly encode needed context or use ordinary links.
All constructors accept PageNavigation; GET additionally accepts NavigationGET.
Only Reference permits repeated `#` placeholders; GET/Links reject them before output.
The GET-only fixture carries theme in its path, accepts exactly one known choice key,
rejects unknown/duplicate fields, then sends 303 to its exact allowlisted path. The
destination must update current/selected state; browser Back must restore the prior page.

The reference keeps source structure/classes, option labels, current link and selected
option. Its extracted defaults reproduce the original parsed DOM. GET adds a form,
named/keyed select and submit button; Links substitutes wrapping mobile links. Both
operational variants explicitly add wrapping/focus/44px-target rules, including for
long desktop labels. These are reviewed usability adaptations, not vendor fidelity.
Source and adapted comparisons must not share these extra rules accidentally.
After editing a select, its value represents a pending destination until submit; it
is not the current route. The app's heading/route and aria-current remain authoritative.
Native history may restore a pending form value; Back must restore the page, not be
misrepresented as a guaranteed reset of browser-owned form state.

## Generation and assurance

Accept only the frozen archive/member/hash. Verify archive membership independently
before adaptation. Require exact reviewed structure, paired option/link labels, one
selected/current item and consistent current/default class sets. Use the existing
HTML-to-templ source map; source or binding drift fails closed with no partial output.
Emit a replayable source-specific envelope (`not-verified`) with implementation,
syntax, template, model and default identities. No paid body enters tracked fixtures.

The existing fixed-inventory private delivery helper may be reused, with exact-source
replay before writing/checking. No installation, compilation, route binding, mutation
of previous exports or catalog certification occurs implicitly.

## Frozen acceptance matrix

- Synthetic structure/hash refusals, invalid model bounds/paths/keys, duplicate and
  missing current state, escaping, caller mutation, cancellation and writer errors.
- Generate twice; exact replay and independent source rejection; compile before/after
  templ formatting. Reference defaults must match namespace-aware DOM, including text.
- Same pinned Tailwind 4.3.0 and embedded Inter baseline for original/reference;
  compare original/reference DOM, geometry and screenshots at 390/1440, light/dark.
  Keep source/reference and operational document identities separate. Fail on leaked
  operational markup/classes; adapted screenshots cannot satisfy source fidelity.
- Actual native GET selection/submission and desktop links, plus JS-disabled static
  links, keyboard focus/current state and browser Back at both widths/themes.
  Assert responsive visibility, one operable representation (hidden controls excluded
  from keyboard/accessibility order), and option/link order/current parity for each item.
- Explicit operational target/wrapping/focus treatment; 256-byte labels and actual
  browser 200% zoom. Preserve any source deficiencies as source failures.
- Private export/check and standalone compilation from a copy; no sibling checkout.

Record static, browser and delivery results separately. A new candidate does not
change the frozen twelve-case denominator or erase unearned runtime/app adoption gates.
The separate [prospective Links proof](../assessments/2026-09-14-navigation-semantic-proof.md)
adds a frozen 24-case contract and independent Go/CUE semantic checks for keyboard,
pointer/emulated touch, 100%/200% browser zoom, current/Back, focus and contrast. It is
not source fidelity, physical touch, screen-reader proof or target certification.

## Revisit when

A consumer needs query context, external destinations, nested navigation or client
tab panels; the source changes; source fidelity or an operational task fails. Review
the relevant model/oracle rather than weakening it or introducing a generic router.
