# Kong: native typed-reference route found, not yet measured

The canonical reading remains 821/821 definitions, 10/10 written references
at C and 12/12 route pairs at A. No metric, tier or L3 reading changes in this
source audit. The former written-reference ceiling has an unbuilt native route.

The exact decK 1.68.0 module pins Kong's Apache-2.0 go-database-reconciler
1.42.4. Its [typed content reader](https://github.com/Kong/go-database-reconciler/blob/v1.42.4/pkg/file/reader.go)
returns Content before native state construction, which resolves services and
routes using the owner's state lookups. The [vendor types](https://github.com/Kong/go-database-reconciler/blob/v1.42.4/pkg/file/types.go)
carry explicit foreign fields before rendering nests entities. This can test
which references were written rather than merely confirming rendered nesting.

One provenance loss matters: the native service decoder normalizes URL into
Host and does not preserve the original URL/Host presence in Content. The
protocol preregisters a read-only observation of its typed fields before that
normalization. No independent YAML scan may choose the native population.
Native service/route/plugin lookup, full outcome classification and authored
explicit-versus-nested, URL-versus-host, unused and invalid-target controls are
required before A evidence. Original native control flow/results must remain
unchanged, with exact source/observer identity and restoration checks.

The owner's [offline validator](https://developer.konghq.com/deck/file/validate/)
checks foreign relationships without communicating with Kong. The probe must
remain offline too, without Docker, credentials, analytics, backend requests or
global installs. Small authored cross-checks precede any calibration clone.
The SDK has not been installed or executed in this audit. Source pins and the
preregistered contract are in calibration/oracles/kong/native-source-pins-r1.json
and native-protocol-r1.json. Raw source downloads stay private.

The old L1 definitions use native rendered entities but independent yaml.v3
line/end pairing; the code explicitly shares that decoder family with the
product. Field authority needs an audit under P-T1. This pass grants no new
native position claim. X-R4's remaining source families still need recorded
checks before any renewed ceiling. Native L2 work is open; L3 waits for D19,
and a shared parent-binding selector remains under T-R2. No model decision
applies.


## Native authored prototype and full-population preregistration

The pinned owner SDK now builds through a verified private source copy and
read-only overlay, with no module-cache/global edits. Twelve native Go race
tests and four Python projection tests pass with zero skips. Raw Host and
consumer/group population mutants fail then restore green; a locator-filtered
native multiset mutant also fails. An actual Fathom-output removal is a future
hidden-read tripwire; changed input alters native truth. Three network-denied
official decK1.68.0 renders agree on the tested fields, including the URL-only
versus explicit-host distinction. A strict host-name control prevents the
native registry's ID alias from becoming a literal host/name binding. Source,
entity and serialized-output cap+1 tests refuse completely.

The full original input set is 1,803 across nine cohorts, while existing ingest
gates score 568 files/821 definitions. Two historical SHA fields identify a
synthetic copy commit. Protocolr2 uses actual source shard pins and separately
hashes the existing YAML-only testdata-to-td copy mapping. No source byte,
required kind, holdout membership, canonical L3 reading or corpus is changed.
The native Go population chooses definitions/fields; independent yaml.v3 only
attaches C ranges and cannot trim its multiset. Full population is still open,
not an availability ceiling or a new A metric claim.

The five-source ledger is calibration/oracles/kong/candidate-search-r1.json,
with versioned primary runtime/parser/LSP candidates and access gaps. The native
commands and all authored receipts are linked from NATIVE.md. A legacy full
module test run attempted docker compose config and failed without starting a
container. That selection was wrong for this helper: only the focused new
L1/L2 location tests are verified; the held L3 test is explicitly unverified,
and no test or golden was weakened. No model decision applies.
