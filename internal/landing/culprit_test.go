package landing

import (
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestRankCulpritsOrdersByReachThenRadius(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "pkg", Name: "Leaf", FilePath: "leaf.go"},
		{Service: "svc", Package: "pkg", Name: "Caller", FilePath: "caller.go", Calls: []string{"Leaf"}},
		{Service: "svc", Package: "pkg", Name: "TestRed", FilePath: "red_test.go", Calls: []string{"Caller"}},
		{Service: "svc", Package: "pkg", Name: "Other", FilePath: "other.go"},
		{Service: "svc", Package: "pkg", Name: "OtherCaller", FilePath: "other.go", Calls: []string{"Other"}},
		{Service: "svc", Package: "pkg", Name: "OtherCaller2", FilePath: "other.go", Calls: []string{"OtherCaller"}},
		{Service: "svc", Package: "pkg", Name: "OtherCaller3", FilePath: "other.go", Calls: []string{"OtherCaller2"}},
	}
	commits := []Commit{
		{Hash: "old", Files: []string{"leaf.go"}},
		{Hash: "middle", Files: []string{"caller.go"}},
		{Hash: "new", Files: []string{"leaf.go"}},
		{Hash: "wide", Files: []string{"other.go"}},
		{Hash: "unknown", Files: []string{"unsupported.xyz"}},
	}
	before := append([]Commit(nil), commits...)
	got := RankCulprits(commits, []string{"TestRed"}, Evidence{Nodes: nodes})
	var hashes []string
	for _, c := range got {
		hashes = append(hashes, c.Commit.Hash)
	}
	if want := []string{"new", "old", "middle", "unknown", "wide"}; !reflect.DeepEqual(hashes, want) {
		t.Fatalf("order = %v, want %v", hashes, want)
	}
	if got[0].BlastRadius != 3 || !reflect.DeepEqual(got[0].ReachedTests, []string{nodes[2].Key()}) {
		t.Fatalf("reach/radius = %+v", got[0])
	}
	if !strings.Contains(got[3].Reason, "no reach evidence") || !strings.Contains(got[3].Reason, "unsupported.xyz") {
		t.Fatalf("unknown = %+v", got[3])
	}
	if !reflect.DeepEqual(commits, before) {
		t.Fatal("ranking mutated the oldest-first input")
	}
	for i, j := 0, len(nodes)-1; i < j; i, j = i+1, j-1 {
		nodes[i], nodes[j] = nodes[j], nodes[i]
	}
	if again := RankCulprits(commits, []string{"TestRed", "TestRed"}, Evidence{Nodes: nodes}); !reflect.DeepEqual(got, again) {
		t.Fatalf("node/input order changed the result: %+v", again)
	}
}

func TestRankCulpritsTestEditsAndIncompleteRadius(t *testing.T) {
	test := core.SymbolNode{Service: "svc", Package: "pkg", Name: "TestRed", FilePath: "red_test.go", Calls: []string{"Leaf"}}
	evidence := Evidence{Nodes: []core.SymbolNode{
		{Service: "svc", Package: "pkg", Name: "Leaf", FilePath: "leaf.go"}, test,
		{Service: "svc", Package: "pkg", Name: "Other", FilePath: "other.go"},
		{Service: "svc", Package: "pkg", Name: "Other2", FilePath: "other.go"},
	}}
	commits := []Commit{
		{Hash: "test-edit", Files: []string{test.FilePath, "leaf.go"}},
		{Hash: "test-only", Files: []string{test.FilePath}},
		{Hash: "partial", Files: []string{"missing.xyz", "other.go"}},
		{Hash: "missing", Files: []string{"missing.xyz"}},
		{Hash: "indexed", Files: []string{"other.go"}},
	}
	for _, failing := range []string{test.Name, test.Key()} {
		got := RankCulprits(commits, []string{failing, failing}, evidence)
		var hashes []string
		for _, c := range got {
			hashes = append(hashes, c.Commit.Hash)
		}
		if want := []string{"test-edit", "test-only", "partial", "missing", "indexed"}; !reflect.DeepEqual(hashes, want) {
			t.Errorf("%s: order = %v, want %v", failing, hashes, want)
		}
		for _, c := range got {
			if strings.HasPrefix(c.Commit.Hash, "test-") && !reflect.DeepEqual(c.ReachedTests, []string{test.Key()}) {
				t.Errorf("direct/graph test reach must be deduplicated: %+v", c)
			}
			if c.Commit.Hash == "partial" && !strings.Contains(c.Reason, "blast radius >=2") {
				t.Errorf("partial radius must be a lower bound: %+v", c)
			}
		}
	}
}

func TestRankCulpritsTypedReachAndMissingEvidence(t *testing.T) {
	leaf := core.SymbolNode{Service: "svc", Package: "pkg", Name: "Leaf", FilePath: "leaf.go"}
	test := core.SymbolNode{Service: "svc", Package: "pkg", Name: "TestRed", FilePath: "red_test.go"}
	evidence := Evidence{Nodes: []core.SymbolNode{leaf, test}, Typed: []core.Edge{
		{FromService: "svc", FromFile: test.FilePath, FromSymbol: test.Key(), ToSymbol: leaf.Key(), Kind: core.EdgeDirect},
	}}
	commits := []Commit{{Hash: "a", Files: []string{"missing.xyz", "leaf.go"}}}
	got := RankCulprits(commits, []string{test.Key()}, evidence)
	if !reflect.DeepEqual(got[0].Commit.Files, []string{"leaf.go", "missing.xyz"}) || commits[0].Files[0] != "missing.xyz" {
		t.Fatalf("output paths must be sorted without modifying input: %+v", got)
	}
	if len(got[0].ReachedTests) != 1 || got[0].BlastRadius != 2 || !strings.Contains(got[0].Reason, "no reach evidence for files: missing.xyz") {
		t.Fatalf("typed/partial evidence = %+v", got)
	}
	for _, failing := range [][]string{nil, {"TestNotIndexed"}} {
		got := RankCulprits(commits, failing, evidence)
		if len(got[0].ReachedTests) != 0 || !strings.Contains(got[0].Reason, "no reach evidence") {
			t.Fatalf("missing test evidence = %+v", got)
		}
	}
	got = RankCulprits(commits, []string{"TestRed"}, Evidence{})
	if !strings.Contains(got[0].Reason, "no reach evidence") || len(got[0].MissingFiles) != 2 {
		t.Fatalf("empty index = %+v", got)
	}
}
