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

## Imports (wave lf6, LB-Q-powershell L3): PowerShellImportDump.ps1, tier A

`PowerShellImportDump.ps1` writes the neutral `imports` (LB-F40, LB-F57) from PowerShell's own
parser and its own path rules, tier A for what they attest:

    pwsh -NoLogo -NoProfile -File PowerShellImportDump.ps1 <clone> > out/powershell-<subject>-imports.json

- Statements: a `CommandAst` whose `InvocationOperator` is `Dot` (dot-sourcing; `&` imports nothing),
  `Import-Module`/`ipmo` with the path `StaticParameterBinder` binds to `-Name`, a
  `UsingStatementAst` of kind `Module`, and a manifest's RootModule (ModuleToProcess) and
  NestedModules string literals.
- Bases, each from PowerShell's semantics: a path opening with `$PSScriptRoot` (an expandable
  string whose only expansion it is) names the file's own directory (about_Automatic_Variables);
  `using module` resolves a relative path against the script holding it (about_Using); a
  manifest's modules resolve against the module base, its own directory, and Test-ModuleManifest
  validates the manifest (a manifest it refuses, for a RequiredModules entry the machine lacks, is
  counted `manifest_data_only` and read by the same rule). A dot-sourced or Import-Module path
  with no `$PSScriptRoot` is relative to the current location at run time, which no static reading
  knows: counted `cwd_relative`, in no truth. A bare module name is PSModulePath's (`by_name`);
  Join-Path, another variable or a hashtable is `dynamic`. Each is counted and disclosed, never
  resolved.
- With no extension, PowerShell's order is tried: a directory's `<leaf>.psd1`, then `<leaf>.psm1`;
  a name plus `.psd1`, `.psm1`, `.ps1`, `.dll`. A target outside the subject or naming no file is
  counted (`outside_or_missing`).

`PowerShellAstDump.ps1 -ModuleCaller '<module>'` pairs a call no function or method encloses with
the dialect's module symbol (K18; the powershell manifest mints one since wave lf6), so its L2
pairs match the dialect's attribution of top-level code; without the flag it writes what it
always did.

Measured 2026-10-10 on the tuning subjects (PowerShell 7.5.4): pester/Pester 6.2.0, 59 dot-sourcings,
146 Import-Module, 2 manifest entries, 86 in the subject, 27 naming no file of it (26 statements
name a path under `bin/`, the build's output, absent from the checkout), 2 cwd-relative, 5 by name,
87 dynamic; PSScriptAnalyzer 1.25.0 `Tests/`, 3 dot-sourcings, 36 Import-Module, 10 manifest
entries, 6 in the subject, 14 naming a file outside `Tests/` or none, 4 by name, 25 dynamic.
