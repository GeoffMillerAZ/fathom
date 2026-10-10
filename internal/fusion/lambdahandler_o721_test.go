package fusion

import (
	"context"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestIngest_LambdaHandlerWrappedExports (O721, LB-F384): a node handler
// exported through CommonJS member assignment of a function
// (`module.exports.x = async () => {}`, `= async function () {}`), through a
// wrapper call (`exports.handler = metricScope(m => async (e) => …)`), or as
// a TypeScript const a wrapper makes (`export const handler =
// withDurableExecution(…)`, a module-level constant, LB-F382) binds its
// function's template, as a plain exported function does.
func TestIngest_LambdaHandlerWrappedExports(t *testing.T) {
	fn := func(name, uri, handler string) string {
		return "  " + name + ":\n    Type: AWS::Serverless::Function\n    Properties:\n      Runtime: nodejs20.x\n      CodeUri: " + uri + "\n      Handler: " + handler + "\n"
	}
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"sam/template.yaml": "Transform: AWS::Serverless-2016-10-31\nResources:\n" +
			fn("Arrow", "speech/", "speech.handler") +
			fn("FnExpr", "speech/", "speech.lambdaHandler") +
			fn("Wrapped", "obs/", "app.handler") +
			fn("Durable", "durable/", "index.handler"),
		"sam/speech/speech.js":   "module.exports.handler = async (event) => {\n  return transcribe(event);\n};\nmodule.exports.lambdaHandler = async function (event) {\n  return 1;\n};\n",
		"sam/obs/app.js":         "const { metricScope } = require(\"aws-embedded-metrics\");\nexports.handler = metricScope((metrics) => async (event) => {\n  return 1;\n});\n",
		"sam/durable/index.ts":   "export const handler = withDurableExecution(async (event: unknown) => {\n  return 1;\n});\n",
		"sam/durable/helpers.ts": "export function step(): number {\n  return 1;\n}\n",
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: "lh", Path: dir}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"lh::sam/template.Arrow -> lh::sam/speech/speech.handler",
		"lh::sam/template.Durable -> lh::sam/durable/index.handler",
		"lh::sam/template.FnExpr -> lh::sam/speech/speech.lambdaHandler",
		"lh::sam/template.Wrapped -> lh::sam/obs/app.handler",
	}
	if got := docXrefEdgeBrief(t, st, "lh"); !reflect.DeepEqual(got, want) {
		t.Fatalf("stored handler edges\n got: %q\nwant: %q", got, want)
	}
}
