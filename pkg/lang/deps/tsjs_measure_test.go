package deps_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
	"github.com/geoffmilleraz/fathom/pkg/lang/deps/internal/depbench"
)

type tsjsSubject struct {
	Name, URL, Commit, Dialect, SelectionSHA256 string
	Prefixes, Suffixes                          []string
	Files                                       int
}

type tsjsNativeResult struct {
	Truth []depbench.TruthFile
	Stats map[string]json.RawMessage
}

var tsjsOracleDir = filepath.Join("..", "..", "..", "calibration", "oracles", "deps-tsjs")

func TestTSJSNativeOracle(t *testing.T) {
	mode := os.Getenv("FATHOM_DEP_TSJS_NATIVE")
	if mode == "" {
		t.Skip("set FATHOM_DEP_TSJS_NATIVE=control or corpus to request the pinned compiler")
	}
	deps.TSJSUseCandidateAdmissionForTest(t)
	compiler := tsjsRequiredPath(t, "FATHOM_DEP_TSJS_COMPILER")
	scratch := tsjsRequiredPath(t, "FATHOM_DEP_TSJS_SCRATCH")
	t.Run("control", func(t *testing.T) { tsjsOracleControl(t, compiler, scratch) })
	switch mode {
	case "control":
		return
	case "corpus":
		t.Run("corpus", func(t *testing.T) {
			tsjsMeasure(t, compiler, scratch, tsjsRequiredPath(t, "FATHOM_DEP_TSJS_SUBJECTS"))
		})
	default:
		t.Fatalf("unexpected FATHOM_DEP_TSJS_NATIVE mode %q", mode)
	}
}

func tsjsRequiredPath(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if !filepath.IsAbs(value) {
		t.Fatalf("%s must name an absolute path", name)
	}
	if _, err := os.Stat(value); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return value
}

func tsjsNative(t *testing.T, compiler, destination, root string, paths []string) tsjsNativeResult {
	t.Helper()
	if err := os.MkdirAll(destination, 0o700); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(destination, "inputs.json")
	if err := os.WriteFile(input, asJSON(t, paths), 0o600); err != nil {
		t.Fatal(err)
	}
	trace, err := filepath.Abs(filepath.Join(tsjsOracleDir, "trace.cjs"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), "node", trace, compiler, root, input, filepath.Join(destination, "trace.log"))
	cmd.Dir = root
	// Node startup must not load modules named by the caller's NODE_OPTIONS.
	cmd.Env = slices.DeleteFunc(os.Environ(), func(v string) bool {
		return strings.HasPrefix(v, "NODE_OPTIONS=") || strings.HasPrefix(v, "NODE_PATH=")
	})
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		t.Fatalf("pinned TypeScript oracle: %v\n%s", err, stderr.Bytes())
	}
	if err := os.WriteFile(filepath.Join(destination, "truth.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	var out tsjsNativeResult
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func tsjsOracleControl(t *testing.T, compiler, scratch string) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"tsconfig.json": `{"compilerOptions":{"module":"esnext","moduleResolution":"bundler","paths":{"@alias":["./alias.ts"]}}}`,
		"package.json":  `{"name":"sample","type":"module","exports":{"./feature":{"types":"./feature.d.ts","import":"./feature.mjs"}}}`,
		"main.ts":       "import { value } from '@alias';\nexport * from 'sample/feature';\nexport const next = import('./target.js');\n",
		"alias.ts":      "export const value = 1;\n",
		"target.ts":     "export const value = 2;\n",
		"feature.d.ts":  "export declare const feature : number;\n",
		"feature.mjs":   "export const feature = 3;\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	native := func(label string) tsjsNativeResult {
		return tsjsNative(t, compiler, filepath.Join(scratch, "control", label), root, []string{"main.ts"})
	}
	before := native("before")
	var imports []string
	for _, row := range before.Truth {
		for _, e := range row.Edges {
			if e.Kind == deps.KindImport {
				imports = append(imports, e.To)
			}
		}
	}
	if !slices.Equal(imports, []string{"alias.ts", "feature.d.ts", "target.ts"}) {
		t.Fatalf("control native operand binding: %v, stats %s", imports, asJSON(t, before.Stats))
	}
	r := tsjsCandidateReader(t)
	files := tsjsReadCorpus(t, r, root, "typescript", []string{"main.ts"})
	o := depbench.Observation{Files: files, Repeat: files, Relocated: files, Truth: before.Truth}
	if score := depbench.Score(o); !score.B1 || !score.B2 || score.Expected < 4 {
		t.Fatalf("control score: %+v", score)
	}
	files[0].Edges = nil
	if score := depbench.Score(o); score.B1 || score.Missing == 0 {
		t.Fatalf("control B1 truth-scope mutant: %+v", score)
	}
	afterReaderMutation := native("after-reader-mutation")
	if !bytes.Equal(asJSON(t, before.Truth), asJSON(t, afterReaderMutation.Truth)) {
		t.Fatal("mutating reader output changed independent oracle truth")
	}
	src := "export * from 'sample/feature';\nexport const next = import('./target.js');\n"
	if err := os.WriteFile(filepath.Join(root, "main.ts"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	if after := native("after-source-mutation"); bytes.Equal(asJSON(t, before.Truth), asJSON(t, after.Truth)) {
		t.Fatal("removing a source import did not change independent oracle truth")
	}
	t.Log("pinned compiler control: alias, export and extension substitution PASS; dropped reader edge rejected, independent truth responds only to source mutation")
}

func tsjsCandidateReader(t *testing.T) *deps.Reader {
	t.Helper()
	r, err := deps.New(deps.TrustUnmeasured("typescript"), deps.TrustUnmeasured("javascript"))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func tsjsMeasure(t *testing.T, compiler, scratch, checkouts string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(tsjsOracleDir, "pins.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pins struct{ Subjects []tsjsSubject }
	if err := json.Unmarshal(data, &pins); err != nil {
		t.Fatal(err)
	}
	reader := tsjsCandidateReader(t)
	pools := make(map[string]depbench.Observation)
	var combined depbench.Observation
	for _, s := range pins.Subjects {
		primary := filepath.Join(checkouts, "primary", s.Name)
		relocated := filepath.Join(checkouts, "relocated", s.Name)
		a, b := tsjsSelection(t, s, primary), tsjsSelection(t, s, relocated)
		if !slices.Equal(a, b) || primary == relocated {
			t.Fatalf("relocated selection differs for %s", s.Name)
		}
		native := tsjsNative(t, compiler, filepath.Join(scratch, s.Name), primary, a)
		// The compiler observation is already fixed before the reader runs.
		first := tsjsReadCorpus(t, reader, primary, s.Dialect, a)
		second := tsjsReadCorpus(t, reader, primary, s.Dialect, b)
		third := tsjsReadCorpus(t, reader, relocated, s.Dialect, b)
		o := depbench.Observation{Files: first, Repeat: second, Relocated: third, Truth: native.Truth}
		score := depbench.Score(o)
		if !score.B3 {
			t.Errorf("%s output differs between runs or checkout paths", s.Name)
		}
		tsjsSelection(t, s, primary)
		tsjsSelection(t, s, relocated)
		t.Logf("DEP-M1 %s %s SCORE %s NATIVE %s OUTPUT %x", s.Dialect, s.Name, asJSON(t, score), asJSON(t, native.Stats), sha256.Sum256(asJSON(t, first)))
		if err := os.WriteFile(filepath.Join(scratch, s.Name, "reader.json"), asJSON(t, first), 0o600); err != nil {
			t.Fatal(err)
		}
		pool := pools[s.Dialect]
		tsjsAppendObservation(&pool, o, s.Name+"/")
		pools[s.Dialect] = pool
		tsjsAppendObservation(&combined, o, s.Name+"/")
	}
	for _, dialect := range []string{"typescript", "javascript"} {
		pool := pools[dialect]
		t.Logf("DEP-M1 %s pooled SCORE %s OUTPUT %x", dialect, asJSON(t, depbench.Score(pool)), sha256.Sum256(asJSON(t, pool.Files)))
	}
	t.Logf("DEP-M1 combined SCORE %s OUTPUT %x", asJSON(t, depbench.Score(combined)), sha256.Sum256(asJSON(t, combined.Files)))
}

func tsjsAppendObservation(dst *depbench.Observation, src depbench.Observation, prefix string) {
	add := func(dst []deps.FileDeps, src []deps.FileDeps) []deps.FileDeps {
		for _, f := range src {
			f.Path = prefix + f.Path
			dst = append(dst, f)
		}
		return dst
	}
	dst.Files = add(dst.Files, src.Files)
	dst.Repeat = add(dst.Repeat, src.Repeat)
	dst.Relocated = add(dst.Relocated, src.Relocated)
	for _, row := range src.Truth {
		row.Path = prefix + row.Path
		dst.Truth = append(dst.Truth, row)
	}
}

func tsjsReadCorpus(t *testing.T, r *deps.Reader, root, dialect string, paths []string) []deps.FileDeps {
	t.Helper()
	inputs := make([]deps.Input, 0, len(paths))
	for _, name := range paths {
		inputs = append(inputs, deps.Input{Path: name, Dialect: dialect})
	}
	files, err := r.ReadFiles(context.Background(), os.DirFS(root), inputs)
	if err != nil {
		t.Fatalf("%s reader: %v", dialect, err)
	}
	return files
}

func tsjsSelection(t *testing.T, subject tsjsSubject, root string) []string {
	t.Helper()
	git := func(args ...string) []byte {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = root
		b, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s checkout: git %s: %v\n%s", subject.Name, args[0], err, b)
		}
		return b
	}
	if strings.TrimSpace(string(git("rev-parse", "HEAD"))) != subject.Commit ||
		strings.TrimSpace(string(git("remote", "get-url", "origin"))) != subject.URL {
		t.Fatalf("%s checkout did not match the pinned identity", subject.Name)
	}
	if len(git("status", "--porcelain", "--untracked-files=all", "--ignored=matching")) != 0 {
		t.Fatalf("%s checkout has modified, untracked or ignored files", subject.Name)
	}
	var files []string
	for line := range bytes.SplitSeq(git("ls-tree", "-rz", subject.Commit), []byte{0}) {
		meta, name, ok := bytes.Cut(line, []byte{'\t'})
		if !ok || !(bytes.HasPrefix(meta, []byte("100644 ")) || bytes.HasPrefix(meta, []byte("100755 "))) {
			continue
		}
		file := string(name)
		if slices.ContainsFunc(subject.Prefixes, func(s string) bool { return strings.HasPrefix(file, s) }) &&
			slices.ContainsFunc(subject.Suffixes, func(s string) bool { return strings.HasSuffix(file, s) }) {
			files = append(files, file)
		}
	}
	slices.Sort(files)
	h := sha256.New()
	for _, file := range files {
		fmt.Fprintf(h, "%s%c", file, 0)
	}
	if len(files) != subject.Files || hex.EncodeToString(h.Sum(nil)) != subject.SelectionSHA256 {
		t.Fatalf("%s selected %d files with digest %x instead of its frozen selection", subject.Name, len(files), h.Sum(nil))
	}
	return files
}
