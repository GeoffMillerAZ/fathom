package fusion

// otelcollector_dialect_test.go — LB-Q-otel-collector (wave c1, item mesh):
// the otel-collector doc-engine dialect's goldens. The dialect is data only
// (internal/extractor/dialects/otel-collector); its runner lives here, beside
// the route pass that reads its exporter endpoints.

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

var updateOtelGoldens = flag.Bool("update-otel-goldens", false, "rewrite the otel-collector dialect goldens from current output")

type otelGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
}

type otelGoldenCross struct {
	From     string `json:"from"`
	Edge     string `json:"edge"`
	Raw      string `json:"raw"`
	Strategy string `json:"strategy"`
	Line     int    `json:"line"`
}

type otelGolden struct {
	Detected  bool               `json:"detected"`
	Symbols   []otelGoldenSymbol `json:"symbols"`
	CrossFile []otelGoldenCross  `json:"cross_file"`
}

// TestOtelCollectorDialectGoldens (LB-Q-otel-collector): every fixture's
// content detection, entities (receivers, processors, exporters,
// connectors and extensions by `<section>/<type>[/<name>]`, pipelines by
// `pipelines/<signal>[/<name>]`, a resource or attributes processor's
// service.name as a TelemetryIdentity, the span-metrics and service-graph
// connectors' derived metric families marked origin=platform), the
// pipeline-to-component references (declared in the same file only) and the
// exporter endpoints as route-host references, against a golden.
func TestOtelCollectorDialectGoldens(t *testing.T) {
	x, err := extractor.NewDocDialectExtractor("otel-collector")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join("..", "extractor", "dialects", "otel-collector", "testdata")
	fixtures, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil || len(fixtures) == 0 {
		t.Fatalf("no fixtures in %s (%v)", dir, err)
	}
	sort.Strings(fixtures)
	for _, fx := range fixtures {
		t.Run(filepath.Base(fx), func(t *testing.T) {
			src, err := os.ReadFile(fx)
			if err != nil {
				t.Fatal(err)
			}
			rel := filepath.Base(fx)
			res, err := x.ExtractDoc("svc", rel, src)
			if err != nil {
				t.Fatal(err)
			}
			got := otelGolden{Detected: x.DetectsContent(src), Symbols: []otelGoldenSymbol{}, CrossFile: []otelGoldenCross{}}
			for _, n := range res.Nodes {
				got.Symbols = append(got.Symbols, otelGoldenSymbol{Name: n.Name, Kind: string(n.Kind), Line: n.Line, EndLine: n.EndLine, Signature: n.Signature, Calls: n.Calls})
			}
			for _, c := range res.CrossFile {
				got.CrossFile = append(got.CrossFile, otelGoldenCross{From: c.From, Edge: c.Edge, Raw: c.Raw, Strategy: c.Strategy, Line: c.Line})
			}
			gp := fx + ".golden.json"
			if *updateOtelGoldens {
				b, _ := json.MarshalIndent(got, "", "  ")
				if err := os.WriteFile(gp, append(b, '\n'), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			raw, err := os.ReadFile(gp)
			if err != nil {
				t.Fatal(err)
			}
			var want otelGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s: %v", gp, err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s\n got: %+v\nwant: %+v", fx, got, want)
			}
		})
	}
}
