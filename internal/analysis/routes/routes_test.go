package routes_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/routes"
)

// -----------------------------------------------------------------
// Go — gin/echo/chi
// -----------------------------------------------------------------

const goGinSrc = `package main

import "github.com/gin-gonic/gin"

func main() {
	r := gin.Default()
	r.GET("/health", HealthHandler)
	r.POST("/users", CreateUser)
	r.PUT("/users/:id", UpdateUser)
	r.DELETE("/users/:id", DeleteUser)
	r.Run()
}
`

func TestExtractGoGin(t *testing.T) {
	got := routes.Extract("go", "main.go", []byte(goGinSrc))
	if len(got) != 4 {
		t.Fatalf("expected 4 routes, got %d: %v", len(got), got)
	}
	// Index by method+path to handle duplicate paths with different methods.
	byKey := make(map[string]routes.Route)
	for _, r := range got {
		byKey[r.Method+":"+r.Path] = r
	}
	checks := []struct{ path, method, handler string }{
		{"/health", "GET", "HealthHandler"},
		{"/users", "POST", "CreateUser"},
		{"/users/:id", "PUT", "UpdateUser"},
		{"/users/:id", "DELETE", "DeleteUser"},
	}
	for _, c := range checks {
		key := c.method + ":" + c.path
		r, ok := byKey[key]
		if !ok {
			t.Errorf("missing route %s %s", c.method, c.path)
			continue
		}
		if r.Handler != c.handler {
			t.Errorf("%s %s: handler=%q want %q", c.method, c.path, r.Handler, c.handler)
		}
		if r.File != "main.go" {
			t.Errorf("file=%q want main.go", r.File)
		}
	}
}

// -----------------------------------------------------------------
// Go — net/http mux.HandleFunc
// -----------------------------------------------------------------

const goNetHTTPSrc = `package main

import "net/http"

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", PingHandler)
	mux.HandleFunc("/api/v1/items", ItemsHandler)
	http.ListenAndServe(":8080", mux)
}
`

func TestExtractGoNetHTTP(t *testing.T) {
	got := routes.Extract("go", "server.go", []byte(goNetHTTPSrc))
	if len(got) < 2 {
		t.Fatalf("expected at least 2 routes, got %d: %v", len(got), got)
	}
	byPath := make(map[string]routes.Route)
	for _, r := range got {
		byPath[r.Path] = r
	}
	if _, ok := byPath["/ping"]; !ok {
		t.Error("missing /ping")
	}
	if _, ok := byPath["/api/v1/items"]; !ok {
		t.Error("missing /api/v1/items")
	}
}

// -----------------------------------------------------------------
// Go — net/http 1.22+ method-prefix patterns
// -----------------------------------------------------------------

const goNetHTTP122Src = `package main

func register(inner *http.ServeMux) {
	inner.HandleFunc("GET /sources", s.handleUISources)
	inner.HandleFunc("POST /intake/briefs", s.handleCreateBrief)
	mux.Handle("DELETE /cases/{id}", s.handleDeleteCase)
	mux.HandleFunc("/healthz", PingHandler)
}
`

func TestExtractGoNetHTTPMethodPrefix(t *testing.T) {
	got := routes.Extract("go", "server.go", []byte(goNetHTTP122Src))
	byKey := make(map[string]routes.Route)
	for _, r := range got {
		byKey[r.Method+" "+r.Path] = r
	}
	// Method prefix is split out; path excludes the method.
	checks := []struct{ key, handler string }{
		{"GET /sources", "s.handleUISources"},
		{"POST /intake/briefs", "s.handleCreateBrief"},
		{"DELETE /cases/{id}", "s.handleDeleteCase"},
		{"ANY /healthz", "PingHandler"}, // prefix-less keeps ANY default
	}
	for _, c := range checks {
		r, ok := byKey[c.key]
		if !ok {
			t.Errorf("missing route %q; got %v", c.key, got)
			continue
		}
		if r.Handler != c.handler {
			t.Errorf("route %q handler=%q want %q", c.key, r.Handler, c.handler)
		}
	}
}

// -----------------------------------------------------------------
// Go — Line numbers
// -----------------------------------------------------------------

const goLinesSrc = `package main
import "github.com/gin-gonic/gin"
func main() {
	r := gin.Default()
	r.GET("/a", HandlerA)
	r.GET("/b", HandlerB)
}
`

func TestExtractGoLineNumbers(t *testing.T) {
	got := routes.Extract("go", "lines.go", []byte(goLinesSrc))
	if len(got) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(got))
	}
	// sorted: /a then /b
	if got[0].Path != "/a" || got[0].Line != 5 {
		t.Errorf("/a: line=%d want 5", got[0].Line)
	}
	if got[1].Path != "/b" || got[1].Line != 6 {
		t.Errorf("/b: line=%d want 6", got[1].Line)
	}
}

// -----------------------------------------------------------------
// Express (JS)
// -----------------------------------------------------------------

const jsSrc = `const express = require('express')
const app = express()
const router = express.Router()

app.get('/health', healthCheck)
app.post('/users', createUser)
router.put('/users/:id', updateUser)
router.delete('/users/:id', deleteUser)
app.use('/api', router)
`

func TestExtractJS(t *testing.T) {
	got := routes.Extract("js", "app.js", []byte(jsSrc))
	if len(got) == 0 {
		t.Fatal("expected routes, got none")
	}
	byMethodPath := make(map[string]routes.Route)
	for _, r := range got {
		byMethodPath[r.Method+":"+r.Path] = r
	}

	checks := []struct{ method, path string }{
		{"GET", "/health"},
		{"POST", "/users"},
		{"PUT", "/users/:id"},
		{"DELETE", "/users/:id"},
	}
	for _, c := range checks {
		key := c.method + ":" + c.path
		if _, ok := byMethodPath[key]; !ok {
			t.Errorf("missing route %s %s", c.method, c.path)
		}
	}
}

func TestExtractJSAlias(t *testing.T) {
	// "javascript" and "ts" should both work
	src := []byte("app.get('/ping', handler)\n")
	a := routes.Extract("javascript", "a.js", src)
	b := routes.Extract("ts", "b.ts", src)
	if len(a) == 0 || len(b) == 0 {
		t.Fatal("expected routes for js/ts aliases")
	}
	if a[0].Method != "GET" || a[0].Path != "/ping" {
		t.Errorf("unexpected route: %+v", a[0])
	}
}

// -----------------------------------------------------------------
// Python — Flask + FastAPI
// -----------------------------------------------------------------

const pySrc = `from flask import Flask
app = Flask(__name__)

@app.route('/health')
def health():
    return 'ok'

@app.route('/users', methods=['GET', 'POST'])
def users():
    pass

@app.get('/items')
def list_items():
    pass

@app.post('/items')
def create_item():
    pass
`

func TestExtractPython(t *testing.T) {
	got := routes.Extract("python", "app.py", []byte(pySrc))
	if len(got) == 0 {
		t.Fatal("expected routes, got none")
	}
	byMethodPath := make(map[string]bool)
	for _, r := range got {
		byMethodPath[r.Method+":"+r.Path] = true
	}

	checks := []string{
		"GET:/health",
		"GET,POST:/users",
		"GET:/items",
		"POST:/items",
	}
	for _, key := range checks {
		if !byMethodPath[key] {
			t.Errorf("missing route %s; have: %v", key, byMethodPath)
		}
	}
}

func TestExtractPythonAlias(t *testing.T) {
	src := []byte("@app.get('/ping')\ndef ping(): pass\n")
	got := routes.Extract("py", "app.py", src)
	if len(got) == 0 {
		t.Fatal("expected route")
	}
	if got[0].Method != "GET" || got[0].Path != "/ping" {
		t.Errorf("unexpected: %+v", got[0])
	}
}

// -----------------------------------------------------------------
// Go — gorilla/mux call chains (VB-31 M8)
// -----------------------------------------------------------------

const goGorillaSrc = `package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

const NameExpression = "-a-zA-Z_0-9."

func register(r *mux.Router) {
	r.HandleFunc("/system/functions", handleList).Methods(http.MethodGet)
	r.HandleFunc("/system/functions", handleCreate).Methods(http.MethodPost)
	r.HandleFunc("/system/function/{name:["+NameExpression+"]+}", handleStatus).Methods(http.MethodGet)
	r.Methods("POST").Path("/legacy/scale").HandlerFunc(handleScale)
	r.HandleFunc("/healthz", handleHealth).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc("/open", handleOpen)
}
`

func TestExtractGoGorillaChain(t *testing.T) {
	got := routes.Extract("go", "server.go", []byte(goGorillaSrc))
	byKey := make(map[string]routes.Route)
	for _, r := range got {
		byKey[r.Method+" "+r.Path] = r
	}

	checks := []struct{ method, path string }{
		{"GET", "/system/functions"},
		{"POST", "/system/functions"},
		// Concatenated path folded via the package-level const NameExpression.
		{"GET", "/system/function/{name:[-a-zA-Z_0-9.]+}"},
		// Reversed chain order: Methods().Path().HandlerFunc().
		{"POST", "/legacy/scale"},
		// A single .Methods() call with two methods fans out to two Routes.
		{"GET", "/healthz"},
		{"HEAD", "/healthz"},
		// HandleFunc with no .Methods() chain defaults to ANY.
		{"ANY", "/open"},
	}
	for _, c := range checks {
		key := c.method + " " + c.path
		if _, ok := byKey[key]; !ok {
			t.Errorf("missing route %q; got %v", key, got)
		}
	}
	if _, ok := byKey["POST /healthz"]; ok {
		t.Errorf("spurious POST /healthz: .Methods(GET, HEAD) must not fan out to POST")
	}
}

// -----------------------------------------------------------------
// Go — gorilla/mux PathPrefix subrouters (VB-31 M8)
// -----------------------------------------------------------------

const goSubrouterSrc = `package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

func register(r *mux.Router) {
	sub := r.PathPrefix("/system").Subrouter()
	sub.HandleFunc("/functions", handleList).Methods(http.MethodGet)

	v2 := sub.PathPrefix("/v2").Subrouter()
	v2.HandleFunc("/functions", handleListV2).Methods(http.MethodGet)
}
`

func TestExtractGoPathPrefixSubrouter(t *testing.T) {
	got := routes.Extract("go", "server.go", []byte(goSubrouterSrc))
	byKey := make(map[string]routes.Route)
	for _, r := range got {
		byKey[r.Method+" "+r.Path] = r
	}
	checks := []string{
		"GET /system/functions",
		// Nested subrouter accumulates both prefixes.
		"GET /system/v2/functions",
	}
	for _, key := range checks {
		if _, ok := byKey[key]; !ok {
			t.Errorf("missing route %q; got %v", key, got)
		}
	}
}

// -----------------------------------------------------------------
// Go — a route path built from a plain variable is skipped, not guessed
// with a bogus method/path combination that could false-match
// -----------------------------------------------------------------

const goUnresolvedPathSrc = `package main

func register(mux *ServeMux, dynamicPath string) {
	mux.HandleFunc(dynamicPath, handleDynamic)
	mux.HandleFunc("/static", handleStatic)
}
`

func TestExtractGoUnresolvedPathNormalizesToWildcard(t *testing.T) {
	// A path built from a bare variable reference folds to the "{}"
	// placeholder (parity with ExtractClientCalls's own "parameters
	// normalize to {}" rule) rather than being skipped outright — it can
	// never spuriously MATCH a concrete client call (no client call's
	// method is ever "ANY"), so this costs nothing on precision.
	got := routes.Extract("go", "server.go", []byte(goUnresolvedPathSrc))
	var sawWildcard, sawStatic bool
	for _, r := range got {
		if r.Path == "{}" {
			sawWildcard = true
		}
		if r.Path == "/static" {
			sawStatic = true
		}
	}
	if !sawWildcard {
		t.Errorf("expected a wildcarded route for the unresolved path, got %v", got)
	}
	if !sawStatic {
		t.Errorf("expected /static, got %v", got)
	}
}

// -----------------------------------------------------------------
// Unknown language
// -----------------------------------------------------------------

func TestExtractUnknownLang(t *testing.T) {
	got := routes.Extract("cobol", "foo.cbl", []byte("something"))
	if got != nil && len(got) != 0 {
		t.Errorf("expected nil/empty for unknown lang, got %v", got)
	}
}

// -----------------------------------------------------------------
// Determinism
// -----------------------------------------------------------------

func TestExtractDeterminism(t *testing.T) {
	src := []byte(goGinSrc)
	a := routes.Extract("go", "main.go", src)
	b := routes.Extract("go", "main.go", src)
	if len(a) != len(b) {
		t.Fatal("non-deterministic length")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("non-deterministic at %d: %+v vs %+v", i, a[i], b[i])
		}
	}
}

// -----------------------------------------------------------------
// Empty source
// -----------------------------------------------------------------

func TestExtractEmpty(t *testing.T) {
	got := routes.Extract("go", "empty.go", []byte(""))
	if got != nil && len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}
