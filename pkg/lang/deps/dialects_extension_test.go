package deps

import (
	"context"
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"
)

func TestDependencyDialects(t *testing.T) {
	want := []DialectInfo{
		{ID: "dockerfile", Kinds: []Kind{KindBuildInput, KindConfigRef}},
		{ID: "go", Kinds: []Kind{KindBuildInput, KindConfigRef, KindEmbed, KindImport}},
		{ID: "javascript", Measured: true, Kinds: []Kind{KindConfigRef, KindImport}},
		{ID: "python", Measured: true, Kinds: []Kind{KindImport}},
		{ID: "typescript", Kinds: []Kind{KindConfigRef, KindImport}},
	}
	var defaults Reader
	rows := defaults.Dialects()
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("listed dialects: %+v", rows)
	}
	rows[0].Kinds[0] = KindExec
	rows[1].ID = "other"
	if got := defaults.Dialects(); !reflect.DeepEqual(got, want) {
		t.Fatalf("listing exposed shared metadata: %+v", got)
	}

	resolver := requestResolver(func(context.Context, ResolveRequest) ResolveResult { return ResolveResult{} })
	replaced, err := New(WithResolver("javascript", resolver), TrustUnmeasured("javascript"),
		WithResolver("typescript", resolver), TrustUnmeasured("typescript"),
		WithResolver("dockerfile", resolver), TrustUnmeasured("dockerfile"),
		WithResolver("unknown-dialect", resolver), TrustUnmeasured("unknown-dialect"))
	if err != nil {
		t.Fatal(err)
	}
	want[2].Measured = false
	if got := replaced.Dialects(); !reflect.DeepEqual(got, want) {
		t.Fatalf("override metadata: %+v", got)
	}
	for _, tc := range []struct {
		path, dialect string
		src           []byte
		trusted       bool
	}{
		{"main.js", "javascript", []byte("export const n = 1"), true},
		{"main.ts", "typescript", []byte("export const n = 1"), false},
		{"Dockerfile", "dockerfile", []byte("FROM scratch"), false},
		{"main.unknown", "unknown-dialect", nil, false},
	} {
		got, err := replaced.ReadFile(t.Context(), fstest.MapFS{}, Input{Path: tc.path, Dialect: tc.dialect}, tc.src)
		if err != nil || got.Measured || got.Trusted != tc.trusted || got.Complete != tc.trusted ||
			hasUnknown(got, UnknownUnmeasuredReader) == tc.trusted {
			t.Errorf("%s override admission: %+v, %v", tc.dialect, got, err)
		}
	}
}

func TestDependencyMeasuredResolverAndDeclaration(t *testing.T) {
	repo := fstest.MapFS{
		"app.js": {Data: []byte("import './lib.js';\n")},
		"lib.js": {Data: []byte("export {};\n")},
	}
	in := Input{Path: "app.js"}
	wantCtx := t.Context()
	resolve := WithResolver("javascript", requestResolver(func(ctx context.Context, req ResolveRequest) ResolveResult {
		if ctx != wantCtx || req.From != "app.js" || req.Kind != KindImport || req.Line != 1 || req.Raw != "./lib.js" {
			t.Errorf("incorrect request: %+v", req)
		}
		if _, err := fs.Stat(req.Repo, "lib.js"); err != nil {
			t.Error(err)
		}
		return ResolveResult{Unknown: &Unknown{Code: UnknownUnresolvedAlias, DecidableBy: []DecidableBy{DecideDeclaration}}}
	}))
	declaration := WithDeclaredEdges(Edge{From: "app.js", To: "lib.js", Kind: KindImport, Line: 1, Resolution: ResolvedFile})
	tests := []struct {
		name                        string
		opts                        []Option
		measured, trusted, complete bool
		codes                       []UnknownCode
		provenance                  Provenance
	}{
		{"built-in", nil, true, false, true, nil, ProvenanceBuiltin},
		{"override", []Option{resolve}, false, false, false, []UnknownCode{UnknownUnmeasuredReader, UnknownUnresolvedAlias}, ""},
		{"trusted with declaration", []Option{resolve, TrustUnmeasured("javascript"), declaration}, false, true, false, []UnknownCode{UnknownUnresolvedAlias}, ProvenanceDeclared},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader, err := New(tc.opts...)
			if err != nil {
				t.Fatal(err)
			}
			got, err := reader.ReadFiles(wantCtx, repo, []Input{in})
			if err != nil || len(got) != 1 {
				t.Fatalf("batch: %+v, %v", got, err)
			}
			one := got[0]
			if one.Dialect != "javascript" || one.Measured != tc.measured || one.Trusted != tc.trusted || one.Complete != tc.complete ||
				len(one.Unknowns) != len(tc.codes) || (tc.provenance == "" && len(one.Edges) != 0) ||
				(tc.provenance != "" && (len(one.Edges) != 1 || one.Edges[0].Provenance != tc.provenance)) {
				t.Fatalf("unexpected reader output: %+v", one)
			}
			for i, u := range one.Unknowns {
				if u.Code != tc.codes[i] {
					t.Errorf("unknown %d: %+v", i, u)
				}
				if u.Code == UnknownUnresolvedAlias && !reflect.DeepEqual(u.DecidableBy, []DecidableBy{DecideDeclaration}) {
					t.Errorf("declaration changed evidence: %+v", u)
				}
			}
		})
	}
}
