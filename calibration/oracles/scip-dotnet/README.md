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
Roslyn syntax dump (`../roslyn/`) instead. Since LB-F54,
`internal/scip.ImportIndex` takes a mapped definition's fathom lines as its
span, so `fathom ingest -scip` stores this index's typed edges (Polly: 36,831
direct; O235); the C# reading has not been re-measured with it.

Re-measure 2026-10-01 (wave lb remeasure, branch lb-remeasure = fdd2bb59 plus LB-F64's commit): the 2026-09-28 indexes and
Roslyn dumps were reused (the checkouts are the pinned SHAs, unchanged). One lang-oracle run per subject, with the dump's
`calls` array removed so the SCIP index is the only calls oracle (the harness takes one per subject), and `-unmatched` for
the L2 call-pair classification:

    python3 -c "import json,sys; d=json.load(open(sys.argv[1])); d.pop('calls'); json.dump(d,open(sys.argv[2],'w'))" out/csharp-Polly.json out/csharp-Polly-nocalls.json
    indexbench lang-oracle -dialect csharp -subject <clone>/Polly -subject-name App-vNext/Polly -tool-oracle out/csharp-Polly-nocalls.json \
        -scip out/csharp-Polly.scip -unmatched out/unmatched-Polly.json -out docs/bench/readings/lang/csharp.json

A generic method called with explicit type arguments (`Policy<int>.Handle<InvalidOperationException>()`, `FuncAsync<int>()`)
has no occurrence in the index for the method name: 5,624 of the 8,265 call pairs the dialect finds and the index does not
prove (O271).
