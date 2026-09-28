package syslink

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/routes"
)

type rowWant struct {
	key, role, node string
	tier            Tier
	line            int
}

func checkRows(t *testing.T, label string, got []KeyRow, want []rowWant) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d rows, want %d:\n%+v", label, len(got), len(want), got)
	}
	for i, w := range want {
		g := got[i]
		if g.Key.String() != w.key || string(g.Role) != w.role || g.NodeID != w.node || g.TierHint != w.tier || g.Line != w.line {
			t.Errorf("%s row %d = {%s %s %s %s %d}, want %+v", label, i, g.Key, g.Role, g.NodeID, g.TierHint, g.Line, w)
		}
	}
}

// TestRouteProvidesFromExtractor is SG-4's contract: routes.Extract's
// registrations become route: provides rows, normalized by §3's grammar,
// with the handler as node id and the literal tier (a registration read
// from code, not from a contract document).
func TestRouteProvidesFromExtractor(t *testing.T) {
	goSrc := []byte(`package api

import "net/http"

func Register(r Router, mux *http.ServeMux) {
	r.GET("/orders/:id", getOrder)
	r.POST("/orders/", createOrder)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {})
}
`)
	got := routeProvides("go", "api/routes.go", goSrc)
	if n := len(routes.Extract("go", "api/routes.go", goSrc)); n != len(got) {
		t.Fatalf("routeProvides = %d rows, routes.Extract = %d; every registration must become one row", len(got), n)
	}
	checkRows(t, "go", got, []rowWant{
		{"route:GET /orders/{}", "provides", "getOrder", TierLiteral, 6},
		{"route:POST /orders", "provides", "createOrder", TierLiteral, 7},
		{"route:ANY /health", "provides", "ANY /health", TierLiteral, 8},
	})
	for _, r := range got {
		if r.NodeKind != "route" || r.File != "api/routes.go" {
			t.Errorf("row %v: node kind %q file %q", r.Key, r.NodeKind, r.File)
		}
	}

	pySrc := []byte(`from flask import Flask
app = Flask(__name__)

@app.route('/items/<int:item_id>', methods=['GET'])
def get_item(item_id):
    pass
`)
	py := routeProvides("python", "app.py", pySrc)
	if len(py) == 0 {
		t.Fatal("python: no route provides from a flask registration")
	}
	for _, r := range py {
		if r.Role != RoleProvides || r.Key.Kind != KindRoute {
			t.Errorf("python row %+v", r)
		}
	}

	if got := routeProvides("ruby", "x.rb", goSrc); got != nil {
		t.Errorf("unsupported dialect produced rows: %v", got)
	}

	// Through the registry, the rows carry the source's id and epoch.
	reg := DefaultRegistry()
	for _, r := range reg.Keys("api/routes.go", goSrc, "go") {
		if r.Role == RoleProvides && (r.Extractor != "routes" || r.ExtractorEpoch < 1) {
			t.Errorf("registry row %v: extractor %q epoch %d", r.Key, r.Extractor, r.ExtractorEpoch)
		}
	}
}

// TestGoClientConsumesTiers is SG-5's contract: ExtractClientCalls's calls
// become route: consumes rows; a key whose holes are all explained (a
// base URL prefix, a trailing id) is `literal`, and a key with a hole where
// a resource name must be is `template`.
func TestGoClientConsumesTiers(t *testing.T) {
	src := []byte(`package c

import (
	"fmt"
	"net/http"
)

const base = "http://orders.example"

func A(id string)          { http.Get(base + "/orders/" + id) }
func B(api, kind string)   { http.Get(api + "/" + kind + "/list") }
func C(id string)          { http.Get(fmt.Sprintf("%s/orders/%s", base, id)) }
func D()                   { http.Post("/orders", "application/json", nil) }
func E(u string)           { http.Get(u) }
func F(api string)         { http.Get(api + "/orders/list") }
func G(kind string)        { http.Get(fmt.Sprintf("/%s/list", kind)) }
// http.Get("/commented/out")
`)
	got := goClientConsumes("client/c.go", src)
	checkRows(t, "go client", got, []rowWant{
		{"route:GET /orders/{}", "consumes", "A", TierLiteral, 10},
		{"route:GET /{}/list", "consumes", "B", TierTemplate, 11},
		{"route:GET /orders/{}", "consumes", "C", TierLiteral, 12},
		{"route:POST /orders", "consumes", "D", TierLiteral, 13},
		{"route:GET /orders/list", "consumes", "F", TierLiteral, 15},
		{"route:GET /{}/list", "consumes", "G", TierTemplate, 16},
	})
	for _, r := range got {
		if r.NodeKind != "call" || r.Attrs["raw_url"] == "" {
			t.Errorf("row %v: node kind %q attrs %v", r.Key, r.NodeKind, r.Attrs)
		}
	}
}
