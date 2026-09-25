# P10 warm fonts: supplemental validator linkage, not observed warm bodies

The [prospectively reviewed diagnostic](../design/catalyst-settings-performance.md#supplemental-warm-representation-identity-prospective-only)
is implemented and has one actual browser result. It links both cached source fonts
to observed prime bytes and the same encoded representations checked through HTTP.
The strict warm sample remains failed; there is no complete delivery/performance gate.
Earlier [font-alias receipts](2026-09-15-catalyst-font-alias-delivery.md) are unchanged.

## What is implemented

`warm-font-identity.mjs` is a pure metadata checker, not a network fallback. It requires:

- One isolated collection identity, exact URLs/request IDs, same frame, distinct
  prime/reload navigation loaders and correlated, ordered request/response/finish events.
- Exactly one prime font per unavailable warm font, an actual per-request cache event,
  GET200, explicit no-service-worker/no-prefetch flags and no transport failures.
- Observed prime decoded SHA/size/MIME equal to the selected pin; identical strong
  ETag, encoding, encoded length, Vary and selected v2 headers in prime/warm and the
  same-encoding raw-HTTP probe from this acquisition.
- Only the exact bounded missing-body error tuple under the selected Playwright
  version/wrapper. Other errors, ambiguous/missing fields and substituted bodies refuse.

The collector adds navigation lineage, cache flags and structured error observations;
existing failure arrays, direct-body availability, byte totals and cache counts remain
unchanged. The smoke writes the strict sample first, then a separate
`warm-identity-004.json`. Supplemental refusal or wrapper unavailability cannot turn
the sample green or prevent its normal strict checks. The original stop/exit rule stays.
Missing wrapper identity and sidecar-write errors are recorded as supplemental gaps,
not new prerequisites for ordinary cold or strict browser collection.

Local inspection of the pinned Playwright1.63.0 `playwright-core/lib/coreBundle.js`
shows the CR session forwards the protocol error message but not its numeric code
(lines34988–35006); protocol/API wrappers add deterministic command/API prefixes
(lines13372–13376,57395–57450). Numeric code is therefore absent in this actual
observation, not an invented `-32000`. The exact captured tuple is ordinary `Error`,
command `Network.getResponseBody`, and the full pinned API-prefixed missing-data message.
A changed name, character, trailing period/newline or wrapper pin refuses linkage.

## Actual acquisition and replay

Private directory: `data/design-assets/p10-cached-static-3j9qHm`.
The same source PID64324 at59234 and Go PID64321 at59229 were revalidated; source
transport, v1 core and Go binary hashes match the preceding assessment. No server
code, source assets, dependencies, consumer or signed-in browser state changed.

| Evidence | SHA256 |
|---|---|
| Plan | `662bc41f392cbd76adf393db80e0b2fe13fe267b5a37f93a19052548ec8009b0` |
| Strict receipt | `f6f9dbfb562fc9de96b306c0c6d6543fb02fda27da1318014d2b79d0d1b2631b` |
| Strict warm sample | `097501b6aa684256f0c80cdd9fa51a919b8212e841b2121277d3122ae95a9504` |
| Supplemental record | `bae7c18e86a6b1f9106a33778aa243ed0c81829fe2d85041a6f3fd8938ba6569` |
| Runner | `0a8f02b5902b7fe5688fff69561a9e2a79746c67106d5040041256f332154e91` |
| Collector | `472a857b4bc5ec99c2c86cab3cded1d3a6813ae267ff3a91d13ec1af6ae9d5b9` |
| Supplemental checker | `7c9ab54d9f5140eddfaaab028b0367a336c20762149b62ea07851eac26067daf` |
| Playwright wrapper | `549070af3acabb3efcc4f55bfe6210f9f7c2fcf633cf7eaa59bfe60719969171` |

All1,296 HTTP representation checks,62 exclusion checks and three cold-browser
samples pass again. The source warm sample has five actual cache events,22 completed
resources and two unavailable font bodies. Its result remains
`failed-partial-delivery-smoke`, exit1; native/enhanced warm samples remain not run.
The supplemental result is `validator-linked-identity-only`, with two font links and
explicit `strict_sample_ok:false` and `direct_warm_body_observed:false`.

Regular links prime request68260.60 to warm68260.148; semibold links68260.64 to68260.152.
Both use one frame and the separately observed prime/reload loaders. Their decoded
prime sizes are111,268/114,812 bytes; selected gzip representations are111,170/114,718
bytes. These are not warm wire-transfer measurements or retrieved warm-body lengths.
Both actual error records retain `code:{availability:absent,value:null}`.

Re-evaluating the retained plan/sample/HTTP metadata reproduces the supplemental JSON
exactly without changing the strict input. Three additional in-memory mutations of
that actual receipt—loader drift, ETag drift and an unrelated collector error—are
rejected. No retained file or historical failure was edited for those canaries.

Environment: Chrome152.0.7977.84, Playwright1.63.0, Node26.3.0, Apple M4/arm64/32GiB,
Darwin25.6.0;390px/dark, reduced motion, loopback without shaping, fresh isolated
contexts. Browser collection was serial; no concurrent browser/benchmark job.

## Verification and remaining work

Root `go test ./...` and `go vet ./...` pass with GOPROXY/GOSUMDB disabled.
The focused Node selection passes68 tests, including pure acceptance/non-mutation,
29 receipt-boundary rejection canaries and11 exact-error rejection canaries in the
new three-test checker suite. Synthetic checks do not replace the actual result above.
Independent review found and closed wrapper gating,204 font admission and ambiguous
error/prefetch metadata gaps before collection. The new suite joins `task check`;
the complete regeneration gate was not rerun.
MCP/catalog tests pass after documentation updates. Twelve root intent structural
groups pass (78 router lines,150 scope lines,44 ADRs), not full host/nested/semantic proof.
Whitespace checks pass, the staged index remains empty, and no isolated browser job remains.

This diagnostic does not authenticate arbitrary submitted reports, retrieve cached
bytes, complete native/enhanced warm coverage or relax the strict measurement contract.
Direct-body completeness, desktop source-prefetch failures, comparative fidelity,
input/network/offline budgets and consumer integration remain open. Keep these gaps
explicit while advancing the other G1–G6 outcomes; do not repeat this same acquisition
without a new question, changed instrument or new target evidence to collect.
No catalog count, compatibility badge, public API or completion percentage changes.
