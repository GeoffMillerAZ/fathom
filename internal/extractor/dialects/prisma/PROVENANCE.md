# prisma -- provenance

- Grammar: `victorhqc/tree-sitter-prisma` at commit `3556b2c1f20ec9ac91e92d32c43d9d2a0ca3cc49` (the repository's head;
  it publishes no tag, so the lock's tag is the commit) (MIT; the top-level `LICENSE`, Victor Quiroz 2022, checked by
  `indexbench lang-pin -tree`; no licence file under `queries/` or `src/`), ABI 15, no external scanner, `src/parser.c`
  shipped (no generate step), grammar at the repository root (`tree_sitter_prisma`), built into
  `internal/extractor/tsengine/grammars/prisma.wasm.gz` (158,815 bytes raw, 61,507 bytes gzip). Pin and digests in
  `internal/extractor/tsengine/grammars/grammars.lock.json`. The catalogue (`lang-pin -id prisma`) names
  LumaKernel/tree-sitter-prisma; the queue names victorhqc's, which is the one pinned.
- `tags.scm`: written in this repository (the grammar ships `queries/highlights.scm` only). Kinds: `PrismaModel`,
  `PrismaView`, `PrismaType` (a composite type), `PrismaEnum`, `PrismaDatasource`, `PrismaGenerator`, `PrismaField`
  (`Model.field`) and `PrismaEnumValue` (`Role.ADMIN`).
- References (`@reference.call`): every non-scalar type a field writes (owned by the field) and each name in an
  `@relation(fields: [...], references: [...])` list (a bare field name: the engine cannot qualify it).
- `testdata/`: fixtures written in this repository, each golden compared line by line with the fixture after the
  engine wrote it (`-update`): `schema.prisma` (datasource, generator, enum, models, a view, an `@relation`, an
  `Unsupported()` column), `composite.prisma` (a mongodb schema with composite types), `models.prisma` (models naming
  each other and an enum, no datasource: one file of a schema folder) and `broken.prisma` (an unclosed list type: the
  blocks on both sides still extract; the parser drops the field after it). Prisma 6 rejects a field attribute that
  spans lines, so no fixture has a multi-line field; the grammar accepts one.
