# Kong native population and filename-selected comparison

Kong's go-database-reconciler 1.42.4 (Apache-2.0), the exact library pinned by
decK 1.68.0, enumerates 37 semantic pairs and 1,332 named definitions over
681 distinct upstream paths. The 1,803 historical selectors include repeated
scratch-root aliases; all are queried and must agree before comparison.

Fathom's filename ownership (O348) chooses only four pairs, all `route.service`,
and 365 definitions on 231 files for comparison. Those owned-file subsets
agree, but the other 33 pairs and 967 definitions in accepted, unowned files
were not compared. **No L2 tier-A recall or full-population L1 grade is granted
from these product-selected subsets.** The receipt keeps native field authority
separate from comparison coverage; it emits no support grade or tiered recall.
Independent source positions remain C.

| Field family | Resolved sites | Distinct pairs | Name-owned pairs compared | Pairs not compared |
|---|---:|---:|---:|---:|
| route.service | 7 | 7 | 4 | 3 |
| plugin.route | 31 | 29 | 0 | 29 |
| plugin.service | 1 | 1 | 0 | 1 |
| service.host to upstream | 0 | 0 | 0 | 0 |
| Total | 39 | 37 | 4 | 33 |

The 352 other written host fields are literals without a named upstream.
The critic's 31 plugin-route figure counts sites; two repeated sites collapse
under the existing `(path, from, to)` semantic-pair key. Receipt field-family
counts preserve both units. No service-host binding is exercised by the corpus.

## Standing scope limits

Canonical 568 files / 821 definitions / 10 written pairs include each owned
state file two or three times through nested scratch-root aliases. Distinct
owned counts are 231 files / 365 definitions / four pairs; ratios and canonical
grades are unaffected. O348's deck counts are 338 unowned of 454 distinct
accepted paths, with 116 owned; GDR has 112 unowned of 219, with 107 owned.
The historical synthetic copy SHA does not attest old scratch-tree bytes;
this replay identifies actual upstream pins and hashes its YAML-only
`testdata` to `td` copy mapping. No canonical reading or corpus is changed.

The canonical oracle and its commands remain in README.md. Supabase's DB-less
file is outside this replay: decK rejects its acls/credentials/comments,
definitions are unattested, and 14 tier-C route pairs are unscored. The standing
trigger is an oracle accepting that DB-less input, including the available but
untried Kong runtime `kong config parse` route. O347 parent selection remains
held by T-R2; O348 filename coverage remains open. L3 work waits for D19.

## Native evaluation and controls

The owner's typed reader enumerates service, route, upstream, target and plugin
families, including unused/unnamed entities and consumer/group plugin scopes.
A byte-verified read-only observer preserves typed Host/URL before owner URL
normalization. Removing it exactly restores the pinned source. Native builder
arguments, control flow and results are unchanged. Explicit foreign fields
remain separate from nested containment; strict literal host/upstream-name
matching rejects the registry's broader ID alias.

The independent C locator attaches positions only after native enumeration and
must match the complete named multiset. Unknown outcomes/scopes, mismatched
locations and resource caps refuse whole units. Caps are 1 MiB source, 10,000
entities plus fields, 8 MiB output and 30 seconds per child. Authored exact and
cap-plus-one controls refuse completely. Native HTTP is denied; execution uses
clean private environment/HOME and OS network denial, never Docker or a backend.

Fourteen native race tests, two native omission mutants, three official decK
cross-checks, output-removal tripwire and changed-input sensitivity precede
replay. The tripwire alone proves no independence claim. Fresh controls bind
to the native binary, `native/main_test.go` and generator hashes. Nine Python
tests and three FAIL-to-GREEN mutants protect locator independence, denial of
subset-based grades, and the distinction between sites and pairs.

`native-protocol-r3.json` is the dated amendment. The old r2 combined guards
refused 325 selector queries over 112 distinct inputs (303/101 in cohort 2,
22/11 in cohort 4). Allowing unchanged deployment-routing Konnect metadata
left four refusals over two cohort-4 inputs. Correcting the mistaken external
file interpretation of `_config` to the owner's in-file PluginConfigs lookup
left zero refusals. These corrections landed with results, rather than being
preregistered in the old r2 snapshot. The amendment records that history;
missing native config entries and unsupported service packages still refuse.

## Reproduce with fresh controls

Use private TMPDIR/GOTMPDIR/GOCACHE/GOMODCACHE, GOWORK=off,
GOTOOLCHAIN=local and MACOSX_DEPLOYMENT_TARGET=14.0 on Darwin. Require one
serial nice-10 heavy job and at least 20 GiB free before each heavy step.
Builds and tests retain the load-six gate. T-R10 permits the full correctness
replay to start at one-minute load at most nine. Throughput requires load at
most six throughout the sampled three-run median and named conditions;
otherwise report it as unmeasured. Throughput never licenses a subset grade.
Use fresh output directories. The full replay needs a private Python with
PyYAML; other scripts use the standard library.

```sh
nice -n 10 python3 -B calibration/oracles/kong/build_native.py -out workspace/oracle/kong-native -verify-second-build workspace/oracle/kong-native-second
nice -n 10 python3 -B calibration/oracles/kong/fetch_deck.py -out workspace/oracle/kong-deck
nice -n 10 python3 -B calibration/oracles/kong/probe_native_controls.py -build workspace/oracle/kong-native -deck workspace/oracle/kong-deck/deck -deck-receipt workspace/oracle/kong-deck/receipt.json -out workspace/artifacts/kong-native-controls
python3 -B calibration/oracles/kong/probe_projection_controls.py -out workspace/artifacts/kong-projection-controls
(cd calibration/oracles/kong && go test -race -count=1 -run '^(TestLocationsRemainIndependentCAndKeepUnusedEntities|TestLocationSourceCapPlusOneRefusesWholeInput)$' . && go build -o ../../../workspace/oracle/kong-locations .)
bash scripts/fathom-go.sh build -o workspace/tmp/indexbench ./cmd/indexbench
nice -n 10 python3 -B calibration/oracles/kong/grade_native_l2.py -native workspace/oracle/kong-native/kong-native -build-receipt workspace/oracle/kong-native/build-receipt.json -controls workspace/artifacts/kong-native-controls/summary.json -projection-controls workspace/artifacts/kong-projection-controls/summary.json -locator workspace/oracle/kong-locations -indexbench workspace/tmp/indexbench -subjects workspace/calibration/kong-native -out workspace/artifacts/kong-native-population
```

The build uses `-trimpath -buildvcs=false` and a private probe module with a
stable `./native-source` replacement. Absolute replacements otherwise survive
in Go build metadata even with trimpath. It checks two independently located
builds with the same target, Go compiler and flags. The second scratch build is
removed after matching. Per-build identity receipts remain necessary across
different compilers or targets. The Verified replay must use freshly generated
controls, rather than the committed historical control summary alone.

The locator's `-locate-source` mode returns before historical route/Compose
paths. Only the new focused location tests are verified here. The legacy
Docker-dependent `TestScanDeployedTiers` remains unverified; an earlier broad
selection invoked `docker compose config` and failed without starting a
container. No test or golden was weakened. Prune measured subjects and source
builds immediately. Publish only ordinal counts/hashes, never native names,
subject paths/people/content or raw logs. No ENGINE or EPOCH change is made.

## Five-source search record

`candidate-search-r1.json` records the X-R4 sources, pins, hashes and access gaps.
No absence ceiling is asserted.

| Source searched | Candidate/version/licence | Result |
|---|---|---|
| Owner toolchain | decK 1.68.0 / GDR 1.42.4, Apache-2.0 | Native full population is available; filename-selected comparison coverage is incomplete. |
| SCIP/LSIF indexer lists and GitHub scip-kong search | Lists queried 2026-10-26; no producer candidate | No Kong entry or query hit; legacy redirects, 403 gaps and rate limits recorded. |
| Language servers | YAML Language Server 2.0.0, MIT | Schema/symbol services are C; no Kong foreign-key evaluation, not installed. Dedicated query had no hit, not a global absence proof. |
| Registry/parser sources | kongfig 1.5.3, MIT; GDR 1.42.4; yaml.v3 3.0.1, Apache-2.0/MIT | kongfig uses legacy APIs/live Admin API and optional JS, untried; yaml.v3 supplies C positions only. |
| Runtime parse/tracing | Kong 3.9.1, Apache-2.0 | `kong config parse` calls declarative parse_file; available untried DB-less route. Import is mutating and separate. No runtime build/install/Docker; 3.14.0 source attempts returned 404. |
