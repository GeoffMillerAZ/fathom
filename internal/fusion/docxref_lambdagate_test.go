package fusion

// docxref_lambdagate_test.go — EW-57 (wave h3, O766): a code file a Lambda
// handler binding may name re-runs the doc-xref pass only where the service
// declares a Lambda function. C3 made every .py, .js, .ts, .java and .rb
// change an input of the whole pass (and of the one-file refresh's
// invalidation, O723/O730), so a repository with no Lambda at all paid the
// pass on every code edit.

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// kindReadCounter is a MemoryStore that counts the Lambda gate's store
// reads (store.KindObservationsStore).
type kindReadCounter struct {
	*store.MemoryStore
	reads atomic.Int32
}

func (c *kindReadCounter) ObservationsOfKinds(ctx context.Context, serviceID string, kinds []core.SourceKind) ([]core.Observation, error) {
	c.reads.Add(1)
	return c.MemoryStore.ObservationsOfKinds(ctx, serviceID, kinds)
}

// noLambdaCorpus has documents the pass reads (a Kubernetes manifest, a
// Dockerfile, a CloudFormation template whose alarm joins its metric filter,
// and a Terraform file declaring no function) beside Python and TypeScript
// code: infrastructure, no Lambda.
func noLambdaCorpus() map[string]string {
	return map[string]string{
		"deploy/app.yaml": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\nspec:\n  template:\n    spec:\n      containers:\n        - name: web\n          image: shop/web:1\n",
		"Dockerfile":      "FROM python:3.12\nCOPY app /app\n",
		"infra/bucket.yaml": "AWSTemplateFormatVersion: '2010-09-09'\nResources:\n" +
			"  Assets:\n    Type: AWS::S3::Bucket\n" +
			"  ErrorFilter:\n    Type: AWS::Logs::MetricFilter\n    Properties:\n      LogGroupName: /app/web\n      FilterPattern: ERROR\n      MetricTransformations:\n        - MetricNamespace: Shop\n          MetricName: Errors\n          MetricValue: \"1\"\n" +
			"  ErrorAlarm:\n    Type: AWS::CloudWatch::Alarm\n    Properties:\n      Namespace: Shop\n      MetricName: Errors\n",
		"tf/main.tf":  "resource \"aws_s3_bucket\" \"logs\" {\n  bucket = \"logs\"\n}\n\nmodule \"vpc\" {\n  source = \"./vpc\"\n}\n",
		"app/main.py": "def handler(event, context):\n    return helper()\n\ndef helper():\n    return 1\n",
		"web/app.ts":  "export const handler = async (e: unknown): Promise<number> => 1;\n",
	}
}

func appendLine(t *testing.T, dir, rel, line string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(b, []byte(line)...), 0o644); err != nil {
		t.Fatal(err)
	}
}

// freshDocXrefEdges is what a fresh ingest of dir stores.
func freshDocXrefEdges(t *testing.T, svc core.ServiceRef) []string {
	t.Helper()
	fresh := store.NewMemoryStore()
	q := New(connector.NewLocalConnector(), fresh, Config{})
	defer q.Close()
	if _, err := q.Ingest(context.Background(), svc); err != nil {
		t.Fatal(err)
	}
	return docXrefEdgeBrief(t, fresh, svc.ID)
}

// TestIngest_DocXrefLambdaGate_NoLambda (EW-57, the product-reach proof):
// in a service that declares no Lambda function — though it declares
// CloudFormation and Terraform resources — a Python or TypeScript edit and
// an ingest do not re-run the doc-xref pass: the Report says it was skipped,
// the gate read the store once, and the stored edges are a fresh ingest's.
// A document edit still re-runs it, with no store read.
func TestIngest_DocXrefLambdaGate_NoLambda(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, noLambdaCorpus())
	st := &kindReadCounter{MemoryStore: store.NewMemoryStore()}
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	svc := core.ServiceRef{ID: "nl", Path: dir}
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	if rep.DocXrefSkipped != "" {
		t.Fatalf("the first ingest skipped the doc-xref pass: %q", rep.DocXrefSkipped)
	}
	if got := docXrefEdgeBrief(t, st, "nl"); len(got) == 0 {
		t.Fatal("the corpus stores no doc-xref edge: the test proves nothing")
	}
	runs := 0
	for i, rel := range []string{"app/main.py", "web/app.ts", "app/main.py"} {
		appendLine(t, dir, rel, "\n// edit "+string(rune('a'+i))+"\n")
		if strings.HasSuffix(rel, ".py") {
			appendLine(t, dir, rel, "def added_"+string(rune('a'+i))+"():\n    return 2\n")
		}
		st.reads.Store(0)
		rep, err := p.Ingest(ctx, svc)
		if err != nil {
			t.Fatal(err)
		}
		if rep.Extracted == 0 {
			t.Fatalf("edit %d (%s): nothing re-extracted: the edit proves nothing", i, rel)
		}
		if rep.DocXrefSkipped != SkippedPassReason {
			runs++
		}
		if n := st.reads.Load(); n != 1 {
			t.Errorf("edit %d (%s): the gate read the store %d times, want one bounded read", i, rel, n)
		}
		timed := false
		for _, pt := range rep.PassTimes {
			timed = timed || (pt.Name == "lambda_gate" && pt.Parent == "extract")
		}
		if !timed {
			t.Errorf("edit %d (%s): the gate's read is not timed as a span of the extract pass: %+v", i, rel, rep.PassTimes)
		}
		if got, want := docXrefEdgeBrief(t, st, "nl"), freshDocXrefEdges(t, svc); !reflect.DeepEqual(got, want) {
			t.Errorf("edit %d (%s): edges after the gated ingest\n got: %q\nfresh: %q", i, rel, got, want)
		}
	}
	if runs != 0 {
		t.Errorf("a code edit in a service with no Lambda re-ran the doc-xref pass %d times, want 0", runs)
	}
	// a document is an input as before, and needs no store read
	appendLine(t, dir, "deploy/app.yaml", "# edited\n")
	st.reads.Store(0)
	if rep, err := p.Ingest(ctx, svc); err != nil || rep.DocXrefSkipped != "" {
		t.Fatalf("a document edit did not re-run the pass (skipped %q): %v", rep.DocXrefSkipped, err)
	}
	if n := st.reads.Load(); n != 0 {
		t.Errorf("a document edit read the store %d times for the Lambda gate, want 0", n)
	}
}

// TestReingestFile_DocXrefLambdaGate_NoLambda (EW-57): the one-file refresh
// of a code file in a service with no Lambda leaves the doc-xref pass's
// record standing, so the next ingest skips the pass and still stores a
// fresh ingest's edges; a document's refresh still invalidates it.
func TestReingestFile_DocXrefLambdaGate_NoLambda(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, noLambdaCorpus())
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	ctx := context.Background()
	svc := core.ServiceRef{ID: "nl", Path: dir}
	if _, err := p.Ingest(ctx, svc); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"app/main.py", "web/app.ts"} {
		appendLine(t, dir, rel, "\n// edited\n")
		rep, err := p.ReingestFile(ctx, st, "nl", dir, rel)
		if err != nil {
			t.Fatal(err)
		}
		if !rep.Refreshed {
			t.Fatalf("%s: not refreshed: the test proves nothing", rel)
		}
		if !p.passStateOK(ctx, "nl", docXrefStateKey) {
			t.Errorf("%s: the one-file refresh of a code file invalidated the doc-xref pass in a service with no Lambda", rel)
		}
	}
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	if rep.DocXrefSkipped != SkippedPassReason {
		t.Errorf("the ingest after the refreshes re-ran the doc-xref pass (skipped %q)", rep.DocXrefSkipped)
	}
	if got, want := docXrefEdgeBrief(t, st, "nl"), freshDocXrefEdges(t, svc); !reflect.DeepEqual(got, want) {
		t.Errorf("edges after ReingestFile and ingest\n got: %q\nfresh: %q", got, want)
	}
	appendLine(t, dir, "infra/bucket.yaml", "# edited\n")
	if _, err := p.ReingestFile(ctx, st, "nl", dir, "infra/bucket.yaml"); err != nil {
		t.Fatal(err)
	}
	if p.passStateOK(ctx, "nl", docXrefStateKey) {
		t.Error("the one-file refresh of a template left the doc-xref pass's record standing")
	}
}

// tfLambdaCorpus is a Terraform aws_lambda_function whose archive_file
// source_dir holds its Python handler.
func tfLambdaCorpus() map[string]string {
	return map[string]string{
		"tf/main.tf": "resource \"aws_lambda_function\" \"api\" {\n  function_name = \"tf-api\"\n  filename      = data.archive_file.api.output_path\n  handler       = \"app.handler\"\n  runtime       = \"python3.12\"\n}\n\n" +
			"data \"archive_file\" \"api\" {\n  type        = \"zip\"\n  source_dir  = \"${path.module}/src\"\n  output_path = \"${path.module}/api.zip\"\n}\n",
		"tf/src/app.py": "def handler(event, context):\n    return 1\n",
		"lib/util.py":   "def util():\n    return 1\n",
	}
}

// TestReingestFile_TerraformLambdaHandlerParity (EW-57): where a Terraform
// file declares the function, a code edit is an input as before — the
// handler renamed drops its edge after an ingest, and after a one-file
// refresh (which invalidates the pass) then an ingest, equal to a fresh
// ingest of the edited tree either way.
func TestReingestFile_TerraformLambdaHandlerParity(t *testing.T) {
	const bound = "tl::tf/main.aws_lambda_function.api -> tl::tf/src/app.handler"
	for _, refresh := range []bool{false, true} {
		dir := t.TempDir()
		writeCorpus(t, dir, tfLambdaCorpus())
		st := store.NewMemoryStore()
		p := New(connector.NewLocalConnector(), st, Config{})
		ctx := context.Background()
		svc := core.ServiceRef{ID: "tl", Path: dir}
		if _, err := p.Ingest(ctx, svc); err != nil {
			t.Fatal(err)
		}
		if got := docXrefEdgeBrief(t, st, "tl"); !reflect.DeepEqual(got, []string{bound}) {
			t.Fatalf("refresh=%v: the handler did not bind: %q", refresh, got)
		}
		rel := "tf/src/app.py"
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte("def handle(event, context):\n    return 1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if refresh {
			if _, err := p.ReingestFile(ctx, st, "tl", dir, rel); err != nil {
				t.Fatal(err)
			}
			if p.passStateOK(ctx, "tl", docXrefStateKey) {
				t.Error("the one-file refresh of a handler file left the doc-xref pass's record standing where a Lambda is declared")
			}
		}
		rep, err := p.Ingest(ctx, svc)
		if err != nil {
			t.Fatal(err)
		}
		got := docXrefEdgeBrief(t, st, "tl")
		p.Close()
		if rep.DocXrefSkipped != "" {
			t.Errorf("refresh=%v: the handler edit skipped the doc-xref pass: %q", refresh, rep.DocXrefSkipped)
		}
		want := freshDocXrefEdges(t, svc)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("refresh=%v: edges after the handler edit\n got: %q\nfresh: %q", refresh, got, want)
		}
		if reflect.DeepEqual(want, []string{bound}) {
			t.Errorf("refresh=%v: the rename changed no edge; the test proves nothing", refresh)
		}
	}
}

// TestIngest_LambdaTemplateAndHandlerBindTogether (EW-57): a template and
// its handler added in one ingest bind, in a service that declared no Lambda
// before; once declared, a handler edit alone re-runs the pass (the store
// holds the declaration), and a handler added before its template binds when
// the template lands.
func TestIngest_LambdaTemplateAndHandlerBindTogether(t *testing.T) {
	sam := "Transform: AWS::Serverless-2016-10-31\nResources:\n  Fn:\n    Type: AWS::Serverless::Function\n    Properties:\n      Runtime: python3.12\n      CodeUri: src/\n      Handler: app.handler\n"
	for _, together := range []bool{true, false} {
		dir := t.TempDir()
		writeCorpus(t, dir, noLambdaCorpus())
		st := &kindReadCounter{MemoryStore: store.NewMemoryStore()}
		p := New(connector.NewLocalConnector(), st, Config{})
		ctx := context.Background()
		svc := core.ServiceRef{ID: "nl", Path: dir}
		if _, err := p.Ingest(ctx, svc); err != nil {
			t.Fatal(err)
		}
		writeCorpus(t, dir, map[string]string{"fn/src/app.py": "def handler(event, context):\n    return 1\n"})
		if !together {
			if rep, err := p.Ingest(ctx, svc); err != nil || rep.DocXrefSkipped != SkippedPassReason {
				t.Fatalf("a handler with no template re-ran the pass (skipped %q): %v", rep.DocXrefSkipped, err)
			}
		}
		writeCorpus(t, dir, map[string]string{"fn/template.yaml": sam})
		st.reads.Store(0)
		if _, err := p.Ingest(ctx, svc); err != nil {
			t.Fatal(err)
		}
		if n := st.reads.Load(); n > 1 {
			t.Errorf("together=%v: the ingest read the store %d times for the Lambda gate, want at most one", together, n)
		}
		const edge = "nl::fn/template.Fn -> nl::fn/src/app.handler"
		got := docXrefEdgeBrief(t, st, "nl")
		if !reflect.DeepEqual(got, freshDocXrefEdges(t, svc)) || !containsString(got, edge) {
			t.Fatalf("together=%v: the template and its handler did not bind as a fresh ingest does: %q", together, got)
		}
		// declared now: a handler edit alone is an input
		if err := os.WriteFile(filepath.Join(dir, "fn", "src", "app.py"), []byte("def other(event, context):\n    return 1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		rep, err := p.Ingest(ctx, svc)
		if err != nil {
			t.Fatal(err)
		}
		got = docXrefEdgeBrief(t, st, "nl")
		p.Close()
		if rep.DocXrefSkipped != "" || containsString(got, edge) || !reflect.DeepEqual(got, freshDocXrefEdges(t, svc)) {
			t.Errorf("together=%v: the handler renamed under a declared function (skipped %q): %q", together, rep.DocXrefSkipped, got)
		}
	}
}
