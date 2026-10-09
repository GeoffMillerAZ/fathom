# prometheus (test-only module)

A separate Go module, never imported by fathom and never a product dependency (ADR-020, ADR-025): it
holds the Prometheus parser so fathom's main module does not.

## The selector differential (LB-F316)

`selectors_diff_test.go` checks fathom's pure-Go selector scanner (`internal/promql`, read from this
checkout through a local `replace`) against the Prometheus parser's own `ExtractSelectors`
(`github.com/prometheus/prometheus` v0.315.0, the Go module of Prometheus v3.15.0, Apache-2.0) with
every optional syntax enabled. On every expression the parser accepts, the two must name the same
selectors, as multisets of matcher sets (the metric name as its `__name__` equality matcher). An
expression the parser refuses is counted and left out; a Grafana range variable (`$__rate_interval`,
`$__interval`, `$__range`) is read as `5m` on both sides first.

```
cd calibration/oracles/prometheus
FATHOM_PROMQL_CORPUS=<thanos>:<kube-prometheus>:<node_exporter>:<opentelemetry-demo> \
  go test -count=1 -run TestSelectorsAgreeWithParser -v .
```

Every `expr` string of every YAML and JSON file is read, a Grafana dashboard held in a ConfigMap
string included. Reading of 2026-10-09 (thanos v0.42.4 45c4f394, kube-prometheus v0.19.0 799f3d73,
node_exporter v1.12.1 6044da78, opentelemetry-demo 3.1.0 dedc0178): 1,592 expressions (456 with a
Grafana range variable), 1,451 agree, 0 disagree, 141 refused by the parser (136 a Grafana template
variable such as `$job` outside a string, which the scanner reads; 3 empty).
