# graphql -- provenance

- Grammar: `bkegley/tree-sitter-graphql` at commit `5e66e961eee421786bdda8495ed1db045e06b5fe` (the repository's head,
  2024-06-07; it publishes no tag, so the lock's tag is the commit) (MIT; the top-level `LICENSE`, bkegley 2021, checked
  by `indexbench lang-pin -tree`; no licence file under `queries/` or `src/`), ABI 13 (the core's floor), no external
  scanner, `src/parser.c` shipped (no generate step), grammar at the repository root (`tree_sitter_graphql`), built into
  `internal/extractor/tsengine/grammars/graphql.wasm.gz` (178,965 bytes raw, 66,018 bytes gzip). Pin and digests in
  `internal/extractor/tsengine/grammars/grammars.lock.json`. Of the 109 `.graphql` files of the two calibration subjects,
  2 carry an ERROR node: `extend schema { query: Query }` with no directive (gqlgen's `issue896.graphql`, twice) is a
  syntax error to this grammar, though graphql-js reads it. The definitions around it still extract.
- `tags.scm`: written in this repository (the grammar ships `queries/graphql/highlights.scm` and `locals.scm` only; neither
  aider nor nvim-treesitter carries GraphQL tags). The kinds keep graphqlextract.go's two SourceKind strings, `GraphQLType`
  (every type definition: object, interface, union, enum, scalar, input) and `GraphQLField` (a field of an object, interface
  or input type, named `Type.field`), and add `GraphQLEnumValue` (`Role.ADMIN`), `GraphQLDirective` (no `@`),
  `GraphQLExtension` (`extend type Query`, named by its target so its fields are `Query.me`), `GraphQLOperation` and
  `GraphQLFragment` (named ones only). A schema definition, a schema extension and an anonymous operation have no name.
- References (`@reference.call`): every named type a definition writes (implements, union members, a field's or argument's
  or input field's type, a directive argument's type, a variable's type, a type condition) and every fragment spread,
  owned by the smallest enclosing definition; the five built-in scalars are none (`#not-any-of?`).
- Ceilings, disclosed: (1) the grammar makes a description part of its definition node, so a described type's or field's
  `Signature` starts at the description (`"""A book on a shelf.""" type Book implements Node`), and an undescribed one
  at its keyword or name; (2) a field's `Signature` is its first line, so a field whose arguments span lines reads
  `author(`; (3) graphqlextract.go's Signature (the lowercased kind, `object`; a field's type string) is not a source span
  and is not kept (`TestGraphQLDialectSignatureContract`); (4) no `imports.scm`: a GraphQL schema is the union of its
  files with no import statement, and the `#import "./a.graphql"` comment convention is a comment node whose text the
  engine's `@import.module` takes whole, so no cross-file reference is resolved and L3 is not attempted (`LB-Q-graphql-rest`).
- `testdata/`: fixtures written in this repository, each golden written by hand from reading the fixture and then
  compared with the engine's output: `schema.graphql` (every type kind, a described type, a multi-line field, a directive
  definition, an enum with a deprecated value), `ops.graphql` (a query with variables, a spread and an inline fragment, a
  fragment, a mutation, an anonymous subscription and a shorthand query), `extend.graphql` (an extension of each kind and
  a schema extension) and `broken.graphql` (an unclosed list type: the definitions on both sides still extract). The hand
  goldens differed from the engine in the calls' order (sorted, not written order), a bare enum-value name (fixed by
  making the kind callable so it qualifies) and the broken file (the parser recovers the broken type itself). The oracle
  (`calibration/oracles/graphql-js`) read the same 34 definitions of the three parseable fixtures at the same lines.
- **Measurement (docs/bench/readings/lang/graphql.json, calibration/corpus.d/graphql.yaml):** graphql-js 16.14.2 (tier A,
  the reference implementation, `calibration/oracles/graphql-js`) on octokit/graphql-schema v15.26.1 and
  99designs/gqlgen v0.17.95: `level_met` L2, L1 and L2 excellent (sym_recall and sym_precision 1 over 12,038 definitions,
  ref_recall and ref_precision 1 over 3,609 pairs). L3 is not attempted: the manifest names no import strategy.
- Dispatch: `.graphql` and `.gql` are read by this dialect and graphqlextract.go is deleted: lang-parity MERGE against it
  (docs/bench/readings/lang/parity/graphql.json): sym_recall .8725 to 1, ref_recall 0 to 1, no SourceKind lost.
