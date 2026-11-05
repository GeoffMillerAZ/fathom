package lang_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Rego: a rule head whose bracket is never closed once sliced past the end of
// the token list and ended the process (`package p\na[\n`, a 13-byte file).
func TestRegoUnclosedBracketsAreRead(t *testing.T) {
	r := newReader(t)
	for _, src := range []string{
		"package p\na[\n", "package p\na(\n", "package p\nf(x\n", "package p\na[x\n  y\n", "package p\nf(x) := y if {\n", "a(", "a[", "package p\nallow if {\n  input.a[\n",
	} {
		got := extract(t, r, "p.rego", src)
		for _, n := range got.Notes {
			if strings.Contains(n, "failed on this input") {
				t.Errorf("%q: the extractor panicked: %v", src, got.Notes)
			}
		}
		if got.Dialect != "rego" {
			t.Errorf("%q: dialect %q", src, got.Dialect)
		}
	}
	// One file's failure does not end the batch.
	if ok := extract(t, r, "ok.tf", tfSrc); ok.Incomplete || len(ok.Symbols) == 0 {
		t.Errorf("the next file: %+v", ok)
	}
}

// The family the Rego panic belongs to: an extractor that indexes past the end
// of what a cut-short file gave it. Every fixture a dialect ships, cut at four
// points, reads without an extractor panic (a panic is caught and reported as
// "failed on this input", which is not what any of these files may read as).
func TestTruncatedFixturesDoNotPanic(t *testing.T) {
	root := filepath.Join("..", "..", "internal", "extractor")
	r := newReader(t)
	files, cuts := 0, 0
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.Contains(filepath.ToSlash(p), "/testdata/") {
			return err
		}
		if info, ierr := d.Info(); ierr != nil || info.Size() > 24<<10 || strings.HasSuffix(p, ".golden") {
			return ierr
		}
		src, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		files++
		rel, _ := filepath.Rel(root, p)
		path := strings.SplitN("/"+filepath.ToSlash(rel), "/testdata/", 2)[1]
		for _, cut := range []int{len(src) / 4, len(src) / 2, 3 * len(src) / 4, len(src) - 1} {
			if cut < 0 || cut >= len(src) {
				continue
			}
			cuts++
			got := extract(t, r, path, string(src[:cut]))
			for _, n := range got.Notes {
				if strings.Contains(n, "failed on this input") {
					t.Errorf("%s cut at %d of %d: %s", p, cut, len(src), n)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files < 300 || cuts < 1000 {
		t.Errorf("only %d fixtures and %d cuts were read; the fixture layout moved", files, cuts)
	}
}
