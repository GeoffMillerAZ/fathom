package main

import (
	"reflect"
	"testing"

	"go.opentelemetry.io/collector/component"
	"gopkg.in/yaml.v3"
)

// TestReadConfig pins the decode: component and pipeline IDs through the
// collector's own grammar, intra-file references only, identities only from
// a literal, derived families under the connector's namespace, and the
// exporter endpoints.
func TestReadConfig(t *testing.T) {
	src := []byte(`receivers:
  otlp:
    protocols: {grpc: {}}
processors:
  resource:
    attributes:
      - {key: service.name, value: checkout, action: upsert}
      - {key: service.name, value: "${env:X}", action: upsert}
exporters:
  otlp_grpc/jaeger:
    endpoint: jaeger:4317
connectors:
  span_metrics:
    namespace: shop
service:
  extensions: [zpages]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [resource]
      exporters: [otlp_grpc/jaeger, span_metrics, debug]
    metrics/spans:
      receivers: [span_metrics]
      exporters: [otlp_grpc/jaeger]
`)
	defs, rules, refs, routes, err := readConfig("c.yaml", src)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, d := range defs {
		names = append(names, d.Kind+" "+d.Name)
	}
	want := []string{"package c.yaml", "data_source receivers/otlp", "step processors/resource", "output exporters/otlp_grpc/jaeger",
		"route connectors/span_metrics", "job pipelines/metrics/spans", "job pipelines/traces"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("defs %q, want %q", names, want)
	}
	var rn []string
	for _, d := range rules {
		rn = append(rn, d.Kind+" "+d.Name)
	}
	if want := []string{"key checkout", "view shop.calls", "view shop.duration"}; !reflect.DeepEqual(rn, want) {
		t.Errorf("rules %q, want %q", rn, want)
	}
	if len(refs) != 6 {
		t.Errorf("refs %v, want 6 (debug and zpages are not declared here)", refs)
	}
	if len(routes) != 1 || routes[0].raw != "jaeger:4317" || routes[0].from != "exporters/otlp_grpc/jaeger" {
		t.Errorf("routes %v", routes)
	}
}

func TestDerivedFamiliesDefault(t *testing.T) {
	id := component.MustNewID("spanmetrics")
	var n yaml.Node
	if got := derivedFamilies(id, &n); !reflect.DeepEqual(got, []string{"traces.span.metrics.calls", "traces.span.metrics.duration"}) {
		t.Errorf("derivedFamilies = %q", got)
	}
}
