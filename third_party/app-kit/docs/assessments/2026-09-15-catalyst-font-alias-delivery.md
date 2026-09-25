# P10 exact font aliases: cold delivery passes; warm body observation remains open

The separate `p10-cached-static-v2` experiment implements the finite font-URL mapping
required by the [performance contract](../design/catalyst-settings-performance.md).
It repairs [v1's actual URL coverage gap](2026-09-15-catalyst-cached-static.md), without
changing that receipt, the original no-store reference or the v1 source implementation.
This is scoped delivery evidence, not performance acceptance or target certification.

## Implemented boundary

The pinned Inter stylesheet has40 URL occurrences and38 distinct literal `?v=4.1`
font aliases. Both hosts derive the same ordered manifest, binding CSS SHA/size and
each alias's canonical URL, MIME, decoded SHA and size. All38 existing WOFF2 pins must
participate. There are43 decoded assets and81 delivery URLs, not81 assets or components.
Manifest SHA256: `64bf2d15e55a9faa7793789b891de6e8b227b796ff26dda1c50342cd843e802b`.

Requests match exact raw aliases only. Other query values, extra/duplicate parameters,
encoding, six RSC/Next headers, wrong authority and non-GET/HEAD methods do not enter
the alias cache. The source passes a fresh canonical lookup object to the unchanged
v1 representation helper; its original request and delegate URL remain unchanged.
There is no generic query stripping, source CSS rewrite or response-caching proxy.

Both hosts retain identity/gzip6, strong representation ETags, private60-second
freshness, Vary and GET/HEAD/304 behavior. V2 experiment/manifest headers identify
selected-static handling only, including its406/no-store representation refusal.
Dynamic/query/veto/authority/method fallbacks omit those headers and remain no-store.
Independent review found and corrected the source's initially broader metadata scope
before this browser run. Positive and negative header checks now run on actual HTTP.
Source runtime JavaScript and native controller/companion/fixture CSS remain no-store.

## Reproducibility and selected listeners

Two offline private builds, `p10-go-4287295258` and `p10-go-818110552`, pass the compiled
form/race rehearsal and match all16 manifest artifacts. Compared with selected v1
module915174955, only server source, server tests and executable change; all13 form,
model, controller, style and other artifacts match. No earlier1196-check browser
attestation transfers to this new binary merely because those files match.

| Identity | SHA256 |
|---|---|
| Go executable | `453f0a44872dbd51eb8d35cd5df330be1c85598676bd96a4340f6fa005d643ef` |
| Generated SHA256SUMS | `b4b3e2130bef88ee3c2e95e09497a43c8fcce0f666de93b4a7e3f687ddfbe777` |
| Source v2 entry | `bf658e71e36f1834705be1be88375f62778acae80a76db1a32b3657944af5237` |
| Unchanged source v1 core | `34358fc90b012dffb301bcb404c3fdc780d547008e410de05abb2d353bc90c53` |
| Alias helper | `e1b3bbd0edf3cf43ca471253c4b018506d3de71d7f8b15dc2cbd6a8a1445036e` |

The source v2 draft listener61691 was stopped after its metadata repair; the reviewed
replacement was PID64324 at59234. Go v2 was PID64321 at59229. Both startup manifests
match the alias SHA above. Old no-store listeners55043/46237 and v1 listeners57492/57768
were revalidated and preserved. These PID/port values are dated observations, not
persistent service assignments; revalidate ownership and loaded identities before reuse.

## Actual browser and HTTP result

Private directory: `data/design-assets/p10-cached-static-U0LcdR`.
Plan SHA256: `e792c4f74fb03b7b0b9f3f7165afdf03237228f4e946017cfdf9e18985b79699`.
Receipt SHA256: `c3b3949f51520012a824c44e5e6ae14b3a00e190f47eb4703079dfa7359c6caa`.

| Check | Observed result |
|---|---|
| 81 URLs ×2 hosts ×2 encodings ×4 GET/HEAD/conditional variants | 1,296 pass, with exact bytes and delivery-header identities |
| Exclusion/no-store and header-scope checks | 62 pass:29 source,33 target |
| 390px dark source cold | pass;22 resources,5 selected CSS/font/SVG responses |
| 390px dark native cold | pass;8 resources,4 selected CSS/font responses |
| 390px dark enhanced cold | pass;9 resources,4 selected CSS/font responses |
| Source warm | five actual cache events; two cached font bodies unavailable; sample fails |
| Native/enhanced warm | two samples not run after source failure |

All three cold samples verify the actual query-bearing font responses against their
decoded pins. The source warm sample has22 responses, all completed, with ten controls
usable and fonts ready. It records five `Network.requestServedFromCache` events for
two CSS files, two fonts and one flag. There are no network-loading-failed events.
Nevertheless, `Network.getResponseBody` fails for both cached fonts with
`No data found for resource with given identifier`. Their warm bodies stay unavailable;
the receipt fails collector/resource/body checks. The same-context prime retained
both decoded font bodies, but this collector does not record response ETags or prove
prime-to-reload representation linkage. Those prior bytes cannot be called observed
warm bytes. No full warm-delivery pass or request-failure suppression is claimed.

Warm sample SHA256: `00db9f24c28e39403916179684442757abfeb8dc021e6564e2fc69301c2ee0c9`.
Runner at collection: `95b1fd25f36c20e2b25ff5de06d3dcbc0e31fcb75aed840e876d46da6a1cbd5e`.
Collector: `eadb872abe823f6a8f6ce542025c4dcf79aff48647237db3629eb71436c8d54d`.
Full tool/input pins remain in the plan. Chrome152.0.7977.84, Playwright1.63.0,
Node26.3.0, Apple M4/arm64/32GiB, Darwin25.6.0; loopback, no shaping, reduced motion.
Browser jobs ran serially, in fresh isolated profiles without signed-in state.

## Next evidence needed

Investigate cached-font body observability in the selected CDP/Chromium implementation.
Preserve the failed sample. Do not refetch a font and call that the loaded cached body,
disable caching, silently substitute prime bytes or turn all unavailable bodies green.
A different proof method needs a prospective contract, exact lineage/refusal tests and
independent review; it must distinguish directly observed bytes from inferred identity.
Wider360-sample, desktop-prefetch, interaction/network/offline and performance gates
remain open. This experiment adds no certified target, public API or consumer migration.

After this run, independent review also added explicit expected38/81/1296/62 count
assertions to the runner; that newer runner is not the hash attested above.

## Prospective validator observations, still a strict failure

A second acquisition, `p10-cached-static-t0ResS`, uses those count assertions and a
bounded response-header allowlist. It records ETag, Vary, encoded Content-Length,
delivery experiment and alias-manifest identity, distinguishing absent, malformed,
truncated and ambiguous values; no raw cookies, credentials or arbitrary headers.
Body-required checks and the strict failed-sample disposition are unchanged.

Plan SHA256: `40f95a937c5aa7106fb2685550cf94080a1cc1d0235d78479dee5a3e659014ab`.
Receipt SHA256: `94ae379217d14feaf8f5ae79c33219ce270815e07f190bed8ca598d185a059aa`.
Warm sample: `4445b4254643335d31fe229c742bd4cdc1e502f7ba2e7caa71979849a8b3126c`.
Runner: `2844b4b287a9f24b2d1c388eac837d6e0279643dc37f121f4418e41b506d21a7`.
Payload helper: `3f68b9bbce28149c955ac4f2da03f08ab3a03b51229ed655cc5b0ce97abded3c`.
Source/target identities and other collector pins remain as above.

The same1,296 HTTP/62 exclusion and three cold checks pass; source warm again has
five cache events and two unavailable font bodies, so the remaining two samples are
not run. Both font records now contain identical prime/warm validator metadata:

| Font | Selected gzip bytes | Strong ETag SHA256, also matched to this run's raw HTTP probe |
|---|---|---|
| Regular | 111,170 | `0ccfcdf223f25a9eda422891cab1019bb2494fc3ec3eb703deed5b3c787fc303` |
| Semibold | 114,718 | `ef2e706180fd2f436ad3c8b35cd727d59479d7864d991d16d803f7d860fffa99` |

These are observed matching fields, not a completed lineage checker or observed warm
decoded bytes. Request/context/frame/epoch linkage, structured error classification
and the [independently reviewed supplemental contract](../design/catalyst-settings-performance.md#supplemental-warm-representation-identity-prospective-only)
remain to implement; strict failures and unknown-body totals are retained.

## Verification scope

Root `go test ./...` and `go vet ./...` pass with GOPROXY/GOSUMDB disabled. Both
explicit private compiled-form builds include race tests. Sixty focused Node tests
pass, including exact aliases, source/cache/HTTP checks, header ambiguity canaries,
controller, zoom/focus and collector failure retention. These suites join `task check`;
the full regeneration gate was not rerun. Twelve root intent structural groups pass
(78 router lines,150 scope lines,44 ADRs), not a nested-component/host/semantic audit.
No consumer/global/dependency change, staging, commit, publication or deployment.

Follow-up: [implemented supplemental warm identity](2026-09-15-catalyst-warm-font-identity.md)
adds new prospective lineage/error observations and two validator links. The older
receipts above remain strict failures; this does not retroactively supply their gaps.
