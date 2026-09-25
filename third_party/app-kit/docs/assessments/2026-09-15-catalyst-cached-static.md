# P10 cached-static delivery: HTTP passes, actual font-URL coverage fails

Separate experiment under [the fixed delivery contract](../design/catalyst-settings-performance.md#fixed-static-asset-delivery-experiment),
not a modification of the no-store baseline or the default360-sample acquisition.
The implemented `p10-cached-static-v1` policy has real HTTP evidence, but the browser
smoke failed. No warm-cache, performance, complete delivery or certification pass is claimed.

## Implementation and exact replay

The source uses a new opt-in Node/Next listener; the Go fixture adds an exclusive
`--cached-static-delivery=p10-cached-static-v1` flag. Original defaults and existing
listeners remain unchanged. Both prepare the packet's43 CSS/font/SVG assets once,
serve identity/gzip level6 with representation-specific strong ETags, encoded lengths,
private60-second freshness and conditional GET/HEAD. Route data, document responses,
errors, mutations and unselected assets remain no-store in this mode.

Source packet/reference SHA256 remains
`94d3ab4c145e33bf29af750918429833d833fff3f9ac8820e1314c7f2c67f53d`.
New Go modules `p10-go-915174955` and `p10-go-1231693542` match all16 manifest artifacts;
their generation runs include private compiled-form race tests, using cached pinned
dependencies with network resolution disabled. Binary SHA256:
`58b17c8f3a91468010622e7448592bc1a327e9ad0ee1a562e65e651ed8925b25`.
SHA256SUMS digest: `54a196c72ddf22306cf68208811accbb79f670d74690180e88b969d0805e3533`.
Only server source, server tests and binary differ from the earlier favicon-fixed
module4253964458; the other13 artifacts, including controller/styles/rendered forms,
match exactly. Earlier review candidates992205456/2102834614 remain private evidence,
not the selected final transport. No old1196-check browser attestation transfers to this binary.

## Actual separate smoke

Directory: `data/design-assets/p10-cached-static-fAcfjg`.
Plan SHA256: `23de917fefb74e304b35592d74721a3f9a03bf43800186d44092bfd23762545a`.
Receipt SHA256: `2daf7485d101f034d2ff38338c88689260524b95f2d6d26b45e7455b7ae57260`.
Sample SHA256: `b88de7d43cdc80e9318f98679deb89026b0418bc08c626fb21049e3956275bae`.

Source PID57492 at57244 and target PID57768 at57316 were attested before acquisition.
The collector verifies each of16 generated file hashes, selected executable mapping,
packet/source closure and actual listener ownership. The old55516/53855 listeners
remain separately running. Revalidate PIDs/URLs before reuse; these are observations,
not persistent service assignments.

| Gate in this run | Result |
|---|---|
| 43 assets ×2 hosts ×2 encodings ×GET/HEAD/conditional GET/conditional HEAD | 688 response checks pass |
| Exclusion/no-store checks, including wrong-host/encoding/empty-static-POST refusals | 32 checks pass |
| 390px dark source cold task | controls/fonts ready; cache-contract failure |
| Remaining source/native/enhanced cold/warm observations | 5 not run of6 planned |
| Terminal status | failed-partial-delivery-smoke, exit1 |

HTTP checks decode and hash actual representation bytes, compare MIME/size and
GET/HEAD/304 metadata, and discard bodies after checks. No redirect is followed.
POST is an empty rejection probe to a selected CSS URL only, not a form mutation.
These direct asset probes are not a user's loaded-page request graph.

That distinction found the failure: the pinned Inter stylesheet requests
`Inter-Regular.woff2?v=4.1` and `Inter-SemiBold.woff2?v=4.1`. The packet lists the
query-free URLs, and v1 explicitly refuses cache admission for any query. Both actual
font responses are therefore no-store, while both selected stylesheets and the visible
flag use the cached delivery headers. All22 source resources are retained; no network,
page or policy errors occurred. The smoke checker rejects absence of a selected font.

The regular/semibold decoded bodies still match their packet pins:
111,268 bytes, SHA `e06f6b1bc553aaea4e4668023ed0ab0a147129c3107f511bc7d03d361b0ae085`;
114,812 bytes, SHA `5cb7103e4e605989afebc03d989c79201e54b21b5183db33981f70db9178a301`.
Byte identity does not authorize stripping query parameters or changing the old receipt.
A synthetic canary now refuses that query-bearing/query-free substitution explicitly.

Runner SHA256: `5d3f1df79e9979b8031765bc105a9efd9f1f4921451647a2d58aff7d413c1768`.
Collector: `eadb872abe823f6a8f6ce542025c4dcf79aff48647237db3629eb71436c8d54d`.
Cache checker: `3d048a53fd0be27d52cadd37634982335d7ff4ca4876fa6145c0fe096a9a5cee`.
Source transport: `34358fc90b012dffb301bcb404c3fdc780d547008e410de05abb2d353bc90c53`.
Full remaining pins are in the plan. Chrome152.0.7977.84/Playwright1.63.0/Node26.3.0;
Apple M4/arm64,32GiB, Darwin25.6.0, loopback/no shaping, reduced motion.

## Next delivery revision

Derive exact font URL aliases from the already-pinned stylesheet, bind each to its
query-free file's SHA/MIME/size, and review that finite admission list before a new
policy revision. No wildcard query support, URL rewriting, source CSS edits or document
caching. Retain v1's failure and independently runnable baseline; re-run the HTTP and
six-sample browser smoke on newly identified transports before broader measurements.
Source runtime JavaScript remains deliberately outside this cache experiment.

Read-only follow-up verifies40 `url()` occurrences /38 unique `?v=4.1` aliases in
the11,497-byte stylesheet, SHA
`46d01c7807f64a24c1b2853b756ef15f3a2facdf4a9f066eaf5d39c0c9935441`.
All38 resolve relative to that stylesheet and map one-to-one to the existing38 WOFF2
pins, with every file hash/size checked. V2 must prove exact alias-set equality rather
than treating this diagnostic count as an executable mapping or a passed repair.

## Verification scope

`GOPROXY=off GOSUMDB=off go test ./...` and `go vet ./...` pass; the explicit private
rehearsals above supply separate paid-input evidence, not an implicit root-test dependency.
The focused controller/zoom/focus/payload/observer/cache Node selection passes51 tests,
including13 synthetic source-listener checks, five cache-checker tests and three smoke
manifest/raw-HTTP tests. New suites join `task check`; the entire regeneration gate was
not rerun. Twelve root intent structural groups pass, with78 router lines,150 scope
lines and44 ADRs; this is not a nested-component, host-wiring or semantic-completion audit.
MCP/catalog checks pass after documentation updates; whitespace checks pass and the staged
index remains empty. No consumer/global configuration, dependency, commit or deployment change.
