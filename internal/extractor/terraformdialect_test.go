package extractor

// terraformdialect_test.go — goldens for the terraform dialect
// (LB-Q-terraform, design §13.2 steps 10/12). No generic "TestDialectGoldens"
// runner exists for a native-engine dialect (only TestTagsDialectParity for
// the tags dialects, and each doc-engine dialect gets its own runner, e.g.
// packagejsondialect_test.go) — this mirrors that same pattern: fixtures and
// hand-written goldens live under dialects/terraform/testdata/, read
// directly and compared against HCLExtractor.Extract's real output.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

type tfGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
	Tier      string   `json:"tier"`
}

// TestTerraformDialectGoldens runs every dialects/terraform/testdata/*.tf
// and *.tfvars fixture through HCLExtractor.Extract and compares the
// result against its hand-written *.golden.json (§13.2 step 10: written
// from reading the fixture, before any extractor change existed for it).
func TestTerraformDialectGoldens(t *testing.T) {
	dir := filepath.Join("dialects", "terraform", "testdata")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir %s: %v", dir, err)
	}
	x := NewHCLExtractor()
	found := 0
	for _, e := range entries {
		name := e.Name()
		if filepath.Ext(name) != ".tf" && filepath.Ext(name) != ".tfvars" {
			continue
		}
		found++
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			nodes, err := x.Extract("svc", name, src)
			if err != nil {
				t.Fatalf("Extract: %v", err)
			}
			goldenPath := filepath.Join(dir, name[:len(name)-len(filepath.Ext(name))]+".golden.json")
			gb, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden %s: %v", goldenPath, err)
			}
			var want []tfGoldenSymbol
			if err := json.Unmarshal(gb, &want); err != nil {
				t.Fatalf("unmarshal golden: %v", err)
			}
			got := make([]tfGoldenSymbol, len(nodes))
			for i, n := range nodes {
				calls := append([]string(nil), n.Calls...)
				sort.Strings(calls)
				got[i] = tfGoldenSymbol{
					Name: n.Name, Kind: string(n.Kind), Line: n.Line, EndLine: n.EndLine,
					Signature: n.Signature, Calls: calls, Tier: string(n.Tier),
				}
			}
			for i := range want {
				sort.Strings(want[i].Calls)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Extract(%s) mismatch:\n got: %+v\nwant: %+v", name, got, want)
			}
		})
	}
	if found == 0 {
		t.Fatal("no terraform fixtures found — testdata missing?")
	}
}
