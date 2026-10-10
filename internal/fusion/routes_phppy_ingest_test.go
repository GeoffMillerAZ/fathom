package fusion_test

// Wave c2 item phppy (LB-F331..LB-F335): an ingest stores what the PHP and
// Python route readers read, a mount as a MOUNT row, and
// serviceref.RoutesFromRows (what BuildFromStore and indexbench
// route-oracle read) composes each mount across the service's files: a
// Laravel api file under its apiPrefix, Symfony attribute controllers under
// their config/routes.yaml import prefix, a Django include chain, a FastAPI
// include_router from another module, a Flask blueprint registered with a
// url_prefix. No MOUNT row survives composition.

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/serviceref"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

var phppyIngestFiles = map[string]string{
	// Laravel 11+: routes/api.php is served under withRouting's apiPrefix.
	"shop/bootstrap/app.php": `<?php
return Application::configure(basePath: dirname(__DIR__))
    ->withRouting(
        web: __DIR__.'/../routes/web.php',
        api: __DIR__.'/../routes/api.php',
        apiPrefix: 'api/v2',
    )->create();
`,
	"shop/routes/api.php": `<?php
use App\Http\Controllers\OrderController;
Route::apiResource('orders', OrderController::class)->only(['index', 'show']);
`,
	"shop/routes/web.php": `<?php
Route::get('/', [HomeController::class, 'index']);
`,
	// Symfony: attribute controllers imported under a prefix.
	"store/config/routes.yaml": `controllers:
    resource: ../src/Controller/
    type: attribute
    prefix: /store
`,
	"store/src/Controller/CartController.php": `<?php
namespace App\Controller;
use Symfony\Component\Routing\Attribute\Route;
#[Route('/cart')]
class CartController
{
    #[Route('/items', name: 'cart_items', methods: ['GET'])]
    public function items(): Response {}
}
`,
	// Django: an include chain.
	"site/mysite/urls.py": `from django.urls import include, path
urlpatterns = [
    path('polls/', include('polls.urls')),
]
`,
	"site/polls/urls.py": `from django.urls import path
from . import views
urlpatterns = [
    path('<int:pk>/', views.detail, name='detail'),
]
`,
	// FastAPI: a router from another module, included with a prefix.
	"api/app/main.py": `from fastapi import FastAPI
from app.routers import items
app = FastAPI()
app.include_router(items.router, prefix="/v1")
`,
	"api/app/routers/items.py": `from fastapi import APIRouter
router = APIRouter(prefix="/items")

@router.get("/{item_id}")
def read_item(item_id: int):
    return {}
`,
	// Flask: a blueprint from another module, registered with a url_prefix.
	"web/web/__init__.py": `from flask import Flask
from web.admin import admin

def create_app():
    app = Flask(__name__)
    app.register_blueprint(admin, url_prefix="/admin")
    return app
`,
	"web/web/admin.py": `from flask import Blueprint
admin = Blueprint("admin", __name__)

@admin.route("/dash", methods=["GET", "POST"])
def dash():
    return "ok"
`,
}

func TestIngest_PHPAndPythonFrameworkRoutes(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	for rel, data := range phppyIngestFiles {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "s.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	p := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 2})
	p.WithGitHead(func(string) string { return "" })
	p.WithGitLog(func(string) string { return "" })
	if _, err := p.Ingest(ctx, core.ServiceRef{ID: "poly", Path: root}); err != nil {
		t.Fatal(err)
	}
	refs, complete, err := st.ServiceRefsFor(ctx, "poly")
	if err != nil || !complete {
		t.Fatalf("ServiceRefsFor: complete=%v err=%v", complete, err)
	}
	mounts := 0
	for _, r := range refs.Routes {
		if r.Method == serviceref.MountMethod {
			mounts++
		}
	}
	if mounts == 0 {
		t.Fatalf("stored rows %+v: want the readers' mounts stored as %s rows", refs.Routes, serviceref.MountMethod)
	}
	var got []string
	for _, r := range serviceref.RoutesFromRows(refs.Routes) {
		if r.Method == serviceref.MountMethod {
			t.Fatalf("a mount row leaked out of composition: %+v", r)
		}
		got = append(got, r.Method+" "+serviceref.NormalizeTemplate(r.Path)+" "+r.Handler)
	}
	sort.Strings(got)
	want := []string{
		"ANY /polls/{} detail",
		"GET / HomeController.index",
		"GET /admin/dash dash",
		"GET /api/v2/orders OrderController.index",
		"GET /api/v2/orders/{} OrderController.show",
		"GET /store/cart/items CartController.items",
		"GET /v1/items/{} read_item",
		"POST /admin/dash dash",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("composed routes:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
