package mcp_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// budgetRecordingBackend answers ScanSources with a partial scan and
// records the budget each handler asked for.
type budgetRecordingBackend struct {
	sourcesFakeBackend
	budgets []corpus.Budget
}

func (b *budgetRecordingBackend) ScanSources(ctx context.Context, _ string, bud corpus.Budget, fn func(core.SourceFile) bool) (corpus.Coverage, error) {
	b.budgets = append(b.budgets, bud)
	cov, err := corpus.ScanSourceFiles(ctx, b.sources[:1], bud, fn)
	cov.Total, cov.Stopped = len(b.sources), corpus.StopBytes
	return cov, err
}

// RT-71 (design §4.6): mcp.QueryInterface names no whole-tree read — the
// only tree read is ScanSources, which every handler calls with a byte
// budget, and a scan that stopped early is named in the answer (I38).
func TestQueryInterfaceHasNoWholeTreeRead(t *testing.T) {
	qi := reflect.TypeOf((*mcp.QueryInterface)(nil)).Elem()
	if _, ok := qi.MethodByName("Sources"); ok {
		t.Fatal("mcp.QueryInterface still carries Sources: query code can name a whole-tree read")
	}
	if _, ok := qi.MethodByName("ScanSources"); !ok {
		t.Fatal("mcp.QueryInterface has no ScanSources")
	}

	fake := &budgetRecordingBackend{sourcesFakeBackend: sourcesFakeBackend{sources: []core.SourceFile{
		{Path: "a.go", Content: []byte("// SPEC-1\npackage a\n"), Lang: "go"},
		{Path: "b.go", Content: []byte("// SPEC-2\npackage b\n"), Lang: "go"},
	}}}
	srv := mcp.NewServer(fake)
	for _, tool := range []string{"traceability", "secret_scan", "cloc"} {
		fake.budgets = nil
		got := callTool(t, srv, tool, map[string]any{"service": "svc"})
		if len(fake.budgets) == 0 {
			t.Fatalf("%s never scanned", tool)
		}
		for _, b := range fake.budgets {
			if b.Bytes <= 0 {
				t.Fatalf("%s scanned with no byte budget: %+v", tool, b)
			}
		}
		if p, _ := got["partial"].(string); !strings.Contains(p, "scanned 1/2 files: budget bytes") {
			t.Fatalf("%s partial = %q, want the named lower bound", tool, got["partial"])
		}
	}
}
