# PgBouncer oracle

Bench only, with separate evidence families. The legacy `pgbouncer_oracle.py`
remains a historical C source-position/reference walk. Do not use its output
to reproduce the new A position/reference reading.

| Family | Instrument | Tier and scope |
|---|---|---|
| Definitions and ranges | Native cfparser callbacks enumerate all explicit tokens; syntax ablation attests every line | A: 161/161 across the unchanged six configs; names/ranges retained through private logfile substitution |
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

## Native source positions (protocol r3)

The error-line follow-up is now measured, rather than an availability ceiling.
The whole native parser first reaches a terminal invalid section appended after
all original tokens. Each enumerated token is then corrupted alone, requiring
a native syntax error at exactly its original line. Five existing logfile
values point to private scratch because their setters open the file while
parsing. Every graded key/section/line remains present. Active syslog refuses.
Baseline keeps original numeric/verbose values; observation verbosity changes
only afterwards. Quoted/unused keys, repeated sections/keys and suppressed
logging are authored controls. An independent locator only places mutations;
missing or extra native callbacks refuse the whole family.

`native-positions-summary-r1.json` records 161/161 A definitions/ranges,
1/1 A references, zero faults and median 40.06 MB/CPU-s. Per-input native counts
are 2, 9, 28, 28, 71 and 23; the single subject's ratios equal the pooled
ratios, all 1. The complete original tracked approximation is 440 files with
matching before/after hashes, zero generated/untracked inputs, symlinks or
gitlinks. The reading covers six files/18,826 bytes, a thin population.

After the stock build above, set private Go caches, check disk/load and use one
serial nice run:

```sh
bash scripts/fathom-go.sh build -o workspace/tmp/indexbench ./cmd/indexbench
nice -n 10 python3 -B calibration/oracles/pgbouncer/grade_native_positions.py -subject workspace/calibration/pgbouncer -pgbouncer workspace/oracle/pgbouncer-native/pgbouncer-1.26.1/pgbouncer -indexbench workspace/tmp/indexbench -out workspace/artifacts/pgbouncer-native-positions
nice -n 10 python3 -B calibration/oracles/pgbouncer/probe_position_controls.py -pgbouncer workspace/oracle/pgbouncer-native/pgbouncer-1.26.1/pgbouncer -out workspace/artifacts/pgbouncer-position-controls
PGB_POSITION_BIN=workspace/oracle/pgbouncer-native/pgbouncer-1.26.1/pgbouncer python3 -B -m unittest discover -s calibration/oracles/pgbouncer
```

The measured binary checksum is in `native-position-build-r1.json`; a rebuild
must freeze its new identity before querying. Adjust the receipt only for a
new reproducible build, never to bypass identity refusal. The position CLI
runs all native authored controls with zero skips before any A output.

Independence: the committed control driver runs the actual Fathom extractor
under `-race`. Removing its two nodes leaves native truth unchanged; changing
an input key changes truth. Mutation: filtering native callbacks by locator
sites makes the native-population omission test FAIL, then restored green.
The driver writes its log and count/hash-only receipt. Caps: the reader refuses
1 MiB + 1 source bytes, 8 MiB + 1 native log bytes and 10,001 callbacks against
10,000, as exercised by `test_all_caps_refuse_whole_input`. Unknown events,
includes, identity drift or early native failure refuse the entire input.
No scanner-selected population, fallback, backend SQL, Docker or L3 is used.
Delete completed subjects/source builds after packaging the receipts.
