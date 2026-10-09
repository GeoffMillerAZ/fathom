# prometheus-oracle (tier A) and the PromQL selector differential

The language-breadth harness's oracle for the `prometheus` doc-engine dialect (LB-Q-prometheus, design §1.3, §10),
and the differential test of fathom's PromQL selector scanner (LB-F316). A separate Go module (`go.mod`:
`github.com/prometheus/prometheus` v0.315.0, the Go module of Prometheus v3.15.0, Apache-2.0; yaml.v3), never
imported by fathom and never a product dependency (ADR-020, ADR-025): it holds the Prometheus parser so fathom's
main module does not, and reads fathom's own `internal/promql` from this checkout through a local `replace`.

## The instrument

promtool 3.15.0, the release binary (`prometheus-3.15.0.<os>-<arch>.tar.gz`, its sha256 checked against the
release's `sha256sums.txt`; darwin-arm64 `920df4d1…77260`, revision 5241a27f), decides which files are Prometheus
files, and Prometheus' own packages name what each declares:

- **A rule file** (a top-level `groups`) is a subject when `promtool check rules` accepts it. `model/rulefmt` (strict,
  UTF-8 names, the parser with every optional syntax on) gives its groups and its alerting and recording rules
  (`section`, `rule`); `promql/parser` `ExtractSelectors` gives the metric names each rule's expr reads, and a
  recording rule of the same file named by one is a reference (`references`).
- **A configuration** (a top-level `scrape_configs` or `rule_files`) is a subject when `promtool check config
  --syntax-only` accepts it. `config.Load` (every service discovery registered, as promtool registers them) gives its
  scrape jobs (`job`, and the `key` that is its telemetry identity) and their `static_configs` addresses.
- **A promtool test file** (a top-level `tests` beside `rule_files`) is a subject when `promtool test rules` loads it
  (a failing unit test is still a test file; one it cannot load is not); promtool's own test file shape, read
  strictly, gives its tests (`test`, `tests/<name>` or `tests/<index>`).
- **Every accepted file** is a `package` named by the file. Its `rule_files` (relative to its directory, a glob
  expanded as Prometheus expands it) name accepted rule files: `cross_references` at file granularity.

Lines are not Prometheus output: they come from yaml.v3, an item's first key, "where it is written", and an end line
is the last line any node under it is written on. A Kubernetes object is never a candidate (a PrometheusRule custom
resource is the kubernetes side pass's), nor is a file under a directory fathom's connector skips (`testdata`,
`vendor`, `node_modules`, …).

`-routes-out` (bench-only; `lang-routes` scores it): `route_references` and `route_unresolved` for each scrape job's
static address and each identifying-label equality matcher of a rule's expr (`promql.IdentityLabels()`, scoped by
the selector's `namespace` matcher; a regex or negative matcher is `regex-matcher` or `negative-matcher`), joined to
a docker-compose service (read by `docker compose config` wherever Compose accepts the file, else yaml.v3) or a
Kubernetes Service. A pair joined to a service Compose itself read is Compose's own DNS (tier A, the traefik
oracle's rule); a pair to a Kubernetes Service or from a label's value is this oracle's own reading (tier C), and the
file is scored at tier C when any pair is.

`-derive-prometheusrules <dir>` writes each PrometheusRule custom resource's `spec.groups` as a plain rule file at
the same relative path, so kube-prometheus' manifests are measured as extracted specs (their ingest path is the
kubernetes dialect's, item mesh; the namespace a custom resource's labels are scoped by is not carried).

## Run

```
cd calibration/oracles/prometheus && go build -o <scratch>/prometheus-oracle .
<scratch>/prometheus-oracle -subject <kube-prometheus>/manifests -derive-prometheusrules <derived>/kube-prometheus/manifests
cd <subject> && <scratch>/prometheus-oracle -subject . -promtool <promtool> \
    -refs-out <o>/<s>-refs.json -routes-out <o>/<s>-routes.json -files-out <o>/<s>-files.txt > <o>/<s>-defs.json
indexbench lang-routes -dialect prometheus -subject <subject> -subject-name <s> \
    -tool-oracle <o>/<s>-defs.json -tool-oracle <o>/<s>-refs.json -tool-oracle <o>/<s>-routes.json \
    -only "$(paste -sd, <o>/<s>-files.txt)" -out docs/bench/readings/lang/prometheus.json
```

The subjects are pinned in `calibration/corpus.d/prometheus.yaml`. `prometheus/prometheus` is read as a derived
tree: `documentation/examples/` as is, and `cmd/promtool/testdata/` copied to `promtool/` (the connector skips a
`testdata` directory, and those are the promtool test files of the release).

## The selector differential (LB-F316)

`selectors_diff_test.go` checks `internal/promql.Selectors` against the parser's `ExtractSelectors` with every
optional syntax enabled. On every expression the parser accepts, the two must name the same selectors, as multisets
of matcher sets (the metric name as its `__name__` equality matcher). An expression the parser refuses is counted
and left out; a Grafana range variable (`$__rate_interval`, `$__interval`, `$__range`) is read as `5m` on both
sides first.

```
cd calibration/oracles/prometheus
FATHOM_PROMQL_CORPUS=<thanos>:<kube-prometheus>:<node_exporter>:<opentelemetry-demo> \
  go test -count=1 -run TestSelectorsAgreeWithParser -v .
```

Every `expr` string of every YAML and JSON file is read, a Grafana dashboard held in a ConfigMap string included.
Reading of 2026-10-09 (thanos v0.42.4 45c4f394, kube-prometheus v0.19.0 799f3d73, node_exporter v1.12.1 6044da78,
opentelemetry-demo 3.1.0 dedc0178): 1,592 expressions (456 with a Grafana range variable), 1,451 agree, 0 disagree,
141 refused by the parser (136 a Grafana template variable such as `$job` outside a string, which the scanner
reads; 3 empty).
