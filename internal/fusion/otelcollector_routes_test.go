package fusion

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// otelRouteRepo: a collector configuration under a name no manifest glob
// claims (the late-YAML dispatch claims it by content, LB-F309) whose
// exporters send to the compose services beside it.
func otelRouteRepo() map[string]string {
	return map[string]string{
		"compose.yaml": `services:
  jaeger:
    image: jaegertracing/jaeger:2
  prometheus:
    image: prom/prometheus:v3
  collector:
    image: otel/opentelemetry-collector-contrib:0.162.0
`,
		"infra/telemetry/gateway.yaml": `receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
exporters:
  otlp_grpc/jaeger:
    endpoint: jaeger:4317
  otlp_http/metrics:
    endpoint: http://prometheus:9090/api/v1/otlp
  otlp_grpc/backend:
    endpoint: ${env:BACKEND}:4317
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlp_grpc/jaeger, otlp_grpc/backend]
    metrics:
      receivers: [otlp]
      exporters: [otlp_http/metrics]
`,
	}
}

// TestIngest_OtelCollectorRoutes (LB-F313, LB-Q-otel-collector): a YAML
// file the late-YAML dispatch claims by content is a route candidate, so an
// ingest stores its exporters' routes to the compose services their
// endpoints name (a ${env:...} host is a variable), and a change to the
// configuration re-runs the route pass (passNeeded reads the same
// predicate) and moves the edge.
func TestIngest_OtelCollectorRoutes(t *testing.T) {
	if !isRouteCandidate("infra/telemetry/gateway.yaml") || !isRouteCandidate("x/y/collector.YML") {
		t.Fatal("a YAML file must be a route candidate whatever its name")
	}
	if isRouteCandidate("src/main.go") {
		t.Fatal("a Go source is no route candidate")
	}
	dir := t.TempDir()
	writeCorpus(t, dir, otelRouteRepo())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "ot", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"infra/telemetry/gateway.yaml|ot::infra/telemetry/gateway.exporters/otlp_grpc/jaeger->ot::compose.jaeger",
		"infra/telemetry/gateway.yaml|ot::infra/telemetry/gateway.exporters/otlp_http/metrics->ot::compose.prometheus",
	}
	if got := storedRouteEdges(t, st, "ot"); !reflect.DeepEqual(got, want) {
		t.Fatalf("stored route-ref edges\n got: %q\nwant: %q", got, want)
	}
	if wantBy := map[string]int{RouteVariable: 1}; !reflect.DeepEqual(rep.RouteRefUnresolved, wantBy) {
		t.Errorf("RouteRefUnresolved = %v, want %v", rep.RouteRefUnresolved, wantBy)
	}
	cfg := filepath.Join(dir, "infra/telemetry/gateway.yaml")
	b, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte(strings.Replace(string(b), "endpoint: jaeger:4317", "endpoint: collector:4317", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	rep2, err := p.Ingest(ctx, core.ServiceRef{ID: "ot", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	if rep2.RouteRefSkipped != "" {
		t.Fatalf("route-ref pass skipped (%q) after the collector configuration changed", rep2.RouteRefSkipped)
	}
	want2 := []string{
		"infra/telemetry/gateway.yaml|ot::infra/telemetry/gateway.exporters/otlp_grpc/jaeger->ot::compose.collector",
		"infra/telemetry/gateway.yaml|ot::infra/telemetry/gateway.exporters/otlp_http/metrics->ot::compose.prometheus",
	}
	if got := storedRouteEdges(t, st, "ot"); !reflect.DeepEqual(got, want2) {
		t.Errorf("edges after the change\n got: %q\nwant: %q", got, want2)
	}
}
