# scip-typescript — tier-A oracle for TypeScript and JavaScript

ADR-048 amendment D2: universal-ctags is demonstrably deficient for
TypeScript/JavaScript (loses class-body scope, reports no end lines), so
those dialects are measured against scip-typescript (tier A) through
`indexbench lang-oracle -scip-defs` (L1) and `-scip` (L2–L4). Node is a
bench-only instrument, never a product dependency.

Install (no global install):

    npm install --prefix <tools>/scip-ts @sourcegraph/scip-typescript   # 0.4.0 on 2026-09-28, node v26.3.0

Run, per subject, in an ORACLE checkout at the reading's SHA (a second clone, never the
checkout lang-oracle measures: node_modules there would be files the connector discovers)
with the subject's own dependencies installed, scripts off (O441, wave le, 2026-10-02):

    # nestjs/nest 9ff83d54 (package-lock.json; 2,133 packages; npm 11.19.1, node v26.10.0)
    npm ci --ignore-scripts --legacy-peer-deps --no-audit --no-fund
    # colinhacks/zod 1fb56a5c (pnpm-lock.yaml; pnpm 10.12.1)
    pnpm install --frozen-lockfile --ignore-scripts
    # axios/axios cdcfd214 (package-lock.json; 1,925 packages)
    npm ci --ignore-scripts --legacy-peer-deps --no-audit --no-fund
    # expressjs/express df0abc93 (no lockfile: versions as of the commit's own date; 489 packages)
    npm install --ignore-scripts --legacy-peer-deps --no-package-lock --no-audit --no-fund --before "$(git log -1 --format=%cI)"

    run.sh <tools> <oracle-clone>/nest    out/typescript-nest.scip    '**/*.ts' '**/*.tsx' '**/*.mts' '**/*.cts'
    run.sh <tools> <oracle-clone>/zod     out/typescript-zod.scip     '**/*.ts' '**/*.tsx' '**/*.mts' '**/*.cts'
    OWN_TSCONFIG=no run.sh <tools> <oracle-clone>/axios out/javascript-axios.scip '**/*.js' '**/*.cjs' '**/*.mjs' '**/*.jsx'
    run.sh <tools> <oracle-clone>/express out/javascript-express.scip '**/*.js' '**/*.cjs' '**/*.mjs' '**/*.jsx'

run.sh's tsconfig extends the subject's own (express has none: ES2021, CommonJS). axios's
tsconfig.json is its d.ts type test's (no allowJs, module node16, types []), not its sources':
extending it, 39 of lib/'s relative imports the oracle resolved stopped resolving, so axios runs
without it (OWN_TSCONFIG=no). Until wave
le it replaced it and installed nothing: nest's `paths` (`@nestjs/common` -> packages/common)
and zod's `zod/v4/core` self-references were lost and every vendor type (rxjs, express,
@types/node) was missing, so 3,292 in-repo imports resolved to nothing, an import binding became
a `local` symbol and a member of a vendor or cross-package receiver got no occurrence at all:
under the same call-site rules nest's call truth holds 6,501 pairs on the old index and 7,972
on this one, and 1,833 of fathom's pairs have no occurrence at any call site (410 on this one).
The 2026-09-28 indexes in out/ predate this.

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

Call sites (LB-F174): the index names a constructor `<constructor>` (referenced at the class
written in `new Foo(`), an accessor `<get>x`, an arrow-function const, a function-typed property
or a parameter by a term or parameter descriptor, and an imported-but-unresolved name or a local
by `local N`; columns are UTF-16 code units. It writes no occurrence at all at `super(...)` or at
a member of a receiver it types as any (LB-F175 leaves such calls out, counted). A definition
inside a function (a nested function declaration, a class declared in a function and its
methods, a named function expression) is a `local` with no enclosing_range; a JavaScript member
assignment (`res.status = function status() {}`) defines no symbol at all.

Outputs stay under the tools dir's `out/`, never committed.
