// Package docs is prose chrome for rendered documentation: a table of
// contents in two viewport treatments, and the heading permalink ("anchor
// nav") that pairs with it. `prose.css`, alongside this package, is the
// third piece — the typographic rules for the markdown body itself.
//
// # Provenance
//
// Promoted from platform-engineering's portal/internal/view/toc.go and the
// tocPanel/tocRail pair in portal/internal/app/docs.go, per
// docs/architecture/portal.md §8 ("`ui/docs` — the honest new component:
// prose chrome, ToC, anchor nav, honesty markers... belongs in the kit
// because Scriptorium-class docs surfaces need the same thing") and
// ADR-0019's "TechDocs chrome is ours to build" cost. See
// docs/journals/app-kit.md (platform-engineering, 2026-08-19) for what
// promoted, what stayed behind, and why.
//
// Two of the row's four named parts did NOT travel here, deliberately:
//
//   - "Honesty markers" — the docs page's staleness callout and provenance
//     list — stayed in the portal. Their wording is ADR-0037's specific
//     review-cadence vocabulary and their "hairline rows, no card" layout is
//     ADR-0060's specific taste (this kit's own chrome.RenderDescriptionList
//     and RenderDescriptionListCard render a grid and a shadowed card,
//     neither of which the portal wanted — a divergence already on record).
//     Baking either into a shared renderer would be exactly the "opinion
//     inside a shared renderer" this package's own layer rules refuse.
//   - The three-rail document layout (tree | article | ToC), the branch
//     navigation tree, and the page heading wiring stayed in the portal:
//     they take a `domain.Doc` and an authorized corpus, which are portal
//     types this package has no reason to know about.
//
// # Anchors are GIVEN, never computed here
//
// Every Heading arrives with its ID already burned in by the CALLER's own
// markdown render pass. This package does not slugify, hash, or otherwise
// derive an id from Text — Text is display only, and TestAnchorsAreGiven
// proves it by feeding a Heading whose ID does not match any slug of its
// Text and asserting the rendered href still carries the given ID. That is
// not a style preference: an anchor that is a function of the heading's
// prose breaks every citation the moment somebody copy-edits the heading —
// the exact failure platform-engineering's gitdocs declared-anchor
// convention (portal/internal/adapters/gitdocs/anchors.go, row P2C-9)
// exists to prevent. A caller that slugifies before calling this package
// has reintroduced that failure one layer up; this package cannot protect
// against that, but it will not cause it.
//
// # One consumer today
//
// ADR-0017 §1 / D17's ordinary bar for an L3b package is two consumers
// (docs/ui-components.md). This package ships with one — the portal —
// because portal.md §8 already named it and ADR-0019 already accepted its
// cost; that is a named, on-the-record exception to the entry bar, not a
// suspension of it. It is not exempt from D17's "zero consumers for two
// releases is demoted" clause.
//
// # DIVERGENCE
//
// No corpus (TailwindPlus/MatDash) table-of-contents, nested-nav, or
// sidebar-anchor-outline pattern exists to adopt — re-searched while
// building this package ("table of contents navigation sidebar sticky",
// "sidebar navigation sticky anchor links"); both return only generic
// multi-level application-shell sidebars, none of them a heading outline.
// The original portal implementation recorded the same finding in 2026-07:
// a borrowed nav component would own its own id generation, and the first
// document with a repeated heading would drift the ToC from the anchors the
// caller's renderer actually burned in — exactly the failure the previous
// section's rule exists to make impossible.
package docs

// Heading is one node of a document's heading tree, flattened in document
// order, with the anchor id its owner's render pass already assigned.
//
// ID is GIVEN, never derived here — see the package doc's "anchors are
// given" section. Text is display only.
type Heading struct {
	Level int    // 1..6, as authored
	Text  string // heading text, plain — inline markup already flattened
	ID    string // the anchor id, burned in by the caller's own render pass
}
