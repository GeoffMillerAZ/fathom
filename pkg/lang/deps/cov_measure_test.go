package deps_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
	"github.com/geoffmilleraz/fathom/pkg/lang/deps/internal/depbench"
)

var covOracleDir = filepath.Join("..", "..", "..", "calibration", "oracles", "dep-coverage")
var covImportCall = regexp.MustCompile(`\bimport\s*\(`)
var covFromImport = regexp.MustCompile(`^from\s+[.A-Za-z_][.A-Za-z_0-9]*\s+import\b`)
var covPythonImport = regexp.MustCompile(`^import\s+[A-Za-z_][.A-Za-z_0-9]*(\s+as\s+[A-Za-z_][A-Za-z_0-9]*)?(\s*,\s*[A-Za-z_][.A-Za-z_0-9]*(\s+as\s+[A-Za-z_][A-Za-z_0-9]*)?)*\s*(#.*)?$`)

// covShape names only located syntax evidence. It never guesses a resolver
// mechanism from an unknown code or treats a masked operand as a literal.
func covShape(src []byte, u deps.Unknown) string {
	lines := strings.Split(string(src), "\n")
	if u.Line < 1 || u.Line > len(lines) {
		return "unknown-shape"
	}
	line := strings.TrimSpace(lines[u.Line-1])
	if strings.HasPrefix(line, "/**") {
		comment := strings.Join(lines[u.Line-1:], "\n")
		if end := strings.Index(comment, "*/"); end >= 0 {
			comment = comment[:end]
		}
		if covImportCall.MatchString(comment) || strings.Contains(comment, "@import ") {
			return "jsdoc-import"
		}
	}
	if covFromImport.MatchString(line) {
		return "python-from-import"
	}
	if covPythonImport.MatchString(line) {
		return "python-import-statement"
	}
	if strings.HasPrefix(line, "import ") || strings.HasPrefix(line, "export ") {
		return "js-module-statement"
	}
	if strings.HasPrefix(line, "///") && strings.Contains(line, "<reference") {
		return "ts-reference-directive"
	}
	return "unknown-shape"
}

type covSubject struct {
	tsjsSubject
	Roots []string
}

type covSubjectReading struct {
	Name, Dialect, Commit, SelectionSHA256, SourceSHA256 string
	Score                                                depbench.COVReading
	Closure                                              depbench.COVClosureCounts
	OracleGaps                                           map[string]int
}

type covPool struct {
	observation depbench.Observation
	shapes      map[string][]string
	gaps        map[string]string
	external    int
	closure     depbench.COVClosureCounts
}

// TestCOVBaseline is opt-in: repeat and relocation are one campaign, not
// separate attempts. A campaign marker prevents accidental replay in place.
func TestCOVBaseline(t *testing.T) {
	if os.Getenv("FATHOM_COV_BASELINE") != "1" {
		t.Skip("set FATHOM_COV_BASELINE=1 for the sealed baseline campaign")
	}
	covMeasure(t)
}

func covMeasure(t *testing.T) {
	t.Helper()
	root := requiredPythonEnv(t, "FATHOM_COV_ROOT")
	compiler := requiredPythonEnv(t, "FATHOM_COV_COMPILER")
	python := requiredPythonEnv(t, "FATHOM_COV_PYTHON")
	artifacts := filepath.Join(root, "reading")
	if err := os.Mkdir(artifacts, 0o700); err != nil {
		t.Fatal("COV campaign already exists or cannot be created; do not rerun the baseline")
	}
	manifest, err := os.ReadFile(filepath.Join(root, "subjects.json"))
	if err != nil {
		t.Fatal("COV subject manifest unavailable")
	}
	// Bind the prepared manifest to the original pins and the landed holdout,
	// rather than trusting a caller-edited subject list.
	expected := covExpectedSubjects(t)
	var cohorts map[string][]covSubject
	if json.Unmarshal(manifest, &cohorts) != nil || !bytes.Equal(asJSON(t, expected), asJSON(t, cohorts)) {
		t.Fatal("COV manifest differs from original population pins")
	}
	public := map[string]any{}
	rev, err := exec.CommandContext(t.Context(), "git", "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal("COV harness revision unavailable")
	}
	public["harness_commit"] = strings.TrimSpace(string(rev))
	sealedClosure := map[string]depbench.COVClosureCounts{}
	var subjects []covSubjectReading
	for _, poolName := range []string{"development", "holdout"} {
		pools := map[string]*covPool{}
		for _, s := range cohorts[poolName] {
			primary := filepath.Join(root, poolName, "primary", s.Name)
			relocated := filepath.Join(root, poolName, "relocated", s.Name)
			first, paths := covFreeze(t, s, primary)
			second, again := covFreeze(t, s, relocated)
			if !slices.Equal(paths, again) || !bytes.Equal(asJSON(t, first), asJSON(t, second)) {
				t.Fatal("COV relocated tracked inputs differ")
			}
			a, ea := os.Stat(primary)
			b, eb := os.Stat(relocated)
			if ea != nil || eb != nil || os.SameFile(a, b) {
				t.Fatal("COV relocation requires physically distinct checkouts")
			}
			// The independent oracle is fixed before constructing a product reader.
			truth, gaps := covTruth(t, python, compiler, poolName, s, primary, paths)
			reader := &deps.Reader{}
			read := func(view fs.FS, selected []string) []deps.FileDeps {
				inputs := make([]deps.Input, len(selected))
				for i, file := range selected {
					inputs[i] = deps.Input{Path: file, Dialect: s.Dialect}
				}
				result, err := reader.ReadFiles(t.Context(), view, inputs)
				if err != nil {
					t.Fatal("COV selected product read failed")
				}
				return result
			}
			firstView, secondView := covView(first), covView(second)
			o := depbench.Observation{Truth: truth, Files: read(firstView, paths)}
			reverse := slices.Clone(paths)
			slices.Reverse(reverse)
			o.Repeat = read(firstView, reverse)
			o.Relocated = read(secondView, paths)
			inventory := depbench.COVInventory(covTree(first), s.Roots)
			score := depbench.COVScore(o, inventory)
			closure := covReach(t, reader, first, paths, o.Files)
			// Native invocation and product traversal must leave both checkouts clean.
			covFreeze(t, s, primary)
			covFreeze(t, s, relocated)
			p := pools[s.Dialect]
			if p == nil {
				p = &covPool{shapes: map[string][]string{}, gaps: map[string]string{}}
				pools[s.Dialect] = p
			}
			covAppend(&p.observation, o, s.Name+"/")
			p.external += score.ExternalAsLocal
			covAddClosure(&p.closure, closure)
			if poolName == "development" {
				gapCounts := map[string]int{}
				for file, gap := range gaps {
					p.gaps[s.Name+"/"+file] = gap
					gapCounts[gap]++
				}
				for _, file := range o.Files {
					for _, u := range file.Unknowns {
						p.shapes[s.Name+"/"+file.Path] = append(p.shapes[s.Name+"/"+file.Path], covShape(first[file.Path].Data, u))
					}
				}
				digest := sha256.New()
				for _, file := range paths {
					digest.Write([]byte(file))
					digest.Write([]byte{0})
				}
				source := sha256.Sum256(asJSON(t, first))
				subjects = append(subjects, covSubjectReading{s.Name, s.Dialect, s.Commit, hex.EncodeToString(digest.Sum(nil)), hex.EncodeToString(source[:]), score, closure, gapCounts})
			}
			// No holdout per-file observation or native row is ever written or logged.
		}
		rows := map[string]any{}
		for _, dialect := range []string{"python", "javascript", "typescript"} {
			p := pools[dialect]
			if p == nil {
				t.Fatal("COV pool missing dialect")
			}
			if poolName == "development" {
				analysis := depbench.COVAnalyze(p.observation, p.shapes, p.gaps, nil)
				covWrite(t, filepath.Join(artifacts, "development-"+dialect+"-observations.json"), asJSON(t, p.observation))
				analysis.Score.ExternalAsLocal = p.external
				analysis.Score.B1 = analysis.Score.B1 && p.external == 0
				rows[dialect] = struct {
					Analysis depbench.COVDevelopment
					Closure  depbench.COVClosureCounts
				}{analysis, p.closure}
			} else {
				score := depbench.COVScore(p.observation, nil)
				score.ExternalAsLocal = p.external
				score.B1 = score.B1 && p.external == 0
				rows[dialect] = json.RawMessage(depbench.COVSeal(score))
				sealedClosure[dialect] = p.closure
			}
		}
		public[poolName] = rows
	}
	public["development_subjects"] = subjects
	covWrite(t, filepath.Join(artifacts, "aggregate.json"), asJSON(t, public))
	// This is aggregate-only, private evidence for the final COV-D3 comparison.
	covWrite(t, filepath.Join(artifacts, "sealed-closure.json"), asJSON(t, sealedClosure))
	t.Logf("COV baseline aggregate SHA256 %x", sha256.Sum256(asJSON(t, public)))
}

func covExpectedSubjects(t *testing.T) map[string][]covSubject {
	t.Helper()
	out := map[string][]covSubject{}
	for _, kind := range []string{"python", "tsjs"} {
		data, err := os.ReadFile(filepath.Join(covOracleDir, "..", "deps-"+kind, "pins.json"))
		if err != nil {
			t.Fatal("COV original pins unavailable")
		}
		var pins struct{ Subjects []covSubject }
		if json.Unmarshal(data, &pins) != nil {
			t.Fatal("COV original pins malformed")
		}
		for _, s := range pins.Subjects {
			if kind == "python" {
				s.Dialect = "python"
				s.Prefixes = []string{""}
				s.Suffixes = []string{".py"}
			}
			out["development"] = append(out["development"], s)
		}
	}
	data, err := os.ReadFile(filepath.Join(covOracleDir, "cov_holdout.json"))
	if err != nil {
		t.Fatal("COV holdout pins unavailable")
	}
	var outHoldout []covSubject
	if json.Unmarshal(data, &outHoldout) != nil {
		t.Fatal("COV holdout pins malformed")
	}
	out["holdout"] = outHoldout
	return out
}

func covFreeze(t *testing.T, s covSubject, root string) (fstest.MapFS, []string) {
	t.Helper()
	git := func(args ...string) []byte {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, args...)...)
		data, err := cmd.Output()
		if err != nil {
			t.Fatal("COV checkout inspection failed")
		}
		return data
	}
	if strings.TrimSpace(string(git("rev-parse", "HEAD"))) != s.Commit || strings.TrimSpace(string(git("remote", "get-url", "origin"))) != s.URL || len(git("status", "--porcelain", "--untracked-files=all", "--ignored=matching")) != 0 {
		t.Fatal("COV checkout identity or cleanliness mismatch")
	}
	tree := fstest.MapFS{}
	var selected []string
	for _, row := range bytes.Split(git("ls-tree", "-rz", s.Commit), []byte{0}) {
		meta, name, ok := bytes.Cut(row, []byte{'\t'})
		if !ok {
			continue
		}
		regular := bytes.HasPrefix(meta, []byte("100644 ")) || bytes.HasPrefix(meta, []byte("100755 "))
		mode := fs.FileMode(0o444)
		if !regular && !bytes.HasPrefix(meta, []byte("120000 ")) {
			continue
		}
		var data []byte
		var err error
		if regular {
			data, err = os.ReadFile(filepath.Join(root, string(name)))
		} else {
			var target string
			target, err = os.Readlink(filepath.Join(root, string(name)))
			data = []byte(target)
			mode |= fs.ModeSymlink
		}
		if err != nil {
			t.Fatal("COV tracked input unavailable")
		}
		file := string(name)
		tree[file] = &fstest.MapFile{Data: data, Mode: mode}
		if regular && slices.ContainsFunc(s.Prefixes, func(p string) bool { return strings.HasPrefix(file, p) }) && slices.ContainsFunc(s.Suffixes, func(p string) bool { return strings.HasSuffix(file, p) }) {
			selected = append(selected, file)
		}
	}
	slices.Sort(selected)
	digest := sha256.New()
	for _, file := range selected {
		digest.Write([]byte(file))
		digest.Write([]byte{0})
	}
	if len(selected) != s.Files || s.SelectionSHA256 != "" && hex.EncodeToString(digest.Sum(nil)) != s.SelectionSHA256 {
		t.Fatal("COV frozen selection mismatch")
	}
	return tree, selected
}

func covTree(tree fstest.MapFS) map[string][]byte {
	out := map[string][]byte{}
	for file, value := range tree {
		if value.Mode.IsRegular() {
			out[file] = value.Data
		}
	}
	return out
}

func covTruth(t *testing.T, python, compiler, pool string, s covSubject, root string, paths []string) ([]depbench.TruthFile, map[string]string) {
	t.Helper()
	gaps := map[string]string{}
	var cmd *exec.Cmd
	if s.Dialect == "python" {
		script, err := filepath.Abs(filepath.Join(covOracleDir, "cov_run.py"))
		if err != nil {
			t.Fatal("COV Python adapter unavailable")
		}
		cmd = exec.CommandContext(t.Context(), python, "-I", "-B", "-S", script, "python-truth", pool, s.Name, root)
	} else {
		// Only this directory contains holdout per-file transient artifacts. The
		// directory is private, and removed before product construction, even on error.
		private := t.TempDir()
		defer os.RemoveAll(private)
		covWrite(t, filepath.Join(private, "inputs.json"), asJSON(t, paths))
		script, err := filepath.Abs(filepath.Join(tsjsOracleDir, "trace.cjs"))
		if err != nil {
			t.Fatal("COV compiler adapter unavailable")
		}
		cmd = exec.CommandContext(t.Context(), "node", script, compiler, root, filepath.Join(private, "inputs.json"), filepath.Join(private, "trace.log"))
	}
	cmd.Dir = root
	cmd.Env = slices.DeleteFunc(os.Environ(), func(v string) bool {
		return strings.HasPrefix(v, "NODE_OPTIONS=") || strings.HasPrefix(v, "NODE_PATH=") || strings.HasPrefix(v, "PYTHONPATH=")
	})
	data, err := cmd.Output()
	if err != nil {
		t.Fatal("COV pinned oracle invocation failed; no per-file diagnostics emitted")
	}
	var truth []depbench.TruthFile
	if s.Dialect == "python" {
		var native pythonOracleSubject
		if json.Unmarshal(data, &native) != nil || len(native.Rows) != len(paths) {
			t.Fatal("COV Python native capture malformed")
		}
		for _, row := range native.Rows {
			if row.Gap != "" {
				gaps[row.Path] = row.Gap
			} else {
				truth = append(truth, depbench.TruthFile{Path: row.Path, Edges: row.Edges})
			}
		}
	} else {
		var native tsjsNativeResult
		if json.Unmarshal(data, &native) != nil {
			t.Fatal("COV compiler native capture malformed")
		}
		truth = native.Truth
		covered := map[string]bool{}
		for _, row := range truth {
			covered[row.Path] = true
		}
		for _, file := range paths {
			if !covered[file] {
				gaps[file] = "native-no-truth"
			}
		}
	}
	return truth, gaps
}

func covReach(t *testing.T, reader *deps.Reader, tree fstest.MapFS, selected []string, initial []deps.FileDeps) depbench.COVClosureCounts {
	t.Helper()
	covViewForReach := covView(tree)
	files := map[string]deps.FileDeps{}
	children := map[string][]string{}
	for file, entry := range tree {
		if entry.Mode.IsRegular() {
			files[file] = deps.FileDeps{Path: file}
			children[path.Dir(file)] = append(children[path.Dir(file)], file)
		}
	}
	read := map[string]bool{}
	for _, f := range initial {
		files[f.Path] = f
		read[f.Path] = true
	}
	todo := slices.Clone(selected)
	seen := map[string]bool{}
	for len(todo) > 0 {
		file := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if seen[file] {
			continue
		}
		seen[file] = true
		if !read[file] {
			if entry := tree[file]; entry != nil && entry.Mode.IsRegular() {
				f, err := reader.ReadFile(t.Context(), covViewForReach, deps.Input{Path: file}, entry.Data)
				if err != nil {
					t.Fatal("COV closure target read failed")
				}
				files[file] = f
			}
			read[file] = true
		}
		for _, e := range files[file].Edges {
			switch e.Resolution {
			case deps.ResolvedFile:
				todo = append(todo, e.To)
			case deps.ResolvedDirectory:
				todo = append(todo, children[e.To]...)
			}
		}
	}
	return depbench.COVClosure(selected, files)
}

func covAddClosure(dst *depbench.COVClosureCounts, src depbench.COVClosureCounts) {
	dst.Selected += src.Selected
	dst.Complete += src.Complete
	dst.OnlyUnmeasured += src.OnlyUnmeasured
	dst.OnlyMeasured += src.OnlyMeasured
	dst.MeasuredAndUnmeasured += src.MeasuredAndUnmeasured
}

func covAppend(dst *depbench.Observation, src depbench.Observation, prefix string) {
	labelEdges := func(edges []deps.Edge) []deps.Edge {
		out := slices.Clone(edges)
		for i := range out {
			e := &out[i]
			e.From = prefix + e.From
			if e.Resolution == deps.ResolvedFile || e.Resolution == deps.ResolvedDirectory {
				e.To = prefix + e.To
			}
		}
		return out
	}
	add := func(dst *[]deps.FileDeps, source []deps.FileDeps) {
		for _, f := range source {
			f.Path = prefix + f.Path
			f.Edges = labelEdges(f.Edges)
			f.Unknowns = slices.Clone(f.Unknowns)
			for i := range f.Unknowns {
				f.Unknowns[i].From = prefix + f.Unknowns[i].From
			}
			*dst = append(*dst, f)
		}
	}
	add(&dst.Files, src.Files)
	add(&dst.Repeat, src.Repeat)
	add(&dst.Relocated, src.Relocated)
	for _, row := range src.Truth {
		row.Path = prefix + row.Path
		row.Edges = slices.Clone(row.Edges)
		for i := range row.Edges {
			e := &row.Edges[i]
			if e.Resolution == deps.ResolvedFile || e.Resolution == deps.ResolvedDirectory {
				e.To = prefix + e.To
			}
		}
		dst.Truth = append(dst.Truth, row)
	}
}

func covWrite(t *testing.T, name string, data []byte) {
	t.Helper()
	if err := os.WriteFile(name, data, 0o600); err != nil {
		t.Fatal("COV private artifact write failed")
	}
}

// Index directories once. fstest.MapFS alone scans the entire corpus at every
// directory probe; this view preserves its bytes and modes without that cost.
type covFrozenView struct {
	tree fstest.MapFS
	info map[string]covFileInfo
	dirs map[string][]fs.DirEntry
}

type covFileInfo struct {
	name string
	size int64
	mode fs.FileMode
}

func (i covFileInfo) Name() string       { return i.name }
func (i covFileInfo) Size() int64        { return i.size }
func (i covFileInfo) Mode() fs.FileMode  { return i.mode }
func (i covFileInfo) ModTime() time.Time { return time.Time{} }
func (i covFileInfo) IsDir() bool        { return i.mode.IsDir() }
func (i covFileInfo) Sys() any           { return nil }

func covView(tree fstest.MapFS) *covFrozenView {
	v := &covFrozenView{tree: tree, info: map[string]covFileInfo{".": {".", 0, fs.ModeDir | 0o555}}, dirs: map[string][]fs.DirEntry{}}
	for file, entry := range tree {
		v.info[file] = covFileInfo{path.Base(file), int64(len(entry.Data)), entry.Mode}
		for dir := path.Dir(file); dir != "."; dir = path.Dir(dir) {
			v.info[dir] = covFileInfo{path.Base(dir), 0, fs.ModeDir | 0o555}
		}
	}
	for file, info := range v.info {
		if file != "." {
			parent := path.Dir(file)
			v.dirs[parent] = append(v.dirs[parent], fs.FileInfoToDirEntry(info))
		}
	}
	for dir, entries := range v.dirs {
		slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
		v.dirs[dir] = entries
	}
	return v
}
func (v *covFrozenView) Open(name string) (fs.File, error) { return v.tree.Open(name) }
func (v *covFrozenView) Lstat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrInvalid
	}
	i, ok := v.info[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return i, nil
}
func (v *covFrozenView) Stat(name string) (fs.FileInfo, error) {
	i, err := v.Lstat(name)
	if err != nil || i.Mode()&fs.ModeSymlink != 0 {
		return fs.Stat(v.tree, name)
	}
	return i, nil
}
func (v *covFrozenView) ReadDir(name string) ([]fs.DirEntry, error) {
	info, err := v.Stat(name)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fs.ErrInvalid
	}
	if _, ok := v.info[name]; !ok || v.info[name].mode&fs.ModeSymlink != 0 {
		return fs.ReadDir(v.tree, name)
	}
	return slices.Clone(v.dirs[name]), nil
}
