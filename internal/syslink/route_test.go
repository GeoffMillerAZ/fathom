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

// TestRouteProvidesFromExtractor is SG-4's contract: a service's routes,
// read as ingest stores them and composed across its files as service_ref
// joins them (serviceref.ExtractFile, RoutesFromRows), become route:
// provides rows, normalized by §3's grammar, with the handler as node id
// and the literal tier (a registration read from code, not from a contract
// document), stamped with the route source's id and epoch.
func TestRouteProvidesFromExtractor(t *testing.T) {
	goSrc := []byte(`package api

import "net/http"

func Register(r Router, mux *http.ServeMux) {
	r.GET("/orders/:id", getOrder)
	r.POST("/orders/", createOrder)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {})
}
`)
	got := routeProvides([]SourceFile{{Path: "api/routes.go", Dialect: "go", Content: goSrc}})
	if n := len(routes.ExtractFile("go", "api/routes.go", goSrc).Routes); n != len(got) {
		t.Fatalf("routeProvides = %d rows, routes.ExtractFile = %d; every registration must become one row", len(got), n)
	}
	checkRows(t, "go", got, []rowWant{
		{"route:GET /orders/{}", "provides", "getOrder", TierLiteral, 6},
		{"route:POST /orders", "provides", "createOrder", TierLiteral, 7},
		{"route:ANY /health", "provides", "ANY /health", TierLiteral, 8},
	})
	for _, r := range got {
		if r.NodeKind != "route" || r.File != "api/routes.go" || r.Extractor != routeExtractorID || r.ExtractorEpoch != routeExtractorEpoch {
			t.Errorf("row %v: node kind %q file %q extractor %q epoch %d", r.Key, r.NodeKind, r.File, r.Extractor, r.ExtractorEpoch)
		}
	}

	pySrc := []byte(`from flask import Flask
app = Flask(__name__)

@app.route('/items/<int:item_id>', methods=['GET'])
def get_item(item_id):
    pass
`)
	py := routeProvides([]SourceFile{{Path: "app.py", Dialect: "python", Content: pySrc}})
	if len(py) == 0 {
		t.Fatal("python: no route provides from a flask registration")
	}
	for _, r := range py {
		if r.Role != RoleProvides || r.Key.Kind != KindRoute {
			t.Errorf("python row %+v", r)
		}
	}

	// Java and PHP routes are keyed too (O691: the per-file registry never
	// listed them): a service's files are read by the route seam's
	// readers, whatever their dialect id.
	javaSrc := []byte(`@RestController
@RequestMapping("/owners")
class OwnerController {
	@GetMapping("/{ownerId}")
	public Owner show(@PathVariable int ownerId) { return null; }
}
`)
	phpSrc := []byte("<?php\nuse Illuminate\\Support\\Facades\\Route;\nRoute::get('/books/{id}', [BookController::class, 'show']);\n")
	jp := routeProvides([]SourceFile{
		{Path: "src/main/java/OwnerController.java", Dialect: "java", Content: javaSrc},
		{Path: "routes/web.php", Dialect: "php", Content: phpSrc},
	})
	checkRows(t, "java+php", jp, []rowWant{
		{"route:GET /books/{}", "provides", "BookController.show", TierLiteral, 3},
		{"route:GET /owners/{}", "provides", "OwnerController.show", TierLiteral, 4},
	})

	if got := routeProvides([]SourceFile{{Path: "x.rb", Dialect: "ruby", Content: goSrc}}); got != nil {
		t.Errorf("a file no route reader reads produced rows: %v", got)
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
