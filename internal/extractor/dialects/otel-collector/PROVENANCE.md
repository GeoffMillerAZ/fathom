# otel-collector — provenance and divergences

LB-Q-otel-collector, design `docs/design/language-breadth.md` §4, §13; wave c1 (item mesh), tier T7
(observability as code). A doc-engine dialect (no `tags.scm`, no upstream grammar, no incumbent: a
collector configuration was a generic YAML file). The decode is `gopkg.in/yaml.v3` through the doc engine.
Fixtures are written for this dialect (no upstream text is copied); their shapes follow the collector's
documentation and the opentelemetry-demo and collector-contrib example configurations (Apache-2.0).

## How a file is claimed

The manifest's globs name the conventional file names (`otelcol*`, `otel-collector-config*`,
`otel-col-config*`, `.yaml` or `.yml`) and `doc.json`'s `detect.content_sniff` the shape: a top-level
component section (`receivers`, `processors`, `exporters`, `connectors` or `extensions`) followed by a
top-level `service:` whose block holds `pipelines:`. `detect.require_keys` is `service`, so a document
without one yields nothing (an Alertmanager file has `receivers:` but no `service`, `not-a-collector.yaml`).
Ingest claims a file for this dialect last among the YAML signals (the late-YAML dispatch, LB-F309): a
Kubernetes object, a compose file or a gateway configuration of the same name stays what it is.

## What it models

- **The file** (`OtelConfig`, canonical `package`), named by the file at its first key: what a
  `service.extensions` reference starts from.
- **Components**, one per key of each section, named `<section>/<type>[/<name>]` (the collector's
  `component.ID` written under its section, so `receivers/otlp` and `exporters/otlp` are two entities):
  receivers (`OtelReceiver`, `data_source`), processors (`OtelProcessor`, `step`), exporters
  (`OtelExporter`, `output`), connectors (`OtelConnector`, `route`), extensions (`OtelExtension`,
  `mixin`). A component written with no value (`batch:`) is one.
- **Pipelines** (`OtelPipeline`, `job`), named `pipelines/<signal>[/<name>]` (`pipeline.ID`).
- **References** (`Calls`, L2, only to a component declared in the same file: `declared_only`): a
  pipeline to each receiver, processor and exporter it lists, a connector named as a receiver or an
  exporter to the connector; the file to each `service.extensions` entry.
- **Route targets** (L3): an exporter's `endpoint`, `endpoints[*]`, `http.endpoint`, `traces_endpoint`,
  `metrics_endpoint` and `logs_endpoint` are `route_ref` references with strategy `route-host`;
  `fusion.ResolveRouteRefs` joins each to the compose service or Kubernetes Service of the repository its
  host names (`jaeger:4317` to the compose service `jaeger`), or counts why not (a `${env:...}` host is a
  variable).
- **Telemetry identity** (TL-5, `TelemetryIdentity`, `key`): a `service.name` set by a processor's
  `attributes[*]` (the resource processor) or `actions[*]` (the attributes processor) list, named by the
  value, signed `service.name=<value>`.
- **Derived metric families** (`OtelDerivedMetric`, `view`, signed `origin=platform connector=<id>`): the
  families a span-metrics connector (`span_metrics`, the deprecated `spanmetrics`) emits,
  `traces.span.metrics.calls` and `.duration` (`.events` when `events.enabled`), or the same under its
  `namespace`; and a service-graph connector's (`service_graph`, `servicegraph`)
  `traces_service_graph_request_total`, `_failed_total`, `_request_server` and `_request_client` (the
  connectors' source at collector-contrib v0.162.0). These are census partition P: a platform-origin
  metric, never an "orphan alert" for want of an in-repository emitter.

## Divergences (each forced by the doc engine's subset, §4)

- **A connector with its own `namespace`** carries both its namespaced families and the default ones: the
  engine has no test for a missing key, so the default-named entities cannot be withheld. The oracle names
  only the namespaced ones.
- **An identity's value is named as written.** The engine has no value filter, so
  `value: ${env:SERVICE_NAME}` is an identity named `${env:SERVICE_NAME}`; TL-5's consumer matches only a
  literal (`telemetryIdentityLiteral`), and the oracle writes none. Any processor's `attributes[*]` or
  `actions[*]` item whose key is `service.name` and that carries a value is read, whatever the processor's
  type and the item's action (a `delete` action is one too); the oracle reads the resource and attributes
  processors' lists and skips `delete`.
- **Layered configurations** (`--config base.yaml --config layer.yaml`, the opentelemetry-demo layout): a
  layer's pipeline naming a component its base declares is no reference (declared in another file); the
  intra-file contract is L2's and the oracle measures the same.
- **A collector configuration inside a Kubernetes ConfigMap or an OpenTelemetryCollector resource** is a
  string or a CRD field, not this dialect's file: deferred (LB-Q-otel-collector-rest), with the
  Instrumentation resource and the SDK's declarative configuration (`otel-config.yml`, `file_format:`).

## Calibration

Subjects in `calibration/corpus.d/otel-collector.yaml`; the oracle is `calibration/oracles/otelcol`
(README there): `otelcol-contrib validate` (tier A acceptance), the collector's own confmap decode of
component and pipeline IDs (tier A definitions and references), the derived families and identities by
rule (tier C, never scored: `oracle_kinds` leaves `key` and `view` out), and the exporter route pairs
(tier C). The reading is `docs/bench/readings/lang/otel-collector.json`; the goldens run in
`internal/fusion/otelcollector_dialect_test.go`.
