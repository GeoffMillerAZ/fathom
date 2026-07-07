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
