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
	"reflect"
	"slices"
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

func TestCOVFinalNativeIsolation(t *testing.T) {
	const fixture = "COV_FINAL_SYNTHETIC_ROOT"
	if root := os.Getenv(fixture); root != "" {
		var subjects []covSubject
		data, err := os.ReadFile(filepath.Join(root, "subjects.json"))
		if err != nil || json.Unmarshal(data, &subjects) != nil {
			t.Fatal("synthetic pins unavailable")
		}
		scenario := os.Getenv("COV_FINAL_SYNTHETIC_CASE")
		if scenario == "checkout" {
			subjects[1].Commit = "invalid"
		}
		config := covFinalConfig{Mode: "final", Python: filepath.Join(root, "python")}
		if scenario == "injection" || scenario == "held-injection" {
			config.InjectNativeFailure = true
		}
		if scenario == "injection" {
			config.Mode = "rehearse"
		}
		for _, pool := range []string{"holdout", "confirmation"} {
			selected := subjects
			if pool == "confirmation" {
				selected = subjects[3:]
			}
			rows := covFinalPool(t, config, root, pool, selected)
			fmt.Println(string(asJSON(t, rows)))
		}
		return
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	write := func(path, content string, mode os.FileMode) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, path), []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}
	git := func(dir string, args ...string) string {
		t.Helper()
		prefix := []string{"-C", dir, "-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false",
			"-c", "user.name=Synthetic", "-c", "user.email=synthetic@example.test"}
		out, err := exec.CommandContext(t.Context(), "git", append(prefix, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("synthetic Git fixture: %v: %s", err, out)
		}
		return string(bytes.TrimSpace(out))
	}
	write("python", `#!/bin/sh
if [ "$5" = _compiler_identity ] && [ "$COV_FINAL_SYNTHETIC_CASE" = identity ]; then exit 1; fi
exit 0
`, 0o700)
	node := `#!/bin/sh
arm=${PWD%/*}
echo "${arm##*/}/${PWD##*/}" >> "$COV_FINAL_SYNTHETIC_ROOT/invocations"
if [ "$7" = --inject-native-failure ] && [ "$6" = --classify-native-exit ]; then
  echo injected >> "$COV_FINAL_SYNTHETIC_ROOT/invocations"
  exit 79
fi
case "$COV_FINAL_SYNTHETIC_CASE:${arm##*/}/${PWD##*/}" in
  generic:primary/failed) exit 9 ;;
  mixed:primary/failed) echo 'partial output'; exit 79 ;;
  parse:primary/failed) echo 'invalid JSON'; exit 0 ;;
  checkout:*|parse:*) ;;
  *:primary/typed)
    if [ "$(grep -c '^primary/typed$' "$COV_FINAL_SYNTHETIC_ROOT/invocations")" -gt 1 ]; then
      echo 'synthetic native diagnostics' >&2; exit 79
    fi ;;
  injection:*) ;;
  *:relocated/failed) echo 'synthetic native diagnostics' >&2; exit 79 ;;
esac
ext=js
if [ "${PWD##*/}" = typed ]; then ext=ts; fi
printf '{"truth":[{"Path":"a.%s"}]}\n' "$ext"
`
	write("node", node, 0o700)
	var subjects []covSubject
	for _, name := range []string{"early", "failed", "later", "typed"} {
		dialect, ext := "javascript", ".js"
		if name == "typed" {
			dialect, ext = "typescript", ".ts"
		}
		url := "https://example.test/synthetic"
		primary := filepath.Join(root, "primary", name)
		if err := os.MkdirAll(primary, 0o700); err != nil {
			t.Fatal(err)
		}
		git(primary, "init", "-q")
		file := "a" + ext
		if name == "later" {
			file = "bad[name]" + ext
		}
		write(filepath.Join("primary", name, file), "export const a = 1\n", 0o600)
		git(primary, "add", ".")
		git(primary, "commit", "-qm", "synthetic")
		git(primary, "remote", "add", "origin", url)
		git(root, "clone", "-q", "--no-local", primary, filepath.Join(root, "relocated", name))
		relocated := filepath.Join(root, "relocated", name)
		git(relocated, "remote", "set-url", "origin", url)
		git(primary, "checkout", "--detach", "-q")
		git(relocated, "checkout", "--detach", "-q")
		subjects = append(subjects, covSubject{tsjsSubject: tsjsSubject{
			Name: name, Dialect: dialect, Commit: git(primary, "rev-parse", "HEAD"), URL: url,
			Files: 1, Prefixes: []string{""}, Suffixes: []string{ext},
		}})
	}
	write("subjects.json", string(asJSON(t, subjects)), 0o600)
	for _, tc := range []struct {
		name, fatal string
	}{
		{"isolation", ""},
		{"injection", ""},
		{"held-injection", ""},
		{"identity", "COV final Python operation denied or failed"},
		{"parse", "COV compiler native capture malformed"},
		{"generic", "COV pinned oracle invocation failed"},
		{"mixed", "COV pinned oracle invocation failed"},
		{"checkout", "COV checkout identity or cleanliness mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			write("invocations", "", 0o600)
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestCOVFinalNativeIsolation$")
			cmd.Env = append(os.Environ(), fixture+"="+root, "COV_FINAL_SYNTHETIC_CASE="+tc.name,
				"PATH="+root+string(os.PathListSeparator)+os.Getenv("PATH"))
			out, err := cmd.CombinedOutput()
			if tc.fatal != "" {
				if err == nil || !bytes.Contains(out, []byte(tc.fatal)) || bytes.Contains(out, []byte("native_exception")) {
					t.Fatalf("protected failure was not fatal: %s", out)
				}
				return
			}
			if err != nil {
				t.Fatalf("native nonzero aborted the synthetic pools: %s", out)
			}
			if bytes.Contains(out, []byte("synthetic native diagnostics")) {
				t.Fatalf("native diagnostics escaped: %s", out)
			}
			lines := bytes.Split(out, []byte("\n"))
			for i, dialect := range []string{"javascript", "typescript"} {
				var rows map[string]json.RawMessage
				if err := json.Unmarshal(lines[i], &rows); err != nil {
					t.Fatalf("synthetic aggregate missing: %s", out)
				}
				files, complete := 3-2*i, 2-i
				want := fmt.Sprintf(`{"native_exception":"ExitError","files":%d,"reader_complete_ungraded":%d}`, files, complete)
				var got, expected map[string]any
				if json.Unmarshal(rows[dialect], &got) != nil || json.Unmarshal([]byte(want), &expected) != nil ||
					!reflect.DeepEqual(got, expected) {
					t.Fatalf("partial truth escaped or reader count was lost: %s", rows[dialect])
				}
				if i == 0 && !bytes.Contains(rows["typescript"], []byte(`"B1":true`)) {
					t.Fatalf("other dialect was not graded: %s", rows["typescript"])
				}
			}
			calls, err := os.ReadFile(filepath.Join(root, "invocations"))
			want := "primary/early\nrelocated/early\nprimary/failed\nrelocated/failed\nprimary/typed\nrelocated/typed\nprimary/typed\n"
			if tc.name == "injection" {
				want = "primary/early\nrelocated/early\ninjected\nprimary/typed\nrelocated/typed\nprimary/typed\n"
			}
			if err != nil || string(calls) != want {
				t.Fatalf("native retry or cross-pool suppression: %v: %s", err, calls)
			}
		})
	}
}

func TestCOVFinalPythonExitStatus(t *testing.T) {
	const fixture = "COV_FINAL_SYNTHETIC_PYTHON"
	if python := os.Getenv(fixture); python != "" {
		mode := os.Getenv("COV_FINAL_SYNTHETIC_MODE")
		config := covFinalConfig{Mode: mode, Python: python}
		_, _, failed := covFinalCapturePython(t, config, "holdout", "synthetic", "primary", []string{"a.py"})
		if !failed {
			t.Fatal("native graph nonzero was not classified")
		}
		return
	}
	for _, tc := range []struct {
		name, mode, script, fatal string
	}{
		{"classified", "final", "exit 73", ""},
		{"legacy", "replacement", "exit 73", "operation denied or failed"},
		{"shim-failure", "final", "exit 1", "operation denied or failed"},
		{"mixed-result", "final", "echo partial; exit 73", "operation denied or failed"},
		{"malformed", "final", "echo invalid", "native capture malformed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			python := filepath.Join(t.TempDir(), "python")
			if err := os.WriteFile(python, []byte("#!/bin/sh\n"+tc.script+"\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestCOVFinalPythonExitStatus$")
			cmd.Env = append(os.Environ(), fixture+"="+python, "COV_FINAL_SYNTHETIC_MODE="+tc.mode)
			out, err := cmd.CombinedOutput()
			if tc.fatal == "" {
				if err != nil {
					t.Fatalf("classified callback failed: %v: %s", err, out)
				}
			} else if err == nil || !bytes.Contains(out, []byte(tc.fatal)) {
				t.Fatalf("protected callback failure was not fatal: %v: %s", err, out)
			}
		})
	}
}

func TestCOVFinalRehearsalGitStore(t *testing.T) {
	const fixture = "FATHOM_COV_REHEARSAL_GIT_FIXTURE"
	if root := os.Getenv(fixture); root != "" {
		covFinalRehearsalDirectories(t, root, filepath.Join(root, "primary", "subject"), filepath.Join(root, "relocated", "subject"))
		return
	}
	tmp := t.TempDir()
	root, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatal(err)
	}
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
	covFinalRehearsalDirectories(t, root, primary, relocated)

	gitBin := t.TempDir()
	marker := filepath.Join(gitBin, "git-ran")
	if err := os.WriteFile(filepath.Join(gitBin, "git"), []byte("#!/bin/sh\n: > \"$COV_FINAL_GIT_MARKER\"\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	for _, tc := range []struct {
		name string
		path string
		make func(string) error
	}{
		{"commondir", filepath.Join(primary, ".git", "commondir"), func(path string) error {
			return os.WriteFile(path, []byte(filepath.Join(relocated, ".git")+"\n"), 0o600)
		}},
		{"symlink-directory", filepath.Join(relocated, ".git", "objects", "pack"), func(path string) error {
			if err := os.Remove(path); err != nil {
				return err
			}
			return os.Symlink(outside, path)
		}},
		{"dangling-symlink-file", filepath.Join(primary, ".git", "objects", "info", "extra"), func(path string) error {
			return os.Symlink(filepath.Join(outside, "missing"), path)
		}},
		{"fifo", filepath.Join(relocated, ".git", "objects", "info", "extra"), func(path string) error {
			return exec.CommandContext(t.Context(), "mkfifo", path).Run()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.make(tc.path); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.Remove(tc.path); err != nil {
					t.Error(err)
				}
			}()
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestCOVFinalRehearsalGitStore$")
			cmd.Env = append(os.Environ(), fixture+"="+root, "PATH="+gitBin, "COV_FINAL_GIT_MARKER="+marker)
			output, err := cmd.CombinedOutput()
			if err == nil || !bytes.Contains(output, []byte("requires a local Git object store")) {
				t.Errorf("redirected or non-regular rehearsal object store was not refused: %s", output)
			}
			if _, err := os.Lstat(marker); !os.IsNotExist(err) {
				t.Errorf("Git ran before object stores were validated: %v", err)
				if err := os.Remove(marker); err != nil {
					t.Error(err)
				}
			}
		})
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

func TestCOVFinalReplacementCohorts(t *testing.T) {
	pools, initial := covFinalCohorts(t, "initial")
	replacementPools, replacement := covFinalCohorts(t, "replacement")
	if !slices.Equal(pools, replacementPools) || !reflect.DeepEqual(initial, replacement) {
		t.Fatal("replacement does not use the exact initial cohorts")
	}
	for pool, count := range map[string]int{"development": 10, "holdout": 5, "confirmation": 1} {
		if len(replacement[pool]) != count {
			t.Fatalf("wrong number of %s pins", pool)
		}
	}
	lastPools, last := covFinalCohorts(t, "final")
	if !slices.Equal(pools, lastPools) || !reflect.DeepEqual(initial, last) {
		t.Fatal("final does not use the exact initial cohorts")
	}
	rehearsalPools, rehearsal := covFinalCohorts(t, "rehearse")
	if !slices.Equal(rehearsalPools, pools) || rehearsal != nil {
		t.Fatal("rehearsal did not leave pin selection to the non-held callback")
	}
	preflightPools, _ := covFinalCohorts(t, "preflight")
	if !slices.Equal(preflightPools, []string{"development"}) {
		t.Fatal("preflight is not development-only")
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
	if config.InjectNativeFailure && config.Mode != "rehearse" {
		t.Fatal("COV native failure injection requires rehearsal")
	}
	pools, expected := covFinalCohorts(t, config.Mode)
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

func covFinalCohorts(t *testing.T, mode string) ([]string, map[string][]covSubject) {
	t.Helper()
	pools := []string{"development", "holdout", "confirmation"}
	switch mode {
	case "preflight":
		return pools[:1], covExpectedSubjects(t)
	case "rehearse":
		return pools, nil
	case "initial", "replacement", "final":
		expected := covExpectedSubjects(t)
		pin, err := os.ReadFile(filepath.Join(covOracleDir, "cov_confirmation.json"))
		var confirmation []covSubject
		if err != nil || json.Unmarshal(pin, &confirmation) != nil || len(confirmation) != 1 ||
			confirmation[0].Dialect != "typescript" || confirmation[0].Files != 979 {
			t.Fatal("COV confirmation pin unavailable")
		}
		expected["confirmation"] = confirmation
		return pools, expected
	default:
		t.Fatal("COV final worker operation denied")
		return nil, nil
	}
}

func (c covFinalConfig) progress(t *testing.T, pool, stage string) {
	t.Helper()
	if c.Mode != "preflight" {
		c.call(t, "_progress", pool, stage)
	}
}

type covFinalConfig struct {
	Mode                string `json:"mode"`
	Root                string `json:"root"`
	Python              string `json:"python"`
	Compiler            string `json:"compiler"`
	Pyright             string `json:"pyright"`
	Scratch             string `json:"scratch"`
	Journal             string `json:"journal,omitempty"`
	Binding             string `json:"binding,omitempty"`
	Nonce               string `json:"nonce,omitempty"`
	InjectNativeFailure bool   `json:"inject_native_failure,omitempty"`
}

func (c covFinalConfig) isolatesNative() bool {
	return c.Mode == "final" || c.Mode == "rehearse"
}

func (c covFinalConfig) call(t *testing.T, args ...string) []byte {
	t.Helper()
	data, err := c.output(t, args...)
	if err != nil {
		t.Fatal("COV final Python operation denied or failed")
	}
	return data
}

func (c covFinalConfig) output(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()
	script, err := filepath.Abs(filepath.Join(covOracleDir, "cov_final.py"))
	if err != nil {
		t.Fatal("COV final Python runner unavailable")
	}
	params := []string{"-I", "-B", "-S", script}
	cmd := exec.CommandContext(t.Context(), c.Python, append(params, args...)...)
	cmd.Stdin = bytes.NewReader(asJSON(t, c))
	return cmd.Output()
}

type covFinalRow struct {
	Score                    depbench.COVReading       `json:"score"`
	Closure                  depbench.COVClosureCounts `json:"closure"`
	NativeClasses            map[string]int            `json:"native_classes,omitempty"`
	DependencyEvidenceSHA256 string                    `json:"dependency_evidence_sha256"`
	ungraded                 *covFinalUngraded
}

type covFinalUngraded struct {
	NativeException        string `json:"native_exception"`
	Files                  int    `json:"files"`
	ReaderCompleteUngraded int    `json:"reader_complete_ungraded"`
}

func (row covFinalRow) MarshalJSON() ([]byte, error) {
	if row.ungraded != nil {
		return json.Marshal(row.ungraded)
	}
	type graded covFinalRow
	return json.Marshal(graded(row))
}

func covFinalPool(t *testing.T, config covFinalConfig, base, pool string, subjects []covSubject) map[string]covFinalRow {
	t.Helper()
	type pooled struct {
		observation depbench.Observation
		row         covFinalRow
		rejectedB3  bool
	}
	acc := map[string]*pooled{}
	out := map[string]covFinalRow{}
	for _, subject := range subjects {
		dialect := subject.Dialect
		failed := out[dialect].ungraded
		row, observation, rejectedB3 := covFinalSubject(t, config, base, pool, subject, failed != nil)
		p := acc[dialect]
		if row.ungraded != nil {
			if failed == nil {
				failed = row.ungraded
				if p != nil {
					failed.Files += len(p.observation.Files)
					failed.ReaderCompleteUngraded += covFinalComplete(p.observation.Files)
				}
				delete(acc, dialect)
				out[dialect] = covFinalRow{ungraded: failed}
			} else {
				failed.Files += row.ungraded.Files
				failed.ReaderCompleteUngraded += row.ungraded.ReaderCompleteUngraded
			}
			continue
		}
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
