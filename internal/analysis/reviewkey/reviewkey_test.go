package reviewkey_test

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/reviewkey"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// baseInputs builds a small, realistic fixture: package a/ (changed) calls
// into package b/ (a dependency NOT in the manifest) via a resolved typed
// edge — the shape the dependency-closure mutation case needs.
func baseInputs() reviewkey.Inputs {
	nodeA := core.SymbolNode{Service: "svc", Package: "a", Name: "Caller", FilePath: "a/a.go", Line: 1}
	nodeB := core.SymbolNode{Service: "svc", Package: "b", Name: "Callee", FilePath: "b/b.go", Line: 1}
	nodes := []core.SymbolNode{nodeA, nodeB}
	edges := []core.Edge{
		{FromService: "svc", FromFile: "a/a.go", FromSymbol: nodeA.Key(), ToSymbol: nodeB.Key(), Kind: core.EdgeDirect},
	}
	fps := map[string]string{"b/b.go": "fp-b-1"}
	return reviewkey.Inputs{
		Manifest: []reviewkey.ManifestEntry{
			{Path: "a/a.go", SHA256: "sha-a-1"},
		},
		Nodes:      nodes,
		TypedEdges: edges,
		Fingerprint: func(path string) (string, bool) {
			h, ok := fps[path]
			return h, ok
		},
		RulesFiles: map[string][]byte{".fathom/rules/archconformance.cue": []byte("rules v1")},
		PlansFiles: map[string][]byte{"docs/plans/x.md": []byte("plan v1")},
		Tooling:    map[string]string{"fathom_version": "v1"},

		ServiceWatermark: "commit-1",
		StoreWatermark:   "commit-1@100",
	}
}

func compute(t *testing.T, in reviewkey.Inputs) reviewkey.Key {
	t.Helper()
	k, err := reviewkey.Compute(context.Background(), in)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	return k
}

func TestCompute_CompleteBaseline(t *testing.T) {
	k := compute(t, baseInputs())
	if !k.Complete || len(k.Unknown) != 0 {
		t.Fatalf("baseline = complete=%v unknown=%v, want complete with no unknowns", k.Complete, k.Unknown)
	}
	if k.SchemaVersion != reviewkey.SchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", k.SchemaVersion, reviewkey.SchemaVersion)
	}
	wantComponents := []string{"closure", "index", "manifest", "plans", "rules", "tooling"}
	var gotComponents []string
	for c := range k.Components {
		gotComponents = append(gotComponents, c)
	}
	sort.Strings(gotComponents)
	if !reflect.DeepEqual(gotComponents, wantComponents) {
		t.Errorf("components = %v, want %v", gotComponents, wantComponents)
	}
}

func TestCompute_Deterministic(t *testing.T) {
	a := compute(t, baseInputs())
	b := compute(t, baseInputs())
	if a.Key != b.Key {
		t.Fatalf("identical inputs produced different keys: %s vs %s", a.Key, b.Key)
	}
	if !reflect.DeepEqual(a.Components, b.Components) {
		t.Fatalf("identical inputs produced different components: %v vs %v", a.Components, b.Components)
	}
}

func TestCompute_UnreadableManifestPathExcludedAndUnknown(t *testing.T) {
	in := baseInputs()
	in.Manifest = append(in.Manifest, reviewkey.ManifestEntry{Path: "docs/missing.md", Unreadable: true})
	k := compute(t, in)
	if k.Complete {
		t.Fatal("an unreadable manifest path must make Complete false")
	}
	found := false
	for _, u := range k.Unknown {
		if u == "manifest: docs/missing.md is unreadable" {
			found = true
		}
	}
	if !found {
		t.Errorf("Unknown = %v, want an entry naming docs/missing.md", k.Unknown)
	}
	// The manifest digest must not hash the unreadable path as an empty
	// file: excluding it must produce the SAME manifest digest as a run
	// that never named the path at all.
	without := baseInputs()
	withoutKey := compute(t, without)
	if k.Components["manifest"] != withoutKey.Components["manifest"] {
		t.Errorf("manifest digest with an excluded unreadable entry = %q, want it to match the digest with the entry simply absent (%q)",
			k.Components["manifest"], withoutKey.Components["manifest"])
	}
}

func TestDiff_MutationMatrix(t *testing.T) {
	base := compute(t, baseInputs())

	cases := []struct {
		name    string
		mutate  func(in *reviewkey.Inputs)
		wantHit map[string]bool
	}{
		{
			name: "source_only_edit",
			mutate: func(in *reviewkey.Inputs) {
				in.Manifest = []reviewkey.ManifestEntry{{Path: "a/a.go", SHA256: "sha-a-2"}}
			},
			wantHit: map[string]bool{"manifest": true},
		},
		{
			name: "dependency_transitive_edit",
			mutate: func(in *reviewkey.Inputs) {
				fps := map[string]string{"b/b.go": "fp-b-2"}
				in.Fingerprint = func(path string) (string, bool) {
					h, ok := fps[path]
					return h, ok
				}
			},
			wantHit: map[string]bool{"closure": true},
		},
		{
			name: "untracked_file_added_to_manifest",
			mutate: func(in *reviewkey.Inputs) {
				// A new file with no nodes/edges of its own: only the
				// manifest digest should move.
				in.Manifest = append(append([]reviewkey.ManifestEntry(nil), in.Manifest...),
					reviewkey.ManifestEntry{Path: "c/c.go", SHA256: "sha-c-1"})
			},
			wantHit: map[string]bool{"manifest": true},
		},
		{
			name: "rule_pack_edit",
			mutate: func(in *reviewkey.Inputs) {
				in.RulesFiles = map[string][]byte{".fathom/rules/archconformance.cue": []byte("rules v2")}
			},
			wantHit: map[string]bool{"rules": true},
		},
		{
			name: "plan_edit",
			mutate: func(in *reviewkey.Inputs) {
				in.PlansFiles = map[string][]byte{"docs/plans/x.md": []byte("plan v2")}
			},
			wantHit: map[string]bool{"plans": true},
		},
		{
			name: "tool_version_change",
			mutate: func(in *reviewkey.Inputs) {
				in.Tooling = map[string]string{"fathom_version": "v2"}
			},
			wantHit: map[string]bool{"tooling": true},
		},
		{
			name: "index_rebuild_watermark_change",
			mutate: func(in *reviewkey.Inputs) {
				in.ServiceWatermark = "commit-2"
				in.StoreWatermark = "commit-2@200"
			},
			wantHit: map[string]bool{"index": true},
		},
	}

	all := []string{"manifest", "closure", "rules", "plans", "tooling", "index"}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := baseInputs()
			c.mutate(&in)
			mutated := compute(t, in)
			diff := reviewkey.Diff(base, mutated)

			gotHit := map[string]bool{}
			for _, d := range diff {
				gotHit[d] = true
			}
			for _, name := range all {
				want := c.wantHit[name]
				if gotHit[name] != want {
					t.Errorf("component %q flipped=%v, want %v (diff=%v)", name, gotHit[name], want, diff)
				}
			}
		})
	}
}

func TestCompute_MissingTypedEdgesIsUnknown(t *testing.T) {
	in := baseInputs()
	in.TypedEdges = nil // the typed pass never ran over package "a"
	k := compute(t, in)
	if k.Complete {
		t.Fatal("a changed package with no typed edges recorded must make Complete false")
	}
	found := false
	for _, u := range k.Unknown {
		if u == "dependency closure: a/a has no typed edges" || u == "dependency closure: a has no typed edges" {
			found = true
		}
	}
	if !found {
		t.Errorf("Unknown = %v, want a dependency-closure entry naming package a", k.Unknown)
	}
	// The closure key still computes (over an empty reached set) — it must
	// not be "" or absent.
	if k.Components["closure"] == "" {
		t.Error("closure component must still compute even when incomplete")
	}
}

func TestCompute_ChangedSincePrior(t *testing.T) {
	priorInputs := baseInputs()
	prior := compute(t, priorInputs)

	in := baseInputs()
	in.Tooling = map[string]string{"fathom_version": "v2"}
	in.Prior = &prior
	current := compute(t, in)

	if len(current.ChangedSincePrior) != 1 || current.ChangedSincePrior[0] != "tooling" {
		t.Errorf("ChangedSincePrior = %v, want [tooling]", current.ChangedSincePrior)
	}
}
