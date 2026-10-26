# PgBouncer oracle

Bench only, with separate evidence families. The legacy `pgbouncer_oracle.py`
remains a C source-position/reference walk. Do not use its output to reproduce
the new A reference reading.

| Family | Instrument | Tier and scope |
|---|---|---|
| Definitions and ranges | Independent scanner, every membership checked by native SHOW DATABASES/PEERS/USERS/CONFIG | C: 161/161 across six configs; membership alone does not attest source positions |
| auth_dbname references | Native SHOW DATABASES enumeration, every source startup, and missing-name control | A: 1/1, the entire canonical local pair population; no fallback |

`native-protocol-r2.json` was committed before the population sweep. The earlier
r1 pair probe did not establish recall completeness and is superseded. The format owner's
runtime is pinned to the canonical subject version and checked against the
build receipt's binary digest. All configs are original whole inputs with
machine/global settings isolated in scratch; backend addresses are replaced by
a nonexistent private Unix socket endpoint. No backend SQL, Docker or hosted
service runs. Every native process terminates on exit. SHOW DATABASES supplies the 52 source
databases before consulting scanner pairs: 49 have no auth user, two have no
external authentication database, and one names a disabled authentication
database. Every native outcome is classified; only then is the native pair set
compared to the scanner. Both sets contain one pair, with zero disagreements. The harness refuses
unattested configs/pairs, wildcard/self-reference controls and identity drift.

## Reproduction

Use a clean checkout of the corpus shard's pinned subject. Python 3 standard
library, C compiler, make and the pinned release archives suffice. Install
nothing globally. `build_native.py` builds PgBouncer 1.26.1 (ISC) and static
libevent 2.1.12-stable (BSD-3-Clause) in a private directory, without TLS/c-ares.
The release loader/client/admin sources must match canonical source hashes.

```sh
python3 calibration/oracles/pgbouncer/build_native.py -cache workspace/oracle/pgbouncer-native
python3 calibration/oracles/pgbouncer/native_oracle.py \
  -subject workspace/calibration/pgbouncer \
  -pgbouncer workspace/oracle/pgbouncer-native/pgbouncer-1.26.1/pgbouncer \
  -build-receipt workspace/oracle/pgbouncer-native/build-receipt.json \
  -work workspace/tmp/pgbouncer-native -out-dir workspace/tmp/pgbouncer-native-results
go build -o workspace/tmp/indexbench ./cmd/indexbench
workspace/tmp/indexbench lang-oracle -dialect pgbouncer \
  -subject workspace/calibration/pgbouncer -subject-name pgbouncer/pgbouncer \
  -tool-oracle workspace/tmp/pgbouncer-native-results/defs.json \
  -tool-oracle workspace/tmp/pgbouncer-native-results/refs.json \
  -out workspace/tmp/pgbouncer-native-reading.json \
  -note 'Native PgBouncer 1.26.1 protocol r2: SHOW DATABASES independently enumerates every native database; classify each unknown-user startup with all other databases disabled before comparing scanner pairs. Full native/scanner pair populations agree. Definitions/ranges remain C. Backend endpoints are nonexistent private sockets; no backend SQL or Docker. One reference is a small population, not broad operational coverage.'
workspace/tmp/indexbench lang-retime -reading workspace/tmp/pgbouncer-native-reading.json \
  -subject pgbouncer/pgbouncer=workspace/calibration/pgbouncer -runs 3 -write
python3 calibration/oracles/pgbouncer/summarize_evidence.py \
  -input workspace/tmp/pgbouncer-native-results/native-evidence.json \
  -out workspace/tmp/pgbouncer-native-summary.json
python3 -m unittest discover -s calibration/oracles/pgbouncer
```

Full native responses and inventories are scratch artifacts; the committed
summary carries counts and digests. The target remains L2; no L3 claim follows.

## X-R4 sources searched, 2026-10-25

These are bounded searches, not a universal assertion of tool absence.

| Required source | Named candidate, version and concrete result |
|---|---|
| Own toolchain / dumps / introspection | PgBouncer 1.26.1 [admin interface](https://www.pgbouncer.org/usage.html), SHOW DATABASES/PEERS/USERS/CONFIG and `src/client.c` at f03e9bf: native auth_dbname error path found and measured, replacing the old reference ceiling. SHOW does not report per-database auth_dbname or source spans. |
| SCIP and LSIF catalogs and GitHub | Read [Sourcegraph's supported indexers](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), [SCIP list](https://github.com/sourcegraph/scip#tools-using-scip), and Sourcegraph-maintained [LSIF.dev](https://lsif.dev/) via its [LSIF guide](https://sourcegraph.com/blog/writing-an-lsif-indexer). No PgBouncer-config producer listed. GitHub repository search `scip-pgbouncer in:name` returned zero. SCIP C indexes the implementation, not runtime config spans. |
| Language servers | GitHub/web query `pgbouncer language server config` and the [LSP server catalog](https://microsoft.github.io/language-server-protocol/implementors/servers/) supplied no dedicated PgBouncer-config server. SQL servers concern SQL, not this INI language. No unlocated binary was described as failing. |
| Registry parsers / type checkers / AST dumpers | PyPI query `pgbouncer parser AST` and [pgbouncer 0.2.0](https://pypi.org/project/pgbouncer/) inspected: AGPL-3 runtime-fixture manager depending on native PgBouncer/psycopg2, not a source-span dump. npm search `pgbouncer parser` returned generic parsers (Babel 8.0.8 etc.), not a native PgBouncer evaluator. Its licence does not exclude bench-only use; the published manager interface is not an entity/source-position instrument. The independent scanner remains C. |
| Runtime call / require tracing | Native 1.26.1 [verbose logging](https://www.pgbouncer.org/config.html) and `lib/usual/cfparser.c` inspected: debug logs include parsed section/key/value and acceptance, while line numbers occur on error paths, not a complete successful source-span API. Runtime auth errors plus controlled mutation provide the measured A pair. A future native span-ablation harness is not claimed impossible or tested here. |

The r2 summary records all 52 native sources, classifications, pair-set agreement
and digests; full source identities and native messages stay in scratch.

The only retained L1 limitation is the measured instrument's independently
scanned ranges. Native population enumeration and runtime reference evidence reach this item's L2
target. The small canonical reference population remains explicitly disclosed.

The concrete L1 follow-up is an error-line ablation using PgBouncer 1.26.1
`lib/usual/cfparser.c`: inject one invalid line after each of the 161 entries
and compare the native error line against its source position. This instrument
has not been built or measured; its existence is an open work item, not a claim
that no native source-position route exists.
