# otelcol-oracle (tier A definitions and references; tier C rules and routes)

The language-breadth harness's oracle for the `otel-collector` doc-engine dialect (LB-Q-otel-collector,
design `docs/design/language-breadth.md` §1.3, §10). A separate Go module (`go.mod`: the collector's
`confmap`, `component` and `pipeline` modules v1.68.0, the core of collector-contrib v0.162.0, and
yaml.v3), never imported by fathom, never a product dependency. Apache-2.0 throughout.

```
cd calibration/oracles/otelcol && go build -o <scratch>/otelcol-oracle . &&
  <scratch>/otelcol-oracle -subject <checkout> -otelcol <otelcol-contrib> \
    -refs-out <scratch>/otel-<subject>-refs.json -routes-out <scratch>/otel-<subject>-routes.json \
    -rules-out <scratch>/otel-<subject>-rules.json -files-out <scratch>/otel-<subject>-files.txt \
    > <scratch>/otel-<subject>.json
indexbench lang-routes -dialect otel-collector -subject <checkout> -tool-oracle <defs.json> \
    -tool-oracle <refs.json> -tool-oracle <routes.json> -only "$(paste -sd, <files.txt>)" \
    -out docs/bench/readings/lang/otel-collector.json
```

## The pinned collector

`otelcol-contrib` 0.162.0, `otelcol-contrib_0.162.0_darwin_arm64.tar.gz` from
github.com/open-telemetry/opentelemetry-collector-releases v0.162.0, sha256
`d5e11974d2e4adac3cc001a25137aad52f6e77ec3034ba7735cac7c219a5f97a` (checked against the release's
`.sha256` asset before extraction). Static use only: `validate` loads and validates, it never starts a
pipeline or opens a port.

## The instruments

- **Acceptance (tier A): the collector.** `otelcol-contrib validate --config=file:<f>` decodes the file
  into the distribution's component factories and runs every component's own validation, the check the
  collector runs before it starts. A file that fails alone is tried as a layer over each sibling that
  passes alone (`--config base --config layer`, the opentelemetry-demo's layout), so a layer whose
  pipelines name its base's components is accepted as the demo runs it. Every `${env:VAR}` the candidates
  reference is set to a placeholder (`4317` for a name with `PORT`), and three error lines about the
  measuring host, not the file, are disclosed on stderr and do not reject it: a Linux-only `root_path`,
  a storage `directory must exist`, and the `service.profilesSupport` feature gate. Any other error
  rejects the file; a rejected file contributes nothing and the harness's `-only` leaves it out (a
  distribution's own component, opentelemetry-demo's `otelcol-ebpf-profiling.yml` `profiling` receiver;
  an example written for an older OTTL, collector-contrib's couchbase).
- **Definitions and references (tier A): the collector's decode.** Each accepted file is loaded with
  `confmap.NewRetrievedFromYAML` and decoded by confmap's own `Unmarshal` into maps keyed by
  `component.ID` and `pipeline.ID` (the collector's ID grammar, `type[/name]`, refusing a malformed id)
  and the service's `extensions` and `pipelines` lists: every component under its section
  (`receivers/otlp`), every pipeline (`pipelines/traces`), and each pipeline's receivers, processors and
  exporters and the service's extensions as references when the component is declared in the same file
  (the dialect's intra-file L2). The file's own entity (`package`, named by the file at its first key) is
  the dialect's identity rule, mirrored.
- **Rules (tier C, `-rules-out`, never scored).** A resource processor's `attributes[*]` or an attributes
  processor's `actions[*]` setting `service.name` to a literal is a TelemetryIdentity (TL-5's contract); a
  span-metrics or service-graph connector derives the metric families its collector-contrib v0.162.0
  source names (`factory.go` `DefaultNamespace`, `connector.go` `metricNameCalls`, `metricNameDuration`,
  `metricNameEvents`; servicegraph's `SetName` literals). The dialect leaves `key` and `view` out of
  `oracle_kinds`, so these are compared in the reading's notes, not in sym_precision.
- **Route pairs (tier C, `-routes-out`, `routes.go`).** An exporter's endpoint fields (the dialect's
  list, mirrored) joined to the subject's compose services and Kubernetes Services by the Kubernetes and
  compose DNS rules, nearest declarations first; the rest unresolved by reason.
- **Lines** are gopkg.in/yaml.v3's: a map entry at its key, a list item at its first line.
