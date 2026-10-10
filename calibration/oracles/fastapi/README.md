# FastAPI route oracle

`fastapi_routes.py` is the FastAPI oracle for `indexbench route-oracle` (LB-F334, LB-F336; shape in
`calibration/oracles/routes/README.md`). Tier A: it imports the subject's application object and
walks FastAPI's own route table with `fastapi.routing.iter_route_contexts`, the walk `app.openapi()`
makes (FastAPI 0.14x keeps an included router as one lazy entry of `app.routes`). `app.openapi()`'s
operation count is printed on stderr as a cross-check.

| Reads | Tier | Emits |
|---|---|---|
| FastAPI's route table after every `include_router` | A | method, path, handler |

## What is FastAPI's

Every path (prefixes already joined), every method set, every endpoint function. The handler is the
endpoint's `__qualname__` (`Class.method` for a method).

## What is not

The scope (an endpoint under site-packages, such as FastAPI's own docs and openapi.json routes, is
the framework's), HEAD beside GET, and `--strip-prefix`: a configured root dropped from every path,
the precedent of the Java converters' `-strip-prefix` (O693). full-stack-fastapi-template includes
its API router under `prefix=settings.API_V1_STR`, a pydantic setting ("/api/v1" by default,
overridable from the environment), which no source reader resolves.

## Run (2026-10-09, CPython 3.14.7, a scratch virtual environment)

```sh
git clone --depth 1 --branch 0.12.0 https://github.com/fastapi/full-stack-fastapi-template.git tpl
uv venv --python 3.14 venv && (cd tpl/backend && VIRTUAL_ENV=../../venv uv pip install -e .)
mkdir -p tpl/backend/app/frontend   # app.frontend() needs the built frontend's directory to exist
SECRET_KEY=x PROJECT_NAME=demo DATABASE_URL=postgresql://u:p@localhost:5432/d \
  FIRST_SUPERUSER=a@b.co FIRST_SUPERUSER_PASSWORD=pw ENVIRONMENT=local \
  venv/bin/python -I fastapi_routes.py tpl tpl/backend app.main:app \
  fastapi/full-stack-fastapi-template "$(git -C tpl rev-parse HEAD)" MIT --strip-prefix=/api/v1 \
  > full-stack-fastapi-template.json
```

full-stack-fastapi-template.json: FastAPI 0.143.0, 22 routes (22 OpenAPI operations); 4 framework
routes left out. No database is opened.
