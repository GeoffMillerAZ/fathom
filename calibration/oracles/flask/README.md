# Flask route oracle

`flask_routes.py` is the Flask oracle for `indexbench route-oracle` (LB-F335, LB-F336; shape in
`calibration/oracles/routes/README.md`). Tier A: it builds the subject's application with its own
factory and reads Werkzeug's URL map, `app.url_map`, the table Flask matches every request against
once every blueprint and flask-restx namespace is registered.

| Reads | Tier | Emits |
|---|---|---|
| `app.url_map.iter_rules()` and `app.view_functions` | A | method, path, handler |

## What is Flask's

Every rule, its methods and its endpoint's view function. A class-based view (a `MethodView`, a
flask-restx `Resource`) is one row per HTTP method the class defines, `Class.method`; a function view
is its name.

## What is not

The scope (a view under site-packages, such as Flask's own static route, is the framework's); the
spelling of `<converter:name>` as `{name}`; HEAD beside GET and Flask's automatic OPTIONS dropped. The
factory's own stdout is sent to stderr so the oracle's stdout stays JSON.

## Run (2026-10-09, CPython 3.11.15 from uv, a scratch virtual environment)

```sh
git clone --depth 1 --branch 3.8.8 https://github.com/CTFd/CTFd.git ctfd
uv venv --python 3.11 venv && VIRTUAL_ENV=venv uv pip install -r ctfd/requirements.txt
DATABASE_URL=sqlite:///scratch/ctfd.db SECRET_KEY=x UPLOAD_FOLDER=scratch/uploads \
  venv/bin/python -I flask_routes.py ctfd ctfd CTFd:create_app CTFd/CTFd "$(git -C ctfd rev-parse HEAD)" Apache-2.0 > ctfd.json
```

ctfd.json: Flask 2.1.3, 221 routes; 4 framework routes left out. The factory creates and migrates
the scratch SQLite database.
