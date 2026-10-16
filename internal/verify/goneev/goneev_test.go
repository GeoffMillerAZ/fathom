package goneev_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/verify"
	"github.com/geoffmilleraz/fathom/internal/verify/goneev"
)

// fakeCorpus lists paths and answers reads, leaving out the files in drop
// (a file the corpus could not read) or failing the listing.
type fakeCorpus struct {
	files   map[string]string
	drop    map[string]bool
	listErr error
}

func (f fakeCorpus) SourcePaths(context.Context, string) ([]string, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []string
	for p := range f.files {
		out = append(out, p)
	}
	return out, nil
}

func (f fakeCorpus) ReadSourceFiles(_ context.Context, _ string, paths []string) ([]core.SourceFile, error) {
	var out []core.SourceFile
	for _, p := range paths {
		if c, ok := f.files[p]; ok && !f.drop[p] {
			out = append(out, core.SourceFile{Path: p, Content: []byte(c)})
		}
	}
	return out, nil
}

func atHeadConfig(c fakeCorpus) goneev.Config {
	return goneev.Config{
		Service: "svc", IndexedCommit: "abc", MetaFound: true,
		Plan:   goneev.PlanTree{Head: "abc"},
		Corpus: c,
	}
}

var corpus = map[string]string{
	"go.mod":         "module m\n",
	"a/a.go":         "package a\n",
	"b/b.go":         "package b\n\nimport _ \"m/old\"\n",
	"docs/readme.md": "nothing to see here\n",
}

func evaluate(t *testing.T, cfg goneev.Config, gone string) verify.Criterion {
	t.Helper()
	c, _ := verify.EvaluateGone(verify.Task{ID: "G-1", Gone: gone}, goneev.New(context.Background(), cfg))
	return c
}

// TestUnreadableFileIsNeverAnAbsence: an importer the corpus could not read
// must leave a gone: import row Unknown with the failure named — before
// O1068 a file dropped from the read simply vanished and the row passed.
func TestUnreadableFileIsNeverAnAbsence(t *testing.T) {
	c := fakeCorpus{files: corpus, drop: map[string]bool{"b/b.go": true}}
	got := evaluate(t, atHeadConfig(c), "import m/old")
	if got.Status != verify.Unknown || !strings.Contains(got.Detail, "could not be read (first: b/b.go)") {
		t.Fatalf("dropped importer: %+v, want unknown naming b/b.go", got)
	}
	// The same file read: a real Fail.
	if got := evaluate(t, atHeadConfig(fakeCorpus{files: corpus}), "import m/old"); got.Status != verify.Fail {
		t.Fatalf("readable importer: %+v, want fail", got)
	}
}

func TestUnlistableCorpusIsNeverAnAbsence(t *testing.T) {
	c := fakeCorpus{files: corpus, listErr: errors.New("budget cut")}
	for _, gone := range []string{"import m/gone", `pattern "retired"`} {
		if got := evaluate(t, atHeadConfig(c), gone); got.Status != verify.Unknown || !strings.Contains(got.Detail, "budget cut") {
			t.Fatalf("%s over an unlistable corpus: %+v, want unknown naming the listing failure", gone, got)
		}
	}
	// Listed and read whole, the same rows pass.
	for _, gone := range []string{"import m/gone", `pattern "retired"`} {
		if got := evaluate(t, atHeadConfig(fakeCorpus{files: corpus}), gone); got.Status != verify.Pass {
			t.Fatalf("%s over the whole corpus: %+v, want pass", gone, got)
		}
	}
}

// TestDeclaredPlanTreeHoldsTheIndex: the plan's tree (declared by a caller
// the daemon cannot read) must be at the ingested commit and clean.
func TestDeclaredPlanTreeHoldsTheIndex(t *testing.T) {
	for _, tc := range []struct {
		name string
		tree goneev.PlanTree
		want string
	}{
		{"moved", goneev.PlanTree{Head: "def"}, "ingested at commit abc, HEAD is now def"},
		{"dirty", goneev.PlanTree{Head: "abc", Dirty: true}, "uncommitted changes"},
		{"unreadable", goneev.PlanTree{HeadErr: "could not resolve git HEAD in /x: boom"}, "could not resolve git HEAD"},
	} {
		cfg := atHeadConfig(fakeCorpus{files: corpus})
		cfg.Plan = tc.tree
		if got := evaluate(t, cfg, "import m/gone"); got.Status != verify.Unknown || !strings.Contains(got.Detail, tc.want) {
			t.Errorf("%s: %+v, want unknown naming %q", tc.name, got, tc.want)
		}
	}
}
