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
