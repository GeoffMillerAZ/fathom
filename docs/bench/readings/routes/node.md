# Node framework routes — the reading (wave c2, item node; LB-F326..LB-F330)

**Verdict, in plain words.** fathom now reads the HTTP routes of Nest, Fastify and Next.js apps the
way the frameworks themselves serve them, and names each route's handler with the same name the
index gives that function, so a route binds to its code: every Nest, Fastify and Next.js route the
running framework reported was found, at the right path, with the right handler, and nothing else was
claimed (bound recall and precision 1.0 on 29 Nest apps, 9 Fastify apps and 2 Next.js apps, 125
routes). Express reads 1.0 precision but .763 recall: the 18 routes it misses all sit in three example
apps that register routes through helpers of their own (a controller loader, a route map, a resource
helper), which no reader of source can follow without running the app; that is the written ceiling.
Before this item, fathom read Node routes with one line regex: it found 0 of 78 Nest routes, 3 of 26
Fastify routes, 1 of 21 Next.js routes and none of them with a bindable handler, and invented routes
from client code (`GET /@undefinedImage.all` in the OpenTelemetry frontend) and from `app.use()`.
Keep the readers; retire the regex (done). Pair them with item java's route seam (mounts compose
across files) and with route-oracle, the runtime oracle that measures them.

Measured with `indexbench route-oracle` (item java, LB-F324) against tier-A oracles that read each
framework's own route table, never source text (`calibration/oracles/{express,fastify,nest,nextjs}/`).
Bar: bound recall and precision at least .90 per framework, the same bar as item java's Java reading.
Machine reading: `node.json` beside this file.

| Framework | Corpus (pinned) | Oracle | Routes | Route R / P | Bound R / P | Verdict |
|---|---|---|---|---|---|---|
| Nest (LB-F326) | nestjs/nest 2731c180 `sample/*`, 29 apps | RouterExplorer via the HTTP adapter, Nest 12.1.2 | 78 | 1.0 / 1.0 | 1.0 / 1.0 | PASS |
| Fastify (LB-F327) | fastify/demo 5cd56012 `src/` (autoload); fastify v5.12.5 `examples/`, 8 apps | `onRoute` hook after `ready()`, Fastify 5.12.5 | 26 | 1.0 / 1.0 | 1.0 / 1.0 | PASS |
| Express (LB-F328) | expressjs/express a419aa73 `examples/`, 25 apps | router stack walk, Express 5.3.0 | 76 | .7632 / 1.0 | .7632 / 1.0 | FAIL: ceiling |
| Next.js (LB-F329) | opentelemetry-demo e5552ee2 `src/frontend` (pages/); next-learn bb255844 `dashboard/final-example` (app/) | `next build` manifests, Next 16.3.8 / 16.0.10 | 21 | 1.0 / 1.0 | 1.0 / 1.0 | PASS |

Every handler fathom names is a symbol the same ingest indexed: 82 of 82 Nest (`Controller.method`),
21 of 21 Next.js (the page component, the route handler's method function), 8 of 8 Express (the
exported function behind `user.list`). Inline arrow handlers bind to `""` on both sides and are
counted (all 26 Fastify routes, most Express ones).

## Before and after the readers, with and without mounts

| Framework | express-regex (c2-seam 6c738a22) route R / P, bound R | Without mounts: bound R / P | With mounts: bound R / P |
|---|---|---|---|
| Nest | 0 / 0, 0 | .9872 / .9872 | 1.0 / 1.0 |
| Fastify | .1154 / .20, .0385 | 1.0 / 1.0 | 1.0 / 1.0 |
| Express | .6974 / .8154, 0 | .7105 / .9818 | .7632 / 1.0 |
| Next.js | .0476 / .125, 0 | 1.0 / 1.0 | 1.0 / 1.0 |

"Without mounts" is the same commit with `Compose` given no mounts (a scratch build): what the
readers resolve inside one file (an Express router mounted in its own file, a Fastify inline plugin's
prefix, and at wave c2 autoload folder prefixes) still applies. Since wave lf2 autoload's folder
prefixes are a Mount (`/**?dirs`, LB-F351, LB-F352), so a no-mounts build would now read fastify/demo
at its plugins' local paths; that column is wave c2's. Mounts carry Nest's global prefix down the module tree
(`sample/24-serve-static`, `/api`) and Express's cross-file `app.use('/api/v1', require('./controllers/api_v1'))`
(`multi-router`). These corpora exercise few cross-file mounts; the fusion ingest test
(`TestIngest_NodeFrameworkRoutes`) proves the Express mount, the Fastify register prefix and the Nest
global prefix through an ingest and a cross-service join. fauxtown has no JS/TS route: its six Go routes
read identically before and after (the links golden is unchanged).

## Where fathom misses (the Express ceiling)

`examples/mvc` (9 routes) loads controllers from a directory at runtime and derives each route from
the export's name (`show` is `GET /<name>/:<name>_id`, `lib/boot.js`); `examples/route-map` (5) walks a
nested object with `app.map`, a helper defined in the example; `examples/resource` (4 of 5) registers
through `app.resource('/users', User)`, another such helper. Each computes the method and path at
runtime (`app[method](path, fn)`). The other 22 apps read 1.0 / 1.0, `static-files` with no route on
either side. Trigger to revisit: a corpus of
production Express apps whose recall falls under .90 for a pattern a reader can see (a route table
array looped into `router.use(r.path, r.route)`, the most common shape the reader does not follow), or
a same-file helper evaluated with its literal arguments.

## What is not measured here

- Nest URI versioning (`@Version`, `{version}`), `RouterModule` paths and a global prefix's `exclude`:
  no sample uses them. The reader writes the version segment and RouterModule mounts (unit tests); it
  cannot apply `enableVersioning({defaultVersion})` or `exclude`, which live in `main.ts` and would need
  the seam to carry them (O695). Wave lf2 left them unbuilt on that evidence: across nestjs/nest
  2731c180 `sample/*`, one `main.ts` calls `setGlobalPrefix('api')` (24-serve-static), with no
  `exclude`, and none calls `enableVersioning`; only the framework's own e2e specs do.
- Fastify `register(module, {prefix})` across files: only the unit and ingest tests exercise it.
- Express `use()` of a module-level route table, Koa, Hono and restify: not read (no reader).

## Cost

On nestjs/nest (2,093 JS/TS files, 5.09 MB) the four readers take 0.11 s together (Express 0.063,
Nest 0.032, Fastify 0.018, Next.js 0.001; byte prefilters first). A scratch ingest of the repo took
4.92 and 4.88 s before and 4.75 and 4.89 s after: the difference is inside the run-to-run spread. The
repo's composed routes went from 82 (regex) to 313.

## Re-read in wave lf2 (LB-F355): the same routes, by a differential

Wave lf2 moved autoload's folder prefixes from the Fastify reader into a recursive directory Mount
(LB-F351, LB-F352) and changed no other Node reader. The Node oracles are not committed (only their
converters are, O682), so the re-read compares fathom with itself instead of re-running each framework:
`indexbench route-oracle` built from origin/main `784657b9` and from `284b658d`, both with the 25-entry
miss cap lifted in a scratch copy, scored every pinned subject against an empty oracle, so each
subject's extra list is fathom's whole bound route set (method, normalized path, handler). On all 68
subjects (every nestjs/nest `sample/*` app and `38-webhooks/partner-service`, fastify/demo `src/`,
fastify `examples/` whole and per app as wave c2 ran it, every express `examples/*` app, both Next.js
apps), the sets are identical, 177 bound routes, with the same handler counts: fastify/demo's 14 and
the examples' 12 among them. Equal sets score equally against wave c2's oracles, so every Node number
above holds at `284b658d`.
