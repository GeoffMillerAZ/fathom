# Fastify route oracle (tier A, runtime) — wave c2, LB-F327

What Fastify itself builds: the app plugin is registered the way the app's
own server entry registers it, and Fastify's `onRoute` hook records every
route with its final url (register prefixes and `@fastify/autoload` folder
prefixes applied by Fastify and autoload), after `ready()`.
`printRoutes()` is kept beside it as Fastify's own count cross-check.

| File | Does |
|---|---|
| `dump_entries.js` | for apps that build their own instance (fastify/fastify's `examples/`): each entry in a child process with the checkout's factory wrapped, so every instance gets the `onRoute` hook and `listen()` becomes `ready()` + print + exit |
| `dump_routes.mjs` | registers the compiled app plugin (`--fp`: wrapped in fastify-plugin, as the demo's `server.ts` does) on a Fastify instance with an `onRoute` hook, `ready()`, `printRoutes()`; handlers located by V8 and named by the TypeScript parser (`../express/locate.js`, `../express/namer.js`) in the compiled file, filed under the source path |

A route whose handler lives in `node_modules` is a library's (swagger-ui,
static files, cors preflight): marked `library: true` and left out of the
oracle file. A HEAD route Fastify adds beside a GET (`exposeHeadRoutes`,
same handler, trailing slash aside) is marked `implicit` and left out, as
Express's implicit HEAD is never in its stack.

## Subject and pins

fastify/demo `5cd560125b3c2f0d42192bc7f493e8e3b9e75e52` (MIT, fastify 5.12.5,
@fastify/autoload 6.5.0): every route is an autoloaded plugin under
`src/routes/` (folder prefixes), 14 routes. Its environment plugin needs the
database settings; no database is reached (knex connects lazily).

```sh
A=<scratch>/fastify-demo   # git clone https://github.com/fastify/demo; git checkout 5cd56012
(cd "$A" && npm install --ignore-scripts && npx tsc -p .)
node dump_routes.mjs --repo "$A" --app "$A/dist/app.js" --typescript <scratch>/tools/node_modules/typescript \
  --fp --service fastify-demo \
  --env "MYSQL_HOST=localhost,MYSQL_PORT=3306,MYSQL_USER=u,MYSQL_PASSWORD=p,MYSQL_DATABASE=d,COOKIE_SECRET=0123456789abcdef0123456789abcdef0123,COOKIE_NAME=s,COOKIE_SECURED=false,RATE_LIMIT_MAX=100,UPLOAD_DIRNAME=uploads,UPLOAD_TASKS_DIRNAME=tasks" \
  --out fastify-dump.json
node ../express/to_route_oracle.mjs --in fastify-dump.json --outdir oracle/fastify \
  --repo-name fastify/demo --sha 5cd560125b3c2f0d42192bc7f493e8e3b9e75e52 --licence MIT \
  --tool "fastify onRoute hook + printRoutes (calibration/oracles/fastify/dump_routes.mjs)" \
  --tool-version 5.12.5 --subject src
# route-oracle: -oracle oracle/fastify/src.json -repo "$A/src"
```

fastify/fastify `ba235fdcd9a83a4c7ccf793f7b2596a8f65389b6` (v5.12.5, MIT),
`examples/*.js`: 8 apps, 12 routes (register prefixes, an inline plugin, a
plugin module, chained shorthand methods). `http2.js` and `https.js` read
certificates the checkout does not carry, `simple.mjs` and
`typescript-server.ts` do not load through the wrapped CommonJS factory: the
dump records them under `errors` and they are not subjects.

```sh
F=<scratch>/fastify   # git clone https://github.com/fastify/fastify; git checkout v5.12.5
(cd "$F" && npm install --ignore-scripts)
node dump_entries.js --fastify "$F" --repo "$F" --typescript <scratch>/tools/node_modules/typescript \
  --out fastify-examples-dump.json $(find "$F/examples" -maxdepth 1 -type f | sort)
node ../express/to_route_oracle.mjs --in fastify-examples-dump.json --outdir oracle/fastify-examples \
  --repo-name fastify/fastify --sha ba235fdcd9a83a4c7ccf793f7b2596a8f65389b6 --licence MIT \
  --tool "fastify onRoute hook on every instance (calibration/oracles/fastify/dump_entries.js)" \
  --tool-version 5.12.5 --group entry
# one route-oracle run per app: -repo "$F/examples" -only <app>.js (use-plugin.js,plugin.js for use-plugin)
```

Run under the wave's runtime lock; outputs stay out of the repository. The
reading is `docs/bench/readings/routes/node.json`.
