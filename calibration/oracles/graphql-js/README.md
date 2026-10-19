# graphql-js: tier-A GraphQL oracle (definitions, references, cross_references)

graphql-js is GraphQL's reference implementation (design 1.3, tier A: "the format's reference parser"). This
program reads every `.graphql` and `.gql` file of a subject with `parse` (the parser `buildSchema` itself calls)
and writes the neutral oracle file `indexbench lang-oracle -tool-oracle` reads. `buildSchema(src,
{assumeValidSDL: true})` cross-checks each file that holds type-system definitions and no operation: the type
names it registers must be the file's non-extension type definitions. Bench-only; never a product dependency.

    npm install --prefix <toolcache>/graphql --ignore-scripts graphql@16.14.2
    node calibration/oracles/graphql-js/graphql_oracle.mjs -tools <toolcache>/graphql -subject <checkout> \
        -out workspace/tmp/oracle/graphql-<subject>.json [-summary-out <file>]

What it writes (lines from graphql-js' own tokens):

- `definitions`: `type` (object, interface, union, enum, scalar, input), `extension` (named by its target), `field`
  (Type.field, including an input type's and an extension's), `enum_value` (Enum.VALUE), `directive` (no `@`), and
  `operation` and `fragment` (named ones only), each at its name's line and ending at its last token. A schema
  definition, a schema extension and an anonymous operation are no definition.
- `references`: (enclosing definition, named type) for implements lists, union members, field, argument, input
  field and variable types and type conditions, and (enclosing operation or fragment, fragment) for every spread.
  The five built-in scalars are none.
- `cross_references`: a reference whose type exactly one other file of the subject defines. A name two files define
  (independent schemas under one repository) is ambiguous: counted in the summary, not written.

First runs (2026-10-19, graphql-js 16.14.2): octokit/graphql-schema v15.26.1 (1 file, 10,439 definitions, 5,704
references, 0 cross-file), 99designs/gqlgen v0.17.95 (108 files outside testdata/ and generated/, 1,599 definitions,
547 references, 3 cross-file pairs, 430 ambiguous). `buildSchema` agrees with the parse on 91 files, cannot build 11
alone (they name a type another file defines).
