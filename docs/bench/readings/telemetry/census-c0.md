# TL-6: the C0 census, how many metric names a static pass can trace to the code that emits them

**Verdict: Stage B does not clear its gate. Do not build it yet.** Of the 586 metric names that 1,391 PromQL
expressions read across three observability stacks, a literal in the corpus' own source code names 85: E is
14.5%, under the 25% ADR-062 asks before name joins (F66, TL-2, TL-3) are built. The rest are platform metrics
(68.4%: `node_`, `kube_`, `container_`, `http_server_`, `traces_span_metrics_`, the OpenTelemetry Collector's
and the Go runtime's own) or names nothing in the corpus emits (17.1%, a third of them recording-rule outputs).
And 14.5% is an upper bound: a literal grep cannot tell an emitter from a consumer, so kube-prometheus' two "E"
names are queries in its e2e tests, not emitters. What this says for fathom: the join that pays on an
observability stack is Stage A's, identity (39% of the expressions pin a service with an equality matcher on
`job`, `service`, `app` or a mesh label; in a Prometheus rule the prometheus dialect's promql mode turns that
matcher into a `deploy:` route target, and dashboards are not yet read by any dialect); a metric name to code
join would name the emitter in one case of seven, and mostly in the one subject that is the emitter itself
(Thanos reading its own metrics, 68%). The census cost nothing: no model, no daemon, static files only,
a few seconds of CPU.

Row TL-6, `docs/plans/telemetry-links.md` (item incident's), produced by wave c1 item prom. Decision: ADR-062
(decision 3, the Stage B gate). Data: `census-c0.json` beside this file. Script:
`calibration/telemetry-census/` (a nested, test-only Go module).

## Method

- **Corpora, pinned, read never run.** prometheus-operator/kube-prometheus v0.19.0 (799f3d73: PrometheusRule
  specs and the Grafana dashboards of `manifests/grafana-dashboardDefinitions.yaml`), open-telemetry/
  opentelemetry-demo 3.1.0 (dedc0178: Grafana dashboards and Grafana-managed alert rules, and the source of its
  services in eleven languages), thanos-io/thanos v0.42.4 (45c4f394: its mixin's rules and dashboards under
  `examples/`, and its Go source). Dashboards are data: no Grafana code is read.
- **Expressions.** Every `expr` of a Prometheus rule (a rule file's or a PrometheusRule's groups), of a
  Grafana alert rule's query (`data[].model.expr`) and of a Grafana dashboard panel's `targets[]` (a dashboard
  file, or one held in a ConfigMap): 300 rule, 11 Grafana-alert and 1,080 dashboard expressions. A Grafana
  variable outside a string is read as PromQL first (`$__rate_interval` as `5m`, `$__interval_ms` as `1`, any
  other as a placeholder that is never a metric name; 592 expressions carry one); one inside a string
  (`job="$job"`) stays. Every expression scanned (0 unscannable).
- **Scanner.** fathom's own `internal/promql.Selectors`, the one the prometheus dialect reads rules with: its
  metric names are the selectors' (a name written before the braces, a quoted name, an equality `__name__`).
  The Prometheus parser agrees with it on every corpus expression it parses (LB-F316).
- **Classes** (`calibration/telemetry-census/classes.json`, data). Each name of a corpus is one of:
  - **E**: a string literal of the corpus' source (`.go`, `.py`, `.java`, `.cs`, `.js`, `.ts`, `.rb`, `.rs`,
    `.php`, `.ex`, `.cpp`, …; vendor and node_modules skipped) names it, compared after one Prometheus series
    suffix (`_bucket`, `_count`, `_sum`, `_total`, `_created`, `_info`) and one OpenTelemetry unit suffix
    (`_seconds`, `_bytes`, …) are stripped and `.`, `-`, `/` are read as `_`: `demo.cart.add_item.latency`
    (unit `s`) emits `demo_cart_add_item_latency_seconds_bucket`;
  - **P**: a platform, exporter or agent prefix or name (`node_`, `kube_`, `container_`, `istio_`, `envoy_`,
    `traces_span_metrics_`, `http_server_`, `process_`, `go_`, `prometheus_`, `thanos_`, `alertmanager_`,
    `up`, …), tested after E;
  - **U**: everything else; a recording rule's output is counted within U (`U_recorded`).
- **Identity.** An expression pins a service when one of its selectors has an equality matcher with a literal
  value on an identifying label (`promql.IdentityLabels()` less `namespace`). The denominator is every
  expression.

## Results

| Corpus | Expressions | With an identity matcher | Names | E | P | U (recorded) | E share |
|---|---|---|---|---|---|---|---|
| kube-prometheus v0.19.0 | 869 | 534 (61.5%) | 364 | 2 | 288 | 74 (39) | 0.5% |
| opentelemetry-demo 3.1.0 | 196 | 4 (2.0%; 26 more with a `$variable`) | 104 | 3 | 81 | 20 | 2.9% |
| thanos v0.42.4 | 326 | 0 | 118 | 80 | 32 | 6 | 67.8% |
| **Pooled** | **1,391** | **538 (38.7%)** | **586** | **85** | **401** | **100 (39)** | **14.5%** |

A name in two corpora counts in each (each corpus' emitter search is its own source). Expressions that read at
least one E name: 234 of 1,391 (16.8%).

- **kube-prometheus** deploys other people's software: its rules and dashboards read Kubernetes, node and
  Prometheus metrics, and it has no service code to emit any; its two E names (`up`,
  `prometheus_remote_storage_samples_in_total`) are its e2e tests' queries.
- **opentelemetry-demo** is the stack an adopter runs: eleven services' SDK instruments emit three of the 104
  names its dashboards read (`demo.cart.add_item.latency`, `demo.cart.get_cart.latency`,
  `demo.recommendation.requests`); the dashboards read the SDKs' and the Collector's semantic-convention metrics
  (`http_server_request_duration_seconds`, `traces_span_metrics_*`, `otelcol_*`, PostgreSQL and NGINX receivers)
  and the U names are receivers' (`pg_stat_*`, `db_client_*`). Its dashboards pin a service through a Grafana
  variable (`service_name="$service_name"`), which only the running app can bind.
- **thanos** reads its own metrics: the emitter is the subject, which is why E is high there and why it does
  not generalize to an adopter's stack.

## What would change the verdict

E counts literals; a client library that builds a name from parts (`Namespace: "thanos"`, `Subsystem:
"sidecar"`) is missed, so a stack whose services register metrics that way reads lower than its truth, and a
consumer's literal (a test's query) reads higher. Neither moves the pooled share across 25% here: removing the
consumers lowers it, and even if every Thanos name (118) and every opentelemetry-demo U name (20) were an
emitter, pooled E would be 143 of 586, 24.4%. A census over adopters' service repositories with hand-written
instruments (the subject Stage B would serve) is the reading that could clear it; it is F66's trigger, with
the F59 amendment the owner records.
