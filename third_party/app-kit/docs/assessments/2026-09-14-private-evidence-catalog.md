# Private historical evidence catalog

## Result and boundary

Two P06 packets now survive independently of their temporary preparation directories
in the private design-assets store. CLI, read-only MCP and Gallery share historical
metadata, exact IDs and revision. Search shows recorded claims; exact get rechecks
retained bytes and original membership. **Neither operation certifies a target.**

All six affected package suites pass with the race detector. Two local browser runs
each pass 57 scoped checks with no errors or forbidden requests. Both reports and all
eight paired screenshots are byte-identical. Independent review's catalog-capacity
finding was fixed and regression-tested; its final bounded review found no other issue.

This advances G1's evidence discovery, not full conversion qualification. Both packets
retain all nine full gates as `not-run`; the aggregate containing a prospective task
remains retrospective. Totals stay **439/657 compiler/DOM, five typed, zero certified**.

## Implementation

[ADR-0043](../adr/0043-private-historical-evidence-catalog.md) and the
[target-evidence guide](../design/target-evidence.md) own the limits and access contract.
`internal/designevidence` handles canonical immutable bundles, original membership,
bounded CUE metadata, deterministic pagination and exact historical rechecks. CLI adds
explicit pin-gated saving/search/get. MCP adds `search_design_evidence` and
`get_design_evidence`; Gallery adds `/target-evidence` and `/api/target-evidence`.
There is no new MCP server, registration, dependency, public runtime API or consumer write.

Synthetic checks cover roundtrip/idempotency, no-clobber/admission and corrupted state,
metadata detachment, cursor identity, private read-only routes, escaped labels and
original recovery. Nine claimed passes still yield `not-verified`. Actual MCP protocol
results match Gallery JSON for the same synthetic store. That is protocol proof, not
evidence that an installed agent host has refreshed its binary or called the new tools.

The initial JSON preflight accidentally limited `entries` to 1,024 despite a declared
4,096-record bound. Only top-level `entries` now receives that allowance; nested arrays
retain their smaller cap. Tests admit 1,025 and reject 4,097 at encode/decode boundaries,
and cover three-result pagination plus limit/filter/revision cursor changes.
Two initial Gallery test failures were test setup errors: HTML library setup intentionally
returns 200 while its API returns 503, and URL fragments are not sent in HTTP requests.
Assertions now model those existing contracts; no production behavior was weakened.

## Saved records

Root: `/Volumes/m2ext/code/geoffmilleraz/app-kit/data/design-assets/evidence`.
Immutable files live at `entries/<bundle-sha>.json`; `catalog.json` is the metadata index.
Files were observed as 0600, both directories as 0700, and all evidence files are ignored
by Git. Backups remain owner-managed; filesystem permissions alone do not prove ACL,
hostile-same-user, power-loss or backup durability.

| Packet | Bundle digest (ID is `evidence@` plus digest) | Matched artifacts |
|---|---|---:|
| Retained P06 history | `0e30ef137f0f8da420db37d7bf88ee8ca3a604ed21b0fd2c2e93ba40db51912a` | 10 |
| History plus prospective Links task | `994e15283cbaa57570986429816bd42fadf91455531985c69c4ca48b7c7bbe1e` | 41 |

| Identity | SHA-256 |
|---|---|
| Retained contract | `854eaeb9483e5df1422a46fd6ab9b5a44e7ba070212dd519914a6baf257430ee` |
| Retained receipt | `4e5df913973635c9e6d51ec6ee184bc4dc790e6c2a7d7492331f3c9e26e44c91` |
| Aggregate contract | `320036c1cf137c45339a7f145d1f9cd50968f75865f93bf5324d59cb18642252` |
| Aggregate receipt | `8d99d0eaeffeeb2c2353efb23967de1e26d1b52668e2f2bc035d785607d74b0b` |
| Source member | `01345180cee4744b57e50a3ceca9461566fdfd7ab5d8f629113654bac052ca31` |
| Adapted output | `8ab0e692df0b1521097ac04f3847c68ddf3c5e9acaca13c13e8aa0289d78bea6` |
| Catalog file | `821d675aa9bb88782aa4969845965317536977df7c8cabb9063d7990eb5707ca` |
| Catalog revision | `8fc126b3a13a6552e2fe6d96093be9c03c668ccbc89c047f47de4b40b5760dd6` |

Subject `reference.navigation-adapter`, profile `static-web`, variant `links`; original
member `html/navigation/tabs/01-tabs-with-underline.html` in
`application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8`.
The second packet retains the separately frozen `quality-2DrLVH` contract/report,
collector/checker/schema, preparation/binary and 24 images from the
[prospective task](2026-09-14-navigation-semantic-proof.md). Its explicit exporter runs
the pinned semantic checker before copying; generic store reads do not execute it.

Live CLI/Gallery page JSON matched byte-for-byte for the subject/static-web/limit10
query, as did both exact details. Fresh results: `integrity=matched`,
`original_membership=matched`, `verification=not-verified`, nine incomplete gates.
No corruption canary or deletion was performed against these real private records.

## Browser evidence

Maintainer fixtures and commands: [README](fixtures/target-evidence/README.md).
Pinned Chrome 152.0.7977.84 and isolated Playwright 1.63.0, macOS/arm64, fresh contexts,
application JavaScript disabled, reduced motion, same-origin GET-only request admission.
No signed-in browser profile or Tailwind portal access was used.

Matrix: 390/1440 CSS-pixel viewports × light/dark. The task follows a one-record page,
Next, browser Back, detail, original and Back; applies an empty profile filter, resets
while retaining theme, and recovers from an invalid limit. Narrow activation uses
natural Tab/Enter; wide activation uses pointer clicks. Detail checks include all nine
gates, explicit uncertified status, no document overflow and visible main controls ≥44px.
This geometry assertion is scoped to detail controls, not every Gallery header control.

Private run parent `/tmp/appkit-target-evidence.BuGtOc`; runs `browser-AWqqWU` and
`browser-EJS5RS`. Both `report.json` hashes:
`80a675595e8ccee14b2f883baff598f88c1aa619040621c89bc76dffd6277721`.
The repeat changes only cleanup placement: version admission now lives inside `try`,
so a tool-version mismatch also reaches browser close/report writing. No assertion or
threshold changed. Eight screenshots per run show list/detail in each lane.
Visual review sampled all four lane types: readable form/result hierarchy, wrapping
identifiers, distinct gate rows, clear recovery actions and no clipped content.
No screen-reader, physical touch, other-engine, contrast-matrix or performance claim.

| Executed artifact | SHA-256 |
|---|---|
| Gallery binary | `c3e2446e369bd7d23217919916587823c2a39b24d0d39d7aa51372ffde7ebc14` |
| CLI binary | `987fb5bacacc6afc40d469381c77ba997ccd26312e53236184b60f5aae425372` |
| Generated Gallery CSS | `23fa83d0a14a2c5d22dec4bb4eb53f2ca85621e48859ff97d5dd982bb19a52fe` |
| Final browser runner | `221f55722059b7f082533342902bff2b2e3a1c04c5eb2d7fa375e505a1eaa1bf` |
| Explicit P06 exporter | `86d9eb32efc656cd31910a001282189047adf3fd880b76a508e88161478ed8b5` |

## Verification and remaining work

`GOWORK=off GOPROXY=off go test -race -count=1` passed for designevidence (6.260s),
designreceipts (3.386s), navigationproof (13.756s), Gallery (22.464s), CLI (5.244s) and
MCP (24.354s). The explicit P06 export test separately passed (0.690s), retaining 10/41
artifacts. Source-only tests skip the opt-in private export; that skip is not live proof.
The three new schema-vet tasks pass separately; combining different CUE packages in a
single file-list invocation was rejected, then corrected without changing schemas.
`go vet` passes for the same six packages. Scoped intent checks pass: living/Unreleased
budgets, required docs, 43 ADR shapes/index entries, paired routing stanzas and 212
relative file links; an additional three-document link check passes 62 links. These
checks exclude anchors, installed skills, host invocation and a full intent-doctor audit.

Complete dependency closure, full applicable pilot gates, independently reviewed
semantic admission, consumer history/promotion and release propagation remain open.
The store preserves evidence, not its authenticity or current compatibility. The full
G1–G6 goal remains active; Windows and CA/GGB remain deferred. No commit, staging,
consumer/dependency/global-configuration change, deployment, push or tag was performed.
