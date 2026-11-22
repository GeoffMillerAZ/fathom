package deps_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
	"github.com/geoffmilleraz/fathom/pkg/lang/deps/internal/depbench"
)

const covPythonDispatch = "585e776c2685fc0c08d8b97a87222fc16c28429b"

type covPythonCrossCheck struct {
	Class, Cohort string
	Truth         []string
}

type covPythonStaticReport struct {
	Tool     json.RawMessage
	Subjects map[string]struct {
		Manifest    json.RawMessage
		Commit      string
		Roots       []string
		GraphSHA256 string `json:"graph_sha256"`
		GraphRows   int    `json:"graph_rows"`
		Compared    map[string]covPythonCrossCheck
	}
}

func TestCOVPythonFromListDevelopment(t *testing.T) {
	if os.Getenv("FATHOM_COV_PYTHON_DEVELOPMENT") != "1" {
		t.Skip("set FATHOM_COV_PYTHON_DEVELOPMENT=1 for the pinned Python development proof")
	}
	python := requiredPythonEnv(t, "FATHOM_COV_PYTHON")
	development := requiredPythonEnv(t, "FATHOM_COV_PYTHON_ARMS")
	baselinePath := requiredPythonEnv(t, "FATHOM_COV_PYTHON_BASELINE")
	out := os.Getenv("FATHOM_COV_PYTHON_EVIDENCE")
	if !filepath.IsAbs(out) {
		t.Fatal("COV Python proof requires an absolute evidence path")
	}
	cache := requiredPythonEnv(t, "FATHOM_COV_PYRIGHT")
	if filepath.Base(development) != "development" || filepath.Base(baselinePath) != "development-python-observations.json" {
		t.Fatal("COV Python proof requires the frozen development inputs")
	}
	if err := os.Mkdir(out, 0o700); err != nil {
		t.Fatal("COV Python proof requires a fresh evidence directory")
	}
	pinsDir := filepath.Join(covOracleDir, "..", "deps-python")
	for _, file := range []string{
		filepath.Join(pinsDir, "modulefinder_oracle.py"), filepath.Join(pinsDir, "test_modulefinder_oracle.py"),
		filepath.Join(covOracleDir, "cov_run.py"), filepath.Join(covOracleDir, "cov_run_test.py"),
		filepath.Join(covOracleDir, "cov_python_pyright.py"), filepath.Join(covOracleDir, "cov_python_pyright_test.py"),
		filepath.Join(pinsDir, "pins.json"),
	} {
		gitPath, ok := strings.CutPrefix(filepath.ToSlash(file), "../../../")
		pinned, err := exec.CommandContext(t.Context(), "git", "show", covPythonDispatch+":"+gitPath).Output()
		actual, readErr := os.ReadFile(file)
		if !ok || err != nil || readErr != nil || !bytes.Equal(pinned, actual) {
			t.Fatal("COV Python oracle dispatch identity differs")
		}
	}

	baseData, err := os.ReadFile(baselinePath)
	var frozen depbench.Observation
	if err != nil || json.Unmarshal(baseData, &frozen) != nil {
		t.Fatal("COV frozen Python development baseline unavailable")
	}
	const baselineDigest = "5c3824904768cd1f4a5c1ec9389d08014e720e29f59c66747aa13a4b7d20e719"
	if got := sha256.Sum256(baseData); hex.EncodeToString(got[:]) != baselineDigest {
		t.Fatal("COV frozen Python development baseline digest differs")
	}
	pinsData, err := os.ReadFile(filepath.Join(pinsDir, "pins.json"))
	var pins struct{ Subjects []covSubject }
	if err != nil || json.Unmarshal(pinsData, &pins) != nil || len(pins.Subjects) != 3 {
		t.Fatal("COV Python development pins unavailable")
	}
	for i := range pins.Subjects {
		pins.Subjects[i].Dialect = "python"
		pins.Subjects[i].Prefixes = []string{""}
		pins.Subjects[i].Suffixes = []string{".py"}
	}

	original, extended, static := covPythonCapture(t, python, development, cache, out, pins.Subjects)
	read := func(reader *deps.Reader, view *covFrozenView, paths []string) depbench.COVBatch {
		t.Helper()
		inputs := make([]deps.Input, len(paths))
		for i, file := range paths {
			inputs[i] = deps.Input{Path: file, Dialect: "python"}
		}
		batch, err := depbench.COVReadIsolated(t.Context(), reader, view, inputs)
		if err != nil {
			t.Fatal("COV Python product read failed")
		}
		return batch
	}

	var candidate, reference, nativeOnly depbench.Observation
	var before, after depbench.COVClosureCounts
	var external, baselineExternal, gained, lost, ungraded, alteredEvidence int
	classes := map[string]int{}
	reader := &deps.Reader{}
	for _, s := range pins.Subjects {
		root := filepath.Join(development, "primary", s.Name)
		relocated := filepath.Join(development, "relocated", s.Name)
		first, paths := covFreeze(t, s, root)
		second, again := covFreeze(t, s, relocated)
		a, ea := os.Stat(root)
		b, eb := os.Stat(relocated)
		if ea != nil || eb != nil || os.SameFile(a, b) ||
			!slices.Equal(paths, again) || !bytes.Equal(asJSON(t, first), asJSON(t, second)) {
			t.Fatal("COV Python development arms differ or share a physical checkout")
		}
		staticSubject, ok := static.Subjects[s.Name]
		if !ok || staticSubject.Commit != s.Commit || !slices.Equal(staticSubject.Roots, s.Roots) ||
			staticSubject.GraphRows != len(paths) || !bytes.Equal(staticSubject.Manifest, original[s.Name].Manifest) {
			t.Fatal("COV Python Pyright development identity differs")
		}
		native, expanded := original[s.Name].Rows, extended[s.Name].Rows
		if len(native) != len(paths) || len(expanded) != len(paths) {
			t.Fatal("COV Python native population differs")
		}
		var expected, oldTruth []depbench.TruthFile
		gaps := make(map[string]string)
		for i, file := range paths {
			if native[i].Path != file || expanded[i].Path != file {
				t.Fatal("COV Python native selection differs")
			}
			if native[i].Gap == "" {
				if !bytes.Equal(asJSON(t, native[i]), asJSON(t, expanded[i])) {
					t.Fatal("COV Python extension changed preserved CPython truth")
				}
				row := depbench.TruthFile{Path: file, Edges: native[i].Edges}
				oldTruth = append(oldTruth, row)
				expected = append(expected, row)
				continue
			}
			check := staticSubject.Compared[file]
			if expanded[i].Gap != "" || check.Cohort != "new" {
				gaps[file] = "native-gap"
				continue
			}
			row, ok := covPythonDualTruth(expanded[i], check)
			if !ok {
				gaps[file] = check.Class
				continue
			}
			expected = append(expected, row)
		}

		view := covView(first)
		primary := read(reader, view, paths)
		reverse := slices.Clone(paths)
		slices.Reverse(reverse)
		repeat, moved := read(reader, view, reverse), read(reader, covView(second), paths)
		if len(primary.Rejected)+len(repeat.Rejected)+len(moved.Rejected) != 0 {
			t.Fatal("COV Python input was rejected")
		}
		current := depbench.Observation{Files: primary.Files, Repeat: repeat.Files, Relocated: moved.Files, Truth: expected}
		baselineFiles := covPythonBaselineFiles(frozen.Files, s.Name)
		if len(baselineFiles) != len(paths) || len(current.Files) != len(paths) {
			t.Fatal("COV Python selected files were lost")
		}
		for i, f := range current.Files {
			old := baselineFiles[i]
			if old.Path != f.Path {
				t.Fatal("COV Python baseline selection differs")
			}
			if !slices.Equal(old.Edges, f.Edges) || slices.ContainsFunc(f.Unknowns, func(u deps.Unknown) bool {
				return !slices.ContainsFunc(old.Unknowns, func(v deps.Unknown) bool { return covPythonSameUnknown(u, v) })
			}) || slices.ContainsFunc(old.Unknowns, func(u deps.Unknown) bool {
				return u.Code != deps.UnknownDynamicImport &&
					!slices.ContainsFunc(f.Unknowns, func(v deps.Unknown) bool { return covPythonSameUnknown(u, v) })
			}) {
				alteredEvidence++
			}
			if old.Complete && !f.Complete {
				lost++
			}
			if !old.Complete && f.Complete {
				gained++
				if gap := gaps[f.Path]; gap != "" {
					ungraded++
					classes[gap]++
				}
			}
		}
		inventory := depbench.COVInventory(covTree(first), s.Roots)
		ownerScore := depbench.COVScore(current, inventory)
		external += ownerScore.ExternalAsLocal
		baseline := depbench.Observation{Files: baselineFiles, Truth: expected}
		baselineExternal += depbench.COVScore(baseline, inventory).ExternalAsLocal
		old := baseline
		old.Truth = oldTruth
		covAppend(&nativeOnly, old, s.Name+"/")
		covAppend(&reference, baseline, s.Name+"/")
		covAppend(&candidate, current, s.Name+"/")

		// Python's selected set contains every tracked .py, so the baseline
		// closure traversal needs no reread with the changed Reader.
		for _, f := range baselineFiles {
			for _, edge := range f.Edges {
				if edge.Resolution == deps.ResolvedDirectory ||
					edge.Resolution == deps.ResolvedFile && !slices.Contains(paths, edge.To) {
					t.Fatal("COV Python baseline closure needs a target outside the preserved observations")
				}
			}
		}
		covAddClosure(&before, covReach(t, reader, first, paths, baselineFiles, nil))
		covAddClosure(&after, covReach(t, reader, first, paths, current.Files, nil))
		covFreeze(t, s, root)
		covFreeze(t, s, relocated)
		t.Logf("%s files=%d truth=%d complete=%d external-as-local=%d", s.Name,
			ownerScore.Files, ownerScore.OracleFiles, ownerScore.Complete, ownerScore.ExternalAsLocal)
	}

	// The archived source of pre-existing truth stays authoritative. Sort
	// independently, since the archive used a different subject ordering.
	sortTruth := func(rows []depbench.TruthFile) {
		slices.SortFunc(rows, func(a, b depbench.TruthFile) int { return strings.Compare(a.Path, b.Path) })
	}
	sortTruth(frozen.Truth)
	sortTruth(nativeOnly.Truth)
	if !bytes.Equal(asJSON(t, frozen.Truth), asJSON(t, nativeOnly.Truth)) {
		t.Fatal("COV Python default truth differs from the frozen baseline")
	}
	preserved := depbench.COVScore(nativeOnly, nil)
	baseline := depbench.COVScore(reference, nil)
	baseline.ExternalAsLocal = baselineExternal
	baseline.B1 = baseline.B1 && baselineExternal == 0
	score := depbench.COVScore(candidate, nil)
	score.ExternalAsLocal = external
	score.B1 = score.B1 && external == 0
	if baseline.B4ComparableEdges == 0 || score.B4ComparableEdges == 0 {
		t.Fatal("COV Python B4 has no comparable edges")
	}
	b4Before := float64(baseline.B4Extra) / float64(baseline.B4ComparableEdges)
	b4After := float64(score.B4Extra) / float64(score.B4ComparableEdges)
	delta := b4After - b4Before
	receipt := struct {
		Dispatch                             string
		Preserved, Baseline, Candidate       depbench.COVReading
		Before, After                        depbench.COVClosureCounts
		Gained, Lost, Ungraded, ChangedEdges int
		UngradedClasses                      map[string]int
		B4Delta                              float64
	}{covPythonDispatch, preserved, baseline, score, before, after, gained, lost, ungraded, alteredEvidence, classes, delta}
	data := asJSON(t, receipt)
	covWrite(t, filepath.Join(out, "summary.json"), data)
	covWrite(t, filepath.Join(out, "observations.json"), asJSON(t, candidate))
	t.Logf("B1=%t B2=%t B3=%t missing=%d wrong=%d inconclusive=%d external-as-local=%d; gained=%d/%d ungraded=%d %v lost=%d changed-evidence=%d",
		score.B1, score.B2, score.B3, score.Missing, score.Wrong, score.B2Inconclusive, external,
		gained, score.Files, ungraded, classes, lost, alteredEvidence)
	t.Logf("complete %d -> %d/%d, closure-complete %d -> %d, B4 %d/%d -> %d/%d (%+.3f pp), receipt SHA256 %x",
		baseline.Complete, score.Complete, score.Files, before.Complete, after.Complete,
		baseline.B4Extra, baseline.B4ComparableEdges, score.B4Extra, score.B4ComparableEdges, delta*100, sha256.Sum256(data))
	if score.Files != 1314 || preserved.Complete != 121 || preserved.OracleFiles != 156 || !baseline.B1 || !baseline.B2 ||
		!score.B1 || !score.B2 || !score.B3 || gained*100 < score.Files*2 ||
		lost != 0 || ungraded != 0 || alteredEvidence != 0 || delta > 0.05 {
		t.Fatal("COV Python development bars failed")
	}
}

func covPythonCapture(t *testing.T, python, development, cache, out string, subjects []covSubject) (
	map[string]pythonOracleSubject, map[string]pythonOracleSubject, covPythonStaticReport,
) {
	t.Helper()
	run := func(args ...string) []byte {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), python, append([]string{"-I", "-B", "-S"}, args...)...)
		data, err := cmd.Output()
		if err != nil {
			if failed, ok := err.(*exec.ExitError); ok {
				covWrite(t, filepath.Join(out, "native-failure.stderr"), failed.Stderr)
			}
			t.Fatal("COV Python pinned oracle invocation failed")
		}
		return data
	}
	var original, extended map[string]pythonOracleSubject
	var static covPythonStaticReport
	for _, arm := range []string{"primary", "relocated"} {
		root := filepath.Join(development, arm)
		armDefault, armExtended := map[string]json.RawMessage{}, map[string]json.RawMessage{}
		for _, s := range subjects {
			args := []string{filepath.Join(covOracleDir, "cov_run.py"), "python-truth", "development", s.Name, filepath.Join(root, s.Name)}
			armDefault[s.Name] = run(args...)
			armExtended[s.Name] = run(append(args, "--declarative-fromlist")...)
		}
		defaultPath := filepath.Join(out, arm+"-default.json")
		extendedPath := filepath.Join(out, arm+"-extension.json")
		covWrite(t, defaultPath, asJSON(t, armDefault))
		covWrite(t, extendedPath, asJSON(t, armExtended))
		report := run(filepath.Join(covOracleDir, "cov_python_pyright.py"), "--cache", cache, "--cohort", root,
			"--frozen", defaultPath, "--extension", extendedPath, "--work", filepath.Join(out, arm+"-pyright"))
		covWrite(t, filepath.Join(out, arm+"-crosscheck.json"), report)
		if arm == "primary" {
			if json.Unmarshal(asJSON(t, armDefault), &original) != nil ||
				json.Unmarshal(asJSON(t, armExtended), &extended) != nil || json.Unmarshal(report, &static) != nil {
				t.Fatal("COV Python development oracle output invalid")
			}
			continue
		}
		for _, suffix := range []string{"-default.json", "-extension.json", "-crosscheck.json"} {
			primary, err := os.ReadFile(filepath.Join(out, "primary"+suffix))
			moved, movedErr := os.ReadFile(filepath.Join(out, "relocated"+suffix))
			if err != nil || movedErr != nil || !bytes.Equal(primary, moved) {
				t.Fatal("COV Python native evidence differs between arms")
			}
		}
	}
	return original, extended, static
}

func covPythonDualTruth(native pythonOracleRow, check covPythonCrossCheck) (depbench.TruthFile, bool) {
	row := depbench.TruthFile{Path: native.Path, Edges: slices.Clone(native.Edges)}
	if check.Class != "exact" && check.Class != "initializer-plus-submodule" || check.Truth == nil {
		return row, false
	}
	targets := map[string]bool{}
	for _, edge := range native.Edges {
		if edge.Resolution == deps.ResolvedFile {
			if !slices.Contains(check.Truth, edge.To) {
				return row, false
			}
			targets[edge.To] = true
		}
	}
	for _, target := range check.Truth {
		if targets[target] {
			continue
		}
		// The independent comparison has already proved a unique shared site.
		// Copy its native initializer's operand and line to the child target.
		parent := path.Dir(target)
		if path.Base(target) == "__init__.py" {
			parent = path.Dir(parent)
		}
		var site depbench.TruthEdge
		attributions := 0
		for _, edge := range native.Edges {
			if edge.Resolution == deps.ResolvedFile && edge.To == path.Join(parent, "__init__.py") {
				site = edge
				attributions++
			}
		}
		if check.Class != "initializer-plus-submodule" || attributions != 1 || site.Line <= 0 {
			return row, false
		}
		site.To = target
		row.Edges = append(row.Edges, site)
	}
	return row, true
}

func covPythonBaselineFiles(files []deps.FileDeps, subject string) []deps.FileDeps {
	prefix := subject + "/"
	var out []deps.FileDeps
	for _, f := range files {
		rel, ok := strings.CutPrefix(f.Path, prefix)
		if !ok {
			continue
		}
		f.Path = rel
		f.Edges = slices.Clone(f.Edges)
		f.Unknowns = slices.Clone(f.Unknowns)
		for i := range f.Edges {
			f.Edges[i].From = rel
			if f.Edges[i].Resolution == deps.ResolvedFile || f.Edges[i].Resolution == deps.ResolvedDirectory {
				f.Edges[i].To = strings.TrimPrefix(f.Edges[i].To, prefix)
			}
		}
		for i := range f.Unknowns {
			f.Unknowns[i].From = rel
		}
		out = append(out, f)
	}
	slices.SortFunc(out, func(a, b deps.FileDeps) int { return strings.Compare(a.Path, b.Path) })
	return out
}

func covPythonSameUnknown(a, b deps.Unknown) bool {
	return a.From == b.From && a.Code == b.Code && a.Raw == b.Raw && a.Line == b.Line &&
		slices.Equal(a.DecidableBy, b.DecidableBy)
}
