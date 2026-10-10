# Nest route oracle (tier A, runtime) — wave c2, LB-F326

What Nest itself maps: each sample app is compiled and bootstrapped by its
own `main.ts`, and every route Nest's `RouterExplorer` hands the HTTP adapter
is recorded with the final path (global prefix, versions, module paths
applied by Nest's `RoutePathFactory`) and the handler, which Nest 12 names
`Controller.method` before registering it.

| File | Does |
|---|---|
| `dump_routes.mjs` | per sample: copies `src/` (or a second app's folder, `sample:folder`) into a scratch env, compiles it with the TypeScript compiler using the sample's own `tsconfig.json` compilerOptions (emit on type errors; non-TS assets such as `.proto` copied as the Nest CLI's assets option does), runs `dist/main.js` with `preload.mjs`; a sample whose `init()` never finishes (a provider that opens a database) is run again in Nest's preview mode, where providers are not instantiated |
| `preload.mjs` | `node --import` hook: wraps the created app's adapter route methods (records method, path, handler name), makes `startAllMicroservices` a no-op and turns `listen` into `init()` + print + exit |

A route whose handler is not `Controller.method` was registered by a library
(`@nestjs/swagger`'s document routes): it is kept in the dump marked
`controller: false` and left out of the oracle files.

## Subject and pins

nestjs/nest `2731c18054fb348779311d81e4e20949e313be78` (MIT, @nestjs/core
12.1.2), `sample/*` with a `@Controller` (28 apps) plus
`sample/38-webhooks:partner-service`, the second app of that sample. The
samples' pinned dependencies (their own `package.json`s, GraphQL-only ones
left out) are installed once into one scratch env:

```sh
E=<scratch>/nest-env   # package.json: the samples' dependencies at their pins, "type": "module"
npm install --prefix "$E" --ignore-scripts --legacy-peer-deps
node dump_routes.mjs --repo <nest checkout> --env "$E" --out nest-dump.json \
  sample/01-cats-app ... sample/38-webhooks sample/38-webhooks:partner-service
node ../express/to_route_oracle.mjs --in nest-dump.json --outdir oracle/nest \
  --repo-name nestjs/nest --sha 2731c18054fb348779311d81e4e20949e313be78 --licence MIT \
  --tool "Nest RouterExplorer via the HTTP adapter (calibration/oracles/nest/dump_routes.mjs)" \
  --tool-version 12.1.2 --group service
# one route-oracle subject per app, the folder the dump compiled: -oracle oracle/nest/sample__<app>.json
#   -repo <nest checkout>/sample/<app>/src (sample/38-webhooks/partner-service for the second app)
```

Run under the wave's runtime lock; outputs stay out of the repository. The
reading is `docs/bench/readings/routes/node.json`.
