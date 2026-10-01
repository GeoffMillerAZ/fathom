package extractor

// openapidialect_test.go — goldens for the openapi doc-engine dialect
// (LB-Q-openapi, a port of openapiextract.go, design §13.2 steps 10/12/14/15):
// operations, schemas, parameters, responses, request bodies, headers and
// security schemes of an OpenAPI 3.x or Swagger 2.0 document (YAML and JSON),
// the internal `$ref`s one makes to another (Calls), and a `$ref` with a file
// part as a cross-file reference. The same golden shape as the cloudformation
// dialect's. A fixture is read as if it sat in api/<name>, so a relative
// `$ref` resolves against a directory.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

type openAPIGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Package   string   `json:"package"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
	Tier      string   `json:"tier"`
}

type openAPIGoldenCross struct {
	From     string `json:"from"`
	Edge     string `json:"edge"`
	Raw      string `json:"raw"`
	Target   string `json:"target"`
	Strategy string `json:"strategy"`
	Line     int    `json:"line"`
}

type openAPIGolden struct {
	Symbols   []openAPIGoldenSymbol `json:"symbols"`
	CrossFile []openAPIGoldenCross  `json:"cross_file"`
}

func loadOpenAPIExtractor(t *testing.T) *DocExtractor {
	t.Helper()
	x, err := NewDocDialectExtractor("openapi")
	if err != nil {
		t.Fatalf("NewDocDialectExtractor(openapi): %v", err)
	}
	return x
}

func openAPIFixtures(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("dialects", "openapi", "testdata")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() || strings.HasSuffix(e.Name(), ".golden.json") {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	sort.Strings(out)
	if len(out) == 0 {
		t.Fatalf("%s: no fixtures", dir)
	}
	return out
}

func openAPIRead(t *testing.T, x *DocExtractor, fx string) openAPIGolden {
	t.Helper()
	rel := "api/" + filepath.Base(fx)
	src, err := os.ReadFile(fx)
	if err != nil {
		t.Fatal(err)
	}
	res, err := x.ExtractDoc("svc", rel, src)
	if err != nil {
		t.Fatalf("ExtractDoc: %v", err)
	}
	got := openAPIGolden{Symbols: []openAPIGoldenSymbol{}, CrossFile: []openAPIGoldenCross{}}
	for _, n := range res.Nodes {
		if n.Service != "svc" || n.FilePath != rel {
			t.Fatalf("%s: plumbing changed: service %q file %q", fx, n.Service, n.FilePath)
		}
		got.Symbols = append(got.Symbols, openAPIGoldenSymbol{
			Name: n.Name, Kind: string(n.Kind), Package: n.Package,
			Line: n.Line, EndLine: n.EndLine, Signature: n.Signature,
			Calls: n.Calls, Tier: tierName(n.Tier),
		})
	}
	for _, c := range res.CrossFile {
		got.CrossFile = append(got.CrossFile, openAPIGoldenCross{
			From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line,
		})
	}
	return got
}

// TestOpenAPIDialectGoldens: every fixture's entities, internal references
// and cross-file references against a hand-checked golden: a 3.0 YAML
// document with every component section and each `$ref` shape (internal,
// into a schema's property, an escaped name, a missing target, a file with
// and without a fragment, a URL), a 2.0 YAML one, a 3.1 JSON one with a
// webhook, a document that does not parse and a file that is not one.
func TestOpenAPIDialectGoldens(t *testing.T) {
	x := loadOpenAPIExtractor(t)
	for _, fx := range openAPIFixtures(t) {
		t.Run(filepath.Base(fx), func(t *testing.T) {
			got := openAPIRead(t, x, fx)
			gp := fx + ".golden.json"
			raw, err := os.ReadFile(gp)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want openAPIGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s: %v", gp, err)
			}
			if want.Symbols == nil {
				want.Symbols = []openAPIGoldenSymbol{}
			}
			if want.CrossFile == nil {
				want.CrossFile = []openAPIGoldenCross{}
			}
			if !reflect.DeepEqual(got.Symbols, want.Symbols) {
				t.Errorf("%s: symbols\n got: %+v\nwant: %+v", fx, got.Symbols, want.Symbols)
			}
			if !reflect.DeepEqual(got.CrossFile, want.CrossFile) {
				t.Errorf("%s: cross_file\n got: %+v\nwant: %+v", fx, got.CrossFile, want.CrossFile)
			}
		})
	}
}

// TestOpenAPIDetectsByContent: the dialect claims no file by name; its
// detect.content_sniff says a .json is one (a top-level "openapi" or
// "swagger" key), and a YAML file is claimed by fusion's classifier on the
// same top-level key.
func TestOpenAPIDetectsByContent(t *testing.T) {
	x := loadOpenAPIExtractor(t)
	for name, c := range map[string]struct {
		src  string
		want bool
	}{
		"yaml openapi":     {"openapi: 3.0.0\npaths: {}\n", true},
		"yaml swagger":     {"swagger: \"2.0\"\n", true},
		"yaml after info":  {"# api\ninfo:\n  title: x\nopenapi: '3.1.0'\n", true},
		"json openapi":     {"{\n  \"openapi\": \"3.0.3\",\n  \"paths\": {}\n}\n", true},
		"minified swagger": {"{\"swagger\":\"2.0\",\"paths\":{}}", true},
		"json tab":         {"{\n\t\"openapi\": \"3.0.0\"\n}", true},
		"kubernetes":       {"apiVersion: v1\nkind: ConfigMap\n", false},
		"cloudformation":   {"AWSTemplateFormatVersion: '2010-09-09'\nResources: {}\n", false},
		"generic json":     {"{\"name\": \"x\", \"version\": \"1\"}\n", false},
		"json with number": {"{\"openapi\": 3}", false},
		"indented yaml":    {"info:\n  swagger: x\n", false},
	} {
		if got := x.DetectsContent([]byte(c.src)); got != c.want {
			t.Errorf("%s: DetectsContent = %v, want %v", name, got, c.want)
		}
	}
}

const openAPIIncumbentFixture = `openapi: 3.0.0
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

func openAPIByName(nodes []core.SymbolNode) map[string]core.SymbolNode {
	m := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		m[n.Name] = n
	}
	return m
}

// TestOpenAPIDialectKeepsIncumbentContracts: each case of the retired
// openapiextract_test.go (operations named "METHOD /path" and signed by their
// operationId, else their summary; schemas from components.schemas and from
// Swagger 2.0's definitions; an empty document; determinism), run against the
// dialect. Every SourceKind the incumbent emitted is kept (OpenAPIOperation,
// OpenAPISchema) with its names, tier and count. The one addition is a
// document entity (OpenAPIDocument, named by the file): an empty document now
// yields it and nothing else, where the incumbent yielded nothing.
func TestOpenAPIDialectKeepsIncumbentContracts(t *testing.T) {
	x := loadOpenAPIExtractor(t)
	nodes, err := x.Extract("svc", "api/widgets.yaml", []byte(openAPIIncumbentFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	byName := openAPIByName(nodes)
	get, ok := byName["GET /widgets"]
	if !ok {
		t.Fatalf("expected operation 'GET /widgets'; got %v", brief(nodes))
	}
	if get.Kind != core.KindOpenAPIOperation || get.Signature != "listWidgets" || get.Tier != core.TierVerified {
		t.Errorf("GET /widgets: kind %q signature %q tier %q, want OpenAPIOperation, the operationId, verified", get.Kind, get.Signature, get.Tier)
	}
	if post, ok := byName["POST /widgets"]; !ok || post.Signature != "Create a widget" {
		t.Errorf("POST /widgets: %+v, want the summary as its signature", post)
	}
	if _, ok := byName["GET /widgets/{id}"]; !ok {
		t.Error("expected operation 'GET /widgets/{id}'")
	}
	for _, want := range []string{"Widget", "WidgetList"} {
		if n, ok := byName[want]; !ok || n.Kind != core.KindOpenAPISchema {
			t.Errorf("schema %q: %+v, want OpenAPISchema", want, n)
		}
	}
	ops, schemas := 0, 0
	for _, n := range nodes {
		switch n.Kind {
		case core.KindOpenAPIOperation:
			ops++
		case core.KindOpenAPISchema:
			schemas++
		}
	}
	if ops != 3 || schemas != 2 || len(nodes) != 6 {
		t.Errorf("counts: %d operations, %d schemas, %d symbols; want 3, 2 and 6 (the document entity)", ops, schemas, len(nodes))
	}

	swagger := `swagger: "2.0"
paths:
  /pets:
    get:
      operationId: listPets
definitions:
  Pet:
    type: object
`
	nodes, err = x.Extract("svc", "swagger.yaml", []byte(swagger))
	if err != nil {
		t.Fatal(err)
	}
	byName = openAPIByName(nodes)
	if _, ok := byName["GET /pets"]; !ok {
		t.Error("expected operation 'GET /pets'")
	}
	if n, ok := byName["Pet"]; !ok || n.Kind != core.KindOpenAPISchema {
		t.Errorf("Pet: %+v, want the definitions-derived OpenAPISchema", n)
	}

	nodes, err = x.Extract("svc", "misc.yaml", []byte("openapi: 3.0.0\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range nodes {
		if n.Kind == core.KindOpenAPIOperation || n.Kind == core.KindOpenAPISchema {
			t.Errorf("an empty document yielded %v", brief(nodes))
		}
	}

	a, _ := x.Extract("svc", "api/widgets.yaml", []byte(openAPIIncumbentFixture))
	b, _ := x.Extract("svc", "api/widgets.yaml", []byte(openAPIIncumbentFixture))
	if !reflect.DeepEqual(a, b) {
		t.Error("Extract is not deterministic")
	}
}
