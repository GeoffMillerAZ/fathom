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
