package fusion

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const (
	oaYAMLDoc  = "openapi: 3.0.0\ninfo:\n  title: x\npaths: {}\n"
	oaSwagger  = "swagger: '2.0'\npaths: {}\n"
	oaJSONDoc  = "{\n  \"openapi\": \"3.1.0\",\n  \"info\": {\"title\": \"x\"},\n  \"paths\": {}\n}\n"
	oaMinified = "{\"swagger\":\"2.0\",\"info\":{},\"paths\":{}}"
)

// TestOpenAPIRoutingByContent (LB-Q-openapi): an OpenAPI or Swagger document
// is routed by its content — YAML through classifyYAML's top-level
// openapi:/swagger: signal (unchanged), JSON through resolveExtractor's sniff
// and decode — to the one OpenAPI extractor, and every other file keeps the
// extractor and tool kind it had: a JSON document that only has a nested
// "openapi" key (a schema, an example, a package manifest), CloudFormation as
// JSON, a Kubernetes manifest, generic JSON. Pipeline.ContentDialect, the
// language-breadth harness's ownership, reports "openapi" for exactly the
// files ingest routes there.
func TestOpenAPIRoutingByContent(t *testing.T) {
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	idx := buildSiblingIndex(nil)
	oa := p.toolExts[ToolOpenAPI]
	if oa == nil {
		t.Fatal("no openapi extractor registered")
	}
	for _, c := range []struct {
		rel, src string
		want     any
		tool     ToolKind
		content  string
	}{
		{"api/openapi.yaml", oaYAMLDoc, oa, ToolOpenAPI, "openapi"},
		{"api/legacy.yml", oaSwagger, oa, ToolOpenAPI, "openapi"},
		{"api/openapi.json", oaJSONDoc, oa, ToolOpenAPI, "openapi"},
		{"api/swagger.json", oaMinified, oa, ToolOpenAPI, "openapi"},
		{"api/example.json", "{\"title\": \"x\", \"example\": {\"openapi\": \"3.0.0\"}}\n", p.exts[".json"], "", ""},
		{"schema/openapi-schema.json", "{\"properties\": {\"openapi\": {\"type\": \"string\"}}}\n", p.exts[".json"], "", ""},
		{"data/data.json", "{\"a\": 1}\n", p.exts[".json"], "", ""},
		{"package.json", "{\"name\": \"x\", \"openapi\": \"3.0.0\", \"info\": {}}\n", p.exts["package.json"], "", ""},
		{"api/no-info.json", "{\"openapi\": \"3.0.0\", \"paths\": {}}\n", p.exts[".json"], "", ""},
		{"infra/stack.json", cfnJSONTemplate, p.toolExts[ToolCloudFormation], ToolCloudFormation, "cloudformation"},
		{"infra/stack.yaml", cfnYAMLTemplate, p.toolExts[ToolCloudFormation], ToolCloudFormation, "cloudformation"},
		{"k8s/deploy.yaml", "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n", p.toolExts[ToolK8sManifest], ToolK8sManifest, ""},
		{"stack/app.yml", "services:\n  web:\n    image: nginx\n", p.toolExts[ToolCompose], ToolCompose, ""},
	} {
		e, tool, ok, unclassified := p.resolveExtractor(c.rel, []byte(c.src), idx)
		if !ok || unclassified {
			t.Errorf("%s: ok=%v unclassified=%v; want an extractor", c.rel, ok, unclassified)
			continue
		}
		if e != c.want || tool != c.tool {
			t.Errorf("%s: dispatches to %T (%q), want %T (%q)", c.rel, e, tool, c.want, c.tool)
		}
		if got := p.ContentDialect(c.rel, []byte(c.src), nil); got != c.content {
			t.Errorf("%s: ContentDialect = %q, want %q", c.rel, got, c.content)
		}
	}
	if got := p.ContentDialect("conf/app.yaml", []byte("name: x\nsettings:\n  a: 1\n"), nil); got != "" {
		t.Errorf("unclassified YAML: ContentDialect = %q, want none", got)
	}
}

func openAPIRepoCorpus() map[string]string {
	return map[string]string{
		"api/main.yaml": "openapi: 3.0.3\ninfo:\n  title: Orders\n  version: '1'\npaths:\n  /orders:\n    get:\n      operationId: listOrders\n      parameters:\n        - $ref: '#/components/parameters/Limit'\n      responses:\n        '200':\n          content:\n            application/json:\n              schema:\n                $ref: '#/components/schemas/Order'\n" +
			"components:\n  parameters:\n    Limit:\n      name: limit\n      in: query\n  schemas:\n    Order:\n      properties:\n        total:\n          $ref: './common/types.yaml#/components/schemas/Money'\n        tag:\n          $ref: 'fragment/tag.yaml'\n        note:\n          $ref: 'https://example.com/note.yaml'\n",
		"api/common/types.yaml": "openapi: 3.0.3\ninfo:\n  title: Types\n  version: '1'\ncomponents:\n  schemas:\n    Money:\n      type: object\n",
		"api/legacy.json":       "{\n  \"swagger\": \"2.0\",\n  \"info\": {\"title\": \"Legacy\", \"version\": \"1\"},\n  \"paths\": {\"/old\": {\"get\": {\"responses\": {\"200\": {\"schema\": {\"$ref\": \"#/definitions/Old\"}}}}}},\n  \"definitions\": {\"Old\": {\"properties\": {\"price\": {\"$ref\": \"common/types.yaml#/components/schemas/Money\"}}}}\n}\n",
		"api/fragment/tag.yaml": "type: string\n",
		"redocly.yaml":          "apis:\n  main:\n    root: api/main.yaml\nopenapi:\n  codeSamples:\n    languages: []\n",
		"conf/settings.json":    "{\"example\": {\"openapi\": \"3.0.0\", \"info\": {}}}\n",
	}
}

// TestIngest_OpenAPIRepo (the product-reach proof): an ingest reads every
// OpenAPI and Swagger document, YAML and JSON, through the openapi dialect —
// the document, its operations, schemas and parameters, the internal $refs
// between them on Calls — stores a $ref whose file part names another
// document of the repository as a file-tier edge to that document's root, and
// leaves a $ref to a fragment (no openapi: key, so not a document), a URL, a
// tool's configuration with an openapi: key and generic JSON alone.
func TestIngest_OpenAPIRepo(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, openAPIRepoCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "oa", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "oa", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	sawJSONKey := false
	for _, o := range obs {
		switch o.SourceKind {
		case core.KindOpenAPIOperation, core.KindOpenAPISchema, "OpenAPIDocument", "OpenAPIParameter":
			got[o.SourceRef+"|"+string(o.SourceKind)+"|"+o.Symbol] = o.Calls
		}
		if o.SourceRef == "redocly.yaml" && (o.SourceKind == core.KindOpenAPIOperation || o.SourceKind == core.KindOpenAPISchema || o.SourceKind == "OpenAPIDocument") {
			t.Errorf("redocly.yaml (a tool config with an openapi: key) read as %s", o.SourceKind)
		}
		if o.SourceRef == "conf/settings.json" {
			if o.SourceKind != core.KindJSONKey {
				t.Errorf("conf/settings.json read as %s; generic JSON keeps the JSON extractor", o.SourceKind)
			}
			sawJSONKey = true
		}
	}
	want := map[string][]string{
		"api/main.yaml|OpenAPIDocument|main.yaml":          nil,
		"api/main.yaml|OpenAPIOperation|GET /orders":       {"Limit", "Order"},
		"api/main.yaml|OpenAPIParameter|Limit":             nil,
		"api/main.yaml|OpenAPISchema|Order":                nil,
		"api/common/types.yaml|OpenAPIDocument|types.yaml": nil,
		"api/common/types.yaml|OpenAPISchema|Money":        nil,
		"api/legacy.json|OpenAPIDocument|legacy.json":      nil,
		"api/legacy.json|OpenAPIOperation|GET /old":        {"Old"},
		"api/legacy.json|OpenAPISchema|Old":                nil,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("observations\n got: %v\nwant: %v", got, want)
	}
	if !sawJSONKey {
		t.Error("conf/settings.json yielded nothing; generic JSON keeps the JSON extractor")
	}
	all, _ := st.TypedEdges(ctx, "oa")
	var edges []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			edges = append(edges, e.FromFile+"->"+e.ToSymbol)
		}
	}
	sort.Strings(edges)
	wantEdges := []string{"api/legacy.json->oa::api/common/types.types.yaml", "api/main.yaml->oa::api/common/types.types.yaml"}
	if !reflect.DeepEqual(edges, wantEdges) {
		t.Fatalf("stored doc-xref edges\n got: %q\nwant: %q", edges, wantEdges)
	}
	if rep.DocXrefEdges != 2 {
		t.Errorf("DocXrefEdges = %d, want 2", rep.DocXrefEdges)
	}
}

// TestReingestFile_OpenAPIRepo: the one-file refresh reads a YAML and a JSON
// document through the dialect, as an ingest does (their content decides).
func TestReingestFile_OpenAPIRepo(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, openAPIRepoCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "oa", Path: dir}); err != nil {
		t.Fatal(err)
	}
	edits := map[string]string{
		"api/common/types.yaml": "openapi: 3.0.3\ninfo:\n  title: Types\n  version: '1'\ncomponents:\n  schemas:\n    Money:\n      type: object\n    Rate:\n      type: number\n",
		"api/legacy.json":       "{\"swagger\": \"2.0\", \"info\": {}, \"paths\": {\"/old\": {\"get\": {}, \"post\": {}}}}\n",
	}
	for rel, body := range edits {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := p.ReingestFile(ctx, st, "oa", dir, rel); err != nil {
			t.Fatal(err)
		}
	}
	obs, err := st.QueryObservations(ctx, "oa", "")
	if err != nil {
		t.Fatal(err)
	}
	res := map[string][]string{}
	for _, o := range obs {
		if o.SourceKind == core.KindOpenAPIOperation || o.SourceKind == core.KindOpenAPISchema {
			res[o.SourceRef] = append(res[o.SourceRef], o.Symbol)
		}
	}
	for rel := range res {
		sort.Strings(res[rel])
	}
	want := map[string][]string{
		"api/main.yaml":         {"GET /orders", "Order"},
		"api/common/types.yaml": {"Money", "Rate"},
		"api/legacy.json":       {"GET /old", "POST /old"},
	}
	if !reflect.DeepEqual(res, want) {
		t.Fatalf("symbols after ReingestFile\n got: %v\nwant: %v", res, want)
	}
}
