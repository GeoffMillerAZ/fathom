# SQL oracle ceiling — 2026-10-25

The present SQL recipe has a documented L2 ceiling; a free native database remains a
credible way to lift it. **Measured:** the existing [reading](../bench/readings/lang/sql.json)
reports 14 inline foreign-key pairs with precision and recall both 1 against sqlglot
27.8.0, tier C. Those are existing measurements, not a new regrade. The disposition is
[LB-Q-sql-l2-rest](../plans/language-breadth.md), under ADR-048 and design section 1.3.
No language reading, oracle tier, corpus scope, extractor or generated board changed.

## Existing work checked

- **Measured, source inspection:** [sqlglot oracle](../../calibration/oracles/sqlglot/README.md)
  supplies CREATE TABLE foreign-key pairs. ALTER TABLE has no enclosing definition in
  this contract; cataloguing every final-schema FK would silently change the question.
- **Claimed by a vendor:** PostgreSQL exposes native FK identities through
  [pg_constraint](https://www.postgresql.org/docs/18/catalog-pg-constraint.html);
  SQLite exposes [foreign_key_list](https://www.sqlite.org/pragma.html#pragma_foreign_key_list).
  These are promising native truth sources rather than new parsers to build.
- **Measured, source inspection:** the existing pglast 8.5 companion supplies ranges,
  not a resolving oracle. A syntax dump can improve tier B without making L2 excellent
  (`internal/langbench/reading.go`, `tierCeiling`).
- **Claimed by a vendor:** [PGlite](https://pglite.dev/docs/) runs PostgreSQL in a
  separate process through Node, offline after installation. Release 0.5.11 was the
  current npm release checked for this assessment. It was installed only as a workspace
  bench instrument, as ADR-048 permits; it adds no product Node dependency.

## Native feasibility check

**Measured:** the exact 28 Sqitch files named by sql.json's existing `-only` scope were
fetched at `ce0550c45ade3527d0c7f981c934e1ea4bdd6c19`. Each unmodified file was passed to
Python sqlite3's `executescript` in a fresh in-memory SQLite 3.43.2 database. Seventeen
completed and eleven failed: two upgrades need `changes`, eight engine fixtures use
PostgreSQL syntax, and one verification needs `__myapp.users`. This tests a direct
fresh-database recipe, not Sqitch's complete lifecycle with prerequisites. Partial
catalogs after errors were not graded as successful native truth.

**Measured:** each of Pagila's three unchanged scoped files, at
`9baf49c4149e43229f6021e6218d6b2ac8ef4f34`, was separately passed to PGlite 0.5.11's
`exec`, with its bundled `btree_gist` extension registered. PostgreSQL identified
itself as 18.6. `pagila-schema-jsonb.sql` completed; `pagila-schema.sql` stopped on
missing extension `vector` (0A000); `pagila-temporal-setup.sql` failed to parse at
`FOR` (42601, position 3779). Its own header requires PostgreSQL 19 and explicitly
says the project's PostgreSQL 18 compose pipeline cannot run it. The PostgreSQL 19
syntax failure occurs during parsing, before missing prerequisite tables can explain it.
The vector dependency may be solvable; it is not claimed to be an intrinsic limit.

Both final probes ran at niceness 10, singly, after the one-minute load fell below 6.
No throughput, fresh grade or impossibility claim follows from these feasibility checks.
The raw probes and downloaded subjects remain gitignored workspace artifacts.

## Next trigger

**Hypothesis:** an offline PostgreSQL 19+ runtime with the subject's vector dependency,
plus SQLite's actual registry-upgrade setup, can provide a tier-A oracle without
changing the corpus. It still needs source attribution that preserves the inline-only
L2 contract. The trigger in LB-Q-sql-l2-rest requires that complete recipe and a new
measurement; it does not authorize excluding hard files, rewriting SQL, or relabelling
sqlglot's existing output. Until then, the existing numbers and grade stand.
