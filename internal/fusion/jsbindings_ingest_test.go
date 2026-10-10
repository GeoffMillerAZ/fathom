package fusion

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestIngest_JSModuleBindings (wave lf5, item tsjs, O749; the product reach of
// TestModuleLevelBindings): `fathom ingest` stores a JavaScript file's
// module-level let and var bindings and its CommonJS value exports as JSConst,
// their initializer's calls their own, and none of a function's locals; and a
// node Lambda handler exported as a value a wrapper call returns
// (`exports.handler = middy(base)`, no function literal written there) binds
// its template, through the export's JSConst (bindHandler, LB-F384).
func TestIngest_JSModuleBindings(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"sam/template.yaml": "Transform: AWS::Serverless-2016-10-31\nResources:\n" +
			"  Api:\n    Type: AWS::Serverless::Function\n    Properties:\n      Runtime: nodejs20.x\n      CodeUri: app/\n      Handler: app.handler\n",
		"sam/app/app.js": "var middy = require(\"middy\");\n" +
			"var express = require(\"express\");\n" +
			"let count = 0;\n" +
			"var server = express();\n" +
			"function base(event) {\n  var local = event.body;\n  let seen = count;\n  return local + seen;\n}\n" +
			"exports.handler = middy(base);\n" +
			"exports.version = \"1.0.0\";\n" +
			"module.exports.router = express.Router();\n",
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "jsb", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "jsb", "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	calls := map[string][]string{}
	for _, o := range obs {
		if o.SourceRef != "sam/app/app.js" {
			continue
		}
		got[o.Symbol] = string(o.SourceKind)
		if len(o.Calls) > 0 {
			calls[o.Symbol] = o.Calls
		}
	}
	want := map[string]string{
		"<module>": "JSModule", "middy": "JSConst", "express": "JSConst", "count": "JSConst", "server": "JSConst",
		"base": "JSFunc", "handler": "JSConst", "version": "JSConst", "router": "JSConst",
	}
	if !reflect.DeepEqual(got, want) {
		var names []string
		for n := range got {
			names = append(names, n)
		}
		sort.Strings(names)
		t.Errorf("stored definitions of app.js = %v (%v)\nwant %v", got, names, want)
	}
	if c := calls["server"]; len(c) != 1 || c[0] != "express" {
		t.Errorf("server's calls = %q, want its initializer's [express]", c)
	}
	if c := calls["handler"]; len(c) != 1 || c[0] != "middy" {
		t.Errorf("handler's calls = %q, want its value's [middy]", c)
	}
	if got, want := docXrefEdgeBrief(t, st, "jsb"), []string{"jsb::sam/template.Api -> jsb::sam/app/app.handler"}; !reflect.DeepEqual(got, want) {
		t.Errorf("stored handler edges\n got: %q\nwant: %q", got, want)
	}
}
