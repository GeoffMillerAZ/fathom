package main

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/console"
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
