package main

// field_shadow_test.go — a Go struct field never shadows the function it
// shares a name with in the CLI's and console's one-target lookups
// (core.WithoutShadowingFields): the export receipt's one definition, the
// console row's reach and the plan-row preflight's anchors.

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// pkg declares Config.Greet and Config.Wave beside the functions Greet and
// Wave; other's own Greet leaves the bare name ambiguous in the call graph.
const fieldShadowSrc = `package pkg

// Config holds fields named like the functions below.
type Config struct {
	Greet bool
	Wave  bool
}

// Greet says hello.
func Greet() string { return "hi" }

// Wave says bye.
func Wave() string { return "bye" }

// Hello calls both.
func Hello() string {
	return Greet() + Wave()
}
`

func fieldShadowFixture(t *testing.T) (storePath string) {
	t.Helper()
	t.Setenv("FATHOM_HOME", t.TempDir())
	repo := newSelfSkipGitRepo(t)
	repo.writePlan("go.mod", "module example.invalid/fshadow\n\ngo 1.22\n")
	repo.writePlan("pkg/a.go", fieldShadowSrc)
	repo.writePlan("pkg/a_test.go", "package pkg\n\nimport \"testing\"\n\nfunc TestHello(t *testing.T) {\n\tif Hello() == \"\" {\n\t\tt.Fatal(\"empty\")\n\t}\n}\n")
	repo.writePlan("other/b.go", "package other\n\n// Greet is another Greet.\nfunc Greet() int { return 1 }\n")
	repo.commitAll("a")
	storePath = filepath.Join(t.TempDir(), "s.sqlite")
	var out, errBuf bytes.Buffer
	if code := run([]string{"ingest", "--service", "fshadow", "--path", repo.root, "--store", storePath}, &out, &errBuf); code != 0 {
		t.Fatalf("ingest exit %d: %s", code, errBuf.String())
	}
	return storePath
}

func TestGoFieldNeverShadowsFuncInOneTargetLookups(t *testing.T) {
	preflightTestEnv(t)
	storePath := fieldShadowFixture(t)

	t.Run("export receipt", func(t *testing.T) {
		var out, errBuf bytes.Buffer
		if code := run([]string{"export", "receipt", "--service", "fshadow", "--store", storePath, "--symbol", "Wave"}, &out, &errBuf); code != 0 {
			t.Fatalf("export receipt --symbol Wave exit %d: %s", code, errBuf.String())
		}
		if !strings.Contains(out.String(), `"line": 13`) {
			t.Errorf("receipt does not cite the function Wave at pkg/a.go:13:\n%s", out.String())
		}
	})

	t.Run("export receipt withholds callers for a field", func(t *testing.T) {
		var out, errBuf bytes.Buffer
		code := run([]string{"export", "receipt", "--service", "fshadow", "--store", storePath, "--symbol", "Config.Wave", "--kind", "callers"}, &out, &errBuf)
		if code == 0 || !strings.Contains(errBuf.String(), "field references are not indexed") || strings.Contains(out.String(), "caller(s)") {
			t.Fatalf("callers receipt for the field Config.Wave: exit %d, stdout %q, stderr %q; want it withheld, naming that field references are not indexed", code, out.String(), errBuf.String())
		}
	})

	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()
	c := client.New(nil, st)
	ctx := context.Background()

	t.Run("console row reach", func(t *testing.T) {
		pc := &consolePlans{deps: &consoleDeps{c: c}}
		f := rowReach(ctx, pc, "fshadow", console.PlanRowFact{ID: "F-1", Symbol: "Greet", Package: "pkg"})
		if f.Err != "" || len(f.Tests) == 0 || !strings.HasSuffix(f.Tests[0].Test, "TestHello") {
			t.Fatalf("row reach for Greet under pkg = %+v, want TestHello through the function", f)
		}
	})

	t.Run("preflight anchors", func(t *testing.T) {
		s, err := newPreflightSources(ctx, c, "fshadow", "")
		if err != nil {
			t.Fatal(err)
		}
		anchors := s.anchorNodes(ctx, "Greet", "pkg")
		if len(anchors) != 1 || anchors[0].Name != "Greet" || anchors[0].Line != 10 {
			t.Fatalf("preflight anchors for Greet under pkg = %+v, want the function alone at pkg/a.go:10", anchors)
		}
	})
}
