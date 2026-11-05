package iacfacts

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
)

// TestIaCFactsNeverDials holds I56 for this package (IF-3): no product path dials, runs a provider, `init`, a plan
// or a registry client. It fails two ways. The static half reads every non-test source file and fails on an import
// that could dial or exec. The dynamic half replaces every default dial path with one that counts and fails, then
// drives the whole product surface (add, import, list, read, verify, bind, remove) and fails on any count.
func TestIaCFactsNeverDials(t *testing.T) {
	forbidden := []string{"net", "net/http", "net/rpc", "net/smtp", "net/textproto", "os/exec", "plugin", "crypto/tls", "syscall", "golang.org/x/net", "google.golang.org/grpc", "github.com/gorilla/websocket"}
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no sources: %v", err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		af, err := parser.ParseFile(token.NewFileSet(), f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range af.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			for _, bad := range forbidden {
				if path == bad || strings.HasPrefix(path, bad+"/") {
					t.Errorf("%s imports %q: the product has no download path and runs no provider (I56)", f, path)
				}
			}
		}
	}

	var dials atomic.Int64
	failing := func() error { dials.Add(1); return errors.New("dial attempted") }
	oldT, oldR := http.DefaultTransport, net.DefaultResolver
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, failing() })
	net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: func(_ context.Context, _, _ string) (net.Conn, error) { return nil, failing() }}
	t.Cleanup(func() { http.DefaultTransport, net.DefaultResolver = oldT, oldR })

	src := twoVersionCatalog(t)
	dir := t.TempDir()
	if _, err := AddIaCCatalog(dir, src); err != nil {
		t.Fatal(err)
	}
	cats, _ := ListIaCCatalogs(dir, true)
	if _, err := ReadIaCCatalog(cats[0].Path, IaCCatalogRequest{Version: "1.0.0", Kind: SectionProvider}); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyIaCCatalog(cats[0].Path); err != nil {
		t.Fatal(err)
	}
	exp := filepath.Join(t.TempDir(), "e.json")
	os.WriteFile(exp, fxExport(t, fxSource, fxProvider(), map[string]any{"widget_thing": fxResource("a")}, nil), 0o644)
	if _, _, err := ImportIaCExports(filepath.Join(t.TempDir(), "iac"), []IaCImportInput{{"1.0.0", exp}}, IaCWriterOptions{}); err != nil {
		t.Fatal(err)
	}
	_ = ResolveIaCBinding(IaCBindingQuery{Repo: fstest.MapFS{"main.tf": file(`resource "widget_thing" "a" {}`)}, Catalogs: cats, Block: IaCBlockRef{Dir: ".", Type: "widget_thing"}})
	if _, err := RemoveIaCCatalog(dir, cats[0].Name); err != nil {
		t.Fatal(err)
	}
	if n := dials.Load(); n != 0 {
		t.Fatalf("%d dial attempts: the product must not reach the network (I56)", n)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
