# PowerShellAstDump - tier-B PowerShell L1/L2 oracle (syntax only)

ADR-048 amendment D2 and D5: PowerShell has no SCIP producer, its grammar
(airbus-cert/tree-sitter-powershell v0.26.5) ships no `queries/tags.scm`, and
universal-ctags 6.2.1's PowerShell parser is deficient as an L1 oracle. The
language's own reference parser, `System.Management.Automation.Language.Parser`,
is the stronger independent oracle. This script walks its AST and writes the
neutral oracle file (`definitions` and `calls`) that `indexbench lang-oracle
-tool-oracle` reads, at tier B: a syntax dump, so callee names are as written,
never resolved (the same tier as the Roslyn and JavaParser dumps).
Bench-only: a PowerShell process run beside the harness, never a product
dependency. `ParseFile` only parses; nothing in the subject is executed.

    pwsh -NoLogo -NoProfile -File PowerShellAstDump.ps1 <clone> > out/powershell-<subject>.json
    # pester 6.2.0: 272 files, 806 definitions, 1,945 call pairs, 0 files with parse errors
    # PSScriptAnalyzer 1.25.0 (subpath Tests): 244 files, 243 definitions, 452 call pairs, 4 with parse errors

PowerShell 7.5.4 (`pwsh` from a system install; any 7.x reads the same AST).
The harness drops entries on files fathom does not read (`-L` scope, design
1.4). Kinds are mapped by `dialects/powershell/dialect.json`
`oracle_kinds.tool` (function, class, enum, enumMember, method, constructor;
`property` is emitted but outside the dialect's kinds, so excluded).

What it counts, and does not:

- Definitions: `FunctionDefinitionAst` (function, filter, workflow),
  `TypeDefinitionAst` (class, enum), `FunctionMemberAst` (method,
  constructor), `PropertyMemberAst` (property, enumMember). The parser also
  wraps every class method's body in a `FunctionDefinitionAst` whose parent
  is the `FunctionMemberAst`; that wrapper is the method, not a function
  statement, and is skipped (found by the first fixture cross-check: without
  the skip every method appeared twice).
- Calls: (enclosing function or method, callee) for every `CommandAst` with a
  literal command name (`& $dynamic` has none) and every
  `InvokeMemberExpressionAst` with a literal member name. Names are bare by
  design 1.4's `BareName` rule. Top-level calls are not pairs.
- Files: every `*.ps1 *.psm1 *.psd1` under the subject, dot-prefixed
  directories included (`Get-ChildItem -Force`; `.github/` holds real scripts),
  `.git` excluded.

## Why not universal-ctags 6.2.1 (D2, measured 2026-09-28)

Over the same files as the two subjects (`ctags --output-format=json
--fields=+nKzSe --extras=-{anonymous} --languages=PowerShell`):

| | AST dump | ctags |
|---|---|---|
| end line on any tag | yes | none (L1 would cap at good, design 1.2) |
| class methods and constructors | 29 | 0 |
| class properties | 15 | 0 |
| `class X` text inside a here-string | not a definition | tagged (11 classes vs 7) |
| `$script:`/`$global:` variables | not a definition | 6,092 tags |

Run alone on the four kinds ctags shares with the dialect (function, class,
enum, enum_member) it reads sym_recall .99 and sym_precision .99 with the
harness (L1 good, range_agreement not measured). The committed reading uses
this dump for both L1 and L2, so `method` can be a required kind and
range_agreement is measured.
