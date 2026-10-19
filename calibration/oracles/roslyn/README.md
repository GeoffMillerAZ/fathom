# RoslynDump — the C# L1 oracle (syntax only, tier B) and L2 call-site oracle (semantic model, tier A)

ADR-048 amendment D2: universal-ctags 6.2.1's C# output held sym_precision
under .90 with no single cause; C# is measured against a Roslyn syntax
dump instead. Bench-only: a .NET program run as a separate process, never a
product dependency. Nothing is compiled; `#if` branches are covered by
parsing each file twice (no symbols; every symbol its directives name) and
unioning definitions (see Program.cs header).

Build outside the tree (keeps obj/ bin/ out of the repo):

    cp RoslynDump.csproj Program.cs <tools>/roslyn-dump/ && cd <tools>/roslyn-dump && dotnet build -c Release -o bin
    dotnet bin/RoslynDump.dll <clone>/Newtonsoft.Json > out/csharp-Newtonsoft.Json.json   # 944 files, 14,298 defs, 18,623 call pairs
    dotnet bin/RoslynDump.dll <clone>/Polly           > out/csharp-Polly.json             # 801 files, 8,150 defs, 18,401 call pairs

Kinds are mapped by dialects/csharp/dialect.json `oracle_kinds.tool`
(indexer, event, field, delegate, destructor, operator and enumMember are
emitted but outside the dialect's kinds, so excluded by the §1.4 rule).
Microsoft.CodeAnalysis.CSharp 5.9.0, .NET SDK 10.0.401, 2026-09-28.

`line` is the definition's name's line, `end` the declaration's last line (O457, wave lh): a node's own first line is
its first attribute list's (`[Theory]`/`[InlineData]` stacks on Polly's tests), while SCIP, the other dumps and the
dialect date a definition by its name, and `ScoreL1` pairs within two lines. The name is the identifier, a namespace's
name, an operator's token, a conversion operator's type, an indexer's `this`, a field's first declarator. Regenerated
2026-10-02 at the pinned SHAs: Polly 8,150 to 8,143 definitions and Newtonsoft.Json 14,298 to 14,284, the difference
being a declaration counted twice before: its two parses (no symbols, every #if symbol) started it on different
lines when an attribute sat inside an `#if` (`#if !NETSTANDARD [Serializable] #endif class X`), and the union is by
(kind, name, line); calls unchanged (18,401 and 18,623).

## Wave lf1 (2026-10-03): a third parse pass, and the `calls` mode (tier A)

**Negated symbols.** A file whose directives negate a symbol (`#if !B`, `#if A && !B`) is parsed a third time with
every symbol but the negated ones: Newtonsoft.Json's `#if HAVE_DYNAMIC` around `#if !HAVE_REFLECTION_BINDER`
(DynamicUtils.cs) and `#if PORTABLE && !NETSTANDARD2_0` (ReflectionUtils.cs) declare members neither of the first two
passes reaches. Regenerated at the pinned SHAs: Polly unchanged (8,143 definitions, 18,401 call pairs), Newtonsoft.Json
14,284 to 14,368 definitions (18,623 to 18,759 call pairs).

**`calls` mode** (`SemanticCalls.cs`): the C# compiler's own semantic model as the call-site oracle beside the
scip-dotnet index, tier A (design §1.3: a native type checker), the counterpart of `../clang-calls` (LB-F116).
scip-dotnet 0.2.14 writes no occurrence for a generic method called with explicit type arguments (O271) and references
the variable, not a callable, where a delegate is invoked; the compiler binds both. The mode opens the subject's
projects with MSBuildWorkspace after a `dotnet restore`, compiles every project under every target framework it builds
for, and writes one record per call-shaped site written in a subject file: the callee's name token (1-based line,
1-based UTF-8 byte column) and a class:

| class | what the compiler binds | lang-oracle (`internal/langbench` SCIPOptions.CSharp, LB-F287) |
|---|---|---|
| resolved | an invocation to a method: ordinary, static, extension, generic with or without type arguments, local function, a delegate's explicit `.Invoke` | a call of the truth, with or without an index reference |
| object | an invocation of a delegate-typed local, parameter, field, property or event (`h(x)`), named as written | a call of the truth |
| construct | `new X(..)`, `new X<T>(..)`, `new N.X { .. }` to a constructor, named by the type's own name token | a call of the truth |
| unresolved | nothing, in every target framework (late-bound `dynamic`, an error) | a fathom pair whose every written site is one is left out of both counts, counted |

and `active_lines`, per file the lines some compiled configuration keeps (a fathom pair written only outside them is
left out of both counts, `calls_inactive`), and `query_clauses`, the LINQ query clauses the compiler binds to a method
with no invocation written (disclosed, never a site). An attribute, a constructor initializer, a target-typed `new(..)`,
array and anonymous-object creation and `nameof` are no sites. A site bound in any target framework takes that class.

`call_shapes` counts the sites by how each is written and what it binds, and `calls_in_lambda` those a lambda or
anonymous method encloses (the call is then the enclosing definition's, D5): `static` (`Type.M(..)`),
`member` (a receiver's, `x.M(..)`, `x?.M(..)`, `this.M`, `base.M`), `bare` (`M(..)`: the type's own, a base's, a
`using static`'s), `extension` (scip-dotnet names the static class's method), `generic` (written with type
arguments), `local_function`, `delegate` (a delegate-typed value invoked, `h(..)` or `h.Invoke(..)`), `event` (an
event's delegate, `E?.Invoke(..)`), `new`, `unresolved`. Disclosure only: lang-oracle writes the counts into the
reading's notes and scores by the class. On
2026-10-03, over both subjects: static 21,920, member 16,470, new 14,091, extension 13,193, generic 6,133, bare
5,413, delegate 1,050, local_function 108, event 20, unresolved 29; 8,879 inside a lambda.

The mode also writes `file_edges`, the (referencing file, declaring file) pairs of every name the compiler binds,
in any target framework, to a type, method, property, field, event or enum member declared in another subject file
(a constructed generic is its definition, an extension method called on its receiver is its declaration, an
implicitly declared constructor is its type), and `file_edges_multi_declared`, the names bound to a symbol more
than one file declares (a partial type), which draw no edge: which file such a name means has no answer, as the
import truth already holds. lang-oracle joins the edges from the documents the scip-dotnet index holds to its
file-edge truth (LB-F287): scip-dotnet 0.2.14 writes no occurrence for a generic type used as a type argument or a
generic method called with type arguments (O271), nor for code its one target framework does not compile (Polly
1,175 and Newtonsoft.Json 750 edges added on 2026-10-03).

Run (2026-10-03, dotnet 10.0.401, Microsoft.CodeAnalysis 5.9.0; a throwaway `scipall.sln` as `../scip-dotnet/run.sh`
makes, in a copy of the pinned checkout so the subject stays clean; the build copy is deleted after):

    dotnet new sln -n scipall --format sln && dotnet sln scipall.sln add $(git ls-files '*.csproj' | grep -vi cake) && dotnet restore scipall.sln
    dotnet bin/RoslynDump.dll calls <copy>/Polly <copy>/Polly/scipall.sln > out/semcalls-Polly.json
    # 53 projects (each target framework one), 0 compile errors outside Snippets (5); 800 of 801 files (cake.cs is in no project);
    # 32,803 sites: resolved 27,540, construct 4,453, object 804, unresolved 6; 2.5 min
    (Src/global.json set to 10.0.401 for the run, as ../scip-dotnet does)
    dotnet bin/RoslynDump.dll calls <copy>/Newtonsoft.Json <copy>/Newtonsoft.Json/Src/scipall.sln _SkipUpgradeNetAnalyzersNuGetWarning=true > out/semcalls-Newtonsoft.Json.json
    # 21 projects (net20 through net8.0, netstandard1.0 through 2.0), 0 compile errors; 944 files;
    # 45,624 sites: resolved 35,768, construct 9,623, object 195, unresolved 38; 219 query clauses; 36 s

then pass the file to `indexbench lang-oracle` as a second `-tool-oracle` beside the scip-dotnet index and the syntax
dump's definitions (the dump's `calls` removed, as before).

## Opt-in native project declaration evidence

`project-declarations <repository-root> <actual.sln|slnx|csproj>
<repository-relative-source-directory> [owned-intermediate-root]` loads the subject's actual MSBuild
selection and writes `fathom.roslyn.project-declarations/v1`. Existing syntax and
`calls` modes and their output schemas are unchanged. This evidence shape is
deliberately separate from the legacy neutral tool-oracle input; no scorer
consumes it and it reports no grade.

The new evidence distinguishes the identifier's span from each declarator's
definition, the full declaration statement, and trivia-inclusive full span.
Every variable in a multi-field/event statement has its own location. Native
semantic kind, namespace, containing symbol, assembly, documentation ID, and
all partial declaration locations remain separate fields. Unresolved symbols,
workspace failures and compilation errors are retained; errors exit nonzero.
Positions use one-based lines and zero-based UTF-16 columns, half-open spans.

Signatures retain both the raw native header and a candidate normalized under
the existing `tsSignature` contract: start to captured body, or only the first
line when no body exists; whitespace collapsed; at most 200 Unicode scalars
(199 plus ellipsis when truncated). The surprising first-line rule is not
silently improved. Conditional source differences stay unjudged. Native kinds
outside the product's current signature contract are emitted and flagged,
not silently removed.

The source directory selects emitted declarations. It does not prevent
MSBuildWorkspace from loading project/dependency files, run a sandbox, or
guarantee offline operation. Use only explicitly selected trusted projects and
locally provisioned dependencies. The command performs no explicit restore or
download. Active project defines are recorded; this is not exhaustive mixed
preprocessor coverage, whole-repository completeness, player execution or
serialized-scene proof. The optional owned intermediate root redirects per-project design-time artifacts
through a hash-recorded output-only MSBuild props import, preserving original
project files. Each selected source hash and decoded text is checked
against the loaded native tree; the project/solution input hash is checked
before and after, but the filesystem is not an atomic snapshot.

Native regressions in `testdata/declarations` cover distinct field and
event locations; attributes; expression/bodiless signatures; escaped, Unicode
and UTF-16 positions; long signatures; partial/generic containers; active
defines; raw operators/destructors/indexers/delegates/records; and explicit
syntax/semantic errors. `*.cs.input` fixtures are not compiled into RoslynDump.
After the native output exists, `verify.py valid|error <output.json>` asserts
the real evidence. Fixture labels are source regression expectations, never
an independent language-grade oracle. Actual execution receipts must name their exact source/binary and fixture
outputs; fixture expectations alone do not establish a pass.

## Project imports and diagnostic-order migration

`project-imports <repository-root> <actual.sln|slnx|csproj>
<repository-relative-source-directory> [owned-intermediate-root]` reuses the
native project-declarations workspace and emits raw
`fathom.roslyn.project-imports/v1` facts. It records active using syntax, native
symbol/reference identity and complete namespace contribution declarations.
Current-compilation and actual CompilationReference sources can supply
declarations. Source, metadata, mixed, unresolved and unsupported cases remain
explicit; there is no namespace union or source association by textual name.

A PE reference with a possible selected-source match remains
UNSUPPORTED/UNKNOWN with zero source targets. Native preference and assembly
identity candidates are diagnostic only. The raw export is neither a closed
neutral scorer payload nor actual-use file dependency truth. An independently
reviewed adapter/closed cohort is required before an import-statement reading;
namespace contribution membership cannot replace file-use edges or a missing
metric denominator. `-imports-from tool` in lang-oracle only selects its neutral
import family beside SCIP and does not directly consume this raw export or
qualify a language grade.

Both native project modes preserve every rendered error diagnostic in ordinal
string order per project. This changes the old error-array byte ordering while
leaving its schema, messages and counts intact. Strict-byte consumers must pin
new error output under this explicit contract. Preserve historical raw outputs;
do not normalize their order to make old/new bytes appear equal. Valid-output
identity and new/new deterministic error output are separate controls from
old/new error-output identity.

The emission selector does not prevent dependency reads or create an OS/offline
sandbox. Use trusted native projects and pre-provisioned local dependencies,
recording exact source/reference/compiler closure and all errors. Generic
controls in `testdata/imports` cover current/referenced sources, partial and
parent namespaces, aliases/static/global/mixed and unused imports, scope,
inaccessible/unresolved/ambiguous cases and wrong/duplicate identities. The
original metadata-source positive remains unqualified. `verify_forward.py`
checks the distinct explicit unsupported-PE negative and new/new error-order
contract; it does not waive that historical positive or claim all native cases,
current-project evidence, default adoption or an overall support grade.
