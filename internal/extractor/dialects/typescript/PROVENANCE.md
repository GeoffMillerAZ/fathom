# typescript — provenance

- Grammar: `tree-sitter/tree-sitter-typescript v0.23.2` (MIT), built into `internal/extractor/tsengine/grammars/typescript.wasm.gz`;
  pins and digests in `internal/extractor/tsengine/grammars/PROVENANCE.md`.
- `tags.scm`: moved unchanged from `internal/extractor/tsqueries/typescript.scm` (LB-F1). Written in this
  repository during the tree-sitter migration (ADR-021 §4); the definition shapes follow the
  grammar's own upstream `queries/tags.scm` (MIT), with documentation predicates dropped.
  Scoped in this repository (LB-Q-typescript-rest): an arrow-function or function-expression
  binding is a definition only as a direct child of the program or of an export statement, a
  method only in a class body, and no object-literal member is one; the header of `tags.scm`
  records the measured reason.
- `imports.scm`: written in this repository for SU-11 (`docs/plans/substrate.md` §B, ADR-041
  decision 5); moved unchanged from the tags query it used to share a file with. The import-equals
  pattern (`import x = require('./y')`, a whole-module binding) was added in this repository in
  wave lh; no upstream query adapted.
- `testdata/`: fixtures written in this repository; the goldens were written by the
  pre-manifest engine at fdbc2ee4 (TestTagsDialectParity). `scope.ts` added in this repository
  (LB-Q-typescript-rest) for the scope rule; since then the goldens are the full ones
  TestDialectGoldens checks, and TestTypeScriptDialectKeepsLegacySymbols holds every symbol the
  earlier goldens listed except the enumerated function-local binding.
- `references.scm` (LB-F74, LB-F75, wave lc): written in this repository; no upstream query adapted.
  The file resolver's references (type names, identifiers, member paths) and re-exports (`export * from`,
  `export { A as B } from`, default exports); behaviour per TypeScript's own module semantics.
- React kinds (LB-F377, wave c3, item react): written in this repository; no upstream query adapted. `tags.scm`
  splits a function or module-scope function binding into function, component (a capitalized name whose body
  writes `</` or `/>`) and hook (a `use[A-Z0-9]` name whose body calls a hook) by predicates over the name and
  body text, a class into class and class component (extends `Component` or `PureComponent`), reads a
  capitalized binding to memo, forwardRef, lazy, a styled template or a capitalized member as a component, a
  member of a `*Props` interface or type alias as a prop (`@member`; the type alias kind is now a container),
  and a capitalized or member JSX tag as a call (a wildcard node with a `<` token and a name field, since the
  grammar is shared with tsx). Fixtures `react.tsx` and `generics.ts` added; `view.tsx`'s golden gains the
  refined kinds, a prop and a JSX call.
