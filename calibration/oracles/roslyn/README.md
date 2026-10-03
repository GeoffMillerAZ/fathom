# RoslynDump — tier-B C# L1/L2 oracle (syntax only)

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
