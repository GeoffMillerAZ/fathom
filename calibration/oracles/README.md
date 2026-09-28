# Bench-only oracle instruments (language breadth, ADR-048 D2/D5)

Each directory is one oracle a reading may name, with its tier, install
commands and exact invocation. None of them is a product dependency; all
run as separate processes against clean clones, and their outputs stay out
of the repository. The neutral output shape is `indexbench lang-oracle
-tool-oracle`'s: `{"tool","version","tier","definitions":[{path,name,line,
end,kind}],"calls":[{path,from,to}]}` (repo-relative slash paths, 1-based
lines, bare names).

| Directory | Tier | Languages |
|---|---|---|
| scip-typescript/ | A | TypeScript, JavaScript |
| scip-dotnet/ | A (def_mapping, typed_recall) | C# |
| roslyn/ | B | C# |
| javaparser/ | B | Java |
| ts-tags/ | C | Bash (calls) |
