# Conjure dashboard validity packet

Private first-party source preservation, not a consumer commit or kit API promotion.
Consumer baseline: `e9d0f2e5d54a3ffd3aca9c548642849e0df274de` on `poc` in
`/Volumes/m2ext/code/geoffmilleraz/conjure`, initially clean.

`dashboard-validity.patch` SHA-256:
`a03e395bbff47d9fcb405cef2d31f95177cca9977fd9edcc33241aa90bc51e16`.
It contains only these 13 paths:

- `internal/ui/dashboard.templ` and its generated `dashboard_templ.go`;
- `internal/webui/dashboard.go`, `dashboard_test.go`, new `dashboard_validity_test.go`,
  and `testdata/golden/dash_degraded.html`;
- `PRODUCT.md`, `docs/app-kit.md`, `AGENTS.md`, `CLAUDE.md`, `CHANGELOG.md`,
  `docs/intent/README.md`, `docs/intent/incidents.md`.

The patch is **already applied in the active Conjure working tree**. Do not apply it
again there. At its original landing, reverse-check passed against that result and
forward cached-check passed with a private temporary index at the exact baseline.
The active result now also includes the incremental browser follow-up below;
reverse that increment first only in an isolated copy when checking the old patch.
No active consumer index/commit/push changed. For later replay,
first review the consumer's current intent and dirt, then check applicability in
an isolated exact-baseline checkout; do not overwrite later work.

Unchanged dependencies/runtime assets, SHA-256:

| Path | Digest |
|---|---|
| `go.mod` | `fe9858cba21b055677a132bbd3f1304404361e59b244befdca609f6755928e30` |
| `go.sum` | `8fa279a76a1a9c55cad4c05cdda01148c42ce5552eaf40322fd17f37fb3719bf` |
| `internal/webui/ui/app.css` | `8d6dcbe030ad6cbb6aac1849da59a4c7215ee7f6346ff0e6e2b155a73c2cc17d` |
| `internal/webui/assets/datastar.js` | `dc1a02369774df324bea65cc7c12aace29d62cee9f53484ef0bcd10a07eb9a85` |

From the candidate's Conjure root:

```sh
go run github.com/a-h/templ/cmd/templ@v0.3.1020 generate -f internal/ui/dashboard.templ
go test -race ./internal/webui ./internal/ui -count=1
go vet ./internal/webui ./internal/ui
git diff --check
```

All passed; do not use the globally installed older templ or `@latest`. No real
Conjure commands are required. The [assessment](../../../2026-09-21-conjure-dashboard-validity.md)
records the original failures, review, proof limits and next bounded tasks.

## Incremental browser follow-up (2026-09-22)

Apply `dashboard-validity.patch` first, then `browser-followup.patch`, to the exact
baseline in an isolated checkout. Both are already applied in the active worktree.
The second patch contains only the opt-in synthetic
`internal/webui/dashboard_browser_fixture_test.go`, `docs/app-kit.md`,
`docs/intent/incidents.md` (open INC-8) and `CHANGELOG.md`. No production change.

`browser-followup.patch` SHA-256:
`319bd2c8ec7f3d8c62ce016f0c796cf3aea88830a4f696f5ef418bc81dfcfd9f`.

The [protocol](BROWSER.md), [browser harness](browser-smoke.mjs), failed/passing
JSON reports and [result/identity receipt](browser-results.md) preserve the bounded
36-assertion success and the unfixed visual/connection findings. This is not a
consumer commit, whole-dashboard acceptance, G3/G4 promotion or a new shared API.
