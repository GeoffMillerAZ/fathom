package fusion_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestTypedPassSkipModulesChangeRerunsPass is EW-68's gate: a change of
// `[typed] skip_modules` re-runs the typed pass at the next ingest with no Go
// change (the pass's state row names the glob set it ran under, as EW-63's
// names the epoch), so a newly skipped module's typed edges are purged and an
// un-skipped one's come back; the same set again skips the pass.
func TestTypedPassSkipModulesChangeRerunsPass(t *testing.T) {
	dir := writeGoModule(t, map[string]string{
		"go.mod":               "module example.com/root\n\ngo 1.21\n",
		"root.go":              "package root\n\nfunc A() int { return b() }\n\nfunc b() int { return 1 }\n",
		"kits/k/go.mod":        "module example.com/root/kits/k\n\ngo 1.21\n",
		"kits/k/cmd/k/main.go": "package main\n\nfunc main() { run() }\n\nfunc run() {}\n",
	})
	ctx := context.Background()
	st := store.NewMemoryStore()
	pipe := fusion.New(connector.NewLocalConnector(), st, fusion.Config{})
	svc := core.ServiceRef{ID: "skipmods", Path: dir}
	ingest := func(step string) fusion.Report {
		t.Helper()
		rep, err := pipe.Ingest(ctx, svc)
		if err != nil || rep.TypedFailed {
			t.Fatalf("%s: err=%v failed=%v %s", step, err, rep.TypedFailed, rep.TypedFailReason)
		}
		return rep
	}
	setSkip := func(body string) {
		t.Helper()
		p := filepath.Join(dir, ".fathom", "config.toml")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	ingest("first ingest")
	if typedEdgesFrom(t, st, "skipmods", "kits/k/") == 0 {
		t.Fatal("first ingest: no typed edge from kits/k")
	}
	if rep := ingest("unchanged"); rep.TypedSkipped == "" || pipe.TypedPassTotal() != 1 {
		t.Fatalf("unchanged re-ingest ran the typed pass (total %d)", pipe.TypedPassTotal())
	}

	setSkip("[typed]\nskip_modules = [\"kits/**\"]\n")
	if rep := ingest("skip set"); rep.TypedSkipped != "" || pipe.TypedPassTotal() != 2 {
		t.Fatalf("a new skip_modules did not re-run the typed pass (skipped %q, total %d)", rep.TypedSkipped, pipe.TypedPassTotal())
	}
	if n := typedEdgesFrom(t, st, "skipmods", "kits/k/"); n != 0 {
		t.Errorf("skipped kits/k still has %d typed edge(s): its stale rows were not purged", n)
	}
	if typedEdgesFrom(t, st, "skipmods", "root.go") == 0 {
		t.Error("the root module's typed edges are gone")
	}
	if rep := ingest("same set"); rep.TypedSkipped == "" || pipe.TypedPassTotal() != 2 {
		t.Fatalf("the same skip set re-ran the typed pass (total %d)", pipe.TypedPassTotal())
	}

	setSkip("# no typed table\n")
	if rep := ingest("skip removed"); rep.TypedSkipped != "" || pipe.TypedPassTotal() != 3 {
		t.Fatalf("removing skip_modules did not re-run the typed pass (skipped %q, total %d)", rep.TypedSkipped, pipe.TypedPassTotal())
	}
	if typedEdgesFrom(t, st, "skipmods", "kits/k/") == 0 {
		t.Error("un-skipped kits/k has no typed edge")
	}
}
