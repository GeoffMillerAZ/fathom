package deps_test

import (
	"context"
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
		return e.To == "sub/input.txt" && e.Kind == deps.BuildInput && e.Line == 4
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
		assertDockerTargets(t, f, nil, []deps.UnknownCode{deps.MissingConfig})
	})
	t.Run("excluded literal", func(t *testing.T) {
		f := readDockerFixture(t, "Dockerfile", "FROM scratch\nCOPY src/drop.tmp /dest\n", files)
		assertDockerTargets(t, f, []string{".dockerignore"}, []deps.UnknownCode{deps.MissingConfig})
	})
	t.Run("symlink", func(t *testing.T) {
		files := fstest.MapFS{"src/link": {Mode: fs.ModeSymlink, Data: []byte("../secret")}, "src/keep": {}}
		f := readDockerFixture(t, "Dockerfile", "FROM scratch\nCOPY src /dest\n", files)
		assertDockerTargets(t, f, []string{"src/keep"}, []deps.UnknownCode{deps.UnsupportedConstruct})
	})
	t.Run("unmatched glob", func(t *testing.T) {
		f := readDockerFixture(t, "Dockerfile", "FROM scratch\nCOPY no-match-*.go /dest\n", files)
		assertDockerTargets(t, f, []string{".dockerignore"}, []deps.UnknownCode{deps.UnboundedGlob})
	})
}

func TestDockerfileUnknownInputs(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	t.Run("external and computed", func(t *testing.T) {
		src := "# syntax=docker/dockerfile:1\nFROM example/base@" + digest + " AS base\n" +
			"FROM ${OTHER}\nCOPY ${PAYLOAD} /dest\nADD https://user:pass@example.com/private /remote\n" +
			"COPY --from=named /root /dest\nRUN --mount=type=secret,id=unseen true\n"
		f := readDockerFixture(t, "Dockerfile", src, nil)
		for _, code := range []deps.UnknownCode{deps.ComputedPath, deps.UnresolvedAlias, deps.ExternalUnpinned, deps.UnsupportedConstruct} {
			if !slices.ContainsFunc(f.Unknowns, func(u deps.Unknown) bool { return u.Code == code }) {
				t.Errorf("no %s in %+v", code, f.Unknowns)
			}
		}
		if !slices.ContainsFunc(f.Edges, func(e deps.Edge) bool {
			return e.Kind == deps.BuildInput && e.Resolution == deps.External && e.To == "docker:example/base" && e.Version == digest
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
		assertDockerTargets(t, f, nil, []deps.UnknownCode{deps.ParseError})
	}
	t.Run("unsupported instruction", func(t *testing.T) {
		f := readDockerFixture(t, "Dockerfile", "FROM scratch\nMAGIC input\nONBUILD COPY input /out\n", nil)
		assertDockerTargets(t, f, nil, []deps.UnknownCode{deps.UnsupportedConstruct})
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
	f, err := reader.ReadFile(context.Background(), files, name, "", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func assertDockerTargets(t *testing.T, got deps.FileDeps, targets []string, codes []deps.UnknownCode) {
	t.Helper()
	var actual []string
	for _, e := range got.Edges {
		if e.Resolution == deps.File || e.Resolution == deps.Directory {
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
		if u.Code != deps.UnmeasuredReader {
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
