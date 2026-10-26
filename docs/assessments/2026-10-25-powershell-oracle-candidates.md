# PowerShell oracle candidates — 2026-10-25, measured 2026-10-26

The [canonical reading](../bench/readings/lang/powershell.json) remains L1
excellent, L2 good and L3 excellent: call precision .9909 / recall .9949 at B,
imports/file edges 1 at A. The existing independent holdout remains separate
from the canonical corpus. Native command discovery and executed-function
tracing are measured below.
No new grade, canonical reading or holdout admission is claimed.

**OL-R11:** pinned native Get-Command / InvokeCommand.GetCommand is A only in
a clean subject-declared load state, recorded in the receipt; undeclared state
remains unresolvable. Native runtime tracing is A only on its executed population,
with coverage disclosed. P-T1 forbids grading whole-written-population recall
from runtime coverage, including a 90% coverage threshold.

## Clean native binder measurement

The [committed recipe](../../calibration/oracles/powershell-native/README.md)
runs PowerShell **7.6.9**, with profiles and module autoload disabled. It loads
the matching subject manifest and the subjects' own declared test harness.
Autoload is disabled before the preamble. The chosen pinned-runtime baseline is
explicitly imported and asserted: Archive, Host, Management, Security, ThreadJob
and Utility; package-management and interactive-editing modules are not part of
that named baseline. Its membership no longer depends on preamble side effects.
The native parser inventories local Import-Module paths, PSScriptRoot-relative
dot-sources and top-level New-Module bodies across every original tracked test
file and the root loader. Literal path assignments, Join-Path and Split-Path
-Parent are interpreted structurally; source expressions are never evaluated to
invent paths. Dynamic expressions do not establish declared local load state.
Each enumerated target is loaded with its hash or retained in the named
`declared_not_loaded` list with a stable declaration id, reason and site count.
The proof compares that inventory with the hashed loaded inputs plus refusals;
there is no hand-maintained helper allowlist.

This includes Pester's original Format and TypeClass sources and PSScriptAnalyzer's
build module, alongside both same-named test-helper modules. PSScriptAnalyzer's
external Pester dependency remains pinned at 5.7.1. Original module blocks are
compiled from their native ScriptBlockAst without running surrounding tests.
Pester has 30 distinct declared targets / 115 declarations: 24 targets load and
six are refused. PSScriptAnalyzer has seven / 33: three load and four are refused.
Public receipts store only aggregate counts, hashes and declaration-id refusals;
private inventories retain paths.

The binder uses a **session-wide union** of declared per-file helpers, including
private product utility preambles. Native module-loader sources use that
module's private session; ordinary files use global lookup with the helper
union. This does not model isolated per-file loading. The union answers **13
Pester sites outside their private helper's declaring files**: the 12 identified
in review, plus one Format-Number site after Format.ps1 is loaded. Ten of the
reviewed sites call utility functions; two sit inside `& $pesterModule { ... }`.
The native diagnostic class now recognises that module-variable ancestor as
private scope when an answer is absent. No blanket claim that private helpers
cannot answer unrelated test files is made.

Executable test bodies are not dot-sourced. Refusals comprise one executable
Pester coverage fixture (three unresolved references) and five undeclared CWD or
PSDrive paths; PSScriptAnalyzer has three executable test-file dot-sources (one
unresolved reference) and the absent DSC fixture module (seven module-qualified
references). The pinned Git tree contains no PSDSCxMachine.psm1, so its nine
imports cannot be loaded; these calls stay in a separate unresolved class.
Refusal attribution is scoped to declaring/source files, avoiding unrelated
same-name functions. External applications, undeclared state and literal member
invocations remain unresolved and count as misses.

Actual targets come from native CommandInfo.Name / AliasInfo.ResolvedCommand.
The native AST supplies only original source scope and caller declarations;
there are no Fathom pairs filtered by whether native discovery answered.
Native definition origins are recorded as counts and content hashes, never a
subject file list. The [aggregate receipt](data/powershell-oracle-2026-10-26/measurements.json)
holds full per-subject counts, module load state and recipe/product identities.
Runtime cmdlet admission includes modules whose base is exactly PSHOME: the
nested Management, Utility and Security DLLs live at that root. Source-less
initial runtime functions are admitted only while their native ScriptBlock
identity matches the clean initial session. Their code-content hashes are
separate from file-origin hashes. Collector helpers are defined afterwards and
cannot qualify. A native answer rejected by admission is classified separately
from a command absent from the declared session.

| Original subject / pin | Full original PowerShell scope | Native resolved sites | Unresolved sites retained | Resolved-pair diagnostic P / R |
|---|---:|---:|---:|---:|
| Pester 6.2.0 `98e56d65da16` | 272 files / 2,713,762 bytes | 15,716/18,842 (83.41%) | 3,126 (16.59%) | 2,722/4,217 (.6455) / 2,722/2,749 (.9902) |
| PSScriptAnalyzer 1.25.0 `f05704df81b2`, `Tests` | 241 files / 865,136 bytes | 5,258/5,838 (90.07%) | 580 (9.93%) | 1,133/1,435 (.7895) / 1,133/1,168 (.9700) |

These pair diagnostics compare with the resolved native subset, not complete
source truth. Predictions outside that subset may be unattested, not false
calls. Resolved-only recall cannot hide the full unresolved-site counts above.
Parser-derived helper loading resolves 40 / 12 additional sites over the
previous session, with zero lost or retargeted answers and an unchanged literal
population. These are measurement repairs, not producer improvements.
PSScriptAnalyzer is **three sites below** its maximum integer unresolved count
of 583. Pester remains above 10%; its 2,175 member sites alone exceed that share
of 18,842. The each-subject condition therefore remains unsatisfied, and no
excellent whole-population L2 grade is established.

Pester has 16,667 command sites and 2,175 member sites. Native loader metadata
identifies 106 original source files belonging to its module; 1,486 command
queries use module scope and 15,181 use global scope. Resolved definition origins
are 13,266 site answers in original source and 2,450 in the pinned runtime, with
117 distinct file-origin/content-hash entries. PSScriptAnalyzer has 5,551 commands /
287 members, all global queries; 4,112 answers come from its matching released
module plus declared test dependency, 86 from original source and 1,060 from
the runtime, with nine distinct file-origin/content-hash entries and one
source-less runtime definition-code hash. The harness supplies 12,422 / 3,301
resolved sites, counting the declared subject-module re-import in the union. No admitted harness-export site remains unresolved; the refused declarations remain explicit misses.

Unresolved classes partition all original unknown sites; these are diagnostic
source-context classes, not fabricated native targets. Private-scope means the
site is inside an InModuleScope/InPesterModuleScope or module-variable invocation AST ancestor. Same-file means
a definition exists in that file, including BeforeAll-defined functions; it does
not claim that the definition has executed or is visible in the clean session.
Found-but-not-admitted records a native command rejected by the provenance
filter; it is zero in both subjects, with a positive rejection fixture. Missing
is the remainder, including unavailable platform commands, dynamic
fixture state and definitions outside that chosen session. No test fixture is
executed merely to make its static query succeed.

| Unresolved class | Pester | PSScriptAnalyzer |
|---|---:|---:|
| Literal member | 2,175 | 287 |
| Declared harness export | 0 | 0 |
| Same-file / BeforeAll definition | 386 | 194 |
| Private module scope | 470 | 0 |
| Declared but not loaded | 3 | 8 |
| External application (not admitted as a hit) | 16 | 5 |
| Found but not admitted | 0 | 0 |
| Missing in declared session | 76 | 86 |
| Total | 3,126 | 580 |

The native population retains **17 original UTF-16/NUL-containing files** in
PSScriptAnalyzer. Product discovery excludes them as binary: 224 files / 775,189
bytes, exactly 17 files / 89,947 bytes fewer. The prior AST recipe's 244-file /
15-UTF-16 description does not describe this fresh tracked set. No original
file is discarded here. Four files have native recovered parse errors; their
sites remain counted, without claiming complete syntax coverage of malformed
input. Native parse errors and tree-sitter ERROR/MISSING rates are different
metrics. The canonical parser was 7.5.4; current 7.6.9 written-name unique pairs
are 4,218 / 1,489 versus historical 4,214 / 1,486. No equal-metric improvement
is inferred from that version difference.

## Native executed-function measurement

Command-breakpoint actions expose actual FunctionInfo through native call-stack
InvocationInfo: resolved target, alias spelling, caller and call-site file/line/
column. `$PSDebugContext` is null in this action and is not the answer source.
The decoder checks exact source position and native caller; it never copies a
parser target or substitutes a parser caller. Cmdlets, members, ambiguous
callbacks and child-process calls remain unattested in this hook.

| Original subject / declared drivers | Attested full original sites | Executed native-pair recall | Driver result |
|---|---:|---:|---|
| Pester default plus its declared remaining phase | 13,332/18,842 (70.76%) | 1,922/2,134 (.9007) | default P tests: 629 passed / 3 failed / 0 skipped, stop before remaining tests; same 3 ANSI failures untraced; remaining phase 2,928 passed / 0 failed / 3 skipped |
| PSScriptAnalyzer's four declared test directories | 3,180/5,838 (54.47%) | 507/511 (.9922) | 1,944 passed / 0 failed / 162 skipped; exit 0 |

Pester's default `test.ps1 -NoBuild` observes 6,117 sites; its own
`-SkipPTests` option observes 7,606. Their native-event union counts overlapping
sites once and retains all 18,842 original sites, not just the executed phase.
PSScriptAnalyzer uses its explicitly declared **Pester 5.7.1**, not Pester 6.2;
the latter's removed `-Pending` parameter caused two containers to fail in an
earlier diagnostic run. Its full pinned documentation and compatibility profiles
are restored for runtime tests, outside the unchanged graded `Tests` subtree.
Those repaired-environment runs supply the published row. The fresh phase receipts record
process exit, native-observer status, test counts, collector/decoder hashes and
phase outcomes separately. Pester's union says a declared phase stopped after
failures, even though both observer processes returned 0. PSScriptAnalyzer's
phase passed with exit 0. The P-test count sums 23 original driver summaries;
the remaining phase and PSScriptAnalyzer use their final test-driver summaries.

An owner engine bug was isolated before publication: script-qualified command
breakpoints in 7.6.9 dereference a null InvocationInfo.ScriptName during
New-Module. The same original assertion test passes 5/5 without breakpoints,
fails two with a no-op script-qualified breakpoint, and passes 5/5 with a global
no-op breakpoint. The final collector uses a global breakpoint and filters by
native source position plus frozen original membership. It retains 2/2 Pester
and 1/1 PSScriptAnalyzer breakpoints, completes without an instrumentation
exception, and preserves both tracked-source digests. The proof fixture includes
New-Module and kills a regression to script-qualified breakpoints. Earlier
crashed runs are not evidence for a ceiling.

All runtime pairs are observed actual function calls; precision is unmeasured
because predictions include unexecuted calls. The remaining 5,510 / 2,658 sites
stay in coverage counts and never enter executed-only recall. These are measured
driver/instrument limits, not a claim that no broader trace can observe more.
No runtime coverage percentage triggers a whole-written-population L2 grade.

### Why .9007 differs from canonical .9949

The canonical B oracle compares written syntax pairs over its whole population.
The A runtime oracle compares actual engine targets only for observed executed
functions. These are different target semantics and populations, so the two
recalls are not interchangeable. Re-decoding the accepted runtime phases against
the corrected binder population reproduces 1,922/2,134 and 507/511; no native
pair is removed to improve either ratio. These runtime events were collected
with the phase-specific collector/decoder hashes recorded in the receipt; the
binder correction reuses those unchanged events and scores them with the merged
product. It does not claim another execution of the original test drivers.

| Cause of an executed pair miss | Pester | PSScriptAnalyzer |
|---|---:|---:|
| Generated mock target, with written pair present in product | 201 | 0 |
| Other engine target differs from captured written invocation | 2 | 0 |
| Case-only caller/target spelling | 3 | 4 |
| Written pair absent from product output | 6 | 0 |
| Total missed pairs | 212 | 4 |

The subject's own mock implementation generates a PesterMock-prefixed function
name with a new GUID and aliases the written command to it. The 201-row class
requires that native target shape **and** actual product capture of the written
caller/invocation pair. This is runtime rebinding, not 201 missing syntax calls.
The two other rebindings are an ordinary alias and an external-script mock
redirected to a generated function. GUID-bearing target identities and truth
hashes vary across runs; pair counts and the cause classification are the
reported comparison. All actual targets remain in the executed score.

The six absent written pairs occur in six files with product ERROR/MISSING
nodes (20, 12, 54, 2, 4 and 5 in the actual bounded lang-probe checks). Five
are missing wrapper/invocation captures; the sixth is a call after an inline
param(...) block, already covered by the existing recovery trigger. That sixth
pair is also redirected to a generated mock, but is counted here because the
written pair is absent. Native pairing does not explain these away.

Controller product routing: the existing case-insensitive binding trigger owns
the seven case-only misses. Broaden the existing grammar-recovery trigger to
include the five lost wrapper/invocation captures (including a switch-value
argument followed by a script block); keep the known inline-param case there.
Separately, controller-serialized call-resolution work must decide whether
executed alias/mock/script rebinding is a product claim before adding that
resolution. The 203 otherwise-captured rebindings are explicit evidence for
that decision, not a whole-written L2 syntax defect or an automatic ceiling.
No shared engine change or new shared queue id is made on this lane.

The decoder also rejects native rows before pairing: Pester 596 = 38 without
a unique original site + 5 invocation-spelling disagreements + 553 caller
mismatches; PSScriptAnalyzer 40 = 1 + 0 + 39. These remain unattested and are
not the 212 / 4 accepted-pair misses. The aggregate receipt preserves both
breakdowns and all per-phase test outcomes.


## Named candidates and concrete limits

| Candidate / route | Version / revision checked | Outcome |
|---|---|---|
| Native command discovery | PowerShell 7.6.9, actual full original populations | A under OL-R11 in the recorded declared sessions; measured unresolved counts above prevent a full-population promotion. Module privacy and disabled autoload are enforced, not assumed. |
| Native function breakpoint tracing | Same released runtime; both original test drivers | A on actual observed executed functions only, with original-site coverage and unexecuted misses disclosed. Cannot grade whole-written L2 recall or precision under P-T1. |
| Trace-Command CommandDiscovery / MemberResolution | PowerShell 7.6.9, actual generic contract probe | Owner providers emit lookup/selection/type-member diagnostics. The probe confirms both providers and actual events, but no structured caller-file/line/column reference table is exported. No complete corpus pairing or unexecuted recall is claimed; these may extend an executed-only adapter, not the whole-source grading rule. |
| Set-PSDebug -Trace 2 | PowerShell 7.6.9, executed generic fixture | Emits source execution and variable diagnostics, not independently resolved targets. It cannot independently grade unexecuted references. Raw diagnostics stay private. |
| Native .NET static member reflection | PowerShell 7.6.9 / its bundled .NET, actual Math.Abs fixture | Exposes static method/overload names for a known receiver type. The fixture does not establish complete source-reference binding, arbitrary instance receivers or undeclared fixture state. No whole-population reflection measurement or inability-to-close-the-gap claim is inferred from this contract probe. |
| Native AstTypeInference / InferTypeOf | PowerShell 7.6.9, released engine source inspected | Internal API returns possible PSTypeName result types, with runtime evaluation disabled by default. InvokeMember handling infers result types through receiver-member inspection; it does not export an actual bound-reference/caller/target table. A custom type/name join would need independent validation rather than inheriting A from type hints. Source-contract check only; not corpus-executed or scored. |
| PowerShell reference parser / AST dump | Existing 7.5.4; current 7.6.9 executed | Native syntax B supplies declarations and literal CommandAst/InvokeMemberExpressionAst scope; syntax alone does not lift L2. Current binder targets are independently obtained from the engine. |
| PowerShellEditorServices reference/definition index | 4.7.0, release source inspected | ReferenceTable groups SymbolVisitor's written `fn ` and `mtd ` names; it has no receiver-resolving reference in these visitors. A custom name-index export is syntax evidence, not a native resolving answer. |
| PSScriptAnalyzer | 1.25.0, actual module/subject and source contracts | Native rule diagnostics/AST access do not establish a complete resolved-call index. Its matching declared module supplies the actual engine session above; no diagnostic-to-reference promotion. |
| Microsoft CodeQL PowerShell extractor | 0.0.1 at `50cc6fcfb330`; source MIT; CLI release 2.27.3 contract checked | Control/data-flow/type-tracking analysis is an independent C route under the binding review, not an admitted owner/maintained-SCIP oracle. Requires a source-built extractor plus separately licensed CodeQL CLI; CLI terms are distinct from source MIT. Not installed/run; no licence prohibition or accuracy result is invented. |
| System.Management.Automation / PowerShell SDK | NuGet 7.6.9, release present | This is the same native parser/binder implementation already measured, not a second independent oracle. Parser-only exports remain B. |
| AlterNET PowerShell parser and Embedded variant | Both 11.0.1, NuGet metadata/licence page checked | Editor/LSP wrappers, with PSES listed as the PowerShell language-server component under its own licence. They do not establish a distinct owner or maintained-SCIP producer. Custom semantic exports are independent C; not installed or accuracy-tested. |
| ShellSyntaxTree | 0.4.0, NuGet metadata | Independent Bash/PowerShell structured AST, C; no native evaluated target population established. Native syntax B already exists. Not corpus-executed. |
| Meziantou.Framework.Language.Shell | 3.0.5, NuGet metadata | Independent cross-shell concrete syntax tree/diagnostics/editing, C. No owner binding or maintained SCIP status; not corpus-executed. |
| PsBash.Core / PsBash.Transpiler | Both 0.14.0, NuGet descriptions | Bash input AST and Bash-to-PowerShell emission, outside this PowerShell-input call-reference target. Translated-input output would not create native A truth. |
| tree-sitter-powershell package / grammar | PyPI 0.26.4 MIT; grammar release 0.26.5 `d3984418` equals product pin | Independent parser queries are C; grammar has no tags.scm producer. Same parser family cannot independently promote the oracle, and the released pin has no parameter-default recovery fix. |
| Native import binding / manifest validation | Existing 7.5.4 recipe | StaticParameterBinder, native path rules and Test-ModuleManifest already supply A for attested imports; this does not promote call syntax. Dynamic/CWD-dependent paths remain disclosed. |
| universal-ctags | 6.2.1, existing measured cross-check | No end lines, zero class methods/constructors, here-string false classes and variable noise; weaker than the native AST dump and no resolving call truth. |
| SCIP/LSIF producer searches | Full tables and repository query checked 2026-10-26 | No PowerShell producer in the checked tables; eight query hits are unrelated script collections. The ten-example scip README is explicitly non-exhaustive, not a registry or universal absence proof. |

## Search receipt (X-R4)

| Required category | Source / query actually checked | Result / limit |
|---|---|---|
| Own compiler/runtime | Released 7.6.9 native Parser, InvokeCommand.GetCommand, Get-PSCallStack, Debugger/CommandBreakpoint source, Get-TraceSource, Trace-Command, Set-PSDebug, .NET reflection and internal AstTypeInference source | Static full populations and runtime drivers measured; sibling trace/reflection contracts executed; type-inference source inspected, limitations above. |
| SCIP and LSIF | [Sourcegraph supported table](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), [writing-indexer table](https://sourcegraph.com/docs/code-navigation/precise-code-navigation/writing_an_indexer), [full LSIF.dev table](https://lsif.dev/), GitHub repository search `scip-powershell` | No producer in checked tables; eight unrelated query hits. [scip README](https://github.com/scip-code/scip/blob/d556dd3c/README.md) is a non-exhaustive example list. |
| Language servers | PSES [ReferenceTable](https://github.com/PowerShell/PowerShellEditorServices/blob/v4.7.0/src/PowerShellEditorServices/Services/Symbols/ReferenceTable.cs) and [SymbolVisitor](https://github.com/PowerShell/PowerShellEditorServices/blob/v4.7.0/src/PowerShellEditorServices/Services/Symbols/Visitors/SymbolVisitor.cs); released editor-wrapper metadata | Actual visitor grouping is written-name indexing; wrapper does not establish a new native resolved index. |
| Package registry | [NuGet query](https://azuresearch-usnc.nuget.org/query?q=powershell%20parser&prerelease=false&take=100), [native SDK versions](https://api.nuget.org/v3-flatcontainer/system.management.automation/index.json), [PyPI parser](https://pypi.org/pypi/tree-sitter-powershell/json), matching PowerShell Gallery module packages | NuGet returns 24 hits; relevant parser/editor candidates are named above. Other hits concern Tcl, MOF or CLI parsing, not PowerShell source references. Pins, output/tier limits and declared Pester dependency checked. |
| Runtime call/require tracing | Own declared test tasks plus native breakpoint/trace/debug/reflection fixture contracts | Measured actual executed function population. Original failures, repaired dependency/data environment and instrumentation bug are disclosed; no missing-runtime excuse or coverage-to-grade promotion. |

Primary source identities: [native engine release](https://github.com/PowerShell/PowerShell/releases/tag/v7.6.9),
[native type-inference implementation](https://github.com/PowerShell/PowerShell/blob/v7.6.9/src/System.Management.Automation/engine/parser/TypeInferenceVisitor.cs),
[CodeQL extractor tree](https://github.com/microsoft/codeql/tree/50cc6fcfb330d8d637bc6307df9b50520e41d7fe/powershell),
[CLI terms](https://github.com/github/codeql-cli-binaries/blob/main/LICENSE.md),
[PSScriptAnalyzer's declared resource versions](https://github.com/PowerShell/PSScriptAnalyzer/blob/f05704df81b2aca17dc027ee39b3fce106d418fc/tools/installPSResources.ps1),
[editor-wrapper licence](https://www.nuget.org/packages/Alternet.Studio.Syntax.Parsers.PowerShell/11.0.1/license).
Searches are bounded and dated; none claims universal absence of a future producer.

## Scope, independence and failure proofs

Exact before/after tracked manifests agree. Pester: 409 regular tracked files /
5,724,617 bytes, digest `c0ae6755def5de48b890d049d121fdab0ba0b6807426161cf4660eb3eb16180e`.
PSScriptAnalyzer `Tests`: 250 / 878,391, digest
`418da8434761da383351ef6a2f04034a1ae44707bc8b6fd861f32638e473c8af`.
Non-PowerShell files reconcile the larger manifest totals. Both scopes have zero
symlinks/gitlinks; no submodule contents are claimed. Runtime-only data outside
`Tests` is not silently added to the graded scope.

Pester's generated non-inline wrapper is declared load-state input, not original
source membership. Before product scoring, its three generated PowerShell files /
20,353 bytes are removed, preserving all tracked originals. Fresh scoring then
matches the exact canonical product counts/bytes, 272 / 2,713,762 and 224 /
775,189. The [scope-cleanup receipt](data/powershell-oracle-2026-10-26/score-scope.json)
records only counts, bytes and the recipe-owned build root. No subject exclusion,
holdout substitution or canonical corpus change is used to fix discovery.

Independence: real `lang-probe` output is placed inside both the subject and native
output directories and replaced with fabricated pairs. Native binder/runtime
truth digests remain equal. Changing the known native alias from NativeOne to
NativeTwo changes exactly two expected caller/target pairs in each route.
The [inventory proof](data/powershell-oracle-2026-10-26/inventory-proof.json) exercises discovery, a dropped-helper regression, per-file refusal attribution and helper caps. The [proof](data/powershell-oracle-2026-10-26/proof.json) also verifies private
module visibility, disabled undeclared autoload and unresolved external/member
calls. The runtime source fixture's New-Module remains valid with the fixed hook.

Mutation: `python3.13 -B calibration/oracles/powershell-native/prove_mutations.py --tools workspace/tools/powershell-native --indexbench workspace/tools/indexbench --out workspace/artifacts/lang-powershell/mutations-final`
kills twenty-one mutations: parser caller substitution, written-alias substitution,
callback-guard removal, raw-byte-cap bypass, dropping the second declared phase,
A-to-C tier, undeclared autoload, private-module scope leakage, static-site-cap
bypass, script-qualified breakpoints, omitted declared harness, changed explicit
runtime baseline, stopped phase mislabelled success, unsupported mock attribution,
rejected runtime-root cmdlets, rejected initial-runtime functions and mislabelled
found-but-rejected commands, dropping a parser-enumerated helper, missing module-variable privacy and leaking refusal attribution across files and fabricating a code hash for an absent target. Each reaches a focused `FAIL:`; source
is restored in `finally` and GREEN rerun. [Mutation summaries](data/powershell-oracle-2026-10-26/mutations.json)
record each assertion. The pure decoder/phase-combination suite passes 26 tests.

Caps: native file 3/2, source bytes N/(N-1), static sites N/(N-1) and unique native
rows 2/1 all refuse the affected unit with no accepted partial oracle. Pure tests
exercise raw bytes N/(N-1), rows 2/1, sites 2/1, combined bytes N/(N-1) and combined
rows 2/1, plus truncated JSON, wrong runtime and native refusal. Production limits
are 2,048 original files, 64 MiB source, 1,000,000 sites/rows, 128 MiB raw bytes and
exactly two phases per native union, with the same total-byte/row limits. No
throughput claim is made. Runnable commands/prerequisites are in the recipe;
raw logs, source, file lists and identity-bearing metadata remain private.

## Disposition / retry

Retain canonical grades. The named owner binder exists and is measured, but its
clean declared state leaves original references unresolved; native function
tracing is measured executed-only. Retry L2 when a released owner/maintained-SCIP
producer independently answers enough full-source targets to leave at most 10% of
original literal sites unresolved **on each subject**, with the recorded
declared harness loaded and all unresolved sites retained as misses; then
re-measure full-population metrics before any grade change. Also retry when a
controller-serialized product repair is regraded against adequate native truth.
A runtime coverage threshold is not a retry-to-excellent condition.
Existing case-insensitive binding, UTF-16 ingestion, CWD/module resolution and
grammar-recovery product triggers (including the newly classified lost wrappers/
invocations above) remain separate and require fresh permitted
measurements. No engine/epoch change, generated file, canonical reading, corpus
shard or plan checkbox changes here.
