# Resume as Code: app-owned theme review readiness

Readiness evidence for G1/G4, not consumer adoption or browser certification.
Contract: [local app-owned guide](../design/app-owned-styleguide.md),
[ADR-0029](../adr/0029-named-themes-and-app-owned-style-guides.md) and
[ADR-0031](../adr/0031-local-app-owned-theme-review.md).
RAC's ADR-009/014 preserve its Synthwave identity, shared server-rendered shell,
no Node runtime/build dependency and app-owned domain behavior. Its autonomy policy
requires approval before dependency or compatibility changes. Existing navigation
authorization does not silently authorize adding the editor to its vendor selection.

## Observed state

Inspected `/Volumes/m2ext/code/geoffmilleraz/resume_as_code` at HEAD
`7a6c56a26dd10a7cf80128d23ef13bab90377ae8`, with existing uncommitted changes.
Kit HEAD is `63b3a4f01168b184f91d1e701414445820e24fdc`; its editor is working-tree
source, not part of that committed revision. Neither HEAD identifies all inputs.

| Input | SHA-256 |
|---|---|
| RAC `cmd/serve/web/app.css` | `1b78ece20de6d17fae8ec2b48e67ca06c5d34ab4a8c1f99266840340c9c2487f` |
| RAC `cmd/serve/web/input.css` | `4260888d309de215b686f9ae0293c0c483b3133f3b7dac18e2638f2c7f61c8e0` |
| RAC `cmd/serve/web/build-css.sh` | `c210aeed00827928c03d2f5bb2f0d58ab2eba962a933597f5d1fcabadfa0d8b5` |
| RAC `appkit.adoption.json` | `4868f6f149ed742e8c1e51b9550cdf28bcaac6bce9eb815f2b4fc8f86ada9d73` |
| RAC `vendor/modules.txt` | `d81467319a904a91a6b4a7a579254fc134f4330f83e6098e60c9110adc0d5c3c` |
| Kit `cue/tokens/synthwave/synthwave.cue` | `5f8bb91dac7ada2613e394bfcf49c21622561ef4be6340f6a43210ec7ab91601` |
| Kit `appkit/ui/themes/synthwave/theme.css` | `e9279a9ae97805889767efba30c5a16f800e69559cae6d28ff26b983fef25a8b` |

RAC declares app-kit `v0.0.0` plus `replace ../app-kit`, but builds from vendor.
Its hash-bound selection contains CSS, ApprovalCard, Shell and navigation. The
navigation/shell pair is the reviewed `63b3a4f` subset, not a whole-kit refresh.
`vendor/modules.txt` lists callout/card/dashboard/hitl/kanban/stat/viz only.

Actual default-vendor lookup, through devbox with `GOWORK=off GOPROXY=off`, fails:

```text
cannot find module providing package github.com/geoffmilleraz/app-kit/apps/styleguide: import lookup disabled by -mod=vendor
```

`go list -deps` for the current kit editor, compared against RAC's vendor package
directories, finds exactly two missing package paths: `appkit/ui/tokens` and
`apps/styleguide` within app-kit. All external package directories exist. This is
not compilation proof: notably protobuf is v1.33.0 in the kit closure and v1.34.2
in RAC. Preserve RAC's versions and prove compilation; do not infer compatibility
from directory presence or refresh unrelated dependencies.

## CSS and identity gaps

RAC's CSS already emits `font-sans`/`font-mono` through `--ak-font-*`, and
`rounded-md/lg/xl/full` through `--ak-radius-*`. Sampled `bg-surface`/`text-ink`
utilities use semantic color variables. The guide appends candidate CSS after
application CSS. These are inspected binding contracts, not computed-style proof.

- `.rounded` is a literal `.25rem`; the pipeline swatch uses it. `.rounded-sm`
  is absent from this compiled asset. Changing the markup requires a CSS rebuild,
  not assuming an ungenerated selector exists. This is the concrete radius-binding
  gap to resolve under ADR-0029's incomplete-switch revisit condition.
- Shell SVG fills, gradients, glow, scrollbar and palette backdrop include literal
  colors. Preserve the established identity; explicitly separate these brand
  effects from semantic theme coverage. Do not claim all pixels follow palette edits.
- `input.css` imports the live sibling Synthwave CSS and has automatic source
  discovery enabled. `Taskfile.yml` still names nonexistent `web/theme.css` as an
  incremental input instead of that imported source. A rebuild is not revision-closed.
  Use the [delivery rules](../design/adoption.md#release-and-measurement) and the
  [Fathom lesson](2026-09-14-stat-reflow.md), not a duplicate unpinned compiler path.

## Fresh synthetic fixture evidence

The existing opt-in `TestExportAppkitPreview` passed through devbox, `GOWORK=off
GOPROXY=off`, on the current RAC worktree: test 0.01s, package 0.577s.
It called actual Today/Pipeline/People GET handlers through `httptest`, with five
fictional states and temporary registry/firm/people stores. It emitted 15 inert
HTML documents and five identical CSS copies. No listener, poller or provider ran;
no normal application routes, source files, dependency pins or real data changed.

Local output:
`/var/folders/2r/2zrkv4fd549bnlc5vwfzd4b00000gn/T/appkit-rac-theme.6p8zjF3yuW`.
Populated Today SHA-256:
`95aa89fa54e97a66de992440e63281728c9b9149d284743d6f03c65180eaf3b7`;
Pipeline: `5f32720953af6c3601e3fc352dad98edfd5f00b7ef27aecb7c2a2df962ebb3c8`;
People: `92e489697d75e7683f12e94e4885f4f16cc7865b22652826d684efc6313d1e04`.
The test uses existing time-dependent app rendering; these are observed bytes, not
a promise that a later day's exports have identical HTML. CSS matches the hash above.
No browser or full-app test was run in this readiness pass.

## Authorized integration and follow-up

The following scope was subsequently implemented with bounded evidence in
[RAC theme-guide adoption](2026-09-15-resume-theme-guide.md). This document retains
the pre-integration findings, not the current vendor/CSS availability claim.

Owner approval received 2026-09-15 for the scope below, including synthetic tests and
changelog notes. This permission does not establish adoption or broaden the exclusions.

1. Add an explicit local-development entrypoint, separate from ordinary `racli serve`
   and hosted routes. No implicit listener or mount. Reuse `apps/styleguide`; do not
   copy its editor or invent a settings persistence backend.
2. Supply app-owned, deterministic, synthetic recipe/state callbacks using existing
   renderers. Today, Pipeline and People are the first real tasks; empty/incomplete
   states matter as much as populated views. Current safe fixtures are test-private:
   deliberately share the render-only seam rather than copying fixtures or exposing
   the full `Server.Handler` as a preview endpoint. Preserve app domain ownership.
3. Vendor only the editor/token source and embed closure required by this entrypoint,
   with exact reviewed identities, source CUE and provenance. Keep ordinary runtime
   independent of the guide. Preserve third-party versions and unrelated vendor work.
4. Make CSS regeneration use pinned local sources and the verified standalone binary;
   explicit source closure, complete incremental inputs, no sibling requirement and
   no unrelated-file influence. Preserve default appearance and runtime token aliases.
5. Select real rendered contrast pairs. Prove candidate/source/reset/stale-export
   tasks with native forms and keyboard at narrow/wide sizes, plus actual computed
   font/radius/color changes. Preserve declared brand exceptions, opaque static
   sandbox, no font/network fetches, no app preference writes and reviewed export.
6. Rebuild twice outside sibling checkouts; prove missing-artifact and stale-CSS
   refusal. Update RAC's selection/change record and return any shared defect to
   the kit. A working-tree rehearsal is not a clean committed release.

This is the next real adoption step, not a replacement for G1/G4's completion gates.
No third-party upgrades, consumer commits, deployment, global configuration or real
data access are proposed. Two-consumer editor promotion and active/font previews
remain unearned; do not build them as part of this integration.
