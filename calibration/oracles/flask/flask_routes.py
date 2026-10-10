"""flask_routes.py: the Flask route oracle, tier A.

Builds the subject's application with its own factory and reads Werkzeug's URL
map, app.url_map, the table Flask matches every request against after every
blueprint (and flask-restx namespace) is registered. Prints the routes oracle
shape (calibration/oracles/routes/README.md).

Usage: python -I flask_routes.py <repo-root> <import-root> <module:factory-or-attr> <subject> <sha> <licence>

Flask's, not ours: every rule, its methods and its endpoint's view function.
Ours: the scope filter (a view defined under site-packages, such as Flask's own
static route, is the framework's), and the spelling: HEAD dropped beside GET,
an automatic OPTIONS dropped, a rule's <converter:name> as {name}, the handler
the view function's name, or Class.method for a class-based view (MethodView,
a flask-restx Resource), one row per method.
"""
import contextlib
import importlib
import inspect
import json
import os
import re
import sys

PARAM = re.compile(r"<(?:[^:<>]+:)?([A-Za-z_][A-Za-z0-9_]*)>")


def where(obj, root):
    try:
        obj = inspect.unwrap(obj)
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
    root = os.path.realpath(root)
    sys.path.insert(0, os.path.realpath(import_root))
    mod, attr = target.split(":")
    with contextlib.redirect_stdout(sys.stderr):  # a factory that prints keeps stdout the oracle's
        obj = getattr(importlib.import_module(mod), attr)
        app = obj() if callable(obj) and not hasattr(obj, "url_map") else obj
    import flask

    out, dropped = [], {"vendor": 0}
    for rule in app.url_map.iter_rules():
        view = app.view_functions.get(rule.endpoint)
        if view is None:
            dropped["vendor"] += 1
            continue
        methods = set(rule.methods or [])
        if "GET" in methods:
            methods.discard("HEAD")
        if getattr(rule, "provide_automatic_options", False) or getattr(view, "provide_automatic_options", None) is not False:
            methods.discard("OPTIONS")
        path = PARAM.sub(lambda m: "{" + m.group(1) + "}", rule.rule)
        cls = getattr(view, "view_class", None)
        for m in sorted(methods):
            if cls is not None:
                fn = getattr(cls, m.lower(), None)
                if fn is None:
                    continue
                f, line = where(fn, root)
                handler = cls.__name__ + "." + m.lower()
            else:
                f, line = where(view, root)
                handler = inspect.unwrap(view).__name__
            if f is None:
                dropped["vendor"] += 1
                continue
            out.append({"method": m, "path": path, "handler": handler})
    out = [dict(t) for t in sorted({tuple(sorted(r.items())) for r in out})]
    out.sort(key=lambda r: (r["path"], r["method"], r["handler"]))
    print("flask_routes: %d routes; dropped as the framework's: %d" % (len(out), dropped["vendor"]), file=sys.stderr)
    print(json.dumps({"tool": "flask-url-map", "tool_version": "flask %s" % getattr(flask, "__version__", "?"),
                      "subject": subject, "sha": sha, "licence": licence, "routes": out}, indent=2))


if __name__ == "__main__":
    main()
