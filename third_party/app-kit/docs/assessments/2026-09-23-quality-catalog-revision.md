# Bind authored quality lookup to the selected capability catalog

## Prospective task and boundary

Baseline: `163e49a27aed466d35ba1271a95c700d0db334b3`.
After selecting a capability, an agent should inspect the authored quality work for
that same catalog, including across server rebuild/reconnect. The existing quality
`revision` hashes canonical inventory only. Catalog metadata can change independently
while the same capability IDs and inventory remain valid; the current list/get tools
cannot express that precondition.

I8/I15/I18 and ADR-0020/0024/0065 apply. ADR-0020's identity-divergence revisit condition
requires examining this cross-tool handoff, not replacing the catalog or adding a
server. This is an additive read-only protocol repair. Catalog metadata identity,
inventory identity, source/evidence verification and actual adoption remain distinct.

## Fixed contract

- Both `list_quality_work` and `get_quality_work` accept optional
  `expected_catalog_revision`. A supplied value must be the exact current catalog's
  64-character SHA-256 in digits/a-f. Malformed or stale values refuse before payload.
  Empty/omitted keeps current lookup; a list without a capability filter still honors
  its supplied guard. Detail also retains the existing inventory `expected_revision`.
- Shared list/detail envelopes add `catalog_revision`, separately from unchanged
  `revision`. Do not combine the hashes or change canonical inventory/pin formats.
  MCP text/structured representations and Gallery JSON return matching identities.
- Bind the reader to the validated catalog's immutable search snapshot, not its
  mutable exported inspection fields. A package-internal catalog-bound constructor
  supports exact independent snapshots; existing Default/List/Get callers still work.
- A two-snapshot test changes actual catalog metadata while retaining identical
  inventory. The quality revision must stay equal and catalog revisions differ;
  list and detail on the second MCP server must refuse the first server's catalog
  guard even when its inventory guard still matches. An inventory-only mutation must
  instead change only the inventory identity. Do not substitute a made-up mismatch
  for the two-snapshot test.
- Keep authored records, selection behavior, bounds, annotations and failure states.
  Extend guidance with exact argument mapping and restart/review on mismatch, never
  dropping a guard automatically. Old installed servers require a reviewed rebuild;
  no installation, global configuration, network, consumer write or evidence execution.
- Gallery gets additive JSON identity only in this packet: no HTML/layout/asset change
  and no new query parameter acceptance. Prove ordinary HTML byte parity from frozen
  old/current handler responses; do not claim a browser or Gallery guard upgrade.
- Retain pre-execution source, tool/environment identities and failed acquisition
  bytes. Run protocol/reader regressions and focused races. Bound this packet to40
  minutes from09:19 UTC; remaining work stays explicit without relaxing acceptance.

Evidence root: `/Volumes/m2ext/appkit-evidence/appkit-quality-catalog-guard.DyRRX097`.

## Results

The first frozen `red` acquisition ran17 test/subtest events and failed three
top-level groups: absent returned/advertised catalog identity, independent identity
projection and detached inspection mutation. Old SDK schema rejection of the unknown
argument passed the invalid-input cases; that is not old catalog-guard enforcement.
The first implementation passes20 events in `green`, including real catalog-only
metadata drift across two separate MCP server sessions and paginated catalog binding.

The independent baseline/candidate handler acquisition proves26 HTML documents
byte-identical and24 JSON responses changed only by the additive catalog identity
(`compare-candidate`). Both builds and all executed inputs are retained.

### Prospective next-action correction

Review of that candidate found that `selection.next_action` still mentions only the
inventory revision. A new frozen `recovery-red` test fails on this omission. Before
the second implementation, admit these exact additional JSON-copy changes: matched
work names both detail guards, filtered-out work carries the catalog guard into a
scope retry, and untracked work names both identities in the contribution handoff.
Statuses, scopes, records, hashes, limits and all other bytes stay fixed. Gallery
does not render this machine-readable field, so HTML byte parity still applies.
The second comparison must allow only those declared complete strings plus the
additive identity field; preserve the first candidate/contract/oracle as separate
evidence, not a retroactive promotion.

The corrected candidate passes21 focused events (`recovery-green`) and1,332 events
across MCP, quality-plan, catalog and Gallery race suites (`regressions`), with no
source drift. `compare-candidate-reviewed` retains byte equality for all26 HTML
documents and permits only the declared catalog-identity/action changes in24 JSON
responses. No renderer, CSS, inventory, catalog source or per-capability pin changed.
Its actual-handler response SHA-256 is
`26e0ea64a0d0c24611abc519101be34e41f88236af76b6ad670a6ef9529e4534`.

An additional private stdio check will use the actual cgo-free MCP executable built
from the frozen candidate, a minimal child environment and only the isolated source
root. It must initialize, advertise the optional schemas, complete capability →
quality list → exact detail with both guards, reject stale/malformed guards without
payload, preserve independent inventory refusal, emit only JSON-RPC on stdout, and
exit/drain cleanly on EOF. Bind executable/source/harness identities before launch.
No installed server, real licensed corpus or agent-host configuration is involved.

The final independent rebuild produces the same response hash above; `compare-final`
passes26 exact HTML and24 explicitly bounded JSON comparisons. The actual executable
(`c783a09fc06f846083f0979adad3ac31e7c08f137301c0d3350e997ba8116613`)
passes51 stdio assertions across12 responses and one initialization notification.
Capability discovery, guarded work lookup and independent stale/malformed refusals
pass with text/structured parity. EOF exits naturally with code0, no signals, and
complete stdout/stderr drain. The harness freezes its bytes, Node/binary/archive/helper
identities and isolated environment before launch; `stdio-check` retains the exact
requests, responses, raw streams and lifecycle. This is a private source-build check,
not proof that an installed Codex/Claude MCP server has been upgraded.
