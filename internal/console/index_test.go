package console

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestIndexProfile_TiersEdgesInventory pins OC-36: languages with their
// engine and tier in name order (never ranked), the typed share as report
// evidence computes it, edge kinds in the typed pass's order, and inventory
// keys — each absent with its reason (never a 0) when its source has
// nothing to say, and an error naming the failure when it could not be read.
func TestIndexProfile_TiersEdgesInventory(t *testing.T) {
	f := IndexFacts{
		Service: "fathom", Source: "the daemon's store /s/fleet.sqlite",
		Languages: []IndexLanguage{
			{Language: "JavaScript", Engine: "tree-sitter-wasm", Tier: "syntax", Files: 3, Symbols: 40},
			{Language: "Go", Engine: "built-in", Tier: "typed", Files: 900, Symbols: 15000},
			{Language: "CUE", Engine: "built-in", Tier: "typed", Files: 20, Symbols: 300},
		},
		EdgeKinds: []IndexEdgeKind{
			{Kind: "unresolved", Count: 10, Resolved: false},
			{Kind: "direct", Count: 70, Resolved: true},
			{Kind: "iface-dispatch", Count: 20, Resolved: true},
		},
		Inventory: map[string]int{"route": 27, "api": 102, "env": 44},
	}
	m := IndexProfile(f)
	var langs []string
	for _, l := range m.Languages {
		langs = append(langs, l.Language+"="+l.Engine+"/"+l.Tier)
	}
	if got := strings.Join(langs, " "); got != "CUE=built-in/typed Go=built-in/typed JavaScript=tree-sitter-wasm/syntax" {
		t.Errorf("languages must be in name order with engine and tier, never ranked: %s", got)
	}
	if m.LanguageSet.V != 3 || m.LanguageSet.State != StatePresent {
		t.Errorf("language count = %+v", m.LanguageSet)
	}
	if m.Edges.V != 100 || m.TypedEdges.V != 90 || m.TypedShare.V != 0.9 || m.TypedShare.State != StatePresent {
		t.Errorf("typed share = %+v of %+v (%+v)", m.TypedEdges, m.Edges, m.TypedShare)
	}
	if !strings.Contains(m.TypedShare.Denominator, "90 resolved of 100") {
		t.Errorf("the share names its denominator: %q", m.TypedShare.Denominator)
	}
	var kinds []string
	for _, k := range m.EdgeKinds {
		kinds = append(kinds, k.Kind)
	}
	if strings.Join(kinds, ",") != "direct,iface-dispatch,unresolved" {
		t.Errorf("edge kinds in the typed pass's order: %v", kinds)
	}
	if m.Inventory.V != 173 || len(m.Keys) != 3 || m.Keys[0].Kind != "api" || m.Keys[2].Kind != "route" {
		t.Errorf("inventory = %+v %+v", m.Inventory, m.Keys)
	}

	// No typed pass: every edge figure is absent with the reason, never 0%.
	noTyped := IndexProfile(IndexFacts{Service: "x", Languages: f.Languages, InventoryNote: "the corpus root /gone is not on this machine"})
	for name, v := range map[string]Value[int]{"edges": noTyped.Edges, "typed": noTyped.TypedEdges, "inventory": noTyped.Inventory} {
		if v.State != StateAbsent || v.Reason == "" {
			t.Errorf("%s with nothing measured = %+v, want absent with a reason", name, v)
		}
	}
	if noTyped.TypedShare.State != StateAbsent || !strings.Contains(noTyped.TypedShare.Reason, "never 0%") {
		t.Errorf("typed share with no pass = %+v", noTyped.TypedShare)
	}
	raw, _ := json.Marshal(noTyped)
	if strings.Contains(string(raw), `"typed_share":{"v"`) {
		t.Errorf("an absent share carries no number in JSON: %s", raw)
	}
	// A resolved count of zero is a measured zero, not an absence.
	zero := IndexProfile(IndexFacts{Service: "x", EdgeKinds: []IndexEdgeKind{{Kind: "unresolved", Count: 5}}})
	if zero.TypedShare.State != StateZero || zero.TypedEdges.State != StateZero || zero.Edges.V != 5 {
		t.Errorf("all-unresolved = %+v / %+v", zero.TypedShare, zero.TypedEdges)
	}
	// Unreadable sources are errors naming the failure.
	broken := IndexProfile(IndexFacts{Service: "x", LanguagesErr: "EPERM", EdgesErr: "database is locked", InventoryErr: "read /r/x.go: EPERM"})
	if broken.LanguageSet.State != StateError || broken.TypedShare.State != StateError || broken.Inventory.State != StateError {
		t.Errorf("unreadable sources = %+v %+v %+v", broken.LanguageSet, broken.TypedShare, broken.Inventory)
	}
	// An empty index names why it has no languages.
	if empty := IndexProfile(IndexFacts{Service: "x"}); empty.LanguageSet.State != StateAbsent || len(empty.Languages) != 0 {
		t.Errorf("empty = %+v", empty.LanguageSet)
	}
}
