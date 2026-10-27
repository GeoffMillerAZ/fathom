package fusion

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestTypedPassNamesUnloadedGoDirectories (EW-82, O1506): every directory of
// a discovered Go file that no module load covers is named with why on the
// run's report — no go.mod, outside ./..., or a nested module not loaded (by
// its go.mod and EW-62's reason) — the first two also in the pass's warning,
// also when the repository has no module at all, and none of it is a
// failure.
func TestTypedPassNamesUnloadedGoDirectories(t *testing.T) {
	ctx := context.Background()
	ingest := func(t *testing.T, id, root string) (Report, string, *Pipeline) {
		t.Helper()
		p := New(connector.NewLocalConnector(), store.NewMemoryStore(), Config{Concurrency: 2})
		var buf strings.Builder
		p.SetLogger(dlog.New(&buf, dlog.LevelDebug, time.Now))
		rep, err := p.Ingest(ctx, core.ServiceRef{ID: id, Path: root})
		if err != nil {
			t.Fatalf("Ingest: %v", err)
		}
		warning := ""
		for _, l := range strings.Split(buf.String(), "\n") {
			if strings.Contains(l, "did not resolve") {
				if warning != "" {
					t.Fatalf("more than one skips warning:\n%s", buf.String())
				}
				warning = l
			}
		}
		return rep, warning, p
	}

	t.Run("root module with an underscore directory and a foreign module", func(t *testing.T) {
		root := writeTypedFixture(t, map[string]string{
			"go.mod":       "module example.com/r\n\ngo 1.23\n",
			"a/a.go":       "package a\n\nfunc A() {}\n",
			"_wip/w.go":    "package wip\n\nimport \"example.com/r/a\"\n\nfunc W() { a.A() }\n",
			"other/go.mod": "module example.org/other\n\ngo 1.23\n",
			"other/o.go":   "package other\n\nfunc O() {}\n",
		})
		rep, warning, _ := ingest(t, "r", root)
		want := []TypedSkip{
			{Reason: skipNotOwn, Count: 1, Names: []string{"other/go.mod"}},
			{Reason: typedSkipOutside, Count: 1, Names: []string{"_wip"}},
		}
		if !reflect.DeepEqual(rep.TypedNotLoaded, want) {
			t.Errorf("not loaded:\n got %+v\nwant %+v", rep.TypedNotLoaded, want)
		}
		if rep.TypedFailed {
			t.Errorf("a directory no load covers is a disclosure, never a failure: %q", rep.TypedFailReason)
		}
		if !strings.Contains(warning, "outside ./... 1 directory(ies): _wip") {
			t.Errorf("warning %q does not name _wip", warning)
		}
		if strings.Contains(warning, "other/go.mod") {
			t.Errorf("warning %q repeats the foreign module (EW-69's line names it)", warning)
		}
	})

	t.Run("no root go.mod, an own nested module and files outside any module", func(t *testing.T) {
		root := writeTypedFixture(t, map[string]string{
			"main.go":          "package main\n\nfunc main() {}\n",
			"scripts/s.go":     "package main\n\nfunc run() {}\n",
			"tools/go.mod":     "module example.com/r/tools\n\ngo 1.23\n",
			"tools/gen/gen.go": "package gen\n\nfunc Gen() { helper() }\n\nfunc helper() {}\n",
		})
		rep, warning, _ := ingest(t, "nr", root)
		want := []TypedSkip{{Reason: typedSkipNoGoMod, Count: 2, Names: []string{".", "scripts"}}}
		if !reflect.DeepEqual(rep.TypedNotLoaded, want) {
			t.Errorf("not loaded:\n got %+v\nwant %+v", rep.TypedNotLoaded, want)
		}
		if rep.TypedFailed || rep.TypedPackages == 0 {
			t.Errorf("the own nested module loads and nothing fails: processed=%d failed=%v %q", rep.TypedPackages, rep.TypedFailed, rep.TypedFailReason)
		}
		if !strings.Contains(warning, "no go.mod 2 directory(ies): ., scripts") {
			t.Errorf("warning %q does not name the directories outside any module", warning)
		}
	})

	t.Run("no module at all", func(t *testing.T) {
		root := writeTypedFixture(t, map[string]string{"scripts/s.go": "package main\n\nfunc main() {}\n"})
		rep, warning, p := ingest(t, "none", root)
		want := []TypedSkip{{Reason: typedSkipNoGoMod, Count: 1, Names: []string{"scripts"}}}
		if !reflect.DeepEqual(rep.TypedNotLoaded, want) {
			t.Errorf("not loaded:\n got %+v\nwant %+v", rep.TypedNotLoaded, want)
		}
		if rep.TypedFailed || rep.TypedPackagesSeen != 0 {
			t.Errorf("no module is not a failure: %+v", rep)
		}
		if !strings.Contains(warning, "no go.mod 1 directory(ies): scripts") {
			t.Errorf("warning %q does not name scripts", warning)
		}
		// The early return records no status and no pass state: recording a
		// status writes the persisted row.
		if _, known := p.TypedEdgeStatus("none"); known {
			t.Errorf("a repository with no module must record no typed-edge status")
		}
	})

	t.Run("from the allowed paths alone", func(t *testing.T) {
		root := writeTypedFixture(t, map[string]string{
			"go.mod":       "module example.com/r\n\ngo 1.23\n",
			"tools/go.mod": "module example.com/r/tools\n\ngo 1.23\n",
			"fork/go.mod":  "module example.org/fork\n\ngo 1.23\n",
		})
		// None of these Go files exists on disk.
		allowed := map[string]bool{
			"a/a.go": true, "tools/t.go": true, "fork/f.go": true,
			"testdata/x/x.go": true, "a/testdata/y.go": true, "vendor/v/v.go": true,
			".hidden/h.go": true, "a/_gen/g.go": true, "a/_gen/g2.go": true,
			"ignored/i.go": false, "README.md": true,
		}
		_, foreign := typedModulesSkipping(root, allowed, nil)
		got := unloadedGoDirs(root, allowed, foreign)
		want := []TypedSkip{
			{Reason: skipNotOwn, Count: 1, Names: []string{"fork/go.mod"}},
			{Reason: typedSkipOutside, Count: 5, Names: []string{".hidden", "a/_gen", "a/testdata", "testdata/x", "vendor/v"}},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("unloadedGoDirs:\n got %+v\nwant %+v", got, want)
		}
	})
}
