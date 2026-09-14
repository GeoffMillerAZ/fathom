package mcp

import "testing"

// TestPromotedVerbs pins VB-1: the four verbs the agent surface leads with,
// each carrying the reason grep cannot give that answer.
func TestPromotedVerbs(t *testing.T) {
	want := []string{"blast_radius", "review_diff", "test_impact", "rename_impact"}
	got := promotedVerbs()
	if len(got) != len(want) {
		t.Fatalf("promotedVerbs: got %d verbs, want %d", len(got), len(want))
	}
	for i, v := range got {
		if v.Name != want[i] {
			t.Errorf("promotedVerbs[%d] = %q, want %q", i, v.Name, want[i])
		}
		if v.Reason == "" {
			t.Errorf("promotedVerbs[%d] %q carries no reason", i, v.Name)
		}
	}
	for _, v := range got {
		if !containsString(AgentSurfaceTools(), v.Name) {
			t.Errorf("promoted verb %q is not on the agent surface", v.Name)
		}
	}
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
