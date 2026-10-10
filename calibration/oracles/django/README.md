# Django route oracle

`django_routes.py` is the Django and Django REST framework oracle for `indexbench route-oracle`
(LB-F333, LB-F336; shape in `calibration/oracles/routes/README.md`). Tier A: after `django.setup()`
it walks `django.urls.get_resolver().url_patterns`, the resolver Django matches requests against,
through every `include()` and every DRF router's generated patterns.

| Reads | Tier | Emits |
|---|---|---|
| Django's root URL resolver, recursively; a DRF view's `actions` map | A | method, path, handler |

## What is Django's

Every pattern and every view callable, and a DRF viewset route's method-to-action map
(`callback.actions`, which DRF builds from the router's routes and the actions the viewset has).

## What is not

- The method of a plain view: Django's resolver is method-agnostic, so a function or class-based
  view is `ANY`, handler its function or class name. A viewset route is one row per mapped method,
  `ViewSet.action`.
- The spelling of a pattern as a template (`pattern_template`): a `path()` converter `<int:pk>` is
  `{pk}`; a regex has its anchors dropped, a named group `{name}`, an unnamed one `{}`, an escaped
  character itself and an optional slash dropped — the same rule fathom's reader applies.
- The scope: a route is the repository's when its view is defined in the repository, or when a URL
  module of the repository declares it (directly, or in a list it includes). A package's URL module
  (`include('social_django.urls')`) is the package's, left out and counted on stderr.

## Run (2026-10-09, CPython 3.13.13 from uv, a scratch virtual environment)

```sh
git clone --depth 1 --branch v4.7.2 https://github.com/netbox-community/netbox.git netbox
uv venv --python 3.13 venv && sed 's/psycopg\[c,pool\]/psycopg[binary,pool]/' netbox/requirements.txt > req.txt
VIRTUAL_ENV=venv uv pip install -r req.txt
cp netbox/netbox/netbox/configuration_example.py netbox/netbox/netbox/configuration.py
#   then ALLOWED_HOSTS = ['*'] and a SECRET_KEY of 50+ characters; no database is opened
DJANGO_SETTINGS_MODULE=netbox.settings venv/bin/python -I django_routes.py netbox \
  netbox-community/netbox "$(git -C netbox rev-parse HEAD)" Apache-2.0 netbox/netbox > netbox.json
```

netbox.json: Django 6.1.1, djangorestframework 3.18.0, 4,050 routes; 4 a package's left out.
