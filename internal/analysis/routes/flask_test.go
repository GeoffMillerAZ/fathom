package routes

import (
	"reflect"
	"testing"
)

func TestReadFlask(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		src    string
		routes []string
		mounts []string
	}{
		{
			name: "blueprint prefix, methods, shortcuts, add_url_rule and MethodView",
			path: "shop/views.py",
			src: `from flask import Blueprint, Flask
from flask.views import MethodView

bp = Blueprint("shop", __name__, url_prefix="/shop")

@bp.route("/")
def index():
    return "x"

@bp.route("/items/<int:item_id>", methods=["GET", "POST"])
@login_required
def item(item_id):
    return "x"

@bp.delete("/items/<int(min=1):item_id>")
def delete_item(item_id):
    pass

def export():
    pass

bp.add_url_rule("/export", "export", export, methods=["POST"])

class CartView(MethodView):
    def get(self):
        pass
    def put(self):
        pass
    def helper(self):
        pass

bp.add_url_rule("/cart", view_func=CartView.as_view("cart"))
`,
			routes: []string{
				"DELETE /shop/items/{item_id} delete_item",
				"GET /shop/ index",
				"GET /shop/cart CartView.get",
				"GET /shop/items/{item_id} item",
				"POST /shop/export export",
				"POST /shop/items/{item_id} item",
				"PUT /shop/cart CartView.put",
			},
		},
		{
			name: "app factory registers local and imported blueprints",
			path: "app/__init__.py",
			src: `from flask import Flask, Blueprint

def create_app():
    app = CTFdFlask(__name__)
    admin = Blueprint("admin", __name__, url_prefix="/admin")

    @admin.route("/dash")
    def dash():
        pass

    with app.app_context():
        from app.views import views
        from .auth import auth as auth_bp
        from app import api
        app.register_blueprint(views)
        app.register_blueprint(auth_bp, url_prefix="/auth")
        app.register_blueprint(api.bp)
        app.register_blueprint(admin, url_prefix="/staff")
    return app
`,
			routes: []string{"GET /staff/dash dash"},
			mounts: []string{
				"/ -> app.api",
				"/ -> app.views",
				"/auth -> ./auth",
			},
		},
		{
			name: "flask-restx Api on a blueprint, namespaces local and imported",
			path: "CTFd/api/__init__.py",
			src: `from flask import Blueprint
from flask_restx import Api, Namespace, Resource
from CTFd.api.v1.awards import awards_namespace

api = Blueprint("api", __name__, url_prefix="/api/v1")
CTFd_API_v1 = Api(api, version="v1")

local_ns = Namespace("local")

@local_ns.route("")
@local_ns.route("/<int:id>")
class Thing(Resource):
    @admins_only
    def get(self, id=None):
        pass
    def delete(self, id):
        pass

CTFd_API_v1.add_namespace(awards_namespace, "/awards")
CTFd_API_v1.add_namespace(local_ns)
`,
			routes: []string{
				"DELETE /api/v1/local Thing.delete",
				"DELETE /api/v1/local/{id} Thing.delete",
				"GET /api/v1/local Thing.get",
				"GET /api/v1/local/{id} Thing.get",
			},
			mounts: []string{"/api/v1/awards -> CTFd.api.v1.awards"},
		},
		{
			name: "a namespace's package mounts its submodules; an imported namespace's routes stay local",
			path: "CTFd/api/v1/statistics/__init__.py",
			src: `from flask_restx import Namespace

statistics_namespace = Namespace("statistics", description="x")

from CTFd.api.v1.statistics import challenges  # noqa
from CTFd.api.v1.statistics import scores  # noqa
from CTFd.utils import helpers
`,
			mounts: []string{
				"/ -> CTFd.api.v1.statistics.challenges",
				"/ -> CTFd.api.v1.statistics.scores",
			},
		},
		{
			name: "a route on an imported namespace",
			path: "CTFd/api/v1/statistics/scores.py",
			src: `from flask_restx import Resource
from CTFd.api.v1.statistics import statistics_namespace

@statistics_namespace.route("/scores/distribution")
class ScoresDistribution(Resource):
    def get(self):
        pass
`,
			routes: []string{"GET /scores/distribution ScoresDistribution.get"},
		},
		{
			name: "no flask import, nothing read",
			path: "lib/x.py",
			src:  "@bp.route('/x')\ndef x():\n    pass\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := readFlask(c.path, []byte(c.src))
			if got := routeLines(res.Routes); !reflect.DeepEqual(got, sortedCopy(c.routes)) && !(len(got) == 0 && len(c.routes) == 0) {
				t.Errorf("routes:\n got  %q\n want %q", got, sortedCopy(c.routes))
			}
			if got := mountLines(res.Mounts); !reflect.DeepEqual(got, sortedCopy(c.mounts)) && !(len(got) == 0 && len(c.mounts) == 0) {
				t.Errorf("mounts:\n got  %q\n want %q", got, sortedCopy(c.mounts))
			}
		})
	}
}
