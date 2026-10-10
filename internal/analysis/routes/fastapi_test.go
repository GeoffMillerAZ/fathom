package routes

import (
	"reflect"
	"testing"
)

func TestReadFastAPI(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		src    string
		routes []string
		mounts []string
	}{
		{
			name: "a router with a prefix, verbs, api_route and websocket",
			path: "app/api/routes/items.py",
			src: `from fastapi import APIRouter, WebSocket

router = APIRouter(prefix="/items", tags=["items"])

@router.get("/", response_model=ItemsPublic)
def read_items(session: SessionDep, skip: int = 0) -> Any:
    ...

@router.get("/{id}")
async def read_item(id: uuid.UUID) -> Any:
    ...

@router.api_route("/{id}/touch", methods=["post", "put"])
def touch_item(id):
    ...

@router.websocket("/ws")
async def ws(websocket: WebSocket):
    ...

def helper():
    pass
`,
			routes: []string{
				"GET /items/ read_items",
				"GET /items/{id} read_item",
				"POST /items/{id}/touch touch_item",
				"PUT /items/{id}/touch touch_item",
				"WS /items/ws ws",
			},
		},
		{
			name: "include_router: local routers inline, imported routers as mounts",
			path: "app/main.py",
			src: `from fastapi import FastAPI, APIRouter
from app.api.routes import items, login
from .users import router as users_router
from app.core.config import settings

app = FastAPI(title="x")
api = APIRouter(prefix="/api")
admin = APIRouter()

@admin.get("/stats")
def stats():
    ...

@app.get("/health")
def health():
    ...

api.include_router(items.router)
api.include_router(login.router, prefix="/auth")
api.include_router(users_router, prefix="/people")
api.include_router(admin, prefix="/admin")
app.include_router(api)
app.include_router(other.router, prefix=settings.API_V1_STR)
`,
			routes: []string{
				"GET /api/admin/stats stats",
				"GET /health health",
			},
			mounts: []string{
				"/api -> app.api.routes.items",
				"/api/auth -> app.api.routes.login",
				"/api/people -> ./users",
			},
		},
		{
			name: "add_api_route and a class-free module without fastapi",
			path: "svc/app.py",
			src: `import fastapi
app = fastapi.FastAPI()
app.add_api_route("/ping", ping, methods=["GET", "HEAD"])
`,
			routes: []string{"GET /ping ping"},
		},
		{
			name: "no fastapi import, nothing read",
			path: "svc/flaskapp.py",
			src:  "@app.get('/x')\ndef x():\n    pass\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := readFastAPI(c.path, []byte(c.src))
			if got := routeLines(res.Routes); !reflect.DeepEqual(got, sortedCopy(c.routes)) && !(len(got) == 0 && len(c.routes) == 0) {
				t.Errorf("routes:\n got  %q\n want %q", got, sortedCopy(c.routes))
			}
			if got := mountLines(res.Mounts); !reflect.DeepEqual(got, sortedCopy(c.mounts)) && !(len(got) == 0 && len(c.mounts) == 0) {
				t.Errorf("mounts:\n got  %q\n want %q", got, sortedCopy(c.mounts))
			}
		})
	}
}
