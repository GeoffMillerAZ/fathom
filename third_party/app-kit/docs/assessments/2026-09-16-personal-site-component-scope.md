# Personal-site component extraction: bounded assessment

Read-only assessment of `/Volumes/m2ext/code/geoffmilleraz/geoff`, the canonical
target of `/Users/geoff/code/geoffmilleraz/geoff`. This is the personal site, not
Resume as Code. No source, dependency, content or deployment was changed.

The checkout has936 status entries; candidate components are untracked. Its Astro ADR
and agent guidance differ from the active Next/React workspace. HEAD alone cannot
identify a conversion input. Freeze selected file hashes before implementation and
review the app's actual build/route contract. A different indexed service named
`geoffmiller-cloud` does not substitute for this checkout.

| Candidate | Evidence / overlap | Size and disposition |
|---|---|---|
| Series journey navigation | `apps/web/src/components/react/dashboard/SeriesNavigation.jsx:10`; used by `apps/web/src/app/blog/[...slug]/page.jsx:40`. Semantic ordered navigation and previous/next links add a task beyond kit SVG Steps. Needs `aria-current="step"`, long-label reflow and dependency-free decoration. | Small-to-medium; recommended single experimental candidate |
| Linked content cards | BlogInventory, GuideInventory and ProjectRepository repeat title/meta/footer presentation. Kit Card supplies slots, not full primary/secondary link semantics. | Medium; defer until a named task; repair focus and competing hit targets first |
| Expandable records | ExpandableList has buttons but lacks expanded/control semantics and stable item identity. Existing kit Disclosure and Badge cover the primitive need. | Small composition; reuse existing primitives, not a new public widget |
| Persona/context switcher | ContextSwitcher ties visual-only selected state to resume-specific updates; keyboard/state contract incomplete. | Medium-to-large; defer, domain ownership and second useful task unresolved |

The smallest useful first slice is a token-themed, source-independent series-navigation
contract with linked/current/first/last states. Verify keyboard,390px/desktop,long labels,
both selected site identities and real previous/next destinations. Keep content/routing
app-owned. A Gallery specimen is useful test evidence, not a second independent app.
Stable promotion remains subject to F3 and ADR-0017; one site could justify an explicitly
approved incubation, not silently satisfy the permanent two-consumer condition.

Themes need no new extraction: `geoff-synthwave` and `geoff-indigo` already exist.
The active Next entrypoint hardcodes gray/indigo; older Synthwave configuration does
not make the current route Synthwave. Dogfooding back is therefore a separate React
integration and theme-routing task, not a drop-in import of a Go renderer.

Do not import personal records/photos or React, Headless UI, Heroicons and Framer Motion
dependencies into kit-core. Component provenance is not established by a README license
claim; derive the semantic contract without copying uncertain third-party template bodies.
Whole-site conversion or a new framework-native package is not a small extension.

Next: owner may admit this single experimental slice; then freeze source, review/update
the site's stale guidance narrowly and obtain exact consumer integration authority.
This assessment did not run tests/browser checks or implement the candidate.
