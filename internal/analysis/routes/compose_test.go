package routes_test

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/routes"
)

// file is one file's reading: routes given as "METHOD /path handler" at
// lines 1.., mounts as (prefix, target) at lines 100...
func file(path string, rts []string, mounts ...[2]string) routes.Result {
	var r routes.Result
	for i, s := range rts {
		var m, p, h string
		fmt.Sscan(s, &m, &p, &h)
		r.Routes = append(r.Routes, routes.Route{Method: m, Path: p, Handler: h, File: path, Line: i + 1})
	}
	for i, mt := range mounts {
		r.Mounts = append(r.Mounts, routes.Mount{Prefix: mt[0], Target: mt[1], File: path, Line: 100 + i})
	}
	return r
}

// keys renders composed routes as "METHOD path handler file:line", sorted.
func keys(rs []routes.Route) []string {
	var out []string
	for _, r := range rs {
		out = append(out, fmt.Sprintf("%s %s %s %s:%d", r.Method, r.Path, r.Handler, r.File, r.Line))
	}
	sort.Strings(out)
	return out
}

func assertKeys(t *testing.T, got []routes.Route, want ...string) {
	t.Helper()
	sort.Strings(want)
	if g := keys(got); !reflect.DeepEqual(g, want) {
		t.Fatalf("composed:\n  %v\nwant:\n  %v", g, want)
	}
}

func TestCompose_UnmountedFileKeepsLocalPaths(t *testing.T) {
	got := routes.Compose([]routes.Result{
		file("a.js", []string{"GET /a ha"}),
		file("b.go", []string{"POST /b hb"}),
	})
	assertKeys(t, got, "GET /a ha a.js:1", "POST /b hb b.go:1")
}

// (a) "./x" and "../x", relative to the mounting file's directory: the
// file with an extension tried, then index/__init__.
func TestCompose_RelativeTargets(t *testing.T) {
	got := routes.Compose([]routes.Result{
		file("src/app.js", nil, [2]string{"/users", "./routes/users"}, [2]string{"/admin", "../lib/admin"}, [2]string{"/api", "./api"}),
		file("src/routes/users.js", []string{"GET /:id show"}),
		file("lib/admin.ts", []string{"GET / dash"}),
		file("src/api/index.js", []string{"GET /ping ping"}),
	})
	assertKeys(t, got,
		"GET /users/:id show src/routes/users.js:1",
		"GET /admin/ dash lib/admin.ts:1",
		"GET /api/ping ping src/api/index.js:1",
	)
}

// An exact file name wins over the extension and index candidates.
func TestCompose_ExactFileWins(t *testing.T) {
	got := routes.Compose([]routes.Result{
		file("app.js", nil, [2]string{"/v1", "./users.js"}),
		file("users.js", []string{"GET /u u"}),
		file("users.js/index.js", []string{"GET /never never"}),
	})
	assertKeys(t, got, "GET /v1/u u users.js:1", "GET /never never users.js/index.js:1")
}

// A directory target ends in "/": every file directly inside it.
func TestCompose_DirectoryTarget(t *testing.T) {
	got := routes.Compose([]routes.Result{
		file("app.php", nil, [2]string{"/api", "./routes/"}),
		file("routes/a.php", []string{"GET /a a"}),
		file("routes/b.php", []string{"GET /b b"}),
		file("routes/deep/c.php", []string{"GET /c c"}),
	})
	assertKeys(t, got, "GET /api/a a routes/a.php:1", "GET /api/b b routes/b.php:1", "GET /c c routes/deep/c.php:1")
}

// (b) "/x" is repo-relative: a file, or a directory with "/".
func TestCompose_RepoRelativeTargets(t *testing.T) {
	got := routes.Compose([]routes.Result{
		file("web/server.js", nil, [2]string{"/a", "/handlers/a"}, [2]string{"/d", "/dir/"}),
		file("handlers/a.js", []string{"GET /x ax"}),
		file("dir/one.js", []string{"GET /y dy"}),
	})
	assertKeys(t, got, "GET /a/x ax handlers/a.js:1", "GET /d/y dy dir/one.js:1")
}

// (c) A dotted module is matched as a path suffix: a/b/urls.<ext> or
// a/b/urls/__init__.py.
func TestCompose_DottedModule(t *testing.T) {
	got := routes.Compose([]routes.Result{
		file("proj/urls.py", nil, [2]string{"/polls/", "polls.urls"}, [2]string{"/shop/", "shop.urls"}),
		file("proj/polls/urls.py", []string{"GET /<int:id>/ detail"}),
		file("shop/urls/__init__.py", []string{"GET /cart cart"}),
		file("otherpolls/urls.py", []string{"GET /no no"}),
	})
	assertKeys(t, got,
		"GET /polls/<int:id>/ detail proj/polls/urls.py:1",
		"GET /shop/cart cart shop/urls/__init__.py:1",
		"GET /no no otherpolls/urls.py:1",
	)
}

// A mounted file's own mounts compose recursively; Handler, File and Line
// are unchanged and the prefix joins with exactly one slash.
func TestCompose_Recursive(t *testing.T) {
	got := routes.Compose([]routes.Result{
		file("app.js", []string{"GET /health h"}, [2]string{"/api/", "./api"}),
		file("api.js", []string{"GET /status s"}, [2]string{"/v1", "./v1"}),
		file("v1.js", []string{"DELETE /items/:id del"}),
	})
	assertKeys(t, got,
		"GET /health h app.js:1",
		"GET /api/status s api.js:1",
		"DELETE /api/v1/items/:id del v1.js:1",
	)
}

// A file mounted N times yields N routes, one per mount path.
func TestCompose_MountedNTimes(t *testing.T) {
	got := routes.Compose([]routes.Result{
		file("app.js", nil, [2]string{"/v1", "./users"}, [2]string{"/v2", "./users"}),
		file("other.js", nil, [2]string{"/legacy", "./users"}),
		file("users.js", []string{"GET /u list"}),
	})
	assertKeys(t, got, "GET /v1/u list users.js:1", "GET /v2/u list users.js:1", "GET /legacy/u list users.js:1")
}

// A mount cycle terminates: with no unmounted file to start from, the
// files are read in path order and a file never mounts itself again.
func TestCompose_Cycle(t *testing.T) {
	got := routes.Compose([]routes.Result{
		file("a.js", []string{"GET /a a"}, [2]string{"/b", "./b"}),
		file("b.js", []string{"GET /bb b"}, [2]string{"/a", "./a"}),
		file("self.js", []string{"GET /s s"}, [2]string{"/again", "./self"}),
	})
	assertKeys(t, got, "GET /a a a.js:1", "GET /b/bb b b.js:1", "GET /s s self.js:1")
}

// Composition stops at depth 8: the ninth file of a chain is read as
// unmounted.
func TestCompose_DepthBound(t *testing.T) {
	var in []routes.Result
	for i := 0; i < 10; i++ {
		var mounts [][2]string
		if i < 9 {
			mounts = append(mounts, [2]string{"/p", fmt.Sprintf("./f%d", i+1)})
		}
		in = append(in, file(fmt.Sprintf("f%d.js", i), []string{fmt.Sprintf("GET /r%d h%d", i, i)}, mounts...))
	}
	got := keys(routes.Compose(in))
	want := map[string]bool{
		"GET /p/p/p/p/p/p/p/p/r8 h8 f8.js:1": true, // depth 8
		"GET /r9 h9 f9.js:1":                 true, // past the bound: local paths
	}
	for _, k := range got {
		delete(want, k)
	}
	if len(want) != 0 {
		t.Fatalf("composed %v, missing %v", got, want)
	}
}

// A mount naming nothing the input holds changes nothing.
func TestCompose_UnresolvedMount(t *testing.T) {
	got := routes.Compose([]routes.Result{
		file("app.js", []string{"GET /x x"}, [2]string{"/ext", "express"}, [2]string{"/up", "../../outside"}),
	})
	assertKeys(t, got, "GET /x x app.js:1")
}

func TestCompose_Deterministic(t *testing.T) {
	in := []routes.Result{
		file("app.js", nil, [2]string{"/v1", "./users"}, [2]string{"/v2", "./users"}),
		file("users.js", []string{"GET /u list", "POST /u create"}),
	}
	a := routes.Compose(in)
	rev := []routes.Result{in[1], in[0]}
	b := routes.Compose(rev)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("input order changed the output:\n%v\n%v", a, b)
	}
	for i := 1; i < len(a); i++ {
		p, q := a[i-1], a[i]
		if p.Method > q.Method || (p.Method == q.Method && p.Path > q.Path) {
			t.Fatalf("output not sorted at %d: %+v then %+v", i, p, q)
		}
	}
}

func TestCompose_Empty(t *testing.T) {
	if got := routes.Compose(nil); len(got) != 0 {
		t.Fatalf("Compose(nil) = %v", got)
	}
}
