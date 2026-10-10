# Express route oracle (tier A, runtime) — wave c2, LB-F328

What Express itself serves: each app is loaded and Express's own router
stack is walked (`dump_routes.js`). Nothing reads source text to find a
route; the source is read only to name a handler the way fathom's TS/JS
dialect names its definition.

| File | Does |
|---|---|
| `dump_routes.js` | loads each entry in a child process, records mount paths and route-registering routers (wrappers on `router`'s `use`/`route` and `application.use`, which only remember), walks the stack from the exported app; routers never reached are walked at "" and marked `mounted: false`; `app.all()` (every method one by one) is ANY |
| `locate.js` | V8's `[[FunctionLocation]]` of a live function through `node:inspector` in-process (bound functions followed) |
| `namer.js` | the TypeScript parser names the function at that location: a declaration, `Class.method`, a module-level `const x = () =>`, `x.y = function` / `X.prototype.y = function` as `y`; an inline argument or object member is `""` |
| `to_route_oracle.mjs` | any of the Node dumps (this, `../nest`, `../fastify`, `../nextjs`) to `indexbench route-oracle` files, one per subject |

Shared by the other Node oracles: `locate.js` and `namer.js` (Fastify),
`to_route_oracle.mjs` (all four).

## Subject and pins

expressjs/express `a419aa7350389a275ab45fde2e849ed1f02449ef` (MIT, Express
5.3.0, router 2.2.0), `examples/*/index.js`: 25 apps, 76 routes. The examples
`require('../..')`, so the checkout's own dependencies are installed into the
scratch checkout; `online` (0.0.1) and `redis` (4.7.1, never connected) for the
two examples that need them.

```sh
S=<scratch>/express   # git clone https://github.com/expressjs/express; git checkout a419aa73
npm install --prefix "$S" --ignore-scripts
npm install --prefix "$S" --ignore-scripts --no-save redis@4 online
npm install --prefix <scratch>/tools typescript@5.9.3
node dump_routes.js --express "$S" --typescript <scratch>/tools/node_modules/typescript \
  --repo "$S" --out express-dump.json $(find "$S/examples" -maxdepth 2 -name index.js | sort)
node to_route_oracle.mjs --in express-dump.json --outdir oracle/express \
  --repo-name expressjs/express --sha a419aa7350389a275ab45fde2e849ed1f02449ef --licence MIT \
  --tool "express router stack walk (calibration/oracles/express/dump_routes.js)" \
  --tool-version 5.3.0 --group entry-dir --groups examples/static-files
# one route-oracle subject per example app: -oracle oracle/express/examples__<app>.json -repo "$S/examples/<app>"
```

Run under the wave's runtime lock; outputs stay out of the repository. The
reading is `docs/bench/readings/routes/node.json`.
