package fusion

// jsonschemadispatch_test.go — LB-Q-json-schema: a JSON Schema document is
// claimed by a root `$schema` naming a json-schema.org meta-schema or by the
// *.schema.json name, in ingest and in ReingestFile alike; a file that only
// points at a schema (an instance), a nested `$schema`, a basename's claim and
// an OpenAPI document stay what they were. A `$ref` with a file part is a
// doc-xref edge between two schema documents.

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
	jsOrder = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "properties": {"total": {"$ref": "common.json#/$defs/Money"}, "line": {"$ref": "#/$defs/Line"}},
  "$defs": {"Line": {"type": "object"}}
}
`
	jsCommon = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "$defs": {"Money": {"type": "object", "properties": {"cur": {"$ref": "#/$defs/Currency"}}}, "Currency": {"type": "string"}}
}
`
	jsByName = "{\"title\": \"No dialect\", \"properties\": {\"a\": {\"type\": \"string\"}}}\n"
	// An instance points at a schema; it is no schema.
	jsInstance = "{\"$schema\": \"https://json.schemastore.org/tsconfig.json\", \"compilerOptions\": {}}\n"
	// A nested $schema (a schema embedded in a document) claims nothing.
	jsNested = "{\"name\": \"x\", \"schema\": {\"$schema\": \"https://json-schema.org/draft/2020-12/schema\", \"type\": \"object\"}}\n"
)

func jsonSchemaRepoCorpus() map[string]string {
	return map[string]string{
		"schemas/order.json":        jsOrder,
		"schemas/common.json":       jsCommon,
		"schemas/plain.schema.json": jsByName,
		"tsconfig.json":             jsInstance,
		"conf/embedded.json":        jsNested,
		"package.json":              "{\"$schema\": \"https://json-schema.org/draft/2020-12/schema\", \"name\": \"example\"}\n",
		"api/openapi.json":          "{\"openapi\": \"3.1.0\", \"info\": {\"title\": \"t\", \"version\": \"1\"}, \"$schema\": \"https://json-schema.org/draft/2020-12/schema\", \"paths\": {}}\n",
		"conf/settings.json":        "{\"name\": \"example\"}\n",
	}
}

func TestJSONSchemaDispatchClaims(t *testing.T) {
	cases := []struct {
		path, content string
		want          ToolKind // "" = not claimed
	}{
		{"schemas/order.json", jsOrder, ToolJSONSchema},
		{"schemas/common.json", jsCommon, ToolJSONSchema},
		{"schemas/plain.schema.json", jsByName, ToolJSONSchema},
		{"schemas/PLAIN.SCHEMA.JSON", jsByName, ""}, // the extension is .JSON, not .json
		{"schemas/plain.json", jsByName, ""},        // no $schema, no .schema.json name
		{"tsconfig.json", jsInstance, ""},
		{"conf/embedded.json", jsNested, ""},
		{"schemas/array.schema.json", "[1, 2]\n", ""},
		{"schemas/broken.schema.json", "{\"$schema\": \"https://json-schema.org/draft/2020-12/schema\", \"$defs\": {\n", ""},
		{"schemas/order.yaml", jsOrder, ""}, // a JSON document
	}
	for _, c := range cases {
		_, got, ok := jsonSchemaDispatch(c.path, []byte(c.content))
		if !ok {
			got = ""
		}
		if got != c.want {
			t.Errorf("%s: claimed as %q, want %q", c.path, got, c.want)
		}
		wantCD := ""
		if c.want == ToolJSONSchema {
			wantCD = "json-schema"
		}
		if cd := JSONSchemaContentDialect(c.path, []byte(c.content)); cd != wantCD {
			t.Errorf("%s: JSONSchemaContentDialect = %q, want %q", c.path, cd, wantCD)
		}
	}
}

// TestIngest_JSONSchemaRepo: ingest reads every schema through the dialect,
// keeps a basename's claim (package.json), an OpenAPI document, an instance
// and generic JSON where they were, and stores the cross-file edge.
func TestIngest_JSONSchemaRepo(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, jsonSchemaRepoCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "js", Path: dir}); err != nil {
		t.Fatal(err)
	}
	got := jsonSchemaObservations(t, st, "js")
	want := map[string][]string{
		"schemas/order.json|JSONSchemaDocument|order.json":               nil,
		"schemas/order.json|JSONSchemaProperty|total":                    nil,
		"schemas/order.json|JSONSchemaProperty|line":                     {"Line"},
		"schemas/order.json|JSONSchemaDef|Line":                          nil,
		"schemas/common.json|JSONSchemaDocument|common.json":             nil,
		"schemas/common.json|JSONSchemaDef|Money":                        {"Currency"},
		"schemas/common.json|JSONSchemaDef|Currency":                     nil,
		"schemas/plain.schema.json|JSONSchemaDocument|plain.schema.json": nil,
		"schemas/plain.schema.json|JSONSchemaProperty|a":                 nil,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("json-schema observations\n got: %v\nwant: %v", got, want)
	}
	obs, _ := st.QueryObservations(ctx, "js", "")
	for _, o := range obs {
		switch o.SourceRef {
		case "tsconfig.json", "conf/embedded.json", "conf/settings.json":
			if o.SourceKind != core.KindJSONKey {
				t.Errorf("%s read as %s; it keeps the JSON extractor", o.SourceRef, o.SourceKind)
			}
		case "api/openapi.json":
			if o.SourceKind == "JSONSchemaDocument" {
				t.Errorf("%s read as a JSON Schema; an OpenAPI document keeps its dialect", o.SourceRef)
			}
		case "package.json":
			if o.SourceKind == "JSONSchemaDocument" {
				t.Errorf("%s read as a JSON Schema; a basename's claim stands", o.SourceRef)
			}
		}
	}
	all, _ := st.TypedEdges(ctx, "js")
	var edges []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved {
			edges = append(edges, e.FromFile+"|"+e.FromSymbol+"->"+e.ToSymbol)
		}
	}
	sort.Strings(edges)
	wantEdges := []string{"schemas/order.json|js::schemas/order.order.json->js::schemas/common.common.json"}
	if !reflect.DeepEqual(edges, wantEdges) {
		t.Errorf("cross-file edges\n got: %q\nwant: %q", edges, wantEdges)
	}
}

// TestReingestFile_JSONSchemaRepo: a one-file refresh reads a schema as ingest
// does, not as the generic JSON extractor its extension names.
func TestReingestFile_JSONSchemaRepo(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, jsonSchemaRepoCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "js", Path: dir}); err != nil {
		t.Fatal(err)
	}
	body := "{\"$schema\": \"https://json-schema.org/draft/2020-12/schema\", \"$defs\": {\"Money\": {\"type\": \"object\"}, \"Tax\": {\"type\": \"number\"}}}\n"
	if err := os.WriteFile(filepath.Join(dir, "schemas", "common.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ReingestFile(ctx, st, "js", dir, "schemas/common.json"); err != nil {
		t.Fatal(err)
	}
	got := jsonSchemaObservations(t, st, "js")
	if _, ok := got["schemas/common.json|JSONSchemaDef|Tax"]; !ok {
		t.Errorf("after ReingestFile schemas/common.json|JSONSchemaDef|Tax is missing; have %v", got)
	}
}

// jsonSchemaObservations maps "file|SourceKind|symbol" to its calls for every
// JSON Schema observation.
func jsonSchemaObservations(t *testing.T, st store.Store, svc string) map[string][]string {
	t.Helper()
	obs, err := st.QueryObservations(context.Background(), svc, "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, o := range obs {
		switch o.SourceKind {
		case "JSONSchemaDocument", "JSONSchemaDef", "JSONSchemaProperty":
			got[o.SourceRef+"|"+string(o.SourceKind)+"|"+o.Symbol] = o.Calls
		}
	}
	return got
}
