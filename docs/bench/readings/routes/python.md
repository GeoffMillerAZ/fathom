# Python framework routes: Django and DRF, FastAPI, Flask (wave c2, item phppy, LB-F336)

**Verdict: FastAPI PASS, Flask PASS; Django reads every route its URL modules declare, and NetBox
declares most of its routes elsewhere (a counted ceiling).** Before this item fathom read Python
routes with a line regex that bound none to its handler and applied no prefix. It now reads them from
a parse of the source, binds each to its function or method, and applies `include()`,
`include_router()` and `register_blueprint()` across files:

| Subject | Routes the app serves | fathom found | Handler right | Verdict |
|---|---|---|---|---|
| fastapi/full-stack-fastapi-template | 22 | all 22, one extra | 22 | PASS |
| CTFd/CTFd (Flask, flask-restx) | 221 | 218, one extra | 218 | PASS |
| netbox-community/netbox (Django, DRF) | 4,050 | 1,735, 106 extra | 1,735 | FAIL (ceiling) |

The comparison is with each framework's own route table, read from the imported application:
Django's URL resolver (`get_resolver().url_patterns`, walked through every include and DRF router),
FastAPI's route table (the one `app.openapi()` is generated from), and Flask's `app.url_map`. fathom
gives the same answer without importing the application, installing its dependencies or configuring a
database.

Keep it, and pair it with the runtime dump on a Django project that registers its views through its
own machinery. On NetBox every route a `urls.py` declares is read, and bound: all 1,634 standard DRF
viewset routes, 32 of 34 API views, every one of the 69 UI paths written in a `urls.py`. What is not
read is NetBox's, not Django's: 1,471 UI routes come from NetBox's own view registry
(`get_model_urls()` over `@register_model_view` decorators), 792 bulk routes from `NetBoxRouter`, a
`DefaultRouter` subclass in another module that remaps the list route, and 50 `@action` extras from
viewsets defined in other modules.

## The five frameworks of this item

| Framework | Reading | Bound recall | Bound precision | Verdict |
|---|---|---|---|---|
| Laravel | php.json: BookStack, koel | 1.0, 1.0 | 1.0, .9967 | PASS |
| Symfony | php.json: symfony/demo | .6316 | .6316 | ceiling: the seam's directory mount is not recursive (1.0 with one) |
| Django and DRF | python.json: NetBox | .4284 | .9424 | ceiling: a project's view registry, router subclass and cross-module viewsets |
| FastAPI | python.json: full-stack-fastapi-template | 1.0 | .9565 | PASS (configured root stripped) |
| Flask | python.json: CTFd | .9864 | .9954 | PASS |

## Method

`indexbench route-oracle` (LB-F324, item java) ingests each subject into a scratch store as a product
ingest does, reads its routes back through `serviceref.RoutesFromRows` and scores them: route recall
and precision on (method, normalized path), bound recall and precision with the handler too. The run
uses `-only '**/*.py'`: the reading measures the Python readers. Without it, the legacy
`express-regex` reader adds 322 routes from CTFd's bundled JavaScript, every one a false route; that
reader is item node's (O699).

| Subject | Pin | Licence | Oracle (tier A) | Runtime |
|---|---|---|---|---|
| netbox-community/netbox | v4.7.2 `251458b8` | Apache-2.0 | Django 6.1.1 URL resolver, DRF 3.18.0 | CPython 3.13.13 |
| fastapi/full-stack-fastapi-template | 0.12.0 `84c5a9e1` | MIT | FastAPI 0.143.0 route table | CPython 3.14.7 |
| CTFd/CTFd | 3.8.8 `b4a2b319` | Apache-2.0 | Flask 2.1.3 `app.url_map` | CPython 3.11.15 |

Each subject's requirements were installed into its own scratch virtual environment (NetBox's
`psycopg[c]` as `psycopg[binary]`), with a scratch configuration (NetBox's `configuration.py` from its
example; the template's settings from environment variables, and an empty `frontend` directory;
CTFd on a scratch SQLite database). The converters are `calibration/oracles/django/django_routes.py`,
`calibration/oracles/fastapi/fastapi_routes.py` and `calibration/oracles/flask/flask_routes.py`; the
oracle files beside them are what they printed.

The converters' rules, all written in their docstrings:

- A route is the repository's when its view is defined in the repository, or when a URL module of
  the repository declares it (NetBox's three `drf_spectacular` schema views). A package's URL module
  (`include('social_django.urls')`, four routes), FastAPI's own docs and schema routes (four) and
  Flask's static route are left out.
- Django's resolver is method-agnostic: a function or class-based view is ANY with its function or
  class name. A viewset route is one row per method DRF maps, handler `ViewSet.action`.
- A path is written as the framework renders it, a converter `<int:pk>` as `{pk}`, a regex as a
  template (anchors dropped, a named group `{name}`), the same rule the reader applies.
- HEAD beside GET is not a route, nor is Flask's automatic OPTIONS. A class-based Flask view is one
  row per HTTP method the class defines, `Class.method`.
- FastAPI: `--strip-prefix=/api/v1`. The template includes its API router under
  `prefix=settings.API_V1_STR`, a pydantic setting ("/api/v1" by default, overridable from the
  environment) that no source reader resolves. The reader reads a prefix named by a setting as the
  configured root, and the converter strips it, the precedent of the Java converters' configured
  root (O693).

Measured at fathom `21e55905`; the numbers are in `python.json`.

## What the readers read

**Django** (`readDjango`): `urlpatterns` assigned, extended and concatenated; `path()` with its
converters, `re_path()`/`url()` with the regex read as a template; `include('a.b.urls')` and an
imported URL module as mounts, a list inline; a view's function, the class of `as_view()`, a viewset's
`as_view({'get': 'list'})`; a DRF router's `register()` as the list and detail routes, narrowed to the
actions of a viewset defined in the module (its mixins, `ReadOnlyModelViewSet`) with its `@action`
extras, `DefaultRouter`'s API root and `.{format}` variants. A `settings.DEBUG` branch and test
modules are not read.

**FastAPI** (`readFastAPI`): `FastAPI()` and `APIRouter(prefix=...)`; the path-operation decorators,
`api_route`, `websocket` and `add_api_route`; `include_router` and `mount` of a router in the module
served under its includer, of a router from another module as a mount of that module.

**Flask** (`readFlask`): `Flask()` (and a subclass), `Blueprint(url_prefix=...)`, flask-restx `Api` and
`Namespace`; `@x.route` on a function and on a Resource or MethodView class, the method shortcuts,
`add_url_rule`, `add_resource`; `register_blueprint` and `add_namespace` as mounts or, in the module,
inline (a given prefix replacing the child's own, as Flask does); a blueprint or namespace package
mounting the submodules it imports.

## Before and after: the python-regex reader

The legacy `python-regex` reader (`legacy_python.go`, this item's after the seam) now reads nothing on
a file the Flask or FastAPI reader reads, and stays the fallback on other Python files (a Quart or
Sanic application's `@app.route`). On the same subjects, at `c2-seam` `6c738a22` (python-regex alone)
and at HEAD:

| Subject | Before: route recall, precision | Before: bound | After: route recall, precision | After: bound |
|---|---|---|---|---|
| full-stack-fastapi-template | .2727, .3158 | 0, 0 | 1.0, .9565 | 1.0, .9565 |
| CTFd | .1636, .3025 | 0, 0 | .9864, .9954 | .9864, .9954 |
| NetBox | 0, 0 | 0, 0 | .4283, .9424 | .4284, .9424 |

Kept beside the new readers, it would add each route it reads again, unbound, at the decorator's
local path, and spell a multi-method rule as one `GET,POST` row: on CTFd its 119 rows bind no handler,
so bound precision would be 218 of 338, .645, instead of .9954. Two seam tests assert the regex reader's old output on a Flask
file (`TestExtractPython`'s `GET,POST:/users`) and that no reader reads YAML (`TestHasReader`); this
item's readers change both by design (O697).

## Composition: with and without mounts

The same binary with the mount rows ignored at read time (a scratch build):

| Subject | Bound recall, mounts applied | Bound recall, mounts ignored |
|---|---|---|
| NetBox | .4284 | .0037 (every app's `include('x.urls')` and `include('x.api.urls')` lost) |
| CTFd | .9864 | .3710 (the restx namespaces, the API blueprint's `/api/v1`) |
| full-stack-fastapi-template | 1.0 | 1.0 (each router carries its own prefix; the API root is configured) |

## Ceilings

- **NetBox's view registry: 1,471 UI routes.** `path('regions/', include(get_model_urls('dcim',
  'region', detail=False)))` includes what `@register_model_view(Region, ...)` decorators registered
  across the views modules at import. A Django project's own registry, not Django's; no per-file
  reader reaches it. Trigger: a reader for the registry's decorators that reports a mount, keyed by
  the model, if a second project uses the pattern.
- **A router subclass in another module: 792 routes.** `NetBoxRouter(DefaultRouter)` adds
  `bulk_update`, `bulk_partial_update` and `bulk_destroy` to the list route in
  `netbox/api/routers.py`; each `urls.py` instantiates it by name. Trigger: a class-scoped mount or
  constant form in the seam, so a module defining a router subclass can report its mapping (O698).
- **Viewsets defined in another module: 50 `@action` extras missed, 106 standard routes extra.** A
  `router.register('x', views.XViewSet)` names a class the `urls.py` cannot see: its `@action` methods
  are not read, and a read-only viewset is read with all six actions. Trigger: a mount target that
  names a class of a module (`views#XViewSet`) in the seam (O698).
- **Plugin API roots: 2 routes,** built in a loop over the installed plugins.
- **A route registered by environment.** The template includes its `private` router only when
  `FASTAPI_ENV` is `development`; the reader reads both branches of an `if`: one extra.
- **CTFd's plugin assets: 3 routes,** each rule built from its plugin's directory name at runtime.

## Ingest cost

Called directly over NetBox's 1,316 Python files (17 MB): the Django reader takes 8.5 ms (a byte
prefilter, `urlpatterns` or a router registration, passes 17 files to the parser), the FastAPI and
Flask readers 2.1 ms and 3.5 ms (their prefilters are the framework's name), the python-regex reader
15.9 ms. A full NetBox ingest takes 10.9 s at `c2-seam` and 12.9 s at HEAD in the readings above, one
run each; the PHP reading's three-run comparison on BookStack is inside noise (php.md).
