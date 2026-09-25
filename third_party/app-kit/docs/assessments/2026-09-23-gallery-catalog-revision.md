# Revision-bound Gallery capability review

## Prospective task and boundary

Baseline: `70238412abcfff1b831947ecbf5f0ca069acaec3`.
MCP exact capability lookup accepts the search revision, but Gallery's generated
detail links do not bind that revision and its HTML/JSON detail endpoints ignore
`expected_revision`. A saved result can silently select newer metadata after a
Gallery rebuild. Quality-work links already demonstrate the revision-bound pattern.

Task: search for a fitting capability, inspect its exact contract, and recover from
a changed catalog before reviewing new metadata. Preserve legacy unguarded access,
canonical per-capability pins, filter/theme context and read-only delivery.
I7/I8/I15/I18 and ADR-0024/0065 govern. This repairs the existing discovery handoff;
no new public Go API, metadata hash format, consumer migration or installed MCP update.
Intent-design preserves static-versus-runtime evidence; no future runtime is built.

## Fixed acceptance and budget

- Gallery-generated `/catalog/{id}` links carry the returned whole-catalog revision.
- HTML and JSON exact routes accept a matching optional `expected_revision`; omitted
  or empty retains current lookup. Matching versus omitted returns identical capability
  main content and byte-identical JSON/canonical pins. HTML theme links must preserve
  a supplied guard rather than discard it to obtain whole-page byte equality.
  Old direct links remain valid.
- A well-formed stale revision returns409 before capability content or a pin. Invalid,
  duplicate or malformed-encoded revision requests return400; no ignored guard.
  Absent IDs still return404 after a valid/omitted guard. POST remains refused.
- Stale HTML offers a native recovery link retaining query/profile/layer/kind/maturity,
  limit and sanitized theme, but removing cursor and expected revision. Following it
  produces current results and a freshly guarded detail. Normal Back keeps valid
  pagination context but removes the exact-detail guard.
  If the saved filters are invalid together, disclose their reset and recover to the
  theme-preserving unfiltered catalog; never link straight into another filter error.
- Synthetic HTTP regressions cover both transports, recovery and exact JSON parity.
  A no-script browser task uses keyboard search/detail/back and stale-link recovery
  at1440/light and390/dark. Inspect captures and confirm completed local assets,
  no unexpected requests, no page overflow and bounded owned-process cleanup.
  Both browser lanes also follow the invalid-filter recovery to a successful list.
- No application JavaScript, dependency, CSS utility or metadata pin change. Preserve
  failed test/harness bytes; do not weaken assertions or infer real host adoption.
- Bound implementation/acquisition to40 minutes from08:52 UTC. If incomplete, preserve
  a coherent patch and precise remaining work; no full G1/G4 promotion is implied.

Evidence root: `/Volumes/m2ext/appkit-evidence/appkit-catalog-review-guard.W3ldf6HR`.

## Results

The frozen `red` acquisition ran six tests and failed five against the old handlers;
it also reports a concurrent edit of the non-executed browser fixture, so retain it
as diagnostic and repeat with stable inputs before production edits. Its original
whole-HTML equality assertion would erase guards from theme-switch links. A separately
frozen correction compares main content and exact JSON while additionally asserting
guard retention on every theme link; no runtime behavior or metadata oracle is relaxed.
The stable `red-final` acquisition ran six top-level tests with five failures before
the production edit; `green` passed all six. Reviewer-discovered invalid-filter recovery
then failed all seven new cases in frozen `recovery-red`; `recovery-green` passes all
14 top-level/subtest events. Initial five-package race checks passed1,499 test events.

The first isolated candidate CSS build added an unused36-byte `.lowercase` utility
from prose in the new Go error string. Equivalent precise error copy avoids that
scanner artifact; `candidate-reviewed` rebuilds byte-identical committed CSS.
Its frozen actual-handler response packet SHA-256 is
`9392ffc3c4f1606ee72597289500bd340103cf4b1feaa5a9146cf508a1e96c49`.

The first browser run stopped after29 checks: the invalid-filter recovery URL also
belongs to two global navigation links, so the whole-document selector found three
anchors. Chrome cleanup passed; retain `candidate-reviewed-browser` and its frozen
inputs as a failed harness acquisition. The corrected selector requires exactly one
matching link inside the main landmark; all native keyboard, status, byte, geometry,
theme, no-script and process assertions remain unchanged.

`candidate-reviewed-browser-fixed` passes68 checks across1440/light and390/dark:
native Tab/Enter list → guarded detail → Back, stale409 → current list → guarded
detail, and invalid saved filters → disclosed reset → current unfiltered list.
All document/CSS responses complete and match the frozen actual-handler bytes;
no application scripts, unexpected requests or page overflow. Owned Chrome exits
and drains without force or unresolved children. Root review inspected all six
captures: both error states wrap readably and their recovery actions remain visible;
the existing capability detail has no page-level clipping in these lanes.

`regressions-final` passes1,507 test events with race detection across Gallery,
MCP, catalog, adoption and quality-plan packages, with no source drift. A second
independent source/renderer build (`final`) reproduces the tested response packet
SHA above and byte-identical46152-byte Gallery CSS. No generated CSS, catalog
metadata, per-capability pin, dependency or runtime script changed. Independent
scoped implementation/fixture review cleared the recovery repair.
`final-checks` also passes MCP/catalog/quality-plan tests, intent living-document
budgets,437 relative links, paired intent stanzas and diff whitespace. This is a
structural subset, not a full intent-doctor or whole-repository test claim.

These are scoped synthetic HTTP/browser checks, not installed-server delivery,
consumer adoption, full accessibility/performance, certification or G1/G4 acceptance.
Rebuild Gallery to receive the guard; no consumer or global configuration was changed.

## Separate continuation

The MCP capability → quality-work handoff has another revision boundary:
`list_quality_work` accepts a capability ID without the catalog revision selected by
`get_capability`, and its quality revision binds inventory bytes rather than catalog
metadata. A catalog-only rebuild can therefore reuse the inventory revision after
the capability changed. Same-process lookup is coherent; reconnect/rebuild is the
case to test. Before changing this separate protocol, freeze a two-snapshot regression
with identical inventory and different catalog metadata. Consider an optional
catalog-revision precondition and explicit returned catalog identity while preserving
the existing quality hash and unguarded latest-snapshot access. No fix, host upgrade
or quality promotion for that boundary is claimed here.
