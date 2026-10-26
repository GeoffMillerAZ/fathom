# SQL native oracle — 2026-10-25

**Measured:** the previous ceiling was wrong. Native SQLite and PostgreSQL catalogs
attest all **17** inline foreign-key pairs in the unchanged D2 scopes, including
three Pagila temporal pairs absent from sqlglot's truth. The new
[reading](../bench/readings/lang/sql.json) has call precision and recall 1 at tier A;
L1 metrics are unchanged. The replay's median of three pooled measurements is
2.09 MB/CPU-s (2.10, 2.09, 1.98), with zero faults. This withdraws the added
LB-Q-sql-l2-rest deferral; LB-Q-sql-rest retains its existing checkbox and L1
contract. No generated tracker, extractor, corpus scope or plan box changed.

## Candidates checked

| Candidate / route | Version | Concrete result and classification |
|---|---|---|
| Universal-ctags | 6.2.1 | Existing L1 names/signatures oracle, tier B; does not supply resolved FK identities. Retained for L1. |
| SQLGlot | 27.8.0 | Replayed existing adapter: Sqitch 14 pairs, Pagila zero; unsupported temporal CREATE TABLEs become command nodes. Known population is 17, so the old 14-pair result omitted 3/17. Independent parser, tier C; superseded for L2. |
| pglast / libpg_query syntax route | pglast 8.5 | Existing range companion; syntax nodes do not evaluate constraints or supply a native catalog. Tier B compiler/reference-parser syntax dump. Used only for ranges and statement boundaries, never FK truth. |
| SQLFluff | 4.4.0 | `Linter.parse_string` returns syntax segments and a PRS violation on the temporal sample, not evaluated/resolved constraints. Syntax parsing cannot establish native tier A for programming-language SQL. |
| Python sqlite3, whole-file route | SQLite 3.43.2 | Historical fresh-database `executescript`: 17/28 Sqitch files complete. Wrong engine for eight PostgreSQL fixtures; missing prerequisites stop upgrades. This failed setup was not an impossibility result. |
| PGlite, whole-file route without vector | 0.5.11, PostgreSQL 18.6 | Historical Pagila main schema stops at missing vector; temporal file stops at unsupported DML. Whole-file parsing hides valid DDL. Superseded below. |
| Native SQLite statement catalogs | Python 3.13.5, SQLite 3.49.1 | All SQLite CREATE TABLE statements accepted; native authorizer and `foreign_key_list` attest inline pairs before later errors/renames. Tier A for declared L2 FK references, not L3 resolution. |
| Native PostgreSQL statement catalogs | PGlite 0.5.11 / PostgreSQL 18.6, pgvector 0.0.12, bundled btree_gist | Version-matched vector extension loads; Sqitch PostgreSQL fixtures run on PostgreSQL in a hand-ordered scenario following script dependency headers; every scoped CREATE TABLE accepted. Catalog deltas attest all three Pagila temporal pairs. Tier A, selected. |

**Measured:** executable candidate checks and their output are in
[data](data/sql-oracle-2026-10-25/candidate-checks.json), with
[the runner](data/sql-oracle-2026-10-25/check_candidates.py).
**Claimed by a vendor:** SQLFluff's
[parse command](https://docs.sqlfluff.com/en/stable/reference/cli/parse.html)
is a syntax-tree interface. ADR-048's reference-parser clause for a tool's own
format is not blanket tier A for any SQL parser: SQL here is a programming language
with native execution available. No parser's names were relabelled native truth.

## Corrected native experiment

**Measured:** Sqitch `ce0550c45ade3527d0c7f981c934e1ea4bdd6c19` contributes the same
28 files and Pagila `9baf49c4149e43229f6021e6218d6b2ac8ef4f34` the same three files.
Their Git blob hashes were checked against the immutable source trees. File and statement SHA-256 digests, source lines and native outcomes
are retained in the [Sqitch report](data/sql-oracle-2026-10-25/sqitch-native-report.json)
and [Pagila report](data/sql-oracle-2026-10-25/pagila-native-report.json).
The two [strict](data/sql-oracle-2026-10-25/sqitch-native.json)
[oracle files](data/sql-oracle-2026-10-25/pagila-native.json) contain only the harness
contract. No holdout was added to the canonical corpus shard.

Every original statement is attempted separately. PostgreSQL catalog deltas after
CREATE TABLE supply local inline keys; ALTER-added keys (37 in the main schema)
and inherited-only partition keys do not belong to the established enclosing-table
contract. SQLite catalogs are observed at table creation, before subsequent rename
or data-copy failure can erase attribution. Its acceptance of a reference to a
missing parent attests the declared L2 reference, not validated data or L3.
The oracle refuses output if any CREATE TABLE fails. Thus its entire 17-pair
population is covered; no invented 100% whole-file execution threshold or partial
FK denominator is needed. The design's 90% import-trace precedent is not silently
turned into a SQL rule.

Six Sqitch failures remain: missing legacy data for INSERTs and missing old tables
for DROPs in the upgrade scenarios. Six Pagila failures remain: four INSERTs need
out-of-scope data (23503); UPDATE and DELETE FOR PORTION OF fail (42601). Each is
named by path, line and code in the reports. All PostgreSQL deploy/verify/revert
fixtures are now run on PostgreSQL in a hand-ordered deploy/verify/revert/rework
scenario following each script's `-- requires:` header; the verify script belongs
to the separate t/sql project, and the order is not derived from sqitch.plan. Files retain fresh
session settings while sharing their scenario database; SQL is not rewritten.

**Claimed by a vendor, confirmed by measured execution:** PostgreSQL
[18 CREATE TABLE](https://www.postgresql.org/docs/18/sql-createtable.html) supports
WITHOUT OVERLAPS and PERIOD foreign keys. The FOR PORTION OF DML work was reverted
before PostgreSQL 19rc1; the [19 UPDATE synopsis](https://www.postgresql.org/docs/19/sql-update.html)
does not contain it. Pagila's PG19 header was therefore not sufficient evidence.
The matching published pgvector package fixes the extension setup independently.

## Reproduction and limits

The [committed recipe](../../calibration/oracles/sql-native/README.md) pins tools,
installation and source identities. Run from the repository root, using its local
tool environment and a fresh output directory:

```sh
python -m unittest discover -s calibration/oracles/sql-native -p 'test_*.py'
nice -n 10 node --test calibration/oracles/sql-native/test_pg_native.mjs
nice -n 10 python calibration/oracles/sql-native/replay.py \
  --sqitch workspace/tmp/oracle-research/sqitch \
  --pagila workspace/tmp/oracle-research/pagila \
  --tools workspace/tools/sql-runtime --ctags workspace/tools/ctags/bin/ctags \
  --indexbench workspace/tools/indexbench --out workspace/tmp/sql-native-replay
```

**Measured:** seven Python and four native PostgreSQL tests pass. The
[summary](data/sql-oracle-2026-10-25/measurement-summary.json) records equal counts,
metrics and grades across all three full runs and hashes the oracle sources.
Historic whole-file probes and outputs are committed beside it under `original_*`
and `original-*`; they reproduce the superseded experiment, not the new grade.
No general data-execution, L3 or L4 improvement is claimed. Controller regeneration
of the computed tracker is required after integrating the changed reading. Both
TestLanguagesTrackerIsGenerated and TestCmdLangTracker currently report the same
expected generated-surface drift.


## Delta-review replay correction

**Measured:** the repaired replay expands the established -only globs before
calling the ranges companion. It now completes on full Pagila at the pinned
commit, with pagila-data.sql, pagila-insert-data.sql and pg_partman-setup.sql
present but outside the measured scope. The three complete runs preserve every
count, metric, tier, grade and oracle identity from the accepted native reading;
the latest median run and source hashes replace the timing receipt. Native oracle
and statement-report JSON remain identical. The committed replay command in the
recipe works for either full checkouts or verified scoped copies.
