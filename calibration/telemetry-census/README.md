# telemetry-census (C0, TL-6)

The C0 census (`docs/bench/readings/telemetry/census-c0.json`, `census-c0.md`): of the metric names an
observability stack's PromQL reads, how many can a static pass trace to the code that emits them? ADR-062
decision 3 gates Stage B (metric and span names joined to their emitters, F66) on the pooled emitter share E
being at least 25 percent. A separate Go module (`go.mod`: yaml.v3, and fathom's own `internal/promql` read
from this checkout through a local `replace`), never imported by fathom and never a product dependency. It
costs nothing: no model, no store, no extractor; static files only, never run.

```
cd calibration/telemetry-census && go build -o <scratch>/telemetry-census .
<scratch>/telemetry-census \
  -corpus prometheus-operator/kube-prometheus=<kube-prometheus v0.19.0> \
  -corpus open-telemetry/opentelemetry-demo=<opentelemetry-demo 3.1.0> \
  -corpus thanos-io/thanos=<thanos v0.42.4> \
  -date <YYYY-MM-DD> -commit <fathom short sha> -out docs/bench/readings/telemetry/census-c0.json
go test .   # the classes, the Grafana variable reading, the YAML shapes
```

Each corpus is a git checkout at its pinned tag (the reading records the sha and the remote). The method,
the classes (`classes.json`, data) and the reading's verdict are in `census-c0.md`.
