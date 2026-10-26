# PowerShell oracle candidates — 2026-10-25

**Measured:** the [committed reading](../bench/readings/lang/powershell.json)
remains L1 excellent, L2 good and L3 excellent: call precision .9909, recall
.9949 (B), and import resolution/file-edge precision/recall 1 (A). The spent
Profiler/posh-git holdout has L2 .9293/.9548 and L3 .9783/1. No new corpus
measurement, holdout admission or level promotion is claimed by this search.

## Named candidates and concrete failures

| Candidate / route | Version / revision checked | Outcome |
|---|---|---|
| PowerShell reference parser / AST dump | Measured 7.5.4; release 7.6.9 checked | Parser.ParseFile returns syntax. The existing dump emits literal CommandAst and InvokeMemberExpressionAst names; it does not bind dynamic commands or receivers. Native syntax B already supplies L1, but cannot lift L2 to excellent. |
| PowerShellEditorServices reference/definition index | 4.7.0, release source inspected | ReferenceTable visits ScriptAst with SymbolVisitor and groups by SymbolReference.Id. VisitCommand constructs `fn ` plus a written unqualified name. VisitInvokeMemberExpression constructs `mtd ` plus a string member name, without a receiver's real symbol. It is a static AST/name index, not the requested native resolving binder. No new accuracy or native-tier claim. |
| PSScriptAnalyzer | 1.25.0, existing measured subject/interface | ScriptAnalyzer's rule diagnostics are not a complete call-target occurrence index. Its AST/rule route cannot turn syntax names into runtime binding; no independent resolved-call export is established. |
| Native import binding / manifest validation | PowerShell 7.5.4, existing committed recipe | StaticParameterBinder, PowerShell's path rules and Test-ModuleManifest already supply A for the attested imports. This does not change the syntax-only call family into A. Dynamic and CWD-dependent paths remain counted, not silently resolved. |
| universal-ctags | 6.2.1, existing measured cross-check | No end lines; zero class methods/constructors, here-string false classes and variable noise. Weaker than the existing native AST dump; cannot supply the missing semantic call truth. |
| Grammar-shipped queries / new grammar pin | tree-sitter-powershell 0.26.5, d3984418 | Checked release equals current pin; no tags.scm producer. Same parser family cannot independently upgrade the oracle. Parameter-default recovery failures remain product parsing gaps. |
| SCIP producer registry route | scip-code/scip d556dd3c, registry inspected 2026-10-25 | No PowerShell producer in that checked list. This is a scoped negative search, not a claim of universal absence. |

**Source inspection:** the decisive server code is release-matched:
[ReferenceTable](https://github.com/PowerShell/PowerShellEditorServices/blob/v4.7.0/src/PowerShellEditorServices/Services/Symbols/ReferenceTable.cs)
and [SymbolVisitor](https://github.com/PowerShell/PowerShellEditorServices/blob/v4.7.0/src/PowerShellEditorServices/Services/Symbols/Visitors/SymbolVisitor.cs).
The existing [AST/import recipe](../../calibration/oracles/powershell-ast/README.md)
records the executed parser, ctags comparison and import contract. The checked
[SCIP registry](https://github.com/scip-code/scip/blob/d556dd3c/README.md) supplies
only the bounded producer search above.

**Hypothesis / retry:** LB-Q-powershell-rest retains its present L2 ceiling for
these candidates. A native resolving call instrument with full-population
accounting would reopen the comparison; a runtime trace would need measured
coverage of at least 90% of the in-repo literal population, not merely successful
test execution. No such trace measurement is claimed here. The separate product
triggers remain case-insensitive command binding, UTF-16 ingestion, CWD/module
resolution and grammar recovery, each requiring a fresh holdout under K19/K29.
No shared engine, canonical corpus, reading, epoch or plan checkbox changes.
