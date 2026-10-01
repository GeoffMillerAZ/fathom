# scip-python — tier-A Python index (calls, def_mapping, typed_recall, file edges)

Installed without a global install, into a tools dir:

    npm install --prefix <tools>/scip-python @sourcegraph/scip-python    # 0.6.6, node v26.3.0

`run.sh` makes a virtual environment holding the subject's dependencies (uv 0.11.19, CPython 3.13.5),
never the subject package itself, and runs scip-python with that environment first on PATH:

    run.sh <tools>/scip-python/node_modules/.bin/scip-python <clone>/fastapi out/python-fastapi.scip <scratch>/venv-fastapi \
      -- -r requirements-tests.txt                                    # fastapi 0.119.1: 77 packages (starlette 0.48.0,
                                                                      # pydantic 2.13.5, httpx 0.28.1, sqlmodel 0.0.27, flask 3.1.3)
    run.sh <tools>/scip-python/node_modules/.bin/scip-python <clone>/flask out/python-flask.scip <scratch>/venv-flask \
      -- blinker click itsdangerous jinja2 markupsafe 'werkzeug>=3.1.0' asgiref python-dotenv pytest
                                                                      # flask 3.1.3: werkzeug 3.1.9, click 8.5.0, jinja2 3.1.6,
                                                                      # pytest 9.1.1 (14 packages)

(`-r requirements-tests.txt` installs fastapi itself editable through `-e .[all]`; run.sh uninstalls the
subject's own distribution afterwards, so its package resolves from the clone.) fastapi indexes 1,175
documents in 13.6 s, flask 83 in 5.8 s (2026-10-01, wave lb).

Why the environment matters (O263): with no dependency installed, scip-python names no symbol for a call
on a receiver typed by another distribution (`client.get` on starlette's TestClient, a pydantic model's
methods), and treats an import it cannot resolve as a local. The 2026-09-28 reading took its index that
way: call_precision read .2782 on fastapi + flask; the same fathom commit against the environment-aware
index reads .6123, before any extraction change.

What the index carries, read by `langbench.SCIPOptionsFor("python")` (LB-F65): a constructor call names
the class (`Foo#`), never a constructor, so a class written as a call (`Foo(`) is a call to `Foo`; an
attribute or a local called (`self.handler(x)`, `cb()`) is likewise a call; a function or method referenced
without a call (`Depends(get_db)`, `@lru_cache`) is no call site. Both are counted in the reading
(`scip_called_names`, `scip_calls_unwritten`) and disclosed in its notes. A decorated definition's
`enclosing_range` starts at its first decorator, and a module-level assignment's covers the statement:
the dialect gives those calls the same owners (`@calls.to`/`@calls.from`, tags.scm).

What it cannot resolve: a call on an unannotated parameter (a pytest fixture's `client.get(...)`, 1,191 of
flask's 1,355 fathom-only pairs are in tests/), and a value read out of a dict (`schema["x"].split()`).
Those are verified oracle deficiencies, disclosed in the reading.
