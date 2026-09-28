# sql — provenance

- Grammar: `DerekStride/tree-sitter-sql v0.3.11` (MIT), commit
  `7b51ecda191d36b92f5a90a8d1bc3faef1c7b8b8`, ABI 15, with a scanner; built
  into `internal/extractor/tsengine/grammars/sql.wasm.gz` (2,102,436 bytes
  raw, 380,125 bytes gzip). This is the first embedded grammar whose
  release ships no `src/parser.c` (only `grammar.js`; the tagged source
  tarball's `src/` holds `scanner.c` alone), so the lock row sets
  `"generate": true` and pins `toolchain.tree_sitter_cli: "0.26.11"` (the
  CLI version matching the pinned core's own `v0.26.11` tag — both come
  from the `tree-sitter/tree-sitter` monorepo) — `build.sh` runs
  `npx --yes tree-sitter-cli@0.26.11 generate` in scratch before compiling.
  Pin and digests in `internal/extractor/tsengine/grammars/grammars.lock.json`
  (`internal/extractor/tsengine/grammars/PROVENANCE.md` regenerates its own
  tables from it).
- `tags.scm`: written in this repository (LB-Q-sql, docs/design/language-breadth.md
  §13), a port of the incumbent regex `sqlextract.go`. DerekStride/tree-sitter-sql
  ships no `queries/tags.scm` of its own (only `highlights.scm`/`indents.scm`),
  and neither aider nor nvim-treesitter carries a `sql-tags.scm`/`locals.scm`
  to start from (checked 2026-09-28). Every shape comes from `indexbench
  lang-probe -dialect sql <fixture>` (§13 step 12), never from
  `grammar.js`/`node-types.json`.
- **Kept from the incumbent** (§11 step 3, port parity): `SQLTable`,
  `SQLView`, `SQLIndex`, `SQLFunction`, `SQLType` source kinds, unchanged
  strings (`CREATE PROCEDURE` also emits `SQLFunction`, matching the
  incumbent — see the grammar-variance note below for why it is never
  captured). **Added beyond the incumbent** (a port may add, never drop):
  `SQLColumn` (the incumbent never emitted table columns at all) and FK
  references as L2 `reference.call` edges (the incumbent's queue note asks
  for this explicitly; the incumbent had no call/reference concept).
- **Grammar variance (disclosed, LB-Q-sql's own queue note)**:
  - `CREATE PROCEDURE` has no production in this grammar at all — the whole
    statement fails to parse into any node (an `ERROR` blob; verified:
    `CREATE PROCEDURE p(...) AS $$ ... $$ LANGUAGE plpgsql;` produces no
    `keyword_procedure` token anywhere, not even as an unrecognized one).
    Unlike the incumbent regex extractor, this dialect cannot detect stored
    procedures with this grammar version. `testdata/broken.sql` exercises
    the survival (a table before, a view after, both still extract).
  - A table-level constraint named before `FOREIGN KEY`
    (`CONSTRAINT fk_x FOREIGN KEY (...) REFERENCES t(c)`) desyncs the
    parser: `CONSTRAINT fk_x FOREIGN` becomes an `ERROR` node, though the
    parser recovers and still yields a usable `constraint` node for the
    `KEY (...) REFERENCES ...` clause itself — `tags.scm`'s
    `(constraint (keyword_references) (object_reference name: (identifier)
    @name)) @reference.call` pattern still captures its FK edge.
    Bare, unnamed `FOREIGN KEY (...) REFERENCES ...` (no `CONSTRAINT`
    prefix) parses cleanly. `testdata/shapes.sql`'s `orders` table exercises
    both the inline-column and the named-constraint form on the same FK.
  - Identifier text keeps whatever quote characters the source wrote
    (`"orders"`, `` `products` ``) — unlike the incumbent's `stripQuotes`,
    this dialect does not unquote them (the query engine is structural,
    §8.1; a text transform is not one of its supported predicates,
    §3.2). `testdata/quoted.sql` documents this as-is.
  - Bracket-quoted T-SQL identifiers (`[dbo].[Album]`) parse for a plain
    `object_reference` (a `CREATE TABLE [dbo].[Album]` header), but a
    bracket-quoted name inside a `CONSTRAINT ... PRIMARY KEY CLUSTERED
    (...)` clause desyncs the parser the same way the named-FOREIGN-KEY
    case does, and `CLUSTERED` itself has no keyword in this grammar.
    Verified directly against `chinook-database`'s SqlServer variant (see
    the reading's `notes`): the recovery misfires a bogus `column_definition`
    named after the constraint, not a real column.
- `testdata/`: fixtures written in this repository. `shapes.sql` — every
  required kind, a table's columns (nesting), a multi-line callable
  signature (`calculate_total`'s two-argument `FUNCTION`), and FK edges via
  both the inline-column and the named-constraint forms (the constraint
  form is also the one syntax error the parser must survive).
  `quoted.sql` — schema-qualified/quoted names, `OR REPLACE`,
  `MATERIALIZED`, `TEMPORARY` and `IF NOT EXISTS` variants (the incumbent's
  own coverage width). `broken.sql` — `CREATE PROCEDURE`'s full-statement
  parse failure, isolated, with a survivor table before and view after.
  Goldens hand-written from `indexbench lang-probe`'s capture and tree
  output, reviewed against each fixture before this dialect's own test
  (`TestSQLDialectGoldens`) first went green.
- **Measurement** (`docs/bench/readings/lang/sql.json`,
  `calibration/corpus.d/sql.yaml`): universal-ctags 6.2.1 (tier B) on
  `lerocha/chinook-database` v1.4.5 (11 `.sql` files, six vendor-DDL
  variants of the same 11-table schema) and `sqitchers/sqitch` v1.6.1
  (66 `.sql` fixtures across `t/sql`, `t/pg` and other per-engine trees).
  `level_met` is `none`: the merged `sym_recall` .2431 and `sym_precision`
  .6649 are dominated by two independent tooling gaps outside this
  dialect's queries, detailed in the reading's own `notes`/`disposition` —
  DerekStride/tree-sitter-sql's T-SQL/Oracle/Db2 gaps above, and
  universal-ctags 6.2.1 separately mistagging MySQL backtick-quoted DDL
  almost entirely (one bogus tag for a whole file, verified directly).
  Isolated to chinook-database's Sqlite-flavoured file alone (ANSI/SQLite
  DDL only), this same dialect measures `sym_recall` 1.0, `sym_precision`
  1.0, L1 good. `docs/plans/language-breadth.md`'s `LB-Q-sql-rest` records
  the same disposition, with a trigger naming both blockers.
- **Merge gate (§11 step 2)**: the target (L2) is not met (L1 itself is
  `none`), so the incumbent (`sqlextract.go`) stays fusion's live `.sql`
  dispatch (`internal/fusion/pipeline.go`) — this dialect is written,
  tested and measured but not yet switched in.
