# Bench-only oracles (language-breadth)

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
