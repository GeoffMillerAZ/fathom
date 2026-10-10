package fusion_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// fieldRefsModule is K17's field-reference fixture (wave lf2, LB-F343): a
// Config in one package read and written from another, through a pointer,
// an embedding, a generic instance, a func-typed field's call, an
// increment, an op-assignment, a composite literal (a package variable's
// initializer included) and a struct's own method.
var fieldRefsModule = map[string]string{
	"go.mod": "module example.com/fieldrefs\n\ngo 1.21\n",
	"conf/conf.go": `package conf

import "time"

type Config struct {
	Timeout time.Duration
	Retries int
	Hook    func() error
}

type Server struct {
	*Config
	name string
}

type Box[T any] struct {
	Val T
}

func Default() Config {
	return Config{Timeout: time.Second, Retries: 3}
}

func (s *Server) Name() string { return s.name }

// state is unexported: no field symbol, so no field reference (the K17
// cost gate).
type state struct{ count int }

func Count(s *state) int { s.count++; return s.count }
`,
	"app/app.go": `package app

import (
	"time"

	"example.com/fieldrefs/conf"
)

var Initial = conf.Config{Timeout: time.Minute}

func Read(c *conf.Config) time.Duration { return c.Timeout }

func Write(c *conf.Config) { c.Timeout = 5 * time.Second }

func Bump(c *conf.Config) { c.Retries++ }

func Both(c *conf.Config) { c.Retries += c.Retries }

func Promoted(s *conf.Server) time.Duration { return s.Timeout }

func CallHook(c *conf.Config) error { return c.Hook() }

func Generic(b conf.Box[int]) int { return b.Val }

func Pointer(c *conf.Config) *int { return &c.Retries }
`,
}

func writeModule(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestEnrichTypedEdges_FieldReferences(t *testing.T) {
	root := writeModule(t, fieldRefsModule)
	ctx := context.Background()
	st := store.NewMemoryStore()
	rep, err := fusion.EnrichTypedEdges(ctx, st, "fr", root, nil)
	if err != nil {
		t.Fatalf("EnrichTypedEdges: %v", err)
	}
	if rep.PackagesFailed != 0 {
		t.Fatalf("packages failed: %+v", rep.FailedPackages)
	}
	edges, err := st.TypedEdges(ctx, "fr")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range edges {
		if e.Kind == core.EdgeFieldRead || e.Kind == core.EdgeFieldWrite {
			got = append(got, e.FromSymbol+" "+string(e.Kind)+" "+e.ToSymbol)
		}
	}
	sort.Strings(got)
	const (
		timeout = "fr::conf/Config.Config.Timeout"
		retries = "fr::conf/Config.Config.Retries"
		hook    = "fr::conf/Config.Config.Hook"
		val     = "fr::conf/Box.Box.Val"
		name    = "fr::conf/Server.Server.name"
	)
	want := []string{
		"fr::app/app.Both field-read " + retries,
		"fr::app/app.Both field-write " + retries,
		"fr::app/app.Bump field-write " + retries,
		"fr::app/app.CallHook field-read " + hook,
		"fr::app/app.Generic field-read " + val,
		"fr::app/app.Pointer field-read " + retries,
		"fr::app/app.Promoted field-read " + timeout,
		"fr::app/app.Read field-read " + timeout,
		"fr::app/app.Write field-write " + timeout,
		"fr::app/app.init field-write " + timeout,
		"fr::conf/Server.Server.Name field-read " + name,
		"fr::conf/conf.Default field-write " + retries,
		"fr::conf/conf.Default field-write " + timeout,
	}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("field references:\n got %q\nwant %q", got, want)
	}
	// A field reference is never counted as a call edge, and the call
	// through the func-typed field stays the unresolved call it was.
	if rep.EdgesDirect != 0 || rep.EdgesIfaceDispatch != 0 || rep.EdgesFuncValue != 0 || rep.EdgesUnresolved != 1 {
		t.Fatalf("call edge counts moved: direct %d dispatch %d func-value %d unresolved %d", rep.EdgesDirect, rep.EdgesIfaceDispatch, rep.EdgesFuncValue, rep.EdgesUnresolved)
	}
}
