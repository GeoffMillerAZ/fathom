package deps_test

import (
	"bytes"
	"context"
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

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
	"github.com/geoffmilleraz/fathom/pkg/lang/deps/internal/depbench"
)

type dockerPins struct {
	Oracle, Go, Selection string
	OracleSum             string `json:"oracle_sum"`
	Subjects              []dockerSubject
}

type dockerSubject struct {
	Name, Commit, URL string
	Files             int
	SelectionSHA256   string `json:"selection_sha256"`
}

type dockerTruthRow struct {
	Path   string
	Parsed bool
	Edges  []depbench.TruthEdge
	Gaps   []string
}

type dockerGrade struct {
	depbench.Reading
	NativeParsed, NativePartial, NativePartialEdges int
	NativeGapFiles                                  map[string]int
}

func TestDockerfileAdmissionReading(t *testing.T) {
	if os.Getenv("DEP_DOCKER_READING") != "1" {
		t.Skip("set DEP_DOCKER_READING=1 for the pinned BuildKit reading")
	}
	pins, oracle := loadDockerOracle(t)
	corpus := requireDockerPath(t, "DEP_DOCKER_CORPUS")
	receipts := requireDockerPath(t, "DEP_DOCKER_RECEIPTS")
	selector := regexp.MustCompile(pins.Selection)
	r, err := deps.New(deps.TrustUnmeasured("dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	var pooled depbench.Observation
	var allRows []dockerTruthRow
	for _, subject := range pins.Subjects {
		first := filepath.Join(corpus, "first", subject.Name)
		second := filepath.Join(corpus, "relocated", subject.Name)
		paths := dockerSelection(t, first, subject, selector)
		if files := dockerSelection(t, second, subject, selector); !slices.Equal(paths, files) {
			t.Fatalf("%s: relocated selection differs", subject.Name)
		}
		raw, rows := runDockerOracle(t, oracle, first, paths)
		repeated, _ := runDockerOracle(t, oracle, first, paths)
		relocated, _ := runDockerOracle(t, oracle, second, paths)
		if !bytes.Equal(raw, repeated) || !bytes.Equal(raw, relocated) {
			t.Fatalf("%s: BuildKit truth changed across reads or checkout paths", subject.Name)
		}
		observation := readDockerObservation(t, r, first, second, paths, rows)
		if after := dockerSelection(t, first, subject, selector); !slices.Equal(paths, after) {
			t.Fatalf("%s: first checkout changed", subject.Name)
		}
		if after := dockerSelection(t, second, subject, selector); !slices.Equal(paths, after) {
			t.Fatalf("%s: second checkout changed", subject.Name)
		}
		writeDockerReceipt(t, receipts, subject.Name+"-native.json", raw)
		writeDockerReceipt(t, receipts, subject.Name+"-product.json", asJSON(t, observation.Files))
		logDockerGrade(t, subject.Name, observation, rows)
		appendDockerObservation(&pooled, subject.Name, observation)
		allRows = append(allRows, rows...)
	}
	reading := logDockerGrade(t, "pooled", pooled, allRows)
	if len(pins.Subjects) < 3 || reading.Files < 30 || !reading.B3 {
		t.Fatalf("population or determinism requirement failed: %d subjects, %d files, B3=%t", len(pins.Subjects), reading.Files, reading.B3)
	}
}

func TestDockerfileNativeFixture(t *testing.T) {
	if os.Getenv("DEP_DOCKER_NATIVE_FIXTURE") != "1" {
		t.Skip("set DEP_DOCKER_NATIVE_FIXTURE=1 to verify the pinned oracle on a synthetic fixture")
	}
	_, oracle := loadDockerOracle(t)
	first, second := t.TempDir(), t.TempDir()
	const source = "FROM scratch AS seed\nCOPY --exclude=drop src /dest\n" +
		"ADD [\"asset.txt\", \"/assets/\"]\nFROM seed\nCOPY --from=0 /somewhere /elsewhere\n" +
		"RUN --mount=type=bind,src=mounted,target=/mnt true\n"
	files := map[string]string{
		"Dockerfile": source, ".dockerignore": "**/*.tmp\n!src/keep.tmp\n",
		"src/keep": "", "src/drop": "", "src/cut.tmp": "", "src/keep.tmp": "",
		"asset.txt": "", "alternate.txt": "", "mounted": "",
	}
	for _, root := range []string{first, second} {
		for name, data := range files {
			dest := filepath.Join(root, name)
			if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dest, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	r, err := deps.New(deps.TrustUnmeasured("dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{"Dockerfile"}
	native, rows := runDockerOracle(t, oracle, first, paths)
	o := readDockerObservation(t, r, first, second, paths, rows)
	s := depbench.Score(o)
	if rows[0].Gaps == nil || len(rows[0].Gaps) != 0 || len(rows[0].Edges) != 5 ||
		s.Expected != 5 || s.CompleteEdges != 5 || !s.B1 || !s.B2 || !s.B3 || s.Extra != 0 {
		t.Fatalf("native control: %+v (%+v)", s, rows[0])
	}

	o.Files[0].Edges = nil
	if score := depbench.Score(o); score.B1 || score.Missing != 5 {
		t.Fatalf("removing the five product edges did not fail B1: %+v", score)
	}
	after, _ := runDockerOracle(t, oracle, first, paths)
	if !bytes.Equal(native, after) {
		t.Fatal("perturbing product evidence changed the native result")
	}
	updated := strings.Replace(source, "asset.txt", "alternate.txt", 1)
	if err := os.WriteFile(filepath.Join(second, paths[0]), []byte(updated), 0o600); err != nil {
		t.Fatal(err)
	}
	after, changed := runDockerOracle(t, oracle, second, paths)
	if bytes.Equal(native, after) || !slices.ContainsFunc(changed[0].Edges, func(e depbench.TruthEdge) bool {
		return e.To == "alternate.txt"
	}) {
		t.Fatal("changing the native input did not change its bound ADD source")
	}
}

func loadDockerOracle(t *testing.T) (dockerPins, string) {
	t.Helper()
	data, err := os.ReadFile("../../../calibration/oracles/deps-docker/pins.json")
	if err != nil {
		t.Fatal(err)
	}
	var pins dockerPins
	if err := json.Unmarshal(data, &pins); err != nil {
		t.Fatal(err)
	}
	bin := requireDockerPath(t, "DEP_DOCKER_ORACLE")
	out, err := exec.Command(bin, "-version").CombinedOutput()
	if err != nil {
		t.Fatalf("pinned BuildKit oracle unavailable: %v (%s)", err, out)
	}
	if strings.TrimSpace(string(out)) != strings.Join([]string{pins.Oracle, pins.OracleSum, pins.Go}, " ") {
		t.Fatalf("oracle pin mismatch: %s", out)
	}
	return pins, bin
}

func requireDockerPath(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if !filepath.IsAbs(value) {
		t.Fatalf("%s must be an absolute path", name)
	}
	if _, err := os.Stat(value); err != nil {
		t.Fatalf("%s unavailable: %v", name, err)
	}
	return value
}

func dockerSelection(t *testing.T, root string, subject dockerSubject, selector *regexp.Regexp) []string {
	t.Helper()
	rev := dockerGit(t, root, "rev-parse", "HEAD")
	if strings.TrimSpace(string(rev)) != subject.Commit ||
		len(dockerGit(t, root, "status", "--porcelain", "--untracked-files=all", "--ignored")) != 0 {
		t.Fatalf("%s: expected a clean checkout at the pinned commit", subject.Name)
	}
	var out []string
	for record := range bytes.SplitSeq(dockerGit(t, root, "ls-files", "--stage", "-z"), []byte{0}) {
		if len(record) == 0 {
			continue
		}
		meta, name, ok := strings.Cut(string(record), "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 || fields[2] != "0" {
			t.Fatal("invalid stage-0 tracked-file selection")
		}
		if (fields[0] == "100644" || fields[0] == "100755") && selector.MatchString(path.Base(name)) {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	digest := sha256.Sum256([]byte(strings.Join(out, "\x00") + "\x00"))
	if len(out) != subject.Files || hex.EncodeToString(digest[:]) != subject.SelectionSHA256 {
		t.Fatalf("%s: selected population count or fingerprint changed", subject.Name)
	}
	return out
}

func dockerGit(t *testing.T, root string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, out)
	}
	return out
}

func runDockerOracle(t *testing.T, bin, root string, paths []string) ([]byte, []dockerTruthRow) {
	t.Helper()
	cmd := exec.Command(bin, append([]string{"-root", root, "--"}, paths...)...)
	out, err := cmd.Output()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			t.Fatalf("BuildKit adapter failed: %v (%s)", err, e.Stderr)
		}
		t.Fatal(err)
	}
	var rows []dockerTruthRow
	if err := json.Unmarshal(out, &rows); err != nil || len(rows) != len(paths) {
		t.Fatalf("invalid native evidence: %d rows for %d paths (%v)", len(rows), len(paths), err)
	}
	for i, row := range rows {
		if row.Path != paths[i] || row.Edges == nil || row.Gaps == nil || !row.Parsed && len(row.Gaps) == 0 {
			t.Fatal("native row missing identity, parse outcome or evidence arrays")
		}
	}
	return out, rows
}

func readDockerObservation(t *testing.T, r *deps.Reader, first, second string, paths []string, rows []dockerTruthRow) depbench.Observation {
	t.Helper()
	read := func(dir string, reversed bool) []deps.FileDeps {
		t.Helper()
		root, err := os.OpenRoot(dir)
		if err != nil {
			t.Fatal(err)
		}
		defer root.Close()
		inputs := make([]deps.Input, 0, len(paths))
		for _, name := range paths {
			inputs = append(inputs, deps.Input{Path: name})
		}
		if reversed {
			slices.Reverse(inputs)
		}
		out, err := r.ReadFiles(context.Background(), dockerCheckoutFS{root.FS()}, inputs)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range out {
			if f.Dialect != "dockerfile" {
				t.Fatalf("selected path did not route to Dockerfile: %q", f.Path)
			}
		}
		return out
	}
	var truth []depbench.TruthFile
	for _, row := range rows {
		if row.Parsed && len(row.Gaps) == 0 {
			truth = append(truth, depbench.TruthFile{Path: row.Path, Edges: row.Edges})
		}
	}
	return depbench.Observation{Files: read(first, false), Repeat: read(first, true), Relocated: read(second, false), Truth: truth}
}

// The read view represents checkout inputs, with Git's own administrative
// directory removed. Submodule directories are empty in the pinned checkouts.
type dockerCheckoutFS struct{ fs.FS }

func (d dockerCheckoutFS) Open(name string) (fs.File, error) {
	if name == ".git" || strings.HasPrefix(name, ".git/") {
		return nil, fs.ErrNotExist
	}
	f, err := d.FS.Open(name)
	if err == nil && name == "." {
		if root, ok := f.(fs.ReadDirFile); ok {
			return dockerCheckoutRoot{root}, nil
		}
	}
	return f, err
}

func (d dockerCheckoutFS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := fs.ReadDir(d.FS, name)
	if name == "." {
		entries = slices.DeleteFunc(entries, func(e fs.DirEntry) bool { return e.Name() == ".git" })
	}
	return entries, err
}

type dockerCheckoutRoot struct{ fs.ReadDirFile }

func (d dockerCheckoutRoot) ReadDir(n int) ([]fs.DirEntry, error) {
	entries, err := d.ReadDirFile.ReadDir(n)
	return slices.DeleteFunc(entries, func(e fs.DirEntry) bool { return e.Name() == ".git" }), err
}

func logDockerGrade(t *testing.T, name string, o depbench.Observation, rows []dockerTruthRow) dockerGrade {
	t.Helper()
	grade := dockerGrade{Reading: depbench.Score(o), NativeGapFiles: make(map[string]int)}
	for _, row := range rows {
		if row.Parsed {
			grade.NativeParsed++
			if len(row.Gaps) > 0 {
				grade.NativePartial++
				grade.NativePartialEdges += len(row.Edges)
			}
		}
		for _, gap := range row.Gaps {
			grade.NativeGapFiles[gap]++
		}
	}
	t.Logf("%s %s", name, asJSON(t, grade))
	return grade
}

func appendDockerObservation(pooled *depbench.Observation, subject string, o depbench.Observation) {
	for i, batch := range [][]deps.FileDeps{o.Files, o.Repeat, o.Relocated} {
		for _, f := range batch {
			f.Path = subject + "/" + f.Path
			if i == 0 {
				pooled.Files = append(pooled.Files, f)
			} else if i == 1 {
				pooled.Repeat = append(pooled.Repeat, f)
			} else {
				pooled.Relocated = append(pooled.Relocated, f)
			}
		}
	}
	for _, row := range o.Truth {
		row.Path = subject + "/" + row.Path
		pooled.Truth = append(pooled.Truth, row)
	}
}

func writeDockerReceipt(t *testing.T, dir, name string, data []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		t.Fatal(err)
	}
}
