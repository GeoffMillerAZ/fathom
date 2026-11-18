package deps

import (
	"context"
	"io/fs"
	"testing"
	"testing/fstest"
)

type requestResolver func(context.Context, ResolveRequest) ResolveResult

func (f requestResolver) Resolve(ctx context.Context, request ResolveRequest) ResolveResult {
	return f(ctx, request)
}

func TestResolverReceivesConstructRequest(t *testing.T) {
	repo := fstest.MapFS{
		"go.mod":         {Data: []byte("module example.org/module\n")},
		"dependency.txt": {Data: []byte("contents")},
	}
	wantCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls int
	r, err := New(TrustUnmeasured("go"), WithResolver("go", requestResolver(func(ctx context.Context, req ResolveRequest) ResolveResult {
		calls++
		if ctx != wantCtx || req.From != "app.go" || req.Raw != "example.org/dependency" ||
			req.Kind != KindImport || req.Line != 2 {
			t.Errorf("resolve context or construct differs: %+v", req)
		}
		if _, err := fs.Stat(req.Repo, "dependency.txt"); err != nil {
			t.Errorf("resolver repository view: %v", err)
		}
		return ResolveResult{Edges: []Edge{{Kind: req.Kind, Resolution: ResolvedFile, To: "dependency.txt"}}}
	})))
	if err != nil {
		t.Fatal(err)
	}
	src := []byte("package app\nimport _ \"example.org/dependency\"\n")
	got, err := r.ReadFile(wantCtx, repo, Input{Path: "app.go", Dialect: "go"}, src)
	if err != nil || calls != 1 || !got.Complete {
		t.Fatalf("typed request produced %+v, %v (%d resolver calls)", got, err, calls)
	}
	for _, edge := range got.Edges {
		if edge.Kind == KindConfigRef && edge.Line == 0 ||
			edge.Kind == KindImport && edge.Line == 2 && edge.Provenance == ProvenanceExtension {
			continue
		}
		t.Errorf("wrong evidence line or provenance: %+v", edge)
	}
}

func TestResolverOverrideVoidsAdmission(t *testing.T) {
	extension := requestResolver(func(context.Context, ResolveRequest) ResolveResult { return ResolveResult{} })
	for _, test := range []struct {
		name                      string
		opts                      []Option
		wantMeasured, wantTrusted bool
		wantComplete              bool
	}{
		{"builtin", nil, true, false, true},
		{"replacement", []Option{WithResolver("go", extension)}, false, false, false},
		{"trusted replacement", []Option{WithResolver("go", extension), TrustUnmeasured("go")}, false, true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			r, err := New(test.opts...)
			if err != nil {
				t.Fatal(err)
			}
			got := newResult("file.go", "go")
			r.finishWithAdmission(&got, measured)
			if got.Measured != test.wantMeasured || got.Trusted != test.wantTrusted || got.Complete != test.wantComplete {
				t.Errorf("override admission: %+v", got.FileDeps)
			}
		})
	}
}

func TestDeclaredEdgeCannotAnswerUnknown(t *testing.T) {
	repo := fstest.MapFS{
		"go.mod":        {Data: []byte("module example.org/module\n")},
		"dependency.go": {Data: []byte("package app\n")},
	}
	resolver := WithResolver("go", requestResolver(func(context.Context, ResolveRequest) ResolveResult {
		return ResolveResult{Unknown: &Unknown{Code: UnknownUnresolvedAlias, DecidableBy: []DecidableBy{DecideDeclaration}}}
	}))
	declaration := Edge{From: "app.go", Kind: KindImport, Raw: "example.org/dependency", To: "dependency.go",
		Resolution: ResolvedFile, Line: 2}
	r, err := New(TrustUnmeasured("go"), resolver, WithDeclaredEdges(declaration))
	if err != nil {
		t.Fatal(err)
	}
	src := []byte("package app\nimport _ \"example.org/dependency\"\n")
	got, err := r.ReadFile(context.Background(), repo, Input{Path: "app.go"}, src)
	if err != nil || got.Complete || !got.Trusted || len(got.Unknowns) != 1 ||
		got.Unknowns[0].Code != UnknownUnresolvedAlias || len(got.Unknowns[0].DecidableBy) != 1 ||
		got.Unknowns[0].DecidableBy[0] != DecideDeclaration {
		t.Fatalf("declaration raised or changed missing evidence: %+v, %v", got, err)
	}
	for _, edge := range got.Edges {
		if edge.To == declaration.To && edge.Provenance == ProvenanceDeclared {
			return
		}
	}
	t.Fatalf("declared evidence was lost: %+v", got.Edges)
}

func TestUnlocatedFileEvidenceUsesZeroLine(t *testing.T) {
	r, err := New()
	if err != nil {
		t.Fatal(err)
	}
	view := fstest.MapFS{"go.mod": {Data: []byte("module example.org/module\n")}}
	got, err := r.ReadFile(context.Background(), view, Input{Path: "broken.go"}, []byte("not a go file"))
	if err != nil || len(got.Unknowns) != 2 || got.Complete {
		t.Fatalf("missing file-wide evidence: %+v, %v", got, err)
	}
	for _, unknown := range got.Unknowns {
		if unknown.Line != 0 {
			t.Errorf("unlocated evidence uses a source line: %+v", unknown)
		}
	}
}
