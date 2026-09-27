package memo

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// TestDefaultGroupsExportCacheBytes (RT-124 × RT-73): every group in the
// Default registry reports its byte count as fathom_cache_bytes{group},
// so /metrics measures the memo caches instead of naming them absent.
func TestDefaultGroupsExportCacheBytes(t *testing.T) {
	g := NewGroup[string, []byte]("metrics-test-group", Options{})
	t.Cleanup(func() { tenancy.RegisterCacheBytes("metrics-test-group", nil) })
	if _, err := g.Get(context.Background(), "k", func(context.Context) ([]byte, int64, error) { return make([]byte, 4096), 4096, nil }); err != nil {
		t.Fatal(err)
	}
	lines := tenancy.ToolCounterLines()
	if !strings.Contains(lines, `fathom_cache_bytes{group="metrics-test-group"} 4096`) {
		t.Errorf("metrics lines lack the group's bytes:\n%s", lines)
	}
}
