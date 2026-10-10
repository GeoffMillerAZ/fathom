"""fastapi_routes.py: the FastAPI route oracle, tier A.

Imports the subject's application object and reads FastAPI's own route table:
app.routes after every include_router has been applied (the table app.openapi()
is generated from), and prints the routes oracle shape
(calibration/oracles/routes/README.md). app.openapi()'s operation count is
printed beside it as a cross-check (a route with include_in_schema=False is in
the table but not in the schema).

Usage: python -I fastapi_routes.py <repo-root> <import-root> <module:attr> <subject> <sha> <licence> [--strip-prefix=/p]

--strip-prefix drops a configured root from every path, the precedent of the
Java converters' -strip-prefix: full-stack-fastapi-template includes its API
router under prefix=settings.API_V1_STR, a pydantic setting ("/api/v1" by
default, overridable from the environment), which no source reader resolves;
the score then counts the part the source declares, and the reading names the
stripped root as a ceiling.

FastAPI's, not ours: every path (prefixes already joined by include_router),
every method set, every endpoint function. Ours: the scope filter (an endpoint
defined under site-packages, such as the docs and openapi.json routes FastAPI
adds itself, is the framework's) and the spelling (HEAD dropped beside GET;
handler the endpoint function's name, Class.method for a method).
"""
import contextlib
import importlib
import inspect
import json
import os
import sys


def where(obj, root):
    try:
        f = inspect.getsourcefile(obj)
        line = inspect.getsourcelines(obj)[1]
    except (TypeError, OSError):
        return None, 0
    if not f:
        return None, 0
    f = os.path.realpath(f)
    if not f.startswith(root + os.sep) or "site-packages" in f:
        return None, 0
    return os.path.relpath(f, root), line


def main():
    root, import_root, target, subject, sha, licence = sys.argv[1:7]
    strip = ""
    for a in sys.argv[7:]:
        if a.startswith("--strip-prefix="):
            strip = a.split("=", 1)[1].rstrip("/")
    root = os.path.realpath(root)
    sys.path.insert(0, os.path.realpath(import_root))
    mod, attr = target.split(":")
    with contextlib.redirect_stdout(sys.stderr):  # an import that prints keeps stdout the oracle's
        app = getattr(importlib.import_module(mod), attr)
    from fastapi.routing import APIRoute, APIWebSocketRoute
    import fastapi

    # FastAPI 0.14x keeps an included router as one lazy entry of app.routes;
    # iter_route_contexts is the walk app.openapi() itself makes over them.
    from fastapi import routing
    walk = routing.iter_route_contexts(app.routes) if hasattr(routing, "iter_route_contexts") else app.routes
    out, dropped = [], {"vendor": 0, "not_api_route": 0}
    for r in walk:
        orig = getattr(r, "original_route", r)
        if isinstance(orig, APIWebSocketRoute):
            methods = ["WS"]
        elif isinstance(orig, APIRoute):
            methods = sorted(r.methods)
            if "GET" in methods and "HEAD" in methods:
                methods.remove("HEAD")
        else:
            dropped["not_api_route"] += 1
            continue
        ep = r.endpoint
        f, line = where(ep, root)
        if f is None:
            dropped["vendor"] += 1
            continue
        handler = ep.__qualname__.replace(".<locals>.", ".")
        path = r.path
        if strip and (path == strip or path.startswith(strip + "/")):
            path = path[len(strip):] or "/"
        for m in methods:
            out.append({"method": m, "path": path, "handler": handler})
    out = [dict(t) for t in sorted({tuple(sorted(r.items())) for r in out})]
    out.sort(key=lambda r: (r["path"], r["method"], r["handler"]))
    ops = sum(len([k for k in v if k in ("get", "put", "post", "delete", "options", "head", "patch", "trace")])
              for v in app.openapi().get("paths", {}).values())
    print("fastapi_routes: %d routes; app.openapi() operations: %d; dropped: %s" % (len(out), ops, dropped), file=sys.stderr)
    version = "fastapi %s" % fastapi.__version__
    if strip:
        version += ", configured root %s stripped" % strip
    print(json.dumps({"tool": "fastapi-routes", "tool_version": version,
                      "subject": subject, "sha": sha, "licence": licence, "routes": out}, indent=2))


if __name__ == "__main__":
    main()
