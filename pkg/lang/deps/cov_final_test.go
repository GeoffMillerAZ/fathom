package deps_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
	"github.com/geoffmilleraz/fathom/pkg/lang/deps/internal/depbench"
)

func TestCOVFinalSynthetic(t *testing.T) {
	tree := fstest.MapFS{
		"ok.js":                {Data: []byte("export const ok = 1\n")},
		"nested/jsconfig.json": {Data: []byte(`{"compilerOptions":{"resolveJsonModule":true}}`)},
		"nested/outside.json":  {Data: []byte("{}\n")},
		"nested/entry.js":      {Data: []byte("import './outside.json'\n")},
		"bad[name].js":         {Data: []byte("export const bad = 1\n")},
	}
	paths := []string{"bad[name].js", "nested/entry.js", "ok.js"}
	truth := []depbench.TruthFile{
		{Path: paths[0]},
		{Path: paths[1], Edges: []depbench.TruthEdge{
			{Kind: deps.KindImport, Raw: "./outside.json", Line: 1, Resolution: deps.ResolvedFile, To: "nested/outside.json"},
		}},
		{Path: paths[2]},
	}
	row, _, _ := covFinalRead(t, covSubject{tsjsSubject: tsjsSubject{Dialect: "javascript"}}, tree, tree, paths, truth)
	if row.Score.Files != 3 || row.Score.Complete != 2 || row.Score.RejectedInputs != 1 ||
		row.Score.RejectedOracleFiles != 1 || !row.Score.B1 || !row.Score.B2 || !row.Score.B3 ||
		row.Closure.Selected != 3 || row.Closure.Complete != 1 || row.Closure.OnlyUnmeasured != 1 || row.Closure.RejectedBlocked != 1 {
		t.Fatalf("synthetic campaign did not account for selected inputs and their closures: %+v", row)
	}
}

func TestCOVFinalRehearsalGitStore(t *testing.T) {
	const fixture = "FATHOM_COV_REHEARSAL_GIT_FIXTURE"
	if root := os.Getenv(fixture); root != "" {
		covFinalRehearsalDirectories(t, root, filepath.Join(root, "primary", "subject"), filepath.Join(root, "relocated", "subject"))
		return
	}
	root := t.TempDir()
	primary := filepath.Join(root, "primary", "subject")
	relocated := filepath.Join(root, "relocated", "subject")
	for _, path := range []string{primary, relocated} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := exec.CommandContext(t.Context(), "git", "-C", path, "init", "-q").Run(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, ".git", "HEAD"), []byte("1111111111111111111111111111111111111111\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(primary, ".git", "commondir"), []byte(filepath.Join(relocated, ".git")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestCOVFinalRehearsalGitStore$")
	cmd.Env = append(os.Environ(), fixture+"="+root)
	output, err := cmd.CombinedOutput()
	if err == nil || !bytes.Contains(output, []byte("requires a local Git object store")) {
		t.Fatalf("redirected rehearsal object store was not refused: %s", output)
	}
}

func TestCOVFinalDualTruth(t *testing.T) {
	old := pythonOracleSubject{Rows: []pythonOracleRow{{Path: "a.py"}, {Path: "b.py", Gap: "old-gap"}, {Path: "c.py", Gap: "old-gap"}}}
	initializer := depbench.TruthEdge{Kind: deps.KindImport, Raw: "pkg", Line: 1, Resolution: deps.ResolvedFile, To: "pkg/__init__.py"}
	extended := pythonOracleSubject{Rows: []pythonOracleRow{
		{Path: "a.py"}, {Path: "b.py", Edges: []depbench.TruthEdge{initializer}}, {Path: "c.py", Edges: []depbench.TruthEdge{initializer}},
	}}
	comparison := map[string]covPythonCrossCheck{
		"b.py": {Class: "initializer-plus-submodule", Cohort: "new", Truth: []string{"pkg/__init__.py", "pkg/child.py"}},
		"c.py": {Class: "ambiguous-site", Cohort: "new"},
	}
	truth, counts := covFinalDualTruth(t, old, extended, comparison)
	if len(truth) != 2 || len(truth[1].Edges) != 2 || truth[1].Edges[1].To != "pkg/child.py" ||
		truth[1].Edges[1].Line != 1 || counts["added/ambiguous-site"] != 1 {
		t.Fatalf("wrong two-source truth: %+v, %+v", truth, counts)
	}
}

// TestCOVFinalMeasure is the pipe-only worker for the leased instrument.
func TestCOVFinalMeasure(t *testing.T) {
	if os.Getenv("FATHOM_COV_FINAL_WORKER") != "1" {
		t.Skip("COV final measurement requires the campaign runner")
	}
	var config covFinalConfig
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	if err != nil || json.Unmarshal(raw, &config) != nil {
		t.Fatal("COV final worker configuration unavailable")
	}
	var pools []string
	var expected map[string][]covSubject
	switch config.Mode {
	case "preflight":
		pools = []string{"development"}
	case "initial", "rehearse":
		pools = []string{"development", "holdout", "confirmation"}
	default:
		t.Fatal("COV final worker operation denied")
	}
	if config.Mode != "rehearse" {
		expected = covExpectedSubjects(t)
	}
	if config.Mode == "initial" {
		pin, err := os.ReadFile(filepath.Join(covOracleDir, "cov_confirmation.json"))
		var confirmation []covSubject
		if err != nil || json.Unmarshal(pin, &confirmation) != nil || len(confirmation) != 1 ||
			confirmation[0].Dialect != "typescript" || confirmation[0].Files != 979 {
			t.Fatal("COV confirmation pin unavailable")
		}
		expected["confirmation"] = confirmation
	}
	result := map[string]map[string]covFinalRow{}
	for _, pool := range pools {
		response := config.call(t, "_active", pool)
		var subjects []covSubject
		if config.Mode == "rehearse" {
			if err := json.Unmarshal(response, &subjects); err != nil || len(subjects) == 0 {
				t.Fatal("COV rehearsal pins unavailable")
			}
		} else {
			subjects = expected[pool]
		}
		base := config.Root
		if config.Mode != "preflight" {
			base = filepath.Join(base, pool)
		}
		result[pool] = covFinalPool(t, config, base, pool, subjects)
	}
	if _, err := fmt.Fprintln(os.Stdout, string(asJSON(t, result))); err != nil {
		t.Fatal("COV final worker could not publish aggregates")
	}
}

func (c covFinalConfig) progress(t *testing.T, pool, stage string) {
	t.Helper()
	if c.Mode != "preflight" {
		c.call(t, "_progress", pool, stage)
	}
}

type covFinalConfig struct {
	Mode     string `json:"mode"`
	Root     string `json:"root"`
	Python   string `json:"python"`
	Compiler string `json:"compiler"`
	Pyright  string `json:"pyright"`
	Scratch  string `json:"scratch"`
	Journal  string `json:"journal,omitempty"`
	Binding  string `json:"binding,omitempty"`
	Nonce    string `json:"nonce,omitempty"`
}

func (c covFinalConfig) call(t *testing.T, args ...string) []byte {
	t.Helper()
	script, err := filepath.Abs(filepath.Join(covOracleDir, "cov_final.py"))
	if err != nil {
		t.Fatal("COV final Python runner unavailable")
	}
	params := []string{"-I", "-B", "-S", script}
	cmd := exec.CommandContext(t.Context(), c.Python, append(params, args...)...)
	cmd.Stdin = bytes.NewReader(asJSON(t, c))
	data, err := cmd.Output()
	if err != nil {
		t.Fatal("COV final Python operation denied or failed")
	}
	return data
}

type covFinalRow struct {
	Score                    depbench.COVReading       `json:"score"`
	Closure                  depbench.COVClosureCounts `json:"closure"`
	NativeClasses            map[string]int            `json:"native_classes,omitempty"`
	DependencyEvidenceSHA256 string                    `json:"dependency_evidence_sha256"`
}

func covFinalPool(t *testing.T, config covFinalConfig, base, pool string, subjects []covSubject) map[string]covFinalRow {
	t.Helper()
	type pooled struct {
		observation depbench.Observation
		row         covFinalRow
		rejectedB3  bool
	}
	acc := map[string]*pooled{}
	for _, subject := range subjects {
		row, observation, rejectedB3 := covFinalSubject(t, config, base, pool, subject)
		p := acc[subject.Dialect]
		if p == nil {
			p = &pooled{rejectedB3: true, row: covFinalRow{NativeClasses: map[string]int{}}}
			acc[subject.Dialect] = p
		}
		p.rejectedB3 = p.rejectedB3 && rejectedB3
		covAppend(&p.observation, observation, subject.Name+"/")
		covAddClosure(&p.row.Closure, row.Closure)
		p.row.Score.ExternalAsLocal += row.Score.ExternalAsLocal
		p.row.Score.RejectedInputs += row.Score.RejectedInputs
		p.row.Score.RejectedOracleFiles += row.Score.RejectedOracleFiles
		for class, count := range row.NativeClasses {
			p.row.NativeClasses[class] += count
		}
	}
	out := make(map[string]covFinalRow, len(acc))
	config.progress(t, pool, "score")
	for dialect, p := range acc {
		p.row.Score.Reading = depbench.COVScore(p.observation, nil).Reading
		p.row.Score.B1 = p.row.Score.B1 && p.row.Score.ExternalAsLocal == 0
		p.row.Score.B3 = p.row.Score.B3 && p.rejectedB3
		type edges struct {
			Path  string
			Edges []deps.Edge
		}
		evidence := make([]edges, 0, len(p.observation.Files))
		for _, file := range p.observation.Files {
			evidence = append(evidence, edges{file.Path, file.Edges})
		}
		// This pool digest commits to retained dependencies across a permitted
		// correction without retaining file-level holdout or confirmation rows.
		p.row.DependencyEvidenceSHA256 = fmt.Sprintf("%x", sha256.Sum256(asJSON(t, evidence)))
		out[dialect] = p.row
	}
	return out
}
