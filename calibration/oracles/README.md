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
| scip-php/ | A (calls, def_mapping, file edges) | PHP |
| roslyn/ | B | C# |
| javaparser/ | B | Java |
| powershell-ast/ | B | PowerShell |
| php-parser/ | B | PHP |
| ts-tags/ | C | Bash (calls) |
| sqlglot/ | C | SQL (foreign-key calls) |

## Tool-format oracles (D4, LB-F30)

Small programs that write `indexbench lang-oracle`'s tool-oracle JSON (`-tool-oracle`,
design §10) for dialects no tier-A/B tool covers. Each is its own Go module — never
imported by fathom, never a product dependency — and is run from its own directory:

| Dialect | Tier | Parser | Emits | Run |
|---|---|---|---|---|
| `github-actions` | C | `github.com/rhysd/actionlint` v1.7.12 (workflows); `gopkg.in/yaml.v3` (action.yml) | definitions, `needs:` references, `cross_references` for local `uses: ./path` resolved from the repository root to the workflow file or the directory's `action.yml`/`action.yaml` (file granularity) | `cd calibration/oracles/github-actions && go run . -subject <checkout> > workspace/tmp/oracle/github-actions-<subject>.json` |
| `kubernetes` | C | `gopkg.in/yaml.v3` walk | definitions (`Kind/[namespace/]name` resources, container images), ServiceAccount/ConfigMap/Secret references, `cross_references` for a Service's `spec.selector` joined to every workload whose pod-template labels (a bare Pod's own labels) include it, same namespace (entity granularity) | `cd calibration/oracles/kubernetes && go run . -subject <checkout> > workspace/tmp/oracle/kubernetes-<subject>.json` |

Tier C, not A: both re-implement the format's rules over an independent parser; neither
evaluates the format the way the tool that consumes it does (GitHub's runner, the
Kubernetes API server). A reference whose target file has no root entity (a reusable
workflow without `name:`) is in the oracle but resolves to no fathom entity — a real,
disclosed recall miss, not an oracle error.
