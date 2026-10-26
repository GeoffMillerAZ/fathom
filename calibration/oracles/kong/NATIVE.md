# Kong native written-field diagnostic

The bench-only prototype uses decK1.68.0's exact owner library,
go-database-reconciler1.42.4 (Apache-2.0). No product dependency, engine,
epoch, generated tracker, corpus or canonical L3 reading changes.
The canonical oracle remains documented in README.md. The original A label
for independent yaml.v3 ranges needs field-authority correction; this probe
keeps independent source positions C. Native reference authority applies only
to fields evaluated over the complete measured population, not to this small
authored sample alone.

The owner's typed Content reader supplies every service, route, upstream,
target and plugin in the declared families, including consumer/group plugin
scopes and unused/unnamed entities. A read-only observer records raw typed
Host/URL before native URL normalization. Removing the observer exactly restores
the pinned original source. Native builder decisions/arguments/returns remain
unchanged. Go disallows module-cache overlays, so the build overlays a verified
private copy through a scratch modfile; the cache is never edited.

Written route/service/plugin fields are distinct from nested implicit
containment. Native owner state/registry evaluation resolves actual foreign
keys. A service host obeys the strict literal upstream-name contract: the
registry's broader ID lookup cannot invent an ID-to-name host alias. This is
static written-field evidence, not runtime traffic or backend reachability.
Every native outcome is classified before comparison. Unknown outcomes,
unsupported typed scopes, missing native config entries, caps or unmatched
location multisets refuse the whole family; no partial refusal becomes truth.
The independent C locator only attaches positions to the complete native named
multiset. It cannot pick, add or remove native recall candidates.

## Authored proof before calibration

`native-authored-controls-r1.json` records 14 native Go tests under -race,
zero skips, two mutation FAIL-to-GREEN controls, three official CLI renders,
8 actual Fathom nodes removed as a future-hidden-read tripwire and changed-input
sensitivity. The tripwire alone proves no independence claim. Discriminating
controls drop the raw Host observation or omit consumer/group populations and
fail the focused tests, then restore green. The full native truth hash changes
when an input fact changes. CLI runs use a private HOME, analytics disabled
and an OS network-denial profile. URL-only input yields a rendered host but no
written-host reference; an explicit host yields one.

`native-projection-controls-r1.json` records four Python controls. Filtering
native definitions by the locator's candidates makes the omission test FAIL,
then restored GREEN. Missing/extra located entities, tier inflation and unknown
native outcomes refuse. Unnamed endpoints and external hosts remain counted
as unscored outcomes.

Caps: source1MiB, native entities plus fields10000, output8MiB, child30s.
Authored exact source/entity/output cap+1 cases refuse the whole input/output.
Native HTTP requests are denied. No server, Docker, backend, subject task,
paid call or global install is part of the native driver. Go nil slices are
normalized to empty JSON families without dropping any nonempty field.

## Full original-population protocol

`native-protocol-r2.json` records all 1,803 historical selectors from nine
cohorts, reconstructed from 681 distinct upstream source paths. The historical record reports 568 scored files and 821 definitions;
no new exclusion or holdout is introduced. Two historical state-cohort SHA
fields point at a synthetic source commit. The new replay uses the actual
shard pins, separately hashes the existing YAML-only testdata-to-td copy mapping
and preserves source bytes. Every real source clone receives exact normative
tracked-set manifests before and after. Raw content, inventories, names and
paths stay private; public receipts contain ordinal counts and hashes only.

The full native population and complete authored controls precede comparison.
The completed diagnostic receipt is native-population-r1.json: 231 unique
scored files, 365/365 definitions C and 4/4 scoped references A. Historical
copy-tree bytes are unavailable; the synthetic old SHA cannot attest them.
This explicit upstream-pin reconstruction is not a bit-for-bit replay of that
old scratch tree. Throughput and the performance-dependent grade are unmeasured. The canonical 821/821 definitions, 10/10 references C
and 12/12 route pairs A remain unchanged until measured/integrated. L3 work
still waits for controller D19 authorization; the shared parent selector remains
held T-R2. This is open measurement work, not an unavailable-oracle ceiling.

## Reproduce in private scratch

Use private TMPDIR/GOTMPDIR/GOCACHE/GOMODCACHE, GOWORK=off,
GOTOOLCHAIN=local and MACOSX_DEPLOYMENT_TARGET=14.0 on Darwin. Before each
heavy step require free disk >=20GiB and one serial nice10 job. T-R10 allows
the full correctness replay to start at 1-minute load<=9; other heavy steps
retain load<=6. Throughput needs load<=6, median of three and named conditions;
otherwise it and the performance-dependent diagnostic grade are unmeasured. Use fresh
output directories. A private Python venv with PyYAML is needed for the full
replay only; the probe/build/control scripts otherwise use the standard library.

```sh
nice -n 10 python3 -B calibration/oracles/kong/build_native.py -out workspace/oracle/kong-native
nice -n 10 python3 -B calibration/oracles/kong/fetch_deck.py -out workspace/oracle/kong-deck
nice -n 10 python3 -B calibration/oracles/kong/probe_native_controls.py -build workspace/oracle/kong-native -deck workspace/oracle/kong-deck/deck -deck-receipt workspace/oracle/kong-deck/receipt.json -out workspace/artifacts/kong-native-controls
python3 -B calibration/oracles/kong/probe_projection_controls.py -out workspace/artifacts/kong-projection-controls
(cd calibration/oracles/kong && go test -race -count=1 -run '^(TestLocationsRemainIndependentCAndKeepUnusedEntities|TestLocationSourceCapPlusOneRefusesWholeInput)$' . && go build -o ../../../workspace/oracle/kong-locations .)
bash scripts/fathom-go.sh build -o workspace/tmp/indexbench ./cmd/indexbench
nice -n 10 python3 -B calibration/oracles/kong/grade_native_l2.py -native workspace/oracle/kong-native/kong-native -build-receipt workspace/oracle/kong-native/build-receipt.json -controls workspace/artifacts/kong-native-controls/summary.json -projection-controls workspace/artifacts/kong-projection-controls/summary.json -locator workspace/oracle/kong-locations -indexbench workspace/tmp/indexbench -subjects workspace/calibration/kong-native -out workspace/artifacts/kong-native-population
```

The locator's `-locate-source` mode returns before historical route/Compose
paths. Do not run the whole legacy module test suite on this no-Docker helper:
TestScanDeployedTiers requires Docker Compose. One overbroad validation attempt
invoked `docker compose config` and failed; no container ran. It is not counted
as a pass or bypassed by changing that test. The two new location tests pass
with -race. The held L3 route test remains unverified here.

Keep per-build identity receipts; no historical binary hash is assumed portable.
Prune measured subjects and completed source copies/downloads immediately.

## X-R4 five-source ledger (2026-10-26)

`candidate-search-r1.json` records source URLs, exact pins, response hashes and
access gaps, including the shared GitHub API rate limit. No absence ceiling is
asserted.

| Source searched | Candidate/version/licence | Result |
|---|---|---|
| Owner toolchain | decK1.68.0/GDR1.42.4 Apache-2.0 | Typed reader, state builder, raw-field observer and owner registries complete all1,803 reconstructed-selector queries;4/4 scoped references A, throughput unmeasured. |
| Sourcegraph SCIP/legacy LSIF lists; GitHub `scip-kong` | Lists queried2026-10-26; no producer candidate | No Kong entry; GitHub query zero. Legacy URL redirects; old-version/direct403 gaps and later API rate limit disclosed. |
| Language-server primary README/registry; dedicated query | YAML Language Server2.0.0 MIT | Schema/symbol services are independent C, not Kong foreign-key evaluation. Dedicated query zero; no global absence claim; not installed. |
| Package registry and typed parser sources | kongfig1.5.3 MIT; GDR1.42.4 Apache-2.0; yaml.v3 3.0.1 Apache-2.0/MIT | kongfig uses legacy apis/consumers and live Admin API, with optional JS; not installed/executed. Native SDK is stronger; yaml.v3 only C locations. |
| Runtime configuration parse/tracing source | Kong3.9.1 Apache-2.0 | `kong config parse` invokes declarative parse_file; import is a separate mutating route. Available untried Kong/OpenResty route; no runtime build/install/Docker. Version3.14.0 source attempts404; pinned3.9.1 source read. |

## Completed diagnostic and scope limits

The script-emitted native-population-r1.json records zero refusals over all
1,803 selector queries. Historical scratch-root prefixes map to681 distinct
upstream paths; aliases are queried and must agree, while comparison counts
one actual source once. The old synthetic shadow SHA cannot attest old scratch
bytes. This reconstructed unique-source population is not directly comparable
to the historical568file/821definition/10reference totals.

The native whole population is1,340 entities,391 written fields and37 semantic
pairs. Existing ingest gates score231 unique files:365/365 definitions and
ranges C,4/4 references A,zero faults. Thirty-three native pairs and967 named
definitions remain outside comparison scope; no full37-pair recall is claimed.
Timing saw load7.61, so throughput and the performance-dependent grade are
unmeasured underT-R10. Canonical readings and held L3 work remain unchanged.

Authored regressions reproduce two overbroad guards: Konnect deployment-routing
metadata does not change offline Gateway fields; native fillPluginConfig
resolves _config against the in-file PluginConfigs map. Neither requires a
backend or external file. Missing native config entries still refuse. Service
packages remain an unclassified whole-input refusal; none refuses in the full
population. Fourteen native race tests, two mutations and three network-denied
CLI cross-checks pass. Six Python scope/projection tests and two focused location
race tests pass; the legacy Docker-dependent L3 test stays unverified.
