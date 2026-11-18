package deps_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
	"github.com/geoffmilleraz/fathom/pkg/lang/deps/internal/depbench"
)

func TestGoDependencyOracle(t *testing.T) {
	root, second := copyControl(t), copyControl(t)
	truth := goTruth(t, root)
	if len(truth) != 4 {
		t.Fatalf("native coverage: %d files", len(truth))
	}
	r, err := deps.New(deps.TrustUnmeasured("go"))
	if err != nil {
		t.Fatal(err)
	}
	read := func(root string, reverse bool) []deps.FileDeps {
		t.Helper()
		inputs := []deps.Input{{Path: "app/main.go"}, {Path: "lib/lib.go"}, {Path: "app/companion.go"}, {Path: "data/data.go"}}
		if reverse {
			slices.Reverse(inputs)
		}
		out, err := r.ReadFiles(context.Background(), os.DirFS(root), inputs)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}

	observation := depbench.Observation{Files: read(root, false), Repeat: read(root, true), Relocated: read(second, false), Truth: truth}
	control := depbench.Score(observation)
	t.Logf("Go %s (%s/%s) control: %+v", runtime.Version(), runtime.GOOS, runtime.GOARCH, control)
	if !control.B1 || !control.B2 || !control.B3 || control.Files != 4 ||
		control.Complete != 3 || control.Expected != 6 || control.CompleteEdges != 6 ||
		control.Unknowns != 1 || control.Extra != 0 {
		t.Fatalf("control result: %+v", control)
	}
	var embedMatched bool
	for _, f := range observation.Files {
		if f.Path != "data/data.go" {
			continue
		}
		for _, e := range f.Edges {
			embedMatched = embedMatched || e.Kind == deps.KindEmbed && e.To == "data/payload.txt"
		}
	}
	if !embedMatched {
		t.Fatal("detect-only embed case lost the native embed input")
	}

	t.Run("B1", func(t *testing.T) {
		sample := observation
		sample.Truth = slices.Clone(sample.Truth)
		for i, f := range sample.Truth {
			if f.Path == "app/main.go" {
				sample.Truth[i].Edges = append(slices.Clone(f.Edges), depbench.TruthEdge{
					Kind: deps.KindImport, Raw: "example.com/control/data", Resolution: deps.ResolvedDirectory, To: "data",
				})
			}
		}
		s := depbench.Score(sample)
		if s.B1 || !s.B2 || s.Missing != 1 || s.Wrong != 0 {
			t.Fatalf("missing native target was not rejected: %+v", s)
		}
	})
	t.Run("B2", func(t *testing.T) {
		sample := observation
		sample.Truth = slices.Clone(sample.Truth)
		sample.Files = slices.Clone(sample.Files)
		for i, f := range sample.Truth {
			if f.Path == "app/main.go" {
				sample.Truth[i].Edges = slices.Clone(f.Edges)
				for j, e := range f.Edges {
					if e.Kind == deps.KindImport {
						sample.Truth[i].Edges[j].To = "data"
					}
				}
			}
		}
		// Leave the required target in the observed set under a different
		// construct. B1 can pass while B2 rejects the mismatched binding.
		for i, f := range sample.Files {
			if f.Path == "app/main.go" {
				sample.Files[i].Edges = append(slices.Clone(f.Edges), deps.Edge{
					Kind: deps.KindImport, Raw: "different-operand", Resolution: deps.ResolvedDirectory, To: "data",
				})
			}
		}
		s := depbench.Score(sample)
		if !s.B1 || s.B2 || s.Missing != 0 || s.Wrong != 1 {
			t.Fatalf("wrong native binding was not rejected: %+v", s)
		}
	})
	if after := goTruth(t, root); !bytes.Equal(asJSON(t, truth), asJSON(t, after)) {
		t.Fatal("perturbing the observation changed the independent truth")
	}
	if err := os.WriteFile(filepath.Join(second, "app", "main.go"), []byte("package app\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if after := goTruth(t, second); bytes.Equal(asJSON(t, truth), asJSON(t, after)) {
		t.Fatal("removing the source import did not change independent truth")
	}
}

func asJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

type nativePackage struct {
	Dir, ImportPath                     string
	Match, GoFiles, Imports, EmbedFiles []string
	ImportMap                           map[string]string
	Module                              *struct{ GoMod string }
}

// Package membership supplies sibling build inputs. The named-file listing
// supplies imports and embeds for exactly that file, independent of the reader.
// Imports are compared by native directory, without expanding its files.
func goTruth(t *testing.T, root string) []depbench.TruthFile {
	t.Helper()
	packages := goList(t, root, "./...")
	var out []depbench.TruthFile
	for _, p := range packages {
		if len(p.Match) == 0 || p.Module == nil {
			continue
		}
		for _, name := range p.GoFiles {
			from := logical(t, root, filepath.Join(p.Dir, name))
			row := depbench.TruthFile{Path: from, Edges: []depbench.TruthEdge{}}
			mod := logical(t, root, p.Module.GoMod)
			row.Edges = append(row.Edges, depbench.TruthEdge{Kind: deps.KindConfigRef, Raw: mod, Resolution: deps.ResolvedFile, To: mod})
			for _, sibling := range p.GoFiles {
				if sibling != name {
					to := logical(t, root, filepath.Join(p.Dir, sibling))
					row.Edges = append(row.Edges, depbench.TruthEdge{Kind: deps.KindBuildInput, Raw: to, Resolution: deps.ResolvedFile, To: to})
				}
			}
			single := goList(t, root, from)
			byName := make(map[string]nativePackage)
			for _, q := range single {
				byName[q.ImportPath] = q
			}
			entry, ok := byName["command-line-arguments"]
			if !ok || len(entry.GoFiles) != 1 || entry.GoFiles[0] != name || len(entry.ImportMap) != 0 {
				t.Fatalf("native named-file fixture has unexpected mapping: %+v", entry)
			}
			for _, operand := range entry.Imports {
				q, ok := byName[operand]
				if !ok {
					t.Fatalf("native import missing its package: %s", operand)
				}
				if rel, err := filepath.Rel(root, q.Dir); err == nil && filepath.IsLocal(rel) {
					row.Edges = append(row.Edges, depbench.TruthEdge{Kind: deps.KindImport, Raw: operand, Resolution: deps.ResolvedDirectory, To: filepath.ToSlash(rel)})
				}
			}
			for _, name := range entry.EmbedFiles {
				// This fixture uses a single literal operand, with no remapping.
				to := logical(t, root, filepath.Join(entry.Dir, name))
				row.Edges = append(row.Edges, depbench.TruthEdge{Kind: deps.KindEmbed, Raw: name, Resolution: deps.ResolvedFile, To: to})
			}
			out = append(out, row)
		}
	}
	return out
}

func logical(t *testing.T, root, full string) string {
	t.Helper()
	rel, err := filepath.Rel(root, full)
	if err != nil || !filepath.IsLocal(rel) {
		t.Fatalf("native target is outside the fixture: %s", full)
	}
	return filepath.ToSlash(rel)
}

func goList(t *testing.T, root, subject string) []nativePackage {
	t.Helper()
	cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "list", "-deps", "-json", subject)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=-mod=readonly", "GOTOOLCHAIN=local", "GOPROXY=off",
		"GOSUMDB=off", "GOWORK=off", "CGO_ENABLED=0", "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, &stderr)
	}
	var pkgs []nativePackage
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		var p nativePackage
		if err := dec.Decode(&p); err == io.EOF {
			return pkgs
		} else if err != nil {
			t.Fatal(err)
		}
		pkgs = append(pkgs, p)
	}
}

func copyControl(t *testing.T) string {
	t.Helper()
	root := nativeGoFixtureRoot(t)
	source := os.DirFS("testdata/control")
	err := fs.WalkDir(source, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dest := filepath.Join(root, filepath.FromSlash(name))
		if d.IsDir() {
			return os.MkdirAll(dest, 0o700)
		}
		b, err := fs.ReadFile(source, name)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, b, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// Native Go commands report physical paths when the gate places temporary
// files under a symlink. Compare their results with the physical fixture root.
func nativeGoFixtureRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestGoDependencyOracleTruthCoverage(t *testing.T) {
	file := deps.FileDeps{Path: "x.go", Complete: true}
	truth := depbench.TruthFile{Path: "x.go"}
	observation := depbench.Observation{Files: []deps.FileDeps{file}, Repeat: []deps.FileDeps{file},
		Relocated: []deps.FileDeps{{Path: "other.go", Complete: true}}}
	if got := depbench.Score(observation); got.B1 || got.B2 || got.B3 || got.CompleteWithoutOracle != 1 {
		t.Fatalf("missing truth or deterministic replay accepted: %+v", got)
	}
	observation.Truth = []depbench.TruthFile{truth, truth}
	if got := depbench.Score(observation); got.B1 || got.B2 || got.CompleteWithOracle != 0 {
		t.Fatalf("duplicate truth accepted: %+v", got)
	}

	file.Edges = []deps.Edge{{Kind: deps.KindInclude, Raw: "extra", Resolution: deps.ResolvedFile, To: "extra"}}
	file.Unknowns = []deps.Unknown{{Code: deps.UnknownComputedPath}}
	file.Complete = false
	observation.Files = []deps.FileDeps{file}
	observation.Truth = []depbench.TruthFile{truth}
	if got := depbench.Score(observation); got.B1 || got.B2 || got.B4Extra != 1 ||
		got.Unknowns != 1 || got.UnknownByCode[deps.UnknownComputedPath] != 1 {
		t.Fatalf("missing B4 evidence or vacuous admission: %+v", got)
	}
}

func TestDependencyB2WithheldOperands(t *testing.T) {
	for _, tc := range []struct {
		name                string
		directives          string
		lines               [2]int
		swap                bool
		passB2              bool
		wrong, inconclusive int
	}{
		{"correct", "//go:embed \"one file\"\n//go:embed \"two file\"", [2]int{3, 4}, false, true, 0, 0},
		{"swapped", "//go:embed \"one file\"\n//go:embed \"two file\"", [2]int{3, 4}, true, false, 2, 0},
		{"same line", "//go:embed \"one file\" \"two file\"", [2]int{3, 3}, true, false, 0, 2},
		{"missing oracle line", "//go:embed \"one file\"\n//go:embed \"two file\"", [2]int{0, 4}, false, false, 0, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := fstest.MapFS{
				"go.mod":   &fstest.MapFile{Data: []byte("module example.com/control\n")},
				"one file": &fstest.MapFile{},
				"two file": &fstest.MapFile{},
			}
			r, err := deps.New(deps.TrustUnmeasured("go"), deps.WithResolver("go", fixtureEmbedResolver{}))
			if err != nil {
				t.Fatal(err)
			}
			src := []byte("package assets\nimport _ \"embed\"\n" + tc.directives + "\nvar assets embed.FS\n")
			f, err := r.ReadFile(context.Background(), repo, deps.Input{Path: "assets.go", Dialect: "go"}, src)
			if err != nil || !f.Complete {
				t.Fatalf("fixture must be complete: %+v, %v", f, err)
			}
			for i, e := range f.Edges {
				if e.Kind != deps.KindEmbed {
					continue
				}
				if e.Raw != "***" {
					t.Fatal("fixture must withhold the raw operand")
				}
				if tc.swap {
					if e.To == "one file" {
						f.Edges[i].To = "two file"
					} else {
						f.Edges[i].To = "one file"
					}
				}
			}
			truth := []depbench.TruthEdge{
				{Kind: deps.KindConfigRef, Raw: "go.mod", Line: 0, Resolution: deps.ResolvedFile, To: "go.mod"},
				{Kind: deps.KindEmbed, Raw: "one file", Line: tc.lines[0], Resolution: deps.ResolvedFile, To: "one file"},
				{Kind: deps.KindEmbed, Raw: "two file", Line: tc.lines[1], Resolution: deps.ResolvedFile, To: "two file"},
			}
			files := []deps.FileDeps{f}
			s := depbench.Score(depbench.Observation{Files: files, Repeat: files, Relocated: files,
				Truth: []depbench.TruthFile{{Path: f.Path, Edges: truth}}})
			if !s.B1 || s.B2 != tc.passB2 || s.Wrong != tc.wrong || s.B2Inconclusive != tc.inconclusive ||
				s.Extra != 0 || s.Missing != 0 {
				t.Errorf("withheld operands: %+v", s)
			}
		})
	}
}

type fixtureEmbedResolver struct{}

func (fixtureEmbedResolver) Resolve(context.Context, deps.ResolveRequest) deps.ResolveResult {
	return deps.ResolveResult{Edges: []deps.Edge{{Kind: deps.KindImport, To: "go:embed", Resolution: deps.ResolvedExternal, Version: "v1"}}}
}

func TestDependencyB2BindingIdentity(t *testing.T) {
	for _, tc := range []struct {
		name        string
		raw         [2]string
		oracleLines [2]int
		readerLines [2]int
		wantPass    bool
	}{
		{"conflicting lines", [2]string{"first", "second"}, [2]int{4, 8}, [2]int{8, 4}, false},
		{"repeated operand without oracle lines", [2]string{"same", "same"}, [2]int{}, [2]int{4, 8}, false},
		{"multiple targets from one located operand", [2]string{"same", "same"}, [2]int{4, 4}, [2]int{4, 4}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			targets := [2]string{"a.txt", "b.txt"}
			var truth []depbench.TruthEdge
			var edges []deps.Edge
			for i, raw := range tc.raw {
				truth = append(truth, depbench.TruthEdge{
					Kind: deps.KindInclude, Raw: raw, Line: tc.oracleLines[i],
					To: targets[i], Resolution: deps.ResolvedFile,
				})
				// Keep the B1 target set while reversing construct-to-target bindings.
				edges = append(edges, deps.Edge{
					From: "source.txt", Kind: deps.KindInclude, Raw: raw, Line: tc.readerLines[i],
					To: targets[1-i], Resolution: deps.ResolvedFile,
				})
			}
			files := []deps.FileDeps{{Path: "source.txt", Complete: true, Edges: edges}}
			score := depbench.Score(depbench.Observation{
				Files: files, Repeat: files, Relocated: files,
				Truth: []depbench.TruthFile{{Path: "source.txt", Edges: truth}},
			})
			if !score.B1 || score.B2 != tc.wantPass || !tc.wantPass && score.Wrong+score.B2Inconclusive == 0 {
				t.Fatalf("B1=%t B2=%t, wrong=%d inconclusive=%d. Want B1=true B2=%t",
					score.B1, score.B2, score.Wrong, score.B2Inconclusive, tc.wantPass)
			}
		})
	}
}

func TestDependencyB2NativeExternalTarget(t *testing.T) {
	files := []deps.FileDeps{{
		Path: "source.txt", Complete: true,
		Edges: []deps.Edge{{From: "source.txt", Kind: deps.KindImport, Raw: "dependency",
			Line: 1, Resolution: deps.ResolvedFile, To: "local.txt"}},
	}}
	score := depbench.Score(depbench.Observation{
		Files: files, Repeat: files, Relocated: files,
		Truth: []depbench.TruthFile{{Path: "source.txt", Edges: []depbench.TruthEdge{{
			Kind: deps.KindImport, Raw: "dependency", Line: 1, Resolution: deps.ResolvedExternal, To: "example:dependency",
		}}}},
	})
	if !score.B1 || score.Expected != 0 || score.Missing != 0 || score.B2 || score.Wrong != 1 {
		t.Fatalf("native external input is outside B1 but refutes a local binding under B2: %+v", score)
	}
}
