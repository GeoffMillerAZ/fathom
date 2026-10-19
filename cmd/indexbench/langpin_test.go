package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

const pinRev = "0123456789abcdef0123456789abcdef01234567"

// pinRoot builds a repository root for lang-pin: the real queue and lock, a
// prior pre-fill holding the hand-kept data (an override, and the two
// verdicts the owner decision rests on), and a small catalogue named by real
// queue ids.
func pinRoot(t *testing.T) (root, catalogue string) {
	t.Helper()
	root = t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, rel := range []string{
		"docs/plans/language-breadth.queue.json",
		"internal/extractor/tsengine/grammars/grammars.lock.json",
	} {
		b, err := os.ReadFile(filepath.Join("..", "..", rel))
		must(err)
		if strings.HasSuffix(rel, "grammars.lock.json") {
			b = withoutLockRow(t, b, "scss") // the fixture's clean grammar, locked since wave lf5 (LB-F465)
		}
		must(os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755))
		must(os.WriteFile(filepath.Join(root, rel), b, 0o644))
	}
	prior := langbench.GrammarPrefill{
		Overrides: map[string]string{"objective-c": "objc"},
		Verdicts: map[string]langbench.PinVerdict{
			"prolog": {Repo: "foxyseta/tree-sitter-prolog", Commit: strings.Repeat("a", 40),
				LicenceVerdict: langbench.LicenceVerdict{Licence: "AGPL-3.0", Basis: langbench.BasisFile, Refused: true, Reason: "AGPL is refused"}},
			"abap": {Repo: "kennyhml/tree-sitter-abap", Commit: strings.Repeat("b", 40),
				LicenceVerdict: langbench.LicenceVerdict{Licence: "MIT", Basis: langbench.BasisDeclared, Reason: "declared in package.json, Cargo.toml, tree-sitter.json"}},
		},
	}
	b, err := langbench.MarshalPrefill(prior)
	must(err)
	must(os.WriteFile(filepath.Join(root, langbench.GrammarPrefillPath), b, 0o644))

	catalogue = t.TempDir()
	must(os.MkdirAll(filepath.Join(catalogue, "sources"), 0o755))
	defs := `{
  "scss":  {"repo": "https://github.com/tree-sitter-grammars/tree-sitter-scss", "rev": "` + strings.Repeat("1", 40) + `"},
  "less":  {"repo": "https://github.com/rhino1998/tree-sitter-less", "rev": "` + strings.Repeat("2", 40) + `", "generate": true},
  "d":     {"repo": "https://github.com/gdamore/tree-sitter-d", "rev": "` + strings.Repeat("3", 40) + `"},
  "vba": {"repo": "https://github.com/harumiWeb/tree-sitter-vba", "rev": "` + strings.Repeat("4", 40) + `"},
  "prolog": {"repo": "https://github.com/Rukiza/tree-sitter-prolog", "rev": "` + strings.Repeat("5", 40) + `"},
  "objc":  {"repo": "https://github.com/tree-sitter-grammars/tree-sitter-objc", "rev": "` + strings.Repeat("6", 40) + `"},
  "fsharp": {"repo": "https://github.com/ionide/tree-sitter-fsharp", "rev": "` + strings.Repeat("7", 40) + `", "directory": "fsharp", "abi_version": 14, "generate": true}
}`
	lic := `{
  "tree-sitter-grammars/tree-sitter-scss": "MIT",
  "rhino1998/tree-sitter-less": "MIT",
  "gdamore/tree-sitter-d": "AGPL-3.0",
  "Rukiza/tree-sitter-prolog": "ISC",
  "tree-sitter-grammars/tree-sitter-objc": "MIT",
  "ionide/tree-sitter-fsharp": "MIT"
}`
	must(os.WriteFile(filepath.Join(catalogue, "sources", "language_definitions.json"), []byte(defs), 0o644))
	must(os.WriteFile(filepath.Join(catalogue, "sources", "license_cache.json"), []byte(lic), 0o644))
	return root, catalogue
}

// withoutLockRow is the lock with blob's row removed.
func withoutLockRow(t *testing.T, lock []byte, blob string) []byte {
	t.Helper()
	var l map[string]any
	if err := json.Unmarshal(lock, &l); err != nil {
		t.Fatal(err)
	}
	rows, _ := l["grammars"].([]any)
	kept := rows[:0]
	for _, r := range rows {
		if m, ok := r.(map[string]any); !ok || m["blob"] != blob {
			kept = append(kept, r)
		}
	}
	l["grammars"] = kept
	out, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func runPin(args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = cmdLangPin(args, &out, &errb)
	return code, out.String(), errb.String()
}

// TestCmdLangPin is LB-F50 end to end: -write pre-fills the queue's unlocked
// tags items from the catalogue and re-reads the hand-kept data; -id prints
// the draft row and exits 1 for a refused or unlicensed grammar.
func TestCmdLangPin(t *testing.T) {
	root, cat := pinRoot(t)
	dataPath := filepath.Join(root, langbench.GrammarPrefillPath)

	// Without -write nothing is written.
	before, _ := os.ReadFile(dataPath)
	code, out, errs := runPin("-root", root, "-catalogue", cat, "-rev", pinRev)
	if code != 0 {
		t.Fatalf("dry run exit %d: %s", code, errs)
	}
	if after, _ := os.ReadFile(dataPath); !bytes.Equal(before, after) {
		t.Error("a run without -write changed the data file")
	}
	if _, err := langbench.ParseGrammarPrefill([]byte(out)); err != nil {
		t.Errorf("the dry run prints the data file: %v", err)
	}

	code, _, errs = runPin("-root", root, "-catalogue", cat, "-rev", pinRev, "-write")
	if code != 0 {
		t.Fatalf("-write exit %d: %s", code, errs)
	}
	first, _ := os.ReadFile(dataPath)
	code, _, errs = runPin("-root", root, "-catalogue", cat, "-rev", pinRev, "-write")
	if code != 0 {
		t.Fatalf("second -write exit %d: %s", code, errs)
	}
	if second, _ := os.ReadFile(dataPath); !bytes.Equal(first, second) {
		t.Error("-write is not byte-stable")
	}
	d, err := langbench.ParseGrammarPrefill(first)
	if err != nil {
		t.Fatal(err)
	}
	if d.Revision != pinRev || d.Source != langbench.CatalogueSource || d.Verdicts["prolog"].Repo != "foxyseta/tree-sitter-prolog" || d.Overrides["objective-c"] != "objc" {
		t.Errorf("header or hand-kept data lost: %+v", d)
	}
	for _, id := range []string{"kotlin", "swift", "ruby", "helm", "dbt"} {
		if _, ok := d.Grammars[id]; ok {
			t.Errorf("%s is already in the lock and must not be pre-filled", id)
		}
	}

	// -id: the draft row for a clean grammar, minus the digests.
	code, out, errs = runPin("-root", root, "-id", "scss")
	if code != 0 {
		t.Fatalf("-id scss exit %d: %s", code, errs)
	}
	var row tsengine.GrammarLockRow
	if err := json.Unmarshal([]byte(out), &row); err != nil {
		t.Fatalf("-id scss prints no JSON row: %v\n%s", err, out)
	}
	want := tsengine.GrammarLockRow{Blob: "scss", Repo: "tree-sitter-grammars/tree-sitter-scss", Tag: strings.Repeat("1", 40), Commit: strings.Repeat("1", 40),
		Licence: "MIT", EntryPoints: []string{"tree_sitter_scss"}, ABI: 14}
	if !equalRows(row, want) {
		t.Errorf("scss row = %+v, want %+v", row, want)
	}
	if row.TarballSHA256 != "" || row.BlobSHA256 != "" || row.BlobBytes != 0 || row.GzBytes != 0 {
		t.Errorf("digests and sizes are build.sh's, not the draft's: %+v", row)
	}

	// A directory grammar carries subdirs; a differing grammar is information (exit 0).
	code, out, errs = runPin("-root", root, "-id", "less")
	if code != 0 || !strings.Contains(errs, "grammar-differs:jimliang/tree-sitter-less") || !strings.Contains(out, `"generate": true`) {
		t.Errorf("-id less: exit %d\nstdout %s\nstderr %s", code, out, errs)
	}
	code, out, _ = runPin("-root", root, "-id", "fsharp")
	if code != 0 || !strings.Contains(out, `"subdirs": [`) {
		t.Errorf("-id fsharp (a catalogue directory): exit %d\n%s", code, out)
	}

	// Refused (AGPL in the catalogue), unlicensed (no licence on record).
	for id, why := range map[string]string{"d": "refused", "vba": "unlicensed"} {
		code, out, errs = runPin("-root", root, "-id", id)
		if code != 1 || strings.Contains(out, `"blob"`) || !strings.Contains(errs, why) {
			t.Errorf("-id %s: exit %d, want 1 with %q and no row\nstdout %s\nstderr %s", id, code, why, out, errs)
		}
	}

	// prolog: the queue's grammar is AGPL (recorded verdict); the catalogue's alternative is named.
	code, out, errs = runPin("-root", root, "-id", "prolog")
	if code != 1 || strings.Contains(out, `"blob"`) {
		t.Errorf("-id prolog: exit %d, want 1 and no row\n%s", code, out)
	}
	for _, s := range []string{"foxyseta/tree-sitter-prolog", "AGPL-3.0", "Rukiza/tree-sitter-prolog", "grammar-differs:foxyseta/tree-sitter-prolog"} {
		if !strings.Contains(errs, s) {
			t.Errorf("-id prolog stderr lacks %q:\n%s", s, errs)
		}
	}

	// abap: absent from the catalogue, accepted as declared at the recorded commit.
	code, out, errs = runPin("-root", root, "-id", "abap")
	if code != 0 || !strings.Contains(out, strings.Repeat("b", 40)) || !strings.Contains(errs, "not-in-catalogue") {
		t.Errorf("-id abap: exit %d\nstdout %s\nstderr %s", code, out, errs)
	}

	// An id with no pre-fill is an error, not an empty row.
	if code, _, errs = runPin("-root", root, "-id", "kotlin"); code != 1 || errs == "" {
		t.Errorf("-id kotlin (locked): exit %d stderr %q, want 1 with a message", code, errs)
	}
}

func equalRows(a, b tsengine.GrammarLockRow) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return bytes.Equal(ab, bb)
}

// TestCmdLangPinChecksCatalogueRevision: a git checkout must be at -rev.
func TestCmdLangPinChecksCatalogueRevision(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root, cat := pinRoot(t)
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", cat, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		for _, kv := range os.Environ() {
			if !strings.HasPrefix(kv, "GIT_") { // an inherited hook environment must not aim git at another repository
				c.Env = append(c.Env, kv)
			}
		}
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	git("add", "-A")
	git("commit", "-q", "-m", "catalogue")
	head := git("rev-parse", "HEAD")

	if code, _, errs := runPin("-root", root, "-catalogue", cat, "-rev", pinRev); code != 1 || !strings.Contains(errs, head) {
		t.Errorf("wrong -rev: exit %d stderr %q, want 1 naming the checkout's %s", code, errs, head)
	}
	if code, _, errs := runPin("-root", root, "-catalogue", cat, "-rev", head); code != 0 {
		t.Errorf("right -rev: exit %d: %s", code, errs)
	}
}

func TestCmdLangPinUsage(t *testing.T) {
	root, cat := pinRoot(t)
	for _, args := range [][]string{
		{},
		{"-root", root, "-catalogue", cat}, // no -rev
		{"-root", root, "-catalogue", cat, "-rev", "abc"},                             // not 40 hex
		{"-root", root, "-id", "scss", "-tree", "x"},                                  // two modes
		{"-root", root, "-write"},                                                     // -write needs -catalogue
		{"-root", root, "-catalogue", filepath.Join(root, "nowhere"), "-rev", pinRev}, // unreadable catalogue
	} {
		if code, _, errs := runPin(args...); code == 0 || errs == "" {
			t.Errorf("lang-pin %v: exit %d stderr %q, want a failure with a message", args, code, errs)
		}
	}
}

// TestCmdLangPinTree is LB-F51 end to end: the verdict on a checkout and on a
// tarball, exit 1 when refused.
func TestCmdLangPinTree(t *testing.T) {
	trees := filepath.Join("..", "..", "internal", "langbench", "testdata", "grammarpin", "trees")
	cases := []struct {
		tree    string
		code    int
		licence string
		basis   string
	}{
		{"mit", 0, "MIT", "LICENSE"},
		{"declared-agree", 0, "MIT", "declared"},
		{"agpl", 1, "AGPL-3.0", "LICENSE"},
		{"declared-disagree", 1, "", ""},
		{"queries-mismatch", 1, "", ""},
	}
	for _, c := range cases {
		code, out, errs := runPin("-tree", filepath.Join(trees, c.tree))
		var v langbench.LicenceVerdict
		if err := json.Unmarshal([]byte(out), &v); err != nil {
			t.Fatalf("%s: verdict is not JSON: %v\n%s", c.tree, err, out)
		}
		if code != c.code || v.Licence != c.licence || v.Basis != c.basis || v.Refused != (c.code == 1) {
			t.Errorf("%s: exit %d verdict %+v, want exit %d licence %q basis %q", c.tree, code, v, c.code, c.licence, c.basis)
		}
		if c.code == 1 && !strings.Contains(errs, v.Reason) {
			t.Errorf("%s: stderr %q lacks the reason %q", c.tree, errs, v.Reason)
		}
	}

	// The same verdict from a tarball as GitHub serves it.
	src := filepath.Join(trees, "declared-agree")
	var tb bytes.Buffer
	zw := gzip.NewWriter(&tb)
	tw := tar.NewWriter(zw)
	_ = fs.WalkDir(os.DirFS(src), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, _ := os.ReadFile(filepath.Join(src, p))
		_ = tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: "acme-tree-sitter-x-abc1234/" + p, Mode: 0o644, Size: int64(len(b))})
		_, _ = tw.Write(b)
		return nil
	})
	_ = tw.Close()
	_ = zw.Close()
	tgz := filepath.Join(t.TempDir(), "x.tar.gz")
	if err := os.WriteFile(tgz, tb.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errs := runPin("-tree", tgz)
	if code != 0 || !strings.Contains(out, `"licence": "MIT"`) || !strings.Contains(out, `"licence_basis": "declared"`) {
		t.Errorf("tarball: exit %d\n%s\n%s", code, out, errs)
	}
	if code, _, errs = runPin("-tree", filepath.Join(t.TempDir(), "absent")); code != 1 || errs == "" {
		t.Errorf("absent tree: exit %d stderr %q", code, errs)
	}
}
