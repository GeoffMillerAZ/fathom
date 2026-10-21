package mcp_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// The default tools/call budget can remove seven detailed dimensions when
// one early dimension has many findings. Their final statuses must survive.
func TestReviewDiff_DimensionStatusSurvivesDispatchElision(t *testing.T) {
	call := func(rules []semantic.Obligation) map[string]any {
		fake := &reviewFakeBackend{
			extendedFakeBackend: extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}},
			obligations:         map[string]semantic.Obligations{"pkg/a.go": {Rules: rules}},
		}
		return callTool(t, mcp.NewServer(fake), "review_diff", map[string]any{
			"service": "svc", "changed_files": []string{"pkg/a.go"},
		})
	}

	small := call(nil)
	detailed, ok := small["dimensions"].([]any)
	if !ok || len(detailed) != 8 || small["elided"] != nil {
		t.Fatalf("under-budget dimensions = %v; elided = %v", small["dimensions"], small["elided"])
	}
	want := make(map[string]any, len(detailed))
	for _, raw := range detailed {
		d := raw.(map[string]any)
		state := map[string]any{"checked": d["checked"], "count": d["count"]}
		if d["checked"] == false {
			state["unchecked_reason"] = d["reason"]
		}
		want[d["name"].(string)] = state
	}
	if !reflect.DeepEqual(small["dimension_status"], want) {
		t.Fatalf("under-budget status = %v; want detailed dimensions = %v", small["dimension_status"], want)
	}
	if want["coverage"].(map[string]any)["count"] != float64(0) || want["coverage"].(map[string]any)["unchecked_reason"] != "no coverage profile supplied" {
		t.Fatalf("fixture lost unchecked zero-count coverage: %v", want["coverage"])
	}

	rules := make([]semantic.Obligation, 90)
	for i := range rules {
		rules[i] = semantic.Obligation{
			Rule: fmt.Sprintf("rule %03d: %s", i, strings.Repeat("detail ", 45)),
			Src:  "pkg/a.go#L1-L1",
		}
	}
	large := call(rules)
	kept, ok := large["dimensions"].([]any)
	if !ok || len(kept) >= len(detailed) {
		t.Fatalf("default dispatch did not elide detailed dimensions: %v", large["dimensions"])
	}
	elided, ok := large["elided"].(map[string]any)
	if !ok || elided["of"] != float64(len(detailed)) || elided["count"] != float64(len(detailed)-len(kept)) {
		t.Fatalf("dispatch elision = %v; kept %d of %d", large["elided"], len(kept), len(detailed))
	}
	want["obligations"].(map[string]any)["count"] = float64(len(rules))
	if !reflect.DeepEqual(large["dimension_status"], want) {
		t.Fatalf("elided status = %v; want %v", large["dimension_status"], want)
	}
}
