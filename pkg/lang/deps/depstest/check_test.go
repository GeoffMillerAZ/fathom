package depstest_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
	"github.com/geoffmilleraz/fathom/pkg/lang/deps/depstest"
)

type targetResolver struct{ to string }

func (r targetResolver) Resolve(_ context.Context, req deps.ResolveRequest) deps.ResolveResult {
	if r.to == "" {
		return deps.ResolveResult{}
	}
	return deps.ResolveResult{Edges: []deps.Edge{{Kind: req.Kind, To: r.to, Resolution: deps.ResolvedFile}}}
}

func fixture(t *testing.T, changed bool) fs.FS {
	t.Helper()
	root := t.TempDir()
	source := "import 'local';\n"
	if changed {
		source = "import 'different';\n"
	}
	for name, src := range map[string]string{
		"app.js": source, "plain.js": "export {};\n",
		"right.js": "export {};\n", "wrong.js": "export {};\n", "Dockerfile": "FROM scratch\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return os.DirFS(root)
}

func TestExtensionConformance(t *testing.T) {
	repo, relocated := fixture(t, false), fixture(t, false)
	inputs := []deps.Input{{Path: "plain.js"}, {Path: "app.js"}}
	edge := depstest.TruthEdge{Kind: deps.KindImport, Raw: "local", Line: 1, To: "right.js", Resolution: deps.ResolvedFile}
	truth := []depstest.TruthFile{{Path: "app.js", Edges: []depstest.TruthEdge{edge}}, {Path: "plain.js"}}

	run := func(to string, truth []depstest.TruthFile, relocated fs.FS) depstest.Reading {
		t.Helper()
		reader, err := deps.New(deps.WithResolver("javascript", targetResolver{to}), deps.TrustUnmeasured("javascript"))
		if err != nil {
			t.Fatal(err)
		}
		got, err := depstest.Check(t.Context(), reader, repo, relocated, inputs, truth)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	if got := run("right.js", truth, relocated); !got.B1 || !got.B2 || !got.B3 || got.Files != 2 ||
		got.Complete != 2 || got.Expected != 1 || got.Unknowns != 0 {
		t.Errorf("sound fixture: %+v", got)
	}
	missingEdge := depstest.TruthEdge{Kind: deps.KindImport, Raw: "other", Line: 2, To: "wrong.js", Resolution: deps.ResolvedFile}
	omittedTruth := []depstest.TruthFile{{Path: "app.js", Edges: []depstest.TruthEdge{edge, missingEdge}}, {Path: "plain.js"}}
	if got := run("right.js", omittedTruth, relocated); got.B1 || !got.B2 || !got.B3 || got.Missing != 1 || got.Wrong != 0 {
		t.Errorf("omitted target: %+v", got)
	}
	if got := run("wrong.js", truth, relocated); got.B1 || got.B2 || !got.B3 || got.Missing != 1 || got.Wrong != 1 {
		t.Errorf("wrong target: %+v", got)
	}
	if got := run("", truth, relocated); !got.B3 || got.Complete != 1 || got.OracleFiles != 2 ||
		got.Expected != 0 || got.UnknownByCode[deps.UnknownUnresolvedAlias] != 1 {
		t.Errorf("disclosed unknown: %+v", got)
	}
	if got := run("right.js", truth[:1], relocated); got.B1 || got.B2 || !got.B3 || got.CompleteWithoutOracle != 1 {
		t.Errorf("missing truth coverage: %+v", got)
	}
	if got := run("right.js", truth, fixture(t, true)); got.B3 {
		t.Errorf("changed relocation passed: %+v", got)
	}

	detectOnly, err := deps.New(deps.TrustUnmeasured("dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := depstest.Check(t.Context(), detectOnly, repo, relocated, []deps.Input{{Path: "Dockerfile"}},
		[]depstest.TruthFile{{Path: "Dockerfile"}})
	if err != nil || got.B1 || got.B2 || !got.B3 || got.OracleFiles != 1 || got.Complete != 0 ||
		got.UnknownByCode[deps.UnknownUnmeasuredReader] != 1 {
		t.Fatalf("detect-only fixture: %+v, %v", got, err)
	}
}
