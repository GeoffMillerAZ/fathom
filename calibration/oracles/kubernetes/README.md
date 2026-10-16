# kubernetes-oracle

The language-breadth harness's oracle for the kubernetes dialect and the Kubernetes incumbent extractor.
Bench-only, its own module (`go.mod`), never a product dependency. The catalog row is
`calibration/oracles/README.md`'s `kubernetes`; this file covers the two modes wave lf8 added.

| Mode | Tier | What it writes |
|---|---|---|
| (default) | C | the independent `gopkg.in/yaml.v3` walk: definitions, references, `cross_references`, route pairs (`routes.go`, `mesh.go`) |
| `-kubectl <bin> -refs-out <json>` | A (definitions) | kubectl v1.36.1's offline decode on stdout (`kubectl.go`); the walk's rest in `-refs-out` |
| `-labeljoins-out <json>` | C | the Stage A label-join truth (`labeljoins.go`) |
| `-selectors-out <json>` | A (one decision) | a core v1 Service's selector joins by `k8s.io/apimachinery` v0.37.0 (`selectors.go`, LB-F456); the walk's own `cross_references` leave its output; the written L3 ceiling as a `disclose` note |
| `-facts-out <json> [-facts-fathom <rows.json>]` | A | the per-container facts `k8s.io/api` v0.37.0's typed structs read (`facts.go`, LB-F451), compared field for field with a scratch store's `K8sFact` rows |

The rules both new modes follow are design §1.8's Kubernetes label-selector paragraph (K19, LB-F456) and
LB-F451's row. Without `-selectors-out` every output is byte-identical to what it was before: TL-8 reads
`-labeljoins-out` and TL-16 the side pass's pairs.

```
cd calibration/oracles/kubernetes
go run . -subject <checkout> -kubectl <kubectl> -refs-out refs.json -files-out files.txt -selectors-out selectors.json > defs.json
indexbench lang-routes -dialect kubernetes -subject <checkout> -tool-oracle defs.json -tool-oracle refs.json -tool-oracle selectors.json -out <reading.json>

fathom ingest --service s --path <checkout> --store s.sqlite
sqlite3 -json s.sqlite "select source_ref as path, symbol, claim from observations where source_kind = 'K8sFact'" > rows.json
go run . -subject <checkout> -facts-out facts.json -facts-fathom rows.json > /dev/null
```

`k8s.io/api` and `k8s.io/apimachinery` are pinned at v0.37.0, the version the module's Prometheus pin
(v0.315.0, the parser `labeljoins.go` and `mesh.go` read) already requires, so adding them moved no other
dependency.
