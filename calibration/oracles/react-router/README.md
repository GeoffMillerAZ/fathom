# React Router route oracle (tier A) — wave c3, LB-F380

What React Router itself is handed. `route_oracle.mjs` bundles an app's route module with esbuild, with the
React Router package it imports replaced by a thin shim over the real one: every route array the app passes to
`createBrowserRouter`, `createHashRouter`, `createMemoryRouter`, `createStaticHandler`, `useRoutes` or
`createRoutesFromElements` is recorded, and `<Routes>` records the route objects React Router's own
`createRoutesFromChildren` makes of its children, with the pathname base of the route it renders under. The
bundle runs in Node: the module is imported, a router factory named by `--call` is called, and a
default-exported component is rendered (streamed to completion, so a `React.lazy` inside `Suspense` loads) at
`/` in a `MemoryRouter`, then at every path found, until none is new (descendant routes). The recorded route
objects are walked as React Router joins them, and each `lazy()` is called.

A route is `GET` (a client route is a page view, as a Next.js page is) at its joined path when it has a path or
is an index route and renders something. Its handler is the component it renders: the element's type (through
an element whose only child is one component element), `Component`, or what `lazy()` resolves to, named by the
function's own name (through `React.lazy`, `memo` and `forwardRef`).

| File | Does |
|---|---|
| `route_oracle.mjs` | the dump: `{tool, tool_version, routes: [{method, path, handler, entry}]}` |

`calibration/oracles/express/to_route_oracle.mjs --group entry` turns dumps into the route-oracle files
(`calibration/oracles/routes/README.md`).

## Subjects and pins

- remix-run/react-router `react-router@6.30.6` (`dead4962`, MIT), `examples/`: 22 apps (multi-app's two), each
  its own subject, against react-router-dom 6.30.6. `view-transitions` does not build against that release (it
  imports `unstable_useViewTransitionState`, which 6.30.6 does not export) and is left out; `ssr-data-router`
  is read through `src/entry.client.tsx` with `--dom 1` (an inert document and window).
- alan2207/bulletproof-react `9506629e` (MIT), `apps/react-vite`, against its own react-router 7.0.2:
  `--entry src/app/router.tsx --call createAppRouter --alias react-router --env-file .env.example`.

```sh
T=<scratch>/npm   # npm --prefix $T install esbuild react@18 react-dom@18 react-router-dom@6.30 @reach/dialog @reach/visually-hidden jsurl localforage
for app in <react-router>/examples/*; do
  node route_oracle.mjs --tools $T --rr $T/node_modules/react-router-dom --app $app --entry src/App.tsx \
    --repo <react-router> --work <scratch>/rr-$(basename $app) --out <scratch>/rr/$(basename $app).json
done
node ../express/to_route_oracle.mjs --in <merged dump> --outdir oracle/react-router --repo-name remix-run/react-router \
  --sha dead4962d6c33e962f266af910b7a09e3ece30cf --licence MIT \
  --tool "react-router route objects (calibration/oracles/react-router/route_oracle.mjs)" --tool-version "react-router-dom 6.30.6" --group entry
indexbench route-oracle -oracle oracle/react-router/examples__basic.json -repo <react-router>/examples/basic ... \
  -out docs/bench/readings/routes/react-router.json
```

The entry is each app's `src/App.tsx` (or `app.tsx`, `app.jsx`; multi-app's `home/App.jsx` and `inbox/App.jsx`).
Run under the wave's runtime lock and the ingest lock (route-oracle ingests each app into a scratch store);
outputs stay out of the repository.
