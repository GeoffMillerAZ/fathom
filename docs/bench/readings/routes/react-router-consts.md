# React Router routes with imported path constants (wave h3; SG-46, SG-47; O735, O774)

**Verdict, in plain words.** A React Router path written as a constant another file defines
(`path: paths.app.users.path`, `paths` imported from `@/config/paths`) now reaches the route seam:
bulletproof-react's v7 router went from 1 of its 9 routes found to all 9 (route recall .1111 to 1.0,
precision 1.0), and nothing else moved: every one of react-router's 22 example apps reads identically to
`react-router.json`. The handlers of those 9 routes stay unbound (bound recall 0 to .2, 2 of 10): each
is `lazy: () => import('./routes/x').then(convert(queryClient))`, whose component is what a function of the
app's own (`convert`) makes of the module, so no reader of source names it (the written ceiling, O774).
Because those routes now exist with an empty handler where the oracle names one, the pooled bound precision
reads lower than before (.95 to .8899) while pooled bound recall reads higher (.8879 to .9065). That is the
honest trade: found routes with an unbound handler, not missing routes. `react-router.json` stays as
LB-F380 pinned it (.8879 / .95); this file is the same corpus re-read after SG-46.

Measured with `indexbench route-oracle` at fathom a458cef0, 2026-10-10, over the oracles of
`calibration/oracles/react-router/` (`route_oracle.mjs`, tier A: React Router's own route table; the same
recipe and pins as LB-F380: remix-run/react-router `dead4962` examples against react-router-dom 6.30.6,
alan2207/bulletproof-react `9506629e` `apps/react-vite` against react-router 7.0.2). The oracles are
regenerated, not committed (97 example routes, as in LB-F380; bulletproof 10 routes at 9 distinct paths).

| Subject | Route R / P | Bound R / P | Before (react-router.json) |
|---|---|---|---|
| 22 example apps | unchanged in every app | unchanged in every app | identical |
| bulletproof-react apps/react-vite | 1.0 / 1.0 (9 of 9) | .2 / .2 (2 of 10) | .1111 / 1.0, 0 / 0 |
| pooled | 1.0 / .977 | .9065 / .8899 | .9059 / .9747, .8879 / .95 |

Machine reading: `react-router-consts.json` beside this file.
