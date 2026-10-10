package fusion

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// cloudwatchCorpus is a SAM application and an operations template beside
// it: an alarm on a metric a metric filter declares, in the filter's
// template and in the other one; metric filters on a function's log group
// named /aws/lambda/<FunctionName>, in the function's template and the other.
func cloudwatchCorpus() map[string]string {
	return map[string]string{
		"infra/app.yaml": "Transform: AWS::Serverless-2016-10-31\nResources:\n" +
			"  Orders:\n    Type: AWS::Serverless::Function\n    Properties:\n      FunctionName: orders-api\n      Runtime: python3.12\n      CodeUri: functions/\n      Handler: orders.app.handler\n" +
			"  Topic:\n    Type: AWS::SNS::Topic\n" +
			"  ErrorFilter:\n    Type: AWS::Logs::MetricFilter\n    Properties:\n      LogGroupName: /aws/lambda/orders-api\n      FilterPattern: ERROR\n      MetricTransformations:\n        - MetricNamespace: Shop\n          MetricName: Errors\n          MetricValue: \"1\"\n" +
			"  ErrorAlarm:\n    Type: AWS::CloudWatch::Alarm\n    Properties:\n      Namespace: Shop\n      MetricName: Errors\n      Dimensions:\n        - Name: FunctionName\n          Value: !Ref Orders\n      AlarmActions:\n        - !Ref Topic\n",
		"infra/ops.yaml": "AWSTemplateFormatVersion: '2010-09-09'\nResources:\n" +
			"  RemoteAlarm:\n    Type: AWS::CloudWatch::Alarm\n    Properties:\n      Namespace: Shop\n      MetricName: Errors\n" +
			"  OrdersFilter:\n    Type: AWS::Logs::MetricFilter\n    Properties:\n      LogGroupName: /aws/lambda/orders-api\n      FilterPattern: WARN\n      MetricTransformations:\n        - MetricNamespace: Shop\n          MetricName: Warnings\n          MetricValue: \"1\"\n" +
			"  AwsAlarm:\n    Type: AWS::CloudWatch::Alarm\n    Properties:\n      Namespace: AWS/Lambda\n      MetricName: Throttles\n",
		"infra/functions/orders/app.py": "def handler(event, context):\n    return 1\n",
		// a stack of its own in another directory declares the same metric:
		// the nearest declaration wins, so neither alarm joins it
		"other/stack.yaml": "AWSTemplateFormatVersion: '2010-09-09'\nResources:\n" +
			"  OtherFilter:\n    Type: AWS::Logs::MetricFilter\n    Properties:\n      LogGroupName: /aws/lambda/other\n      FilterPattern: ERROR\n      MetricTransformations:\n        - MetricNamespace: Shop\n          MetricName: Errors\n          MetricValue: \"1\"\n",
	}
}

// docXrefEdgeBrief is the stored doc-xref edges, "from -> to" by symbol key.
func docXrefEdgeBrief(t *testing.T, st store.Store, svc string) []string {
	t.Helper()
	all, err := st.TypedEdges(context.Background(), svc)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range all {
		if e.Kind == core.EdgeFileResolved && isDocCandidate(e.FromFile) { // the doc-xref pass's, not the file resolver's
			out = append(out, e.FromSymbol+" -> "+e.ToSymbol)
		}
	}
	sort.Strings(out)
	return out
}

// TestIngest_CloudWatchAlarmJoins (LB-F368, LB-F369, the product-reach
// proof): an ingest stores an alarm's references to the resource its
// Dimensions name and the topic its actions notify (Calls), and, as doc-xref
// edges, the alarm to the metric filter's metric it watches in its own
// template and in another, and a metric filter on /aws/lambda/<name> to the
// function of that name in its own template and in another; an alarm on an
// AWS namespace metric no template declares joins nothing, and a stack in
// another directory declaring the same metric is not the nearest.
func TestIngest_CloudWatchAlarmJoins(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, cloudwatchCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "cw", Path: dir}); err != nil {
		t.Fatal(err)
	}
	obs, err := st.QueryObservations(ctx, "cw", "")
	if err != nil {
		t.Fatal(err)
	}
	calls := map[string][]string{}
	for _, o := range obs {
		if o.Symbol == "ErrorAlarm" {
			calls[o.Symbol] = o.Calls
		}
	}
	if want := []string{"Orders", "Topic"}; !reflect.DeepEqual(calls["ErrorAlarm"], want) {
		t.Errorf("ErrorAlarm Calls = %v, want %v (the Dimensions resource and the action's topic)", calls["ErrorAlarm"], want)
	}
	got := docXrefEdgeBrief(t, st, "cw")
	want := []string{
		"cw::infra/app.ErrorAlarm -> cw::infra/app.Shop/Errors",
		"cw::infra/app.ErrorFilter -> cw::infra/app.orders-api",
		"cw::infra/app.Orders -> cw::infra/functions/orders/app.handler",
		"cw::infra/ops.OrdersFilter -> cw::infra/app.orders-api",
		"cw::infra/ops.RemoteAlarm -> cw::infra/app.Shop/Errors",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stored doc-xref edges\n got: %q\nwant: %q", got, want)
	}
}

// lambdaCorpus is a SAM template whose Globals give Runtime and CodeUri
// defaults, functions in each runtime family (python, node, a TypeScript
// esbuild function, java with and without a method, ruby), and functions
// that bind nothing (an image, inline code, a handler the code does not
// define, a dotnet runtime); and a plain CloudFormation template whose
// AWS::Lambda::Function has a package-able Code path and one whose code is
// a ZipFile.
func lambdaCorpus() map[string]string {
	return map[string]string{
		"app/template.yaml": "Transform: AWS::Serverless-2016-10-31\nGlobals:\n  Function:\n    Runtime: python3.12\n    CodeUri: src/\n" +
			"Resources:\n" +
			"  Py:\n    Type: AWS::Serverless::Function\n    Properties:\n      Handler: orders.app.lambda_handler\n" +
			"  PySlash:\n    Type: AWS::Serverless::Function\n    Properties:\n      Handler: orders/app.lambda_handler\n" +
			"  Node:\n    Type: AWS::Serverless::Function\n    Properties:\n      Runtime: nodejs20.x\n      CodeUri: web/\n      Handler: handlers/get.handler\n" +
			"  Ts:\n    Type: AWS::Serverless::Function\n    Properties:\n      Runtime: nodejs20.x\n      CodeUri: ts/\n      Handler: app.lambdaHandler\n    Metadata:\n      BuildMethod: esbuild\n      BuildProperties:\n        EntryPoints:\n          - src/app.ts\n" +
			"  Java:\n    Type: AWS::Serverless::Function\n    Properties:\n      Runtime: java21\n      CodeUri: java/\n      Handler: com.example.App::handleOrder\n" +
			"  JavaBare:\n    Type: AWS::Serverless::Function\n    Properties:\n      Runtime: java21\n      CodeUri: java/\n      Handler: com.example.App\n" +
			"  Ruby:\n    Type: AWS::Serverless::Function\n    Properties:\n      Runtime: ruby3.3\n      CodeUri: rb/\n      Handler: function.handler\n" +
			"  Image:\n    Type: AWS::Serverless::Function\n    Properties:\n      PackageType: Image\n      ImageUri: repo/app:1\n" +
			"  Inline:\n    Type: AWS::Serverless::Function\n    Properties:\n      Handler: index.handler\n      InlineCode: \"def handler(e, c): pass\"\n" +
			"  Missing:\n    Type: AWS::Serverless::Function\n    Properties:\n      Handler: orders.app.not_there\n" +
			"  Dotnet:\n    Type: AWS::Serverless::Function\n    Properties:\n      Runtime: dotnet8\n      Handler: Api::Api.Function::Handler\n" +
			"  ByParam:\n    Type: AWS::Serverless::Function\n    Properties:\n      Runtime: !Ref NodeRuntime\n      CodeUri: web/\n      Handler: handlers/get.handler\n",
		"app/src/orders/app.py":                       "def helper():\n    pass\n\ndef lambda_handler(event, context):\n    return helper()\n",
		"app/web/handlers/get.js":                     "exports.handler = async (event) => ({ statusCode: 200 });\n",
		"app/ts/src/app.ts":                           "export const lambdaHandler = async (event: unknown): Promise<number> => 1;\n",
		"app/java/src/main/java/com/example/App.java": "package com.example;\n\npublic class App {\n  public String handleOrder(Object in) { return \"\"; }\n  public String handleRequest(Object in, Object ctx) { return \"\"; }\n}\n",
		"app/rb/function.rb":                          "def handler(event:, context:)\n  1\nend\n",
		"legacy/stack.yaml": "AWSTemplateFormatVersion: '2010-09-09'\nResources:\n" +
			"  Legacy:\n    Type: AWS::Lambda::Function\n    Properties:\n      Runtime: nodejs18.x\n      Handler: index.handler\n      Code: ./code\n" +
			"  Zip:\n    Type: AWS::Lambda::Function\n    Properties:\n      Runtime: python3.12\n      Handler: index.handler\n      Code:\n        ZipFile: |\n          def handler(event, context):\n            return 1\n",
		"legacy/code/index.mjs": "export async function handler(event) {\n  return 1;\n}\n",
	}
}

var lambdaCorpusEdges = []string{
	"lh::app/template.ByParam -> lh::app/web/handlers/get.handler",
	"lh::app/template.Java -> lh::app/java/src/main/java/com/example/com.example.App.handleOrder",
	"lh::app/template.JavaBare -> lh::app/java/src/main/java/com/example/com.example.App.handleRequest",
	"lh::app/template.Node -> lh::app/web/handlers/get.handler",
	"lh::app/template.Py -> lh::app/src/orders/app.lambda_handler",
	"lh::app/template.PySlash -> lh::app/src/orders/app.lambda_handler",
	"lh::app/template.Ruby -> lh::app/rb/function.handler",
	"lh::app/template.Ts -> lh::app/ts/src/app.lambdaHandler",
	"lh::legacy/stack.Legacy -> lh::legacy/code/index.handler",
}

// TestIngest_LambdaHandlerBinding (LB-F370, the product-reach proof): an
// ingest stores each function's edge to its handler symbol by its runtime's
// rule — python's module path, node's file and export (a TypeScript esbuild
// entry point), java's Class::method and a bare class's handleRequest,
// ruby's file and method — with SAM's Globals defaults, and a Runtime that
// is a parameter read from the code; an image, inline
// code, a handler the code does not define and a dotnet runtime bind
// nothing.
func TestIngest_LambdaHandlerBinding(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, lambdaCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: "lh", Path: dir}); err != nil {
		t.Fatal(err)
	}
	if got := docXrefEdgeBrief(t, st, "lh"); !reflect.DeepEqual(got, lambdaCorpusEdges) {
		t.Fatalf("stored handler edges\n got: %q\nwant: %q", got, lambdaCorpusEdges)
	}
}

// TestResolveLambdaHandlers_Unbound: what binds nothing is counted by reason.
func TestResolveLambdaHandlers_Unbound(t *testing.T) {
	corpus := lambdaCorpus()
	files := docFiles(t, "cloudformation", map[string]string{"app/template.yaml": corpus["app/template.yaml"], "legacy/stack.yaml": corpus["legacy/stack.yaml"]})
	p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{})
	t.Cleanup(p.Close)
	paths := map[string]bool{}
	for k := range corpus {
		paths[k] = true
	}
	code := LambdaCode{
		Paths: paths,
		Read:  func(f string) ([]byte, bool) { s, ok := corpus[f]; return []byte(s), ok },
		Symbols: func(f string) []core.SymbolNode {
			be, _ := p.ExtractForBench("lh", nil, f, []byte(corpus[f]))
			return be.Nodes
		},
	}
	xs, st := ResolveLambdaHandlers(files, code)
	if len(xs) != len(lambdaCorpusEdges) || st.Functions != 14 || st.Bound != 9 {
		t.Fatalf("functions %d, bound %d, edges %d; want 14, 9, %d", st.Functions, st.Bound, len(xs), len(lambdaCorpusEdges))
	}
	if want := map[string]int{"image": 1, "inline": 2, "no-symbol": 1, "runtime": 1}; !reflect.DeepEqual(st.Unbound, want) {
		t.Errorf("unbound = %v, want %v", st.Unbound, want)
	}
}

// TestReingestFile_LambdaHandlerParity (LB-F370): a one-file refresh of the
// template or of a handler file, then an ingest, stores the same doc-xref
// edges a fresh ingest of the edited tree does — a handler renamed in the
// code drops its edge, one renamed in the template moves it.
func TestReingestFile_LambdaHandlerParity(t *testing.T) {
	edits := []map[string]string{
		{"app/src/orders/app.py": "def helper():\n    pass\n\ndef handle(event, context):\n    return helper()\n"},
		{"app/template.yaml": strings.Replace(lambdaCorpus()["app/template.yaml"], "orders.app.lambda_handler", "orders.app.helper", 1)},
	}
	for i, edit := range edits {
		dir := t.TempDir()
		writeCorpus(t, dir, lambdaCorpus())
		st := store.NewMemoryStore()
		p := New(connector.NewLocalConnector(), st, Config{})
		ctx := context.Background()
		svc := core.ServiceRef{ID: "lh", Path: dir}
		if _, err := p.Ingest(ctx, svc); err != nil {
			t.Fatal(err)
		}
		for rel, body := range edit {
			if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := p.ReingestFile(ctx, st, "lh", dir, rel); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := p.Ingest(ctx, svc); err != nil {
			t.Fatal(err)
		}
		got := docXrefEdgeBrief(t, st, "lh")
		p.Close()

		fresh := store.NewMemoryStore()
		q := New(connector.NewLocalConnector(), fresh, Config{})
		if _, err := q.Ingest(ctx, svc); err != nil {
			t.Fatal(err)
		}
		want := docXrefEdgeBrief(t, fresh, "lh")
		q.Close()
		if !reflect.DeepEqual(got, want) {
			t.Errorf("edit %d: edges after ReingestFile and ingest\n got: %q\nfresh: %q", i, got, want)
		}
		if reflect.DeepEqual(want, lambdaCorpusEdges) {
			t.Errorf("edit %d changed no edge; the test proves nothing", i)
		}
	}
}

// TestIngest_TerraformCloudWatch (LB-F371, the product-reach proof): a
// Terraform alarm joins the metric filter's literal metric and a literal
// /aws/lambda/<name> log group joins the function of that literal name (the
// identity join), across the dialects too (a CloudFormation alarm on the
// Terraform metric); an aws_lambda_function's handler binds through the
// archive_file whose source_dir its filename reads, and through a source
// block whose content is a file() (its runtime a variable, read from the
// code); one whose zip no archive_file builds binds nothing.
func TestIngest_TerraformCloudWatch(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("..", "extractor", "dialects", "terraform", "testdata", "cloudwatch.tf"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"tf/main.tf":      string(fixture),
		"tf/src/app.py":   "def handler(event, context):\n    return 1\n",
		"tf/fn/worker.py": "def run(event, context):\n    return 1\n",
		"cfn/alarm.yaml": "AWSTemplateFormatVersion: '2010-09-09'\nResources:\n" +
			"  CfnAlarm:\n    Type: AWS::CloudWatch::Alarm\n    Properties:\n      Namespace: TF/App\n      MetricName: Errors\n",
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	if _, err := p.Ingest(context.Background(), core.ServiceRef{ID: "tfcw", Path: dir}); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range docXrefEdgeBrief(t, st, "tfcw") {
		if !strings.Contains(e, "terraform.") { // the label-selector joins are LB-F84's
			got = append(got, e)
		}
	}
	want := []string{
		"tfcw::cfn/alarm.CfnAlarm -> tfcw::tf/main.TF/App/Errors",
		"tfcw::tf/main.aws_cloudwatch_log_metric_filter.errors -> tfcw::tf/main.tf-api",
		"tfcw::tf/main.aws_cloudwatch_metric_alarm.errors -> tfcw::tf/main.TF/App/Errors",
		"tfcw::tf/main.aws_cloudwatch_metric_alarm.math -> tfcw::tf/main.TF/App/Errors",
		"tfcw::tf/main.aws_lambda_function.api -> tfcw::tf/src/app.handler",
		"tfcw::tf/main.aws_lambda_function.worker -> tfcw::tf/fn/worker.run",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stored edges\n got: %q\nwant: %q", got, want)
	}
}

// TestResolveIdentityXrefs_Nearest: an identity reference joins the
// referring file's own declaration, else the deepest enclosing directory's,
// never itself; an identity no entity is signed with joins nothing.
func TestResolveIdentityXrefs_Nearest(t *testing.T) {
	metric := "AWSTemplateFormatVersion: '2010-09-09'\nResources:\n  F:\n    Type: AWS::Logs::MetricFilter\n    Properties:\n      LogGroupName: g\n      MetricTransformations:\n        - MetricNamespace: N\n          MetricName: M\n          MetricValue: \"1\"\n"
	alarm := "AWSTemplateFormatVersion: '2010-09-09'\nResources:\n  A:\n    Type: AWS::CloudWatch::Alarm\n    Properties:\n      Namespace: N\n      MetricName: M\n"
	files := docFiles(t, "cloudformation", map[string]string{
		"a/one.yaml":       alarm,
		"a/b/metric.yaml":  metric,
		"a/b/alarm.yaml":   alarm,
		"c/metric.yaml":    metric,
		"d/lonely.yaml":    alarm,
		"both/stack.yaml":  strings.Replace(metric, "Resources:\n", "Resources:\n  A:\n    Type: AWS::CloudWatch::Alarm\n    Properties:\n      Namespace: N\n      MetricName: M\n", 1),
		"both/other.yaml":  metric,
		"none/alarm.yaml":  strings.Replace(alarm, "Namespace: N", "Namespace: AWS/Lambda", 1),
		"none/metric.yaml": metric,
	})
	got := xrefBrief(ResolveIdentityXrefs(files))
	want := []string{
		"a/b/alarm.yaml:A -metric-> a/b/metric.yaml:N/M",
		"a/one.yaml:A -metric-> a/b/metric.yaml:N/M",
		"both/stack.yaml:A -metric-> both/stack.yaml:N/M",
		"d/lonely.yaml:A -metric-> a/b/metric.yaml:N/M",
		"d/lonely.yaml:A -metric-> both/other.yaml:N/M",
		"d/lonely.yaml:A -metric-> both/stack.yaml:N/M",
		"d/lonely.yaml:A -metric-> c/metric.yaml:N/M",
		"d/lonely.yaml:A -metric-> none/metric.yaml:N/M",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}
