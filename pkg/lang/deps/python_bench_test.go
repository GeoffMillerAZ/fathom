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

type pythonOracleRow struct {
	Path          string
	Edges         []depbench.TruthEdge
	Gap           string
	OptimizedAway int `json:"optimized_away"`
}

type pythonOracleSubject struct {
	Manifest json.RawMessage
	Rows     []pythonOracleRow
}

func TestPythonDependencyReading(t *testing.T) {
	if os.Getenv("FATHOM_DEPS_PYTHON_READ") != "1" {
		t.Skip("set FATHOM_DEPS_PYTHON_READ=1 to request the pinned native reading")
	}
	corpus := requiredPythonEnv(t, "FATHOM_DEPS_PYTHON_CORPUS")
	relocated := requiredPythonEnv(t, "FATHOM_DEPS_PYTHON_RELOCATED")
	scratch := requiredPythonEnv(t, "FATHOM_DEPS_PYTHON_EVIDENCE")
	executable := requiredPythonEnv(t, "FATHOM_DEPS_PYTHON_EXECUTABLE")
	if corpus == relocated {
		t.Fatal("B3 requires a second corpus checkout path")
	}

	// The adapter obtains truth before a Reader exists. Separate captures
	// also prove that its trace and coverage are insensitive to location.
	first := pythonNativeTruth(t, executable, corpus, scratch, "oracle-a")
	second := pythonNativeTruth(t, executable, relocated, scratch, "oracle-b")
	if !bytes.Equal(first, second) {
		t.Fatal("oracle trace changed between pinned checkout paths")
	}
	var native map[string]pythonOracleSubject
	if err := json.Unmarshal(first, &native); err != nil {
		t.Fatal(err)
	}
	reader, err := deps.New(deps.TrustUnmeasured("python"))
	if err != nil {
		t.Fatal(err)
	}

	var pooled depbench.Observation
	names := make([]string, 0, len(native))
	for name := range native {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		firstRoot, err := os.Stat(filepath.Join(corpus, name))
		if err != nil {
			t.Fatal(err)
		}
		secondRoot, err := os.Stat(filepath.Join(relocated, name))
		if err != nil {
			t.Fatal(err)
		}
		if os.SameFile(firstRoot, secondRoot) {
			t.Fatalf("%s: B3 subject checkouts identify the same directory", name)
		}
		subject := native[name]
		inputs := make([]deps.Input, 0, len(subject.Rows))
		observation := depbench.Observation{}
		coverageGaps := map[string]int{}
		optimizedAway := 0
		for _, row := range subject.Rows {
			inputs = append(inputs, deps.Input{Path: row.Path})
			if row.Gap != "" {
				coverageGaps[row.Gap]++
				continue
			}
			observation.Truth = append(observation.Truth, depbench.TruthFile{Path: row.Path, Edges: row.Edges})
			optimizedAway += row.OptimizedAway
		}
		read := func(root string) []deps.FileDeps {
			t.Helper()
			out, err := reader.ReadFiles(t.Context(), os.DirFS(filepath.Join(root, name)), inputs)
			if err != nil {
				t.Fatal(err)
			}
			return out
		}
		observation.Files = read(corpus)
		slices.Reverse(inputs)
		observation.Repeat = read(corpus)
		observation.Relocated = read(relocated)
		summaries := asJSON(t, depbench.Score(observation))
		t.Logf("%s %s gaps=%s optimized-away=%d manifest=%s", name, summaries,
			asJSON(t, coverageGaps), optimizedAway, subject.Manifest)
		writePythonEvidence(t, scratch, name+"-reader.json", asJSON(t, observation))
		addPythonSubject(&pooled, name, observation)
	}
	score := depbench.Score(pooled)
	t.Logf("pooled %s", asJSON(t, score))
	if len(names) < 3 || score.Files < 300 {
		t.Fatalf("inconclusive population: %d subjects, %d Python files", len(names), score.Files)
	}
	if !score.B3 {
		t.Error("B3 failed: outputs differ by run order or checkout path")
	}
	// B1/B2 determine the published admission. A test that always fails on
	// a measured miss would hide the reproducible detect-only reading.
	t.Logf("candidate admission B1=%t B2=%t B3=%t", score.B1, score.B2, score.B3)
}

func requiredPythonEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("native reading requires %s", name)
	}
	path, err := filepath.Abs(value)
	if err != nil {
		t.Fatal(err)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return path
}

func pythonNativeTruth(t *testing.T, executable, corpus, scratch, label string) []byte {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), executable, "-I", "-B", "-S",
		filepath.Join("..", "..", "..", "calibration", "oracles", "deps-python", "modulefinder_oracle.py"), corpus)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	writePythonEvidence(t, scratch, label+".stderr", stderr.Bytes())
	if err != nil {
		t.Fatalf("pinned modulefinder failed: %v (see %s)", err, label+".stderr")
	}
	writePythonEvidence(t, scratch, label+".json", data)
	return data
}

func writePythonEvidence(t *testing.T, scratch, name string, data []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(scratch, name), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// Subject prefixes are confined to the scoring copy so equal relative
// filenames in different repositories cannot alias each other's native truth.
func addPythonSubject(pooled *depbench.Observation, subject string, o depbench.Observation) {
	label := func(path string) string { return subject + "/" + path }
	addFiles := func(dst *[]deps.FileDeps, source []deps.FileDeps) {
		for _, f := range source {
			f.Path = label(f.Path)
			f.Edges = slices.Clone(f.Edges)
			for i := range f.Edges {
				e := &f.Edges[i]
				e.From = label(e.From)
				if e.Resolution == deps.ResolvedFile {
					e.To = label(e.To)
				}
			}
			f.Unknowns = slices.Clone(f.Unknowns)
			for i := range f.Unknowns {
				f.Unknowns[i].From = label(f.Unknowns[i].From)
			}
			*dst = append(*dst, f)
		}
	}
	for _, row := range o.Truth {
		row.Path = label(row.Path)
		row.Edges = slices.Clone(row.Edges)
		for i := range row.Edges {
			if row.Edges[i].Resolution == deps.ResolvedFile {
				row.Edges[i].To = label(row.Edges[i].To)
			}
		}
		pooled.Truth = append(pooled.Truth, row)
	}
	addFiles(&pooled.Files, o.Files)
	addFiles(&pooled.Repeat, o.Repeat)
	addFiles(&pooled.Relocated, o.Relocated)
}
