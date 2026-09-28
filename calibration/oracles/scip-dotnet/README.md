# scip-dotnet — tier-A C# index (def_mapping, typed_recall)

Installed without sudo into a tools dir:

    dotnet-install.sh --version 10.0.401 --install-dir <tools>/dotnet --no-path   # Polly's global.json pins 10.0.401
    dotnet tool install --tool-path <tools>/scip-dotnet scip-dotnet                  # 0.2.14

Run (2026-09-28, both clean):

    run.sh <tools> <clone>/Polly           .   out/csharp-Polly.scip                     # 49 s, 882 documents
    LibraryFrameworks=net8.0 TestFrameworks=net8.0 \
    run.sh <tools> <clone>/Newtonsoft.Json Src out/csharp-Newtonsoft.Json.scip 10.0.401  # 29 s, 948 documents

Newtonsoft.Json's `Src/global.json` pins SDK 9.0.300 (rollForward
latestFeature, so 10.x is refused); the run overrides it for the run only,
and the two framework properties cut its nine target frameworks to net8.0.
Its TestConsole and FuzzTests projects fail restore under that cut (2 of
944 files); the index covers the library and its tests.

What the index carries: no `enclosing_range` on any definition, no
`SymbolInformation.kind`, and `documentation[0]` is the C# declaration
(`class Bulkhead`, `public void Bulkhead.M()`). Without enclosing ranges it
yields no call pairs by itself; C# L1 and call pairs are measured with the
Roslyn syntax dump (`../roslyn/`) instead.
