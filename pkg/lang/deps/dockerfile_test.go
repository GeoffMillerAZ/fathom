package deps_test

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
)

func TestDockerfileDependencies(t *testing.T) {
	src, err := os.ReadFile("testdata/dockerfile/multistage.Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	f := readDockerFixture(t, "Dockerfile", string(src), fstest.MapFS{
		"plain.txt":               {},
		"name with space.txt":     {},
		"sub/input.txt":           {},
		"app.txt":                 {},
		".dockerignore":           {Data: []byte("name with space.txt\n")},
		"Dockerfile.dockerignore": {},
	})
	if f.Dialect != "dockerfile" {
		t.Fatalf("dialect = %q", f.Dialect)
	}
	want := []string{"Dockerfile.dockerignore", "app.txt", "name with space.txt", "plain.txt", "sub/input.txt"}
	assertDockerTargets(t, f, want, nil)
	if !slices.ContainsFunc(f.Edges, func(e deps.Edge) bool {
		return e.Kind == deps.KindConfigRef && e.To == "Dockerfile.dockerignore" && e.Line == 0
	}) {
		t.Fatalf("Dockerfile-specific ignore input must be unlocated: %+v", f.Edges)
	}
	if !slices.ContainsFunc(f.Edges, func(e deps.Edge) bool {
		return e.To == "sub/input.txt" && e.Kind == deps.KindBuildInput && e.Line == 4
	}) {
		t.Fatalf("continuation did not keep the instruction line: %+v", f.Edges)
	}
}

func TestDockerfileContextInputs(t *testing.T) {
	files := fstest.MapFS{
		".dockerignore":       {Data: []byte("\ufeff# comment\n**/*.tmp\n!src/keep.tmp\n")},
		"src/data.go":         {},
		"src/other.go":        {},
		"src/drop.tmp":        {},
		"src/keep.tmp":        {},
		"src/deep/a.tmp":      {},
		"src/deep/include.md": {},
		"root.txt":            {},
		"nested/root.txt":     {},
	}
	t.Run("root", func(t *testing.T) {
		f := readDockerFixture(t, "Containerfile", "FROM scratch\nCOPY --exclude=other.go /src /dest\nCOPY src/*.go /dest\nADD ../root.txt /out\n", files)
		assertDockerTargets(t, f, []string{".dockerignore", "root.txt", "src/data.go", "src/deep/include.md", "src/keep.tmp", "src/other.go"}, nil)
	})
	t.Run("nested", func(t *testing.T) {
		f := readDockerFixture(t, "nested/Dockerfile.dev", "FROM scratch\nCOPY root.txt /dest\n", files)
		assertDockerTargets(t, f, nil, []deps.UnknownCode{deps.UnknownMissingConfig})
	})
	t.Run("excluded literal", func(t *testing.T) {
		f := readDockerFixture(t, "Dockerfile", "FROM scratch\nCOPY src/drop.tmp /dest\n", files)
		assertDockerTargets(t, f, []string{".dockerignore"}, []deps.UnknownCode{deps.UnknownMissingConfig})
	})
	t.Run("symlink", func(t *testing.T) {
		files := fstest.MapFS{"src/link": {Mode: fs.ModeSymlink, Data: []byte("../secret")}, "src/keep": {}}
		f := readDockerFixture(t, "Dockerfile", "FROM scratch\nCOPY src /dest\n", files)
		assertDockerTargets(t, f, []string{"src/keep"}, []deps.UnknownCode{deps.UnknownUnsupportedConstruct})
	})
	t.Run("unmatched glob", func(t *testing.T) {
		f := readDockerFixture(t, "Dockerfile", "FROM scratch\nCOPY no-match-*.go /dest\n", files)
		assertDockerTargets(t, f, []string{".dockerignore"}, []deps.UnknownCode{deps.UnknownUnboundedGlob})
	})
}

func TestDockerfileUnknownInputs(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	t.Run("external and computed", func(t *testing.T) {
		src := "# syntax=docker/dockerfile:1\nFROM example/base@" + digest + " AS base\n" +
			"FROM ${OTHER}\nCOPY ${PAYLOAD} /dest\nADD https://user:pass@example.com/private /remote\n" +
			"COPY --from=named /root /dest\nRUN --mount=type=secret,id=unseen true\n"
		f := readDockerFixture(t, "Dockerfile", src, nil)
		for _, code := range []deps.UnknownCode{deps.UnknownComputedPath, deps.UnknownUnresolvedAlias, deps.UnknownExternalUnpinned, deps.UnknownUnsupportedConstruct} {
			if !slices.ContainsFunc(f.Unknowns, func(u deps.Unknown) bool { return u.Code == code }) {
				t.Errorf("no %s in %+v", code, f.Unknowns)
			}
		}
		if !slices.ContainsFunc(f.Edges, func(e deps.Edge) bool {
			return e.Kind == deps.KindBuildInput && e.Resolution == deps.ResolvedExternal && e.To == "docker:example/base" && e.Version == digest
		}) {
			t.Errorf("pinned image edge missing: %+v", f.Edges)
		}
		if output := string(asJSON(t, f)); strings.Contains(output, "pass") || strings.Contains(output, "private") || strings.Contains(output, "PAYLOAD") {
			t.Error("operand leaked")
		}
	})
	t.Run("bind", func(t *testing.T) {
		src := "FROM scratch\nRUN --mount=type=bind,source=/src/data,target=/mnt true\n"
		f := readDockerFixture(t, "Dockerfile", src, fstest.MapFS{"src/data": {}})
		assertDockerTargets(t, f, []string{"src/data"}, nil)
	})
	for _, src := range []string{
		"# escape=x\nFROM scratch\n",
		"FROM scratch\nCOPY [1, 2]\n",
		"FROM scratch\nCOPY <<EOF /x\nCOPY hidden /y\n",
		"FROM scratch\nCOPY x /y \\\n",
	} {
		f := readDockerFixture(t, "Dockerfile", src, nil)
		assertDockerTargets(t, f, nil, []deps.UnknownCode{deps.UnknownParseError})
	}
	t.Run("unsupported instruction", func(t *testing.T) {
		f := readDockerFixture(t, "Dockerfile", "FROM scratch\nMAGIC input\nONBUILD COPY input /out\n", nil)
		assertDockerTargets(t, f, nil, []deps.UnknownCode{deps.UnknownUnsupportedConstruct})
	})
}

func TestDockerfileTypedResolver(t *testing.T) {
	// Even an override sees a confined repository view and an attributable
	// construct. Resolver-owned source, raw, provenance and line are ignored.
	files := fstest.MapFS{"source": {}, "resolved": {}}
	called := false
	override := dockerFixtureResolver(func(ctx context.Context, req deps.ResolveRequest) deps.ResolveResult {
		called = true
		if ctx.Err() != nil || req.From != "build.input" || req.Raw != "source" ||
			req.Kind != deps.KindBuildInput || req.Line != 2 {
			t.Fatalf("request = %+v", req)
		}
		if _, err := fs.Stat(req.Repo, "source"); err != nil {
			t.Fatal(err)
		}
		return deps.ResolveResult{Edges: []deps.Edge{{
			From: "another", Raw: "overridden", Kind: deps.KindBuildInput,
			To: "resolved", Resolution: deps.ResolvedFile, Line: 99,
			Provenance: deps.ProvenanceDeclared,
		}}}
	})
	r, err := deps.New(deps.WithResolver("dockerfile", override))
	if err != nil {
		t.Fatal(err)
	}
	f, err := r.ReadFile(context.Background(), files, deps.Input{Path: "build.input", Dialect: "dockerfile"},
		[]byte("FROM scratch\nCOPY source /dest\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := deps.Edge{From: "build.input", Raw: "source", Kind: deps.KindBuildInput, To: "resolved",
		Resolution: deps.ResolvedFile, Line: 2, Provenance: deps.ProvenanceExtension}
	if !called || !slices.Equal(f.Edges, []deps.Edge{want}) || f.Measured || f.Complete {
		t.Fatalf("override = %+v", f)
	}
}

type dockerFixtureResolver func(context.Context, deps.ResolveRequest) deps.ResolveResult

func (r dockerFixtureResolver) Resolve(ctx context.Context, req deps.ResolveRequest) deps.ResolveResult {
	return r(ctx, req)
}

func TestDockerfileCaps(t *testing.T) {
	t.Run("source", func(t *testing.T) {
		f := readDockerFixture(t, "Dockerfile", strings.Repeat("x", (2<<20)+1), nil)
		assertDockerTargets(t, f, nil, []deps.UnknownCode{deps.UnknownReadLimit})
	})
	t.Run("ignore rules", func(t *testing.T) {
		const cap = 4096
		files := fstest.MapFS{".dockerignore": {Data: []byte(strings.Repeat("ignored\n", cap+1))}, "payload": {}}
		f := readDockerFixture(t, "Dockerfile", "FROM scratch\nCOPY payload /dest\n", files)
		assertDockerTargets(t, f, []string{".dockerignore"}, []deps.UnknownCode{deps.UnknownMissingConfig, deps.UnknownReadLimit})
	})
	t.Run("context entries", func(t *testing.T) {
		const cap = 4096
		files := fstest.MapFS{}
		for i := 0; i <= cap; i++ {
			files[fmt.Sprintf("context/f%04d", i)] = &fstest.MapFile{}
		}
		f := readDockerFixture(t, "Dockerfile", "FROM scratch\nCOPY context /dest\n", files)
		assertDockerTargets(t, f, nil, []deps.UnknownCode{deps.UnknownReadLimit})
	})
}

func readDockerFixture(t *testing.T, name, src string, files fstest.MapFS) deps.FileDeps {
	t.Helper()
	if files == nil {
		files = fstest.MapFS{}
	}
	reader, err := deps.New(deps.TrustUnmeasured("dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := reader.ReadFile(context.Background(), files, deps.Input{Path: name}, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func assertDockerTargets(t *testing.T, got deps.FileDeps, targets []string, codes []deps.UnknownCode) {
	t.Helper()
	var actual []string
	for _, e := range got.Edges {
		if e.Resolution == deps.ResolvedFile || e.Resolution == deps.ResolvedDirectory {
			actual = append(actual, e.To)
		}
	}
	slices.Sort(actual)
	actual = slices.Compact(actual)
	if !slices.Equal(actual, targets) {
		t.Errorf("targets = %v, want %v (edges %+v)", actual, targets, got.Edges)
	}
	var unknowns []deps.UnknownCode
	for _, u := range got.Unknowns {
		if u.Code != deps.UnknownUnmeasuredReader {
			unknowns = append(unknowns, u.Code)
		}
	}
	slices.Sort(unknowns)
	unknowns = slices.Compact(unknowns)
	if !slices.Equal(unknowns, codes) {
		t.Errorf("unknowns = %v, want %v (%+v)", unknowns, codes, got.Unknowns)
	}
	if len(codes) > 0 && got.Complete || len(codes) == 0 && (got.Measured || got.Trusted) && !got.Complete {
		t.Errorf("unexpected completeness: %+v", got)
	}
}
