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
