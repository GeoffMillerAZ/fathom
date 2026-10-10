package serviceref_test

// The route seam's storage (LB-F321): a mount a reader reports is stored as
// a MountMethod row and composed away before any route is used — it never
// reaches a hop or an inventory item (nor a route: key: internal/syslink's
// TestRouteProvides_NoMountKeys).

import (
	"bufio"
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/inventory"
	"github.com/geoffmilleraz/fathom/internal/analysis/routes"
	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// A test-only reader for Python files: a comment line "#ROUTE METHOD path
// handler" is a route, "#MOUNT prefix target" a mount. No real Python
// source carries either, so the legacy python-regex reader's output in
// this test binary is unchanged.
func init() {
	routes.Register(routes.Reader{Framework: "zz-test-mounts", Langs: []string{"py"}, Read: readTestMounts})
}

func readTestMounts(path string, src []byte) routes.Result {
	var res routes.Result
	sc := bufio.NewScanner(bytes.NewReader(src))
	for line := 1; sc.Scan(); line++ {
		f := strings.Fields(sc.Text())
		switch {
		case len(f) == 4 && f[0] == "#ROUTE":
			res.Routes = append(res.Routes, routes.Route{Method: f[1], Path: f[2], Handler: f[3], File: path, Line: line})
		case len(f) == 3 && f[0] == "#MOUNT":
			res.Mounts = append(res.Mounts, routes.Mount{Prefix: f[1], Target: f[2], File: path, Line: line})
		}
	}
	return res
}

const (
	mountApp   = "#MOUNT /api ./users\n#MOUNT /v2/api ./users\n#ROUTE GET /health health\n"
	mountUsers = "#ROUTE GET /users/:id Users.show\n"
	// mountClient calls the composed route, and a pathological "MOUNT /api"
	// that only a leaked mount row could answer.
	mountClient = `package main

import (
	"fmt"
	"net/http"
)

func fetch(id int) {
	http.NewRequest("GET", fmt.Sprintf("http://catalog/api/users/%d", id), nil)
	http.NewRequest("MOUNT", "http://catalog/api", nil)
	http.NewRequest("MOUNT", "http://catalog/./users", nil)
}
`
)

// mountStore writes the catalog service (a mounting app and the mounted
// users file) and the checkout client into a memory store, as an ingest
// does.
func mountStore(t *testing.T) *store.MemoryStore {
	t.Helper()
	st := store.NewMemoryStore()
	put := func(svc string, files map[string]string) {
		var all store.ServiceRefs
		var paths []string
		for p, c := range files {
			r := serviceref.ExtractFile(p, []byte(c))
			all.Routes = append(all.Routes, r.Routes...)
			all.Calls = append(all.Calls, r.Calls...)
			paths = append(paths, p)
		}
		if err := st.ReplaceServiceRefs(context.Background(), svc, paths, all, true); err != nil {
			t.Fatal(err)
		}
	}
	put("catalog", map[string]string{"app/main.py": mountApp, "app/users.py": mountUsers})
	put("checkout", map[string]string{"client.go": mountClient})
	return st
}

// ExtractFile stores each mount as a MountMethod row {Path: Prefix,
// Handler: Target} beside the file's routes.
func TestExtractFile_StoresMountRows(t *testing.T) {
	refs := serviceref.ExtractFile("app/main.py", []byte(mountApp))
	var mounts, rts []string
	for _, r := range refs.Routes {
		if r.Method == serviceref.MountMethod {
			mounts = append(mounts, r.Path+" "+r.Handler+" "+r.File)
			continue
		}
		rts = append(rts, r.Method+" "+r.Path)
	}
	if strings.Join(mounts, ";") != "/api ./users app/main.py;/v2/api ./users app/main.py" {
		t.Fatalf("mount rows = %v", mounts)
	}
	if strings.Join(rts, ";") != "GET /health" {
		t.Fatalf("route rows = %v", rts)
	}
}

// BuildFromStore composes each service's stored rows before the join: a
// client calling the mounted path joins the mounted file's handler.
func TestBuildFromStore_ComposesMounts(t *testing.T) {
	hops, _, err := serviceref.BuildFromStore(context.Background(), mountStore(t), []string{"catalog", "checkout"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hops) != 1 {
		t.Fatalf("hops = %+v, want the one GET /api/users/{} hop", hops)
	}
	h := hops[0]
	if h.Method != "GET" || h.Template != "/api/users/{}" || h.ToFile != "app/users.py" || h.ToHandler != "Users.show" || h.ToLine != 1 {
		t.Fatalf("hop = %+v, want GET /api/users/{} to app/users.py:1 Users.show", h)
	}
}

func TestServiceRefs_NoMountRowLeaks(t *testing.T) {
	st := mountStore(t)

	// No hop: the "MOUNT /api" client call finds no route.
	hops, _, err := serviceref.BuildFromStore(context.Background(), st, []string{"catalog", "checkout"})
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hops {
		if h.Method == serviceref.MountMethod || strings.HasPrefix(h.ToHandler, "./") {
			t.Fatalf("a mount row reached a hop: %+v", h)
		}
	}

	// No composed route.
	refs, _, err := st.ServiceRefsFor(context.Background(), "catalog")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range serviceref.RoutesFromRows(refs.Routes) {
		if r.Method == serviceref.MountMethod {
			t.Fatalf("RoutesFromRows returned a mount: %+v", r)
		}
		got = append(got, r.Method+" "+r.Path+" "+r.Handler)
	}
	if want := "GET /api/users/:id Users.show;GET /health health;GET /v2/api/users/:id Users.show"; strings.Join(got, ";") != want {
		t.Fatalf("composed routes = %v, want %s", got, want)
	}

	// No inventory item; the composed route is one.
	items := inventory.Extract([]inventory.File{
		{Path: "app/main.py", Content: []byte(mountApp)},
		{Path: "app/users.py", Content: []byte(mountUsers)},
	}, nil)
	var keys []string
	for _, it := range items {
		if strings.Contains(it.Key, serviceref.MountMethod) {
			t.Fatalf("a mount reached an inventory item: %+v", it)
		}
		keys = append(keys, it.Key)
	}
	if want := "route:GET /api/users/:id;route:GET /health;route:GET /v2/api/users/:id"; strings.Join(keys, ";") != want {
		t.Fatalf("inventory keys = %v, want %s", keys, want)
	}

	// No route: key: syslink keys a service's composed routes
	// (TestRouteKeys_ComposedInEveryConsumer, TestRouteProvides_NoMountKeys
	// in internal/syslink, LB-F353).
}
