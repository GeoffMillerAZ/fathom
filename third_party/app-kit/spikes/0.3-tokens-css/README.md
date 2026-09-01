# Spike 0.3 — CUE → tokens → Tailwind pipeline · VERDICT

**Status: PASS (with one documented deviation), 2026-06-10.** This spike
graduated directly into kit packages rather than living here as a throwaway —
the roadmap's "permanence" intent, applied early. The code is the verdict:

- `appkit/ui/tokens` — schema + Load + emitters
- `internal/tailwind` — the pinned-binary rig
- `internal/tailwind/conformance_test.go` — the end-to-end gate

## Pass criteria, checked

| Criterion | Result |
|---|---|
| One command regenerates everything | `task gen` (≡ `appkit gen`) — ✅ |
| Second run byte-identical | `task check` runs gen + `git diff --exit-code` — ✅ |
| Invalid token value fails the build with a locatable error | `tokens.Load` returns the CUE diagnostic; tests `TestLoad_RejectsIncompleteSet` / `TestLoad_RejectsBadName` — ✅ (diagnostics name the failing path; usability acceptable) |
| CI regeneration-cleanliness check works | the `task check` step above — ✅ (CI workflow itself lands Phase 1) |
| templ component styled by generated classes | `testdata/fixture/page.templ` + `TestCSSBuild_EndToEnd`: utilities scanned from templ source resolve through `var(--ak-color-*)` — ✅ |

## Deviation (the documented fallback, taken deliberately)

`cue exp gengotypes` was **not** used: the Go types in
`appkit/ui/tokens/tokens.go` are handwritten, with cuecontext validation at
load time as the drift guard — exactly the spike's pre-committed fallback
("handwrite the Go types, keep CUE as the authoring + runtime-validation
layer, keep codegen for CSS/JSON"). Rationale: the token schema is small and
stable; the experimental gengotypes adds churn risk with no payoff at this
size. **Revisit trigger:** the first schema large enough that hand-sync hurts
(likely the `app.cue` manifest in Phase 1) — evaluate gengotypes there before
spreading the handwritten pattern.

ADR-0007 stands unamended: codegen for CSS/JSON emission is in place; the
single-source-of-truth property is preserved by runtime validation + the
regeneration check.

## Additions beyond the spike scope (recorded, not silent)

- Runtime theme switching contract baked into the emitted CSS:
  `data-theme="light"|"dark"` forced schemes, OS-preference fallback when
  unset, `color-scheme` hints, and the `--ak-font-scale` hook for the
  font-size display setting — the foundation for the kit settings surface
  (light/dark/system + text size) every app inherits.
- DTCG export ships dark values under `$extensions["com.appkit.scheme"]`.
