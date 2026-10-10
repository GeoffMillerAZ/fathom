# Next.js route oracle (tier A) — wave c2, LB-F329

What Next itself compiled: `next build --experimental-build-mode=compile`
(compile only, so no page is prerendered and no data source is needed), then
the build's own manifests.

| File | Does |
|---|---|
| `manifest_routes.mjs` | reads `.next/routes-manifest.json` (every route Next serves), `app-paths-manifest.json` (each app/ entry's source path) and `server/pages-manifest.json`; a page is GET, a `pages/api` route ANY, an app/ route handler one route per exported HTTP-method function; handlers named from the source by the TypeScript parser (the default export's name, a wrapper's innermost named argument, the method function); `[x]`, `[...x]`, `[[...x]]` are `{}` |

Next's own entries that are not source routes (`_app`, `_document`,
`_error`, `404`, `500`, `/_not-found`, metadata files such as
`/favicon.ico`) are listed under `excluded` with a reason.

## Subjects and pins

- open-telemetry/opentelemetry-demo `e5552ee25bcede2900d89188f076c464cd73c074`
  (Apache-2.0), `src/frontend` (Next 16.3.8, pages router, its own
  `package-lock.json`).
- vercel/next-learn `bb2558441a6673ab76c89914c25018bffa27a2ba` (MIT),
  `dashboard/final-example` (app router: route groups, dynamic segments, route
  handlers; Next 16.0.10 and React 19.2.1, the versions its `pnpm-lock.yaml`
  resolves `latest` to).

```sh
F=<scratch>/opentelemetry-demo/src/frontend
(cd "$F" && npm ci --ignore-scripts && npx next build --experimental-build-mode=compile)
node manifest_routes.mjs --app "$F" --repo <scratch>/opentelemetry-demo \
  --typescript <scratch>/tools/node_modules/typescript --service src/frontend --out next-otel-dump.json
D=<scratch>/next-learn/dashboard/final-example
(cd "$D" && npm install --ignore-scripts --no-package-lock next@16.0.10 react@19.2.1 react-dom@19.2.1 \
   && npx next build --experimental-build-mode=compile)
node manifest_routes.mjs --app "$D" --repo <scratch>/next-learn \
  --typescript <scratch>/tools/node_modules/typescript --service dashboard/final-example --out next-learn-dump.json
node ../express/to_route_oracle.mjs --in next-otel-dump.json --outdir oracle/nextjs \
  --repo-name open-telemetry/opentelemetry-demo --sha e5552ee2... --licence Apache-2.0 \
  --tool "next build manifests (calibration/oracles/nextjs/manifest_routes.mjs)" --tool-version 16.3.8 --group service
# route-oracle: -oracle oracle/nextjs/src__frontend.json -repo "$F" (and the next-learn pair alike)
```

Run under the wave's runtime lock; outputs stay out of the repository. The
reading is `docs/bench/readings/routes/node.json`.
