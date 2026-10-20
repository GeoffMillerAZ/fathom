# thrift -- provenance

- Grammar: `duskmoon314/tree-sitter-thrift` tag `v0.5.0`, commit `82cd6a45626ac2dd4c8c1705170aa58b81212782` (MIT; the
  top-level `LICENSE`, Amaan Qureshi and Campbell He 2023, checked by `indexbench lang-pin -tree`; no licence file under
  `queries/` or `src/`), ABI 14, no external scanner, `src/parser.c` shipped (no generate step), grammar at the repository
  root (`tree_sitter_thrift`), built into `internal/extractor/tsengine/grammars/thrift.wasm.gz` (231,129 bytes raw, 78,324
  bytes gzip). Pin and digests in `internal/extractor/tsengine/grammars/grammars.lock.json`. The catalogue
  (`lang-pin -id thrift`) names `tree-sitter-grammars/tree-sitter-thrift` (the same grammar's organisation fork, ABI 14,
  head `68fd0d8`); the queue names duskmoon314's, whose release tag is the one pinned.
- `tags.scm`: written in this repository (the grammar ships highlights, folds, indents, injections and locals queries
  only; neither aider nor nvim-treesitter carries Thrift tags). Kinds: `ThriftNamespace` (named by its scope, `java`: this
  grammar reads a dotted namespace value as sibling nodes, so no one node spans it; the signature is not carried),
  `ThriftTypedef`, `ThriftConst`, `ThriftEnum` and `ThriftEnumValue` (`Color.RED`), `ThriftStruct`, `ThriftUnion` and
  `ThriftException` with `ThriftField` (`Order.total`), `ThriftService` and `ThriftFunction` (`Orders.get`). Callables
  (they carry a `Signature` and own references): typedef, const, enum value, field, service and function. A function's
  `Signature` runs from its start to its trailing `,` or `;` (so a multi-line signature is whole); the last function of
  a service, which has no separator, reads its first line.
- References (`@reference.call`): a type name written in a field, a parameter, a throws list, a return type, a const or a
  typedef, and the service a service `extends`. An unqualified name is the name (the file's own type in Thrift), a
  qualified one keeps its qualifier as written (`shared.Money`), so the same-file resolver never binds it to a Money of
  the file's own. A built-in type is none. `uuid` is built in but this grammar version predates it and parses it as a type
  name, so it is excluded by name (`#not-any-of?`).
- `imports.scm`: `include "x.thrift"` is a whole-file import of that path; `cpp_include` parses as the same node and is
  told apart by its text. `imports.strategy: file-symbol` with `relative: true` looks the path up beside the including
  file first, then under the repository root (the compiler's `-I .`), and a whole import brings every top-level name of its
  file into scope.
- `references.scm` (LB-F74): a qualified type (`shared.Money`, a field's, argument's, return's or throws' type, a
  typedef's, and `extends`) is read as its last identifier, which the file resolver binds among the names the file's
  includes bring into scope. An unqualified name is always the file's own type, so it is not read here. The qualifier
  (the included file's stem) is not used: no file-symbol parameter binds a prefix to a file name, so two included files that
  define one name leave it unbound (never mis-bound).
- `testdata/`: fixtures written in this repository, each golden written by hand from reading the fixture before the
  queries were final: `shared.thrift` (namespaces, typedefs with an annotation, consts, an enum with values and an
  annotation, a struct with a default, an exception), `audit.thrift` and `orders.thrift` (includes, a cpp_include, a union,
  qualified types in fields, arguments, throws and returns, a multi-line function signature, a `oneway` function, services
  extending a qualified and an unqualified service, an empty service) and `broken.thrift` (a field with no name: the
  declarations on both sides of it still extract). `internal/fusion/testdata/thriftfixture` pins the L3 contract.
