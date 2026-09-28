# scip-typescript — tier-A oracle for TypeScript and JavaScript

ADR-048 amendment D2: universal-ctags is demonstrably deficient for
TypeScript/JavaScript (loses class-body scope, reports no end lines), so
those dialects are measured against scip-typescript (tier A) through
`indexbench lang-oracle -scip-defs` (L1) and `-scip` (L2–L4). Node is a
bench-only instrument, never a product dependency.

Install (no global install):

    npm install --prefix <tools>/scip-ts @sourcegraph/scip-typescript   # 0.4.0 on 2026-09-28, node v26.3.0

Run, per subject (clean shallow clone at the reading's SHA):

    run.sh <tools> <clone>/nest  out/typescript-nest.scip  '**/*.ts' '**/*.tsx' '**/*.mts' '**/*.cts'
    run.sh <tools> <clone>/zod   out/typescript-zod.scip   '**/*.ts' '**/*.tsx' '**/*.mts' '**/*.cts'
    run.sh <tools> <clone>/axios out/javascript-axios.scip '**/*.js' '**/*.cjs' '**/*.mjs' '**/*.jsx'
    # express has no tsconfig of its own: scip-typescript index --infer-tsconfig covers all 152 files

What the index carries (measured 2026-09-28, scip-typescript 0.4.0):

- `SymbolInformation.kind` is never set (UnspecifiedKind on every symbol).
- `documentation[0]` is a fenced signature whose first word is the
  declaration keyword: `class`, `interface`, `type`, `enum`, `function`,
  `var`/`let`/`const`, `module`, `constructor`, `get`/`set`, or a
  parenthesized role — `(method)`, `(property)`, `(parameter)`,
  `(enum member)`, `(type parameter)`.
- `enclosing_range` is set on class, interface, type alias, enum, function,
  constructor, module and most methods (nest: 2,035 of 2,359 `(method)`
  definitions), never on properties or parameters, and on few `var`s.

Outputs stay under the tools dir's `out/`, never committed.
