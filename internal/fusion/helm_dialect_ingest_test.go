package fusion

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/query"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestHelmDialectIngestsEndToEnd (LB-Q-helm): the tags+doc `helm` dialect, put
// in dispatch for its basenames and extension on a fresh pipeline (production
// dispatch stays on the incumbents until the K8s-kinds decision, LB-Q-helm-rest),
// ingests a two-chart repository through fusion's own gates and passes: a
// chart, a named template and the call made inside it, a .Values reference, a
// template reference that shares its name (and symbol key) with a template
// defined in the same file — both stay findable, and the callers of the
// template are still its callers — and the doc-xref pass storing the
// chart-to-chart dependency edge for a composite extractor.
func TestHelmDialectIngestsEndToEnd(t *testing.T) {
	dir := t.TempDir()
	writeCorpus(t, dir, map[string]string{
		"common/Chart.yaml":            "name: common\nversion: 2.0.0\n",
		"stack/Chart.yaml":             "name: stack\nversion: 1.0.0\ndependencies:\n  - name: common\n    version: 2.x.x\n    repository: oci://registry.example/charts\n",
		"stack/values.yaml":            "image:\n  tag: v1\n",
		"stack/templates/_helpers.tpl": "{{- define \"stack.a\" -}}\n{{ .Values.image.tag }}\n{{- end }}\n{{- define \"stack.b\" -}}\n{{ include \"stack.a\" . }}\n{{- end }}\n",
	})
	st := store.NewMemoryStore()
	p := New(connector.NewLocalConnector(), st, Config{})
	t.Cleanup(p.Close)
	x, err := extractor.NewTagsDocExtractor("helm")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(x.Close)
	for _, k := range []string{"Chart.yaml", "values.yaml", ".tpl"} {
		p.RegisterExtractor(k, x)
	}
	ctx := context.Background()
	rep, err := p.Ingest(ctx, core.ServiceRef{ID: "h", Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	if rep.DocXrefEdges != 1 {
		t.Fatalf("DocXrefEdges = %d, want the one stack -> common dependency edge", rep.DocXrefEdges)
	}
	edges, _ := st.TypedEdges(ctx, "h")
	var got []string
	for _, e := range edges {
		if e.Kind == core.EdgeFileResolved {
			got = append(got, e.FromFile+"|"+e.FromSymbol+"->"+e.ToSymbol)
		}
	}
	if want := []string{"stack/Chart.yaml|h::stack/Chart.stack->h::common/Chart.common"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("stored edges %q, want %q", got, want)
	}

	obs, _ := st.QueryObservations(ctx, "h", "")
	var seen []string
	for _, o := range obs {
		seen = append(seen, string(o.SourceKind)+" "+o.Symbol)
	}
	sort.Strings(seen)
	want := []string{
		"HelmChart common", "HelmChart stack", "HelmDependency common",
		"HelmTemplate stack.a", "HelmTemplate stack.b", "HelmTemplateRef stack.a",
		"HelmValue image", "HelmValue image.tag", "HelmValueRef image.tag",
	}
	if !reflect.DeepEqual(seen, want) {
		t.Fatalf("observations %q, want %q", seen, want)
	}

	e := query.New(st)
	found, _ := e.FindSymbol(ctx, "h", "stack.a")
	if len(found) != 2 {
		t.Fatalf("stack.a is a definition and a reference in one file: both must stay findable, got %d", len(found))
	}
	callers, err := e.FindCallers(ctx, "h", "stack.a")
	if err != nil || len(callers) != 1 || callers[0].Name != "stack.b" {
		t.Fatalf("callers of stack.a = %+v, %v; want the template stack.b", callers, err)
	}
}
