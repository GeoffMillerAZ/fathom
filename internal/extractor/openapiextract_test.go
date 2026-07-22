package extractor_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

const openAPIFixture = `openapi: 3.0.0
info:
  title: Widgets API
  version: 1.0.0
paths:
  /widgets:
    get:
      operationId: listWidgets
      summary: List widgets
    post:
      summary: Create a widget
  /widgets/{id}:
    get:
      operationId: getWidget
components:
  schemas:
    Widget:
      type: object
    WidgetList:
      type: array
`

func TestOpenAPIExtractor_Operations(t *testing.T) {
	x := extractor.NewOpenAPIExtractor()
	nodes, err := x.Extract("svc", "api/widgets.yaml", []byte(openAPIFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := openAPIByName(nodes)

	get, ok := byName["GET /widgets"]
	if !ok {
		t.Fatalf("expected operation 'GET /widgets'; got %v", openAPINodeNames(nodes))
	}
	if get.Kind != core.KindOpenAPIOperation {
		t.Errorf("Kind: got %q, want KindOpenAPIOperation", get.Kind)
	}
	if get.Signature != "listWidgets" {
		t.Errorf("Signature: got %q, want operationId 'listWidgets'", get.Signature)
	}

	post, ok := byName["POST /widgets"]
	if !ok {
		t.Fatal("expected operation 'POST /widgets'")
	}
	if post.Signature != "Create a widget" {
		t.Errorf("Signature: got %q, want summary fallback", post.Signature)
	}

	if _, ok := byName["GET /widgets/{id}"]; !ok {
		t.Error("expected operation 'GET /widgets/{id}'")
	}
}

func TestOpenAPIExtractor_Schemas(t *testing.T) {
	x := extractor.NewOpenAPIExtractor()
	nodes, err := x.Extract("svc", "api/widgets.yaml", []byte(openAPIFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	byName := openAPIByName(nodes)
	for _, want := range []string{"Widget", "WidgetList"} {
		n, ok := byName[want]
		if !ok {
			t.Errorf("expected schema %q; got %v", want, openAPINodeNames(nodes))
			continue
		}
		if n.Kind != core.KindOpenAPISchema {
			t.Errorf("schema %q Kind: got %q, want KindOpenAPISchema", want, n.Kind)
		}
	}
}

func TestOpenAPIExtractor_Swagger2Definitions(t *testing.T) {
	src := `swagger: "2.0"
paths:
  /pets:
    get:
      operationId: listPets
definitions:
  Pet:
    type: object
`
	x := extractor.NewOpenAPIExtractor()
	nodes, err := x.Extract("svc", "swagger.yaml", []byte(src))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	byName := openAPIByName(nodes)
	if _, ok := byName["GET /pets"]; !ok {
		t.Error("expected operation 'GET /pets'")
	}
	if n, ok := byName["Pet"]; !ok {
		t.Error("expected definitions-derived schema 'Pet'")
	} else if n.Kind != core.KindOpenAPISchema {
		t.Errorf("Pet Kind: got %q, want KindOpenAPISchema", n.Kind)
	}
}

func TestOpenAPIExtractor_NotAnOpenAPIDoc(t *testing.T) {
	// A document with no paths/components/definitions produces zero nodes —
	// this extractor is only ever reached via the classifier's openapi:/
	// swagger: signal, so it need not defend against arbitrary YAML, but it
	// must not panic on it.
	x := extractor.NewOpenAPIExtractor()
	nodes, err := x.Extract("svc", "misc.yaml", []byte("openapi: 3.0.0\n"))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

func TestOpenAPIExtractor_Determinism(t *testing.T) {
	x := extractor.NewOpenAPIExtractor()
	a, _ := x.Extract("svc", "api/widgets.yaml", []byte(openAPIFixture))
	b, _ := x.Extract("svc", "api/widgets.yaml", []byte(openAPIFixture))
	if !reflect.DeepEqual(a, b) {
		t.Error("Extract is not deterministic")
	}
}

func openAPIByName(nodes []core.SymbolNode) map[string]core.SymbolNode {
	m := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		m[n.Name] = n
	}
	return m
}

func openAPINodeNames(nodes []core.SymbolNode) []string {
	names := make([]string, len(nodes))
	for i, n := range nodes {
		names[i] = string(n.Kind) + ":" + n.Name
	}
	return names
}
