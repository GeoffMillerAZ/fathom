# P06 private page-navigation rehearsal

Contract: [page-navigation-adapter](../../../design/page-navigation-adapter.md). This
is a source-specific private candidate, not a generic router, ARIA tab widget, certified
target or consumer adoption claim. Purchased source and generated derivatives stay private.

## Discover

Search `search_capabilities(query="page navigation private adapter", profile="cli")`,
then `get_capability("reference.navigation-adapter")` and
`get_kit_decision("navigation-adapter")`. Gallery serves the same record at
`/catalog/reference.navigation-adapter` and `/api/catalog/reference.navigation-adapter`.
These source-free metadata routes need no private archive. They describe experimental
CLI tooling, deliberately excluded from web-profile compatibility results; they neither
generate licensed source nor install an app component. Rebuild Gallery/MCP for catalog
changes and restart MCP to reload this guide. Consumer pins require separate review.

## Generate and use

```sh
go run ./cmd/appkit data assets-adapt-navigation
go run ./cmd/appkit data assets-adapt-navigation -out /existing/private/parent/new-navigation
go run ./cmd/appkit data assets-adapt-navigation -check /existing/private/parent/new-navigation
```

JSON is the read-only default. Export/check independently reverify the frozen archive,
create no overwrite, and return exit 0 for match, 1 for observed drift, or 2 for usage/
could-not-check. Copy the immutable five-file delivery into a private app package, run
pinned `go tool templ generate`, and scan both `navigation.go` and `navigation.templ`.
Generated code depends on templ, not app-kit.

`NewPageNavigationReference` reproduces only the source specimen and permits its `#`
placeholders. It does not supply usable narrow navigation. `NewPageNavigationGET` adds
a native GET form; the app must enforce one allowlisted submitted key and map it to a
route rather than redirecting to submitted URL data. `NewPageNavigationLinks` supplies
ordinary wrapping links for static delivery. The app owns routes, authorization and the
truthful current key. Operational focus/wrapping/target rules are deliberate adaptations
and must never be credited to source fidelity.

## Reproduce cached evidence

```sh
go test ./internal/designadapt ./cmd/appkit -run '^(TestNavigation|TestAssetsAdaptNavigation)'
APPKIT_PROBE_CONVERSION_KIT_ROOT="$PWD" go test ./cmd/appkit -run '^TestAssetsAdaptNavigationLive$' -count=1
node docs/assessments/fixtures/navigation-adapter/prepare.mjs "$PWD"
node docs/assessments/fixtures/navigation-adapter/browser.mjs /printed/private/temporary/directory
```

Preparation creates a fresh 0700 system-temporary directory and installs nothing. It
uses the existing cached Go/templ/Tailwind 4.3.0 module pins and licensed Inter bytes,
exports/checks the five files, injects the exact source/default constants only into the
private runtime test, compiles/tests raw and templ-formatted copies, verifies its own
module and cached checksums, excludes app-kit runtime dependencies, builds `preview.test`,
and records artifact/binary/CSS identities in `prepare.json`. `TestServeNavigationPreview`
is present in that binary for the browser harness and is not enabled by ordinary unit tests.

The isolated `browser.mjs` consumes the prepared binary/CSS identities and synthetic
routes. It separates original/reference fidelity from GET/Links quality, covering
390/1440, light/dark, native keyboard type-ahead/submission, pointer/keyboard links,
browser Back, responsive visibility/tab-order/current state, sampled text contrast,
long-label CSS geometry and actual 200% zoom. App JavaScript is disabled; the zoom
fixture first configures Chrome in its own fresh temporary profile, never a signed-in
profile. The profile is removed after browser close; evidence stays private and no global setting changes.
OS native-popup arrow navigation, screen readers and physical touch remain unverified. Selects
can truncate long options; choose Links where complete long labels must remain visible.
Original 40px targets remain failures; only explicit adaptations receive 44px treatment.
See the [dated results and limits](../../../assessments/2026-09-14-navigation-typed-adapter.md).

The separate `quality.mjs` collector and `internal/navigationproof` CUE/Go checker freeze
24 Links cases before browser tasks, then independently derive input/current/Back,
focus/target/contrast and actual browser-zoom results. This includes emulated touch,
not physical touch or the GET select variant. Use the [prospective proof instructions](../../../assessments/2026-09-14-navigation-semantic-proof.md)
with both the printed private directory and the pre-run contract digest; missing or
stale requested evidence fails. Collector completion alone is not a semantic pass.

Temporary evidence is private and not backed up by Git. Re-run into a new directory
after input changes. No live backend, release, host invocation or G1 completion follows.
