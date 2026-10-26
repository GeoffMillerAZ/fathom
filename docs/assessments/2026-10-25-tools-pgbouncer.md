# PgBouncer: native reference evidence at L2

The original 161/161 definitions and 1/1 reference counts are unchanged. The
reference family now uses PgBouncer 1.26.1 itself over the entire canonical
population, moving L2 good to excellent; independently scanned source positions
keep L1 good. Six configs contain one real auth_dbname pair. That is the full
small population and provides no claim of broad operational coverage.

The protocol was committed before native queries. Native SHOW commands check
all entity memberships. For every declared local auth_dbname pair, the harness
disables its target and sends an unknown-user startup to its source database.
PgBouncer's authentication path emits the disabled authentication database
identity before backend connection. A scratch negative control changes only
that value to an undeclared name; native protocol error and log distinguish the
missing target. Wildcard/self-reference controls fail closed, as do missing
configs, membership disagreements, version/checksum drift and empty families.
No independent-scanner fallback is emitted in A reference output.

All backend endpoints point to a nonexistent private local socket directory.
No PostgreSQL backend, backend SQL, Docker or external service was used. Native
processes run only on private scratch Unix sockets and terminate at exit. The
release build uses pinned ISC PgBouncer and static BSD libevent without TLS;
this is a bench instrument and introduces no product dependency.

`native-build-r1.json` records source/archive and binary identities.
`native-summary-r1.json` retains counts and hashes of full native evidence;
subject contents and per-file inventories stay in scratch. Both successful
and negative controls ran for the complete one-pair population. The canonical
reading was produced by lang-oracle and identity-checked three-run lang-retime:
39.97, 39.47, 38.58 MB/CPU-s, median 39.47, zero faults. These tiny-file timings
are not a general throughput benchmark. No extractor, epoch, generated tracker
or checkbox changed. The oracle README records reproduction and all five X-R4
search sources. No model decision applies to these deterministic probes.
