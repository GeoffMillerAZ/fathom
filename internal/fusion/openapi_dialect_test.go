package fusion

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
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
		{"schema/openapi.schema.json", "{\"properties\": {\"openapi\": {\"type\": \"string\"}}}\n", p.exts[".json"], "", ""},
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
