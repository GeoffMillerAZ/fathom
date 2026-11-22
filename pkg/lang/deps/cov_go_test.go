package deps_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
	"github.com/geoffmilleraz/fathom/pkg/lang/deps/internal/depbench"
)

func TestCOVGoInventory(t *testing.T) {
	tree := map[string][]byte{
		"go.mod":        []byte("module example.com/local\n"),
		"a/a.go":        []byte("package a\n"),
		"nested/go.mod": []byte("module example.com/other\n"),
		"nested/b/b.go": []byte("package b\n"),
	}
	inventory := depbench.COVGoInventory(tree)
	for _, name := range []string{"go:example.com/local/a", "go:example.com/other/b"} {
		if !inventory[name] {
			t.Fatalf("missing tracked Go package identity %s", name)
		}
	}
	if inventory["go:example.com/local/nested/b"] {
		t.Fatal("crossed a nested module boundary")
	}
	f := deps.FileDeps{Path: "a/a.go", Complete: true, Edges: []deps.Edge{{Kind: deps.KindImport, Resolution: deps.ResolvedExternal, To: "go:example.com/other/b"}}}
	files := []deps.FileDeps{f}
	score := depbench.COVGoScore(depbench.Observation{Files: files, Repeat: files, Relocated: files, Truth: []depbench.TruthFile{{Path: f.Path}}}, inventory)
	if score.B1 || score.ExternalAsLocal != 1 {
		t.Fatalf("local external claim escaped COV-D6: %+v", score)
	}
}

func TestCOVGoModulePrefix(t *testing.T) {
	inventory := depbench.COVGoInventory(map[string][]byte{
		"go.mod": []byte("module example.com/local\n"),
	})
	for _, target := range []string{"go:example.com/local", "go:example.com/local/sub"} {
		f := deps.FileDeps{Path: "x.go", Complete: true, Edges: []deps.Edge{{Kind: deps.KindImport, Resolution: deps.ResolvedExternal, To: target}}}
		files := []deps.FileDeps{f}
		score := depbench.COVGoScore(depbench.Observation{Files: files, Repeat: files, Relocated: files, Truth: []depbench.TruthFile{{Path: f.Path}}}, inventory)
		if score.B1 || score.ExternalAsLocal != 1 {
			t.Fatalf("tracked module external claim escaped: %+v", score)
		}
	}
	f := deps.FileDeps{Path: "x.go", Complete: true, Edges: []deps.Edge{{Kind: deps.KindImport, Resolution: deps.ResolvedExternal, To: "go:example.com/locality/sub"}}}
	files := []deps.FileDeps{f}
	score := depbench.COVGoScore(depbench.Observation{Files: files, Repeat: files, Relocated: files, Truth: []depbench.TruthFile{{Path: f.Path}}}, inventory)
	if !score.B1 || score.ExternalAsLocal != 0 {
		t.Fatalf("module prefix crossed component boundary: %+v", score)
	}
}

// TestCOVGoNativeScoring tests the admission adapter against actual product
// evidence, then perturbs evidence and source independently. Corpus data is absent.
func TestCOVGoNativeScoring(t *testing.T) {
	if os.Getenv("FATHOM_COV_GO_CONTROL") != "1" {
		t.Skip("opt-in pinned native scoring control")
	}
	root := copyControl(t)
	script, err := filepath.Abs(filepath.Join(covOracleDir, "cov_go.py"))
	if err != nil {
		t.Fatal(err)
	}
	capture := func() ([]depbench.TruthFile, []byte) {
		t.Helper()
		data, err := exec.CommandContext(t.Context(), "python3", "-B", script, "--fixture", root).Output()
		if err != nil {
			t.Fatal("native scoring fixture failed")
		}
		var native struct {
			Truth []depbench.TruthFile
			Raw   json.RawMessage
			Gaps  map[string]string
		}
		if json.Unmarshal(data, &native) != nil || len(native.Gaps) != 0 {
			t.Fatal("native scoring fixture lacks truth")
		}
		return native.Truth, native.Raw
	}
	truth, raw := capture()
	reader, err := deps.New(deps.TrustUnmeasured("go"))
	if err != nil {
		t.Fatal(err)
	}
	files, err := reader.ReadFiles(t.Context(), os.DirFS(root), []deps.Input{{Path: "app/main.go"}, {Path: "app/companion.go"}, {Path: "lib/lib.go"}, {Path: "data/data.go"}})
	if err != nil {
		t.Fatal(err)
	}
	o := depbench.Observation{Files: files, Repeat: files, Relocated: files, Truth: truth}
	score := depbench.COVGoScore(o, nil)
	if !score.B1 || !score.B2 || !score.B3 || score.Complete != 3 {
		t.Fatalf("native scoring control: %+v", score)
	}
	o.Files = slices.Clone(files)
	for i, f := range o.Files {
		if f.Path == "app/main.go" {
			o.Files[i].Edges = slices.DeleteFunc(slices.Clone(f.Edges), func(e deps.Edge) bool { return e.Kind == deps.KindImport })
		}
	}
	if mutated := depbench.COVGoScore(o, nil); mutated.B1 || mutated.Missing != 1 {
		t.Fatalf("dropped product target escaped: %+v", mutated)
	}
	_, after := capture()
	if !bytes.Equal(raw, after) {
		t.Fatal("product evidence mutation changed raw native evidence")
	}
	if err := os.WriteFile(filepath.Join(root, "app", "main.go"), []byte("package app\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, after = capture()
	if bytes.Equal(raw, after) {
		t.Fatal("source mutation did not change raw native evidence")
	}
}
