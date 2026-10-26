# Native SQL inline foreign-key oracle

Bench-only, offline after installation. Original SQL is executed by SQLite and
PostgreSQL; native catalog entries supply the names, never fathom or sqlglot.
Pins: PGlite **0.5.11** (PostgreSQL **18.6**), its version-matched
`@electric-sql/pglite-pgvector` **0.0.12**, pglast **8.5** as a scanner only,
Python **3.13.5** (SQLite **3.49.1** on the measured distribution), universal-ctags
**6.2.1** for unchanged L1 truth. Node was **26.12.0**. Runtime dependencies are
measurement tools, installed under gitignored workspace or the lane cache; none
is linked, vendored or installed for the product.

## Install and replay

Use a local Python environment; every download, SDK and cache stays under workspace
or a lane-specific external cache. Set TMPDIR, GOTMPDIR, GOCACHE and GOWORK=off there
as the worker recipe requires. Check `uptime`; run only below one-minute load 6,
at niceness 10, with the machine's single measurement lock.

```sh
npm install --prefix workspace/tools/sql-runtime --ignore-scripts --no-audit --no-fund \
  @electric-sql/pglite@0.5.11 @electric-sql/pglite-pgvector@0.0.12
# In a local Python 3.13.5 environment:
python -m pip install pglast==8.5
python -m unittest discover -s calibration/oracles/sql-native -p 'test_*.py'
nice -n 10 node --test calibration/oracles/sql-native/test_pg_native.mjs
nice -n 10 go build -o workspace/tools/indexbench ./cmd/indexbench
nice -n 10 python calibration/oracles/sql-native/replay.py \
  --sqitch workspace/calibration/sqitch --pagila workspace/calibration/pagila \
  --tools workspace/tools/sql-runtime --ctags workspace/tools/ctags/bin/ctags \
  --indexbench workspace/tools/indexbench --out workspace/tmp/sql-native-replay
```

The two source roots are pinned at Sqitch
`ce0550c45ade3527d0c7f981c934e1ea4bdd6c19` and Pagila
`9baf49c4149e43229f6021e6218d6b2ac8ef4f34`, from `calibration/corpus.d/sql.yaml`.
A source copy without `.git` uses lang-oracle's explicit `-subject-sha`; verify
its file blobs against that immutable tree before measuring. The measured copies
contained every file of the existing `-only` scopes (28 Sqitch, 3 Pagila), each
verified by Git blob SHA1. No scope was narrowed. Replay writes all native receipts,
ranges companions restricted to the same expanded scope globs (even on full
checkouts), mismatches and three complete readings. It asserts equal counts,
metrics, tiers and grades, then copies the median CPU-throughput run to `sql.json`.
Use a fresh output directory. It never writes the committed reading implicitly.

## Contract and coverage

L2 pairs are `(source file, enclosing CREATE TABLE name, referenced table name)`.
The established contract excludes ALTER TABLE keys, which have no enclosing table
definition. PostgreSQL catalog deltas are taken after each successful statement;
only local keys created by CREATE TABLE count. Inherited-only partition keys are
excluded and counted. SQLite's native authorizer identifies table creation and its
`foreign_key_list` supplies targets, before a later rename or data-copy error can
lose attribution. SQLite accepts a declared FK even when its parent is not loaded:
this attests the written L2 reference, not L3 resolution or validated data.

The scanners preserve original statement bytes (including CRLF and dollar quotes).
pglast supplies PostgreSQL boundaries, not FK truth. SQLite uses its own
`complete_statement`, including compound triggers. Every source file and statement
has a digest and source line in the raw workspace report. Published summaries
contain aggregate counts/digests and failure path hashes, never subject file lists
or source text. Failed CREATE TABLE statements refuse the oracle rather than
promote a subset. Other failures are counted and identified by path digest, source line and native
error code in published summaries. Reports are separate from the strict neutral oracle JSON.

Pagila's main schema runs before its temporal file, with pgvector and btree_gist
available. Each file starts with fresh-session settings (`RESET ALL` over the same
database), matching separate psql invocations; subject SQL is not rewritten. Its
JSONB schema runs separately. Sqitch's PostgreSQL deploy/verify/revert fixtures run
in a hand-ordered deploy/verify/revert/rework scenario following each script's
`-- requires:` header; reworked deploy/revert follows in the same
scenario. SQLite registry files and upgrades run individually, statement by
statement. Missing legacy data causes named INSERT/DROP errors, but every original
CREATE TABLE executes and attests all three upgrade FK pairs.

The final native population is **17/17** known inline pairs: Sqitch 14, Pagila 3.
All scoped CREATE TABLE statements execute. Six other Sqitch statements and six
Pagila statements fail; all are retained in receipts. Four Pagila INSERTs need
out-of-scope data; two FOR PORTION OF DML statements are unsupported. That DML is
not PostgreSQL 19 evidence: the feature was reverted before 19rc1. Temporal
WITHOUT OVERLAPS / PERIOD FK DDL already works on PostgreSQL 18.

This is tier A for native evaluation of the complete inline-FK population. The
section 1.3 90% runtime-import-trace precedent is not a new SQL coverage threshold:
no FK-declaring CREATE TABLE is missing here. Successful execution of every data
statement is not asserted. No L3, resolved-data or general SQL-execution grade is
claimed.

## Publishing receipts

Keep raw oracle/report JSON in workspace. Export summaries before copying receipts
to docs; the publisher replaces all subject path/name populations with counts and
canonical JSON SHA-256 digests, preserving native failure code/kind/line with a
hashed source path. Measurement summary counts/metrics can be copied separately.

```sh
python calibration/oracles/sql-native/publish.py \
  --input workspace/tmp/sql-native-replay --out workspace/tmp/sql-native-public
```

Historical candidate/whole-file receipts use the same publisher. Published JSON
is a receipt summary, not a `-tool-oracle` input; replay produces the raw strict
inputs afresh. No fresh runtime measurement is implied by republishing a receipt.
