"""django_routes.py: the Django (and Django REST framework) route oracle, tier A.

Runs inside the subject's own environment after django.setup(): walks
django.urls.get_resolver().url_patterns, the resolver Django itself matches
requests against, including every include() and every DRF router's generated
patterns, and prints the routes oracle shape (calibration/oracles/routes/README.md).

Usage: DJANGO_SETTINGS_MODULE=<settings> python -I django_routes.py <repo-root> <subject> <sha> <licence> [<import-root>]
(<import-root>, default <repo-root>, is put first on sys.path: -I keeps the
current directory and PYTHONPATH off it.)

Django's, not ours: every pattern, every view callable, a DRF viewset's
method-to-action map (callback.actions, which DRF builds from the router's
routes and the actions the viewset defines). Ours: the scope filter, the
spelling of a pattern as a template (pattern_template below, the same rule the
reader applies), and the method of a plain view. A route is the repository's
when its view is defined in the repository, or when a urls module of the
repository declares it (path('api/schema/', SpectacularAPIView.as_view()) with a
package's view, directly or in a list the module includes); a package's urls
module (include('social_django.urls')) is the package's. A package's pattern
list included by the repository (admin.site.urls) would read as declared; no
subject includes one. Django's
resolver is method-agnostic, so a function or class-based view is ANY with its
function or class name; a viewset route is one row per method it maps, handler
ViewSet.action.
"""
import contextlib
import inspect
import json
import os
import re
import sys

import django
from django.urls import URLPattern, URLResolver, get_resolver

GROUP = re.compile(r"\(\?P<([A-Za-z_][A-Za-z0-9_]*)>")


def regex_template(rx):
    """A Django regex pattern as a path template: anchors dropped, a named
    group {name}, an unnamed group {}, an escaped character itself, an
    optional trailing slash dropped."""
    s = rx
    if s.startswith("^"):
        s = s[1:]
    if s.endswith("\\Z"):
        s = s[:-2]
    if s.endswith("$"):
        s = s[:-1]
    out = []
    i = 0
    while i < len(s):
        c = s[i]
        if c == "(":
            m = GROUP.match(s, i)
            name = m.group(1) if m else ""
            depth = 0
            j = i
            while j < len(s):
                if s[j] == "\\":
                    j += 2
                    continue
                if s[j] == "(":
                    depth += 1
                elif s[j] == ")":
                    depth -= 1
                    if depth == 0:
                        break
                j += 1
            out.append("{" + name + "}")
            i = j + 1
            if i < len(s) and s[i] in "?*+":
                i += 1
            continue
        if c == "\\" and i + 1 < len(s):
            out.append(s[i + 1])
            i += 2
            continue
        if c == "?" and out and out[-1] == "/":
            i += 1
            continue
        out.append(c)
        i += 1
    return "".join(out)


ROUTE_PARAM = re.compile(r"<(?:[A-Za-z_][A-Za-z0-9_]*:)?([A-Za-z_][A-Za-z0-9_]*)>")


def pattern_template(p):
    rx = getattr(p, "_regex", None)
    route = getattr(p, "_route", None)
    if route is not None:
        return ROUTE_PARAM.sub(lambda m: "{" + m.group(1) + "}", str(route))
    return regex_template(str(rx if rx is not None else p))


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


def module_in_repo(urlconf, root):
    """A urlconf given as a module or a dotted name: is its file in the repo?"""
    mod = urlconf
    if isinstance(urlconf, str):
        mod = sys.modules.get(urlconf)
    f = getattr(mod, "__file__", None)
    if not f:
        return False
    f = os.path.realpath(f)
    return f.startswith(root + os.sep) and "site-packages" not in f


def walk(patterns, prefix, root, out, dropped, declared):
    for p in patterns:
        piece = pattern_template(p.pattern)
        if isinstance(p, URLResolver):
            uc = p.urlconf_name
            # A urls module declares what it lists; an included list
            # (include(_patterns), include(router.urls)) is declared where
            # the including module is.
            inner = module_in_repo(uc, root) if not isinstance(uc, (list, tuple)) else declared
            walk(p.url_patterns, prefix + piece, root, out, dropped, inner)
            continue
        if not isinstance(p, URLPattern):
            continue
        path = "/" + (prefix + piece).lstrip("/")
        cb = p.callback
        actions = getattr(cb, "actions", None)
        cls = getattr(cb, "cls", None) or getattr(cb, "view_class", None)
        if actions and cls is not None:
            for method, action in sorted(actions.items()):
                fn = getattr(cls, action, None)
                f, line = where(fn, root) if fn is not None else (None, 0)
                if f is None:
                    f, line = where(cls, root)
                if f is None:
                    dropped["vendor"] += 1
                    continue
                out.append({"method": method.upper(), "path": path, "handler": cls.__name__ + "." + action,
                            "handler_file": f, "handler_line": line})
            continue
        target = cls if cls is not None else cb
        f, line = where(target, root)
        name = cls.__name__ if cls is not None else getattr(cb, "__name__", "")
        if f is None:
            if not declared:
                dropped["vendor"] += 1
                continue
            out.append({"method": "ANY", "path": path, "handler": name})
            continue
        out.append({"method": "ANY", "path": path, "handler": name, "handler_file": f, "handler_line": line})


def main():
    root, subject, sha, licence = os.path.realpath(sys.argv[1]), sys.argv[2], sys.argv[3], sys.argv[4]
    sys.path.insert(0, os.path.realpath(sys.argv[5]) if len(sys.argv) > 5 else root)
    with contextlib.redirect_stdout(sys.stderr):  # a settings module that prints keeps stdout the oracle's
        django.setup()
        patterns = get_resolver().url_patterns
    out, dropped = [], {"vendor": 0}
    walk(patterns, "", root, out, dropped, module_in_repo(get_resolver().urlconf_module, root))
    seen = set()
    uniq = []
    for r in out:
        k = (r["method"], r["path"], r["handler"])
        if k not in seen:
            seen.add(k)
            uniq.append({"method": r["method"], "path": r["path"], "handler": r["handler"]})
    uniq.sort(key=lambda r: (r["path"], r["method"], r["handler"]))
    import rest_framework
    print("django_routes: %d routes; dropped as a package's: %d" % (len(uniq), dropped["vendor"]), file=sys.stderr)
    print(json.dumps({"tool": "django-url-resolver",
                      "tool_version": "django %s, djangorestframework %s" % (django.get_version(), rest_framework.VERSION),
                      "subject": subject, "sha": sha, "licence": licence, "routes": uniq}, indent=2))


if __name__ == "__main__":
    main()
