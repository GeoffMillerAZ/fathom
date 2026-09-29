package main

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/langbench"
	"github.com/geoffmilleraz/fathom/internal/support"
)

// TestRenderSupportNextNamesReplan (G2-6 in What's next): an item a refused
// need holds is listed under "needs re-planning" with its plan row, and the
// wave's heading counts it, instead of it hiding among the blocked.
func TestRenderSupportNextNamesReplan(t *testing.T) {
	v := console.SupportNext([]console.SupportNextItem{
		{Kind: support.KindLanguage, ID: "lua", Name: "Lua", Wave: 2, PlanRow: "LB-Q-lua", Needs: []string{"LB-F4"}},
		{Kind: support.KindLanguage, ID: "ruby", Name: "Ruby", Wave: 2, PlanRow: "LB-Q-ruby", Needs: []string{"LB-F2"}},
	}, map[string]console.SupportRowState{
		"LB-F2": console.SupportRowDispositioned, "LB-F4": console.SupportRowRefused,
		"LB-Q-lua": console.SupportRowOpen, "LB-Q-ruby": console.SupportRowOpen,
	})
	got := renderSupportNext(v)
	for _, want := range []string{"0 eligible, 1 blocked, 1 needs re-planning", "Needs re-planning", "Lua", "LB-Q-lua"} {
		if !strings.Contains(got, want) {
			t.Errorf("render lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Ruby") {
		t.Errorf("a deferred need blocks: Ruby is not listed:\n%s", got)
	}
}

// TestSupportNextItemsCarryRefusal: a queue item the plan refuses (§14.2, its
// row or its -rest sibling: langbench.RefusedItems, the tracker's own notion)
// reaches What's next as Refused, so the panel lists it under re-planning
// instead of offering it while its own row stays open.
func TestSupportNextItemsCarryRefusal(t *testing.T) {
	src := supportSources{
		Queue: []langbench.QueueEntry{
			{ID: "swift", Name: "Swift", Kind: "language", Wave: 2, PlanRow: "LB-Q-swift", DependsOn: []string{"LB-F1"}},
			{ID: "lua", Name: "Lua", Kind: "language", Wave: 2, PlanRow: "LB-Q-lua", DependsOn: []string{"LB-F1"}},
		},
		Refused: map[string]string{"swift": "LB-Q-swift-rest: parse errors"},
	}
	got := map[string]bool{}
	for _, it := range supportNextItems(src) {
		got[it.ID] = it.Refused
	}
	if !got["swift"] || got["lua"] {
		t.Fatalf("Refused per item = %v, want swift only", got)
	}
	v := console.SupportNext(supportNextItems(src), map[string]console.SupportRowState{
		"LB-F1": console.SupportRowClaimed, "LB-Q-swift": console.SupportRowOpen, "LB-Q-lua": console.SupportRowOpen,
	})
	if v.Eligible != 1 || v.Replan != 1 || len(v.Waves) != 1 || v.Waves[0].Replan[0].ID != "swift" {
		t.Fatalf("What's next = %+v, want lua eligible and swift under re-planning", v)
	}
}
