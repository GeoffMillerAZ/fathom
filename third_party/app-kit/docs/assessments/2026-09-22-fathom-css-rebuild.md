# Frozen Fathom candidate: actual CSS rebuild

## Prospective contract

R5's [review packet](fixtures/adoption-analytics-2026-09-21/fathom/README.md)
explicitly lacks a real pinned-compiler CSS rebuild. Its fake-compiler tests and
later native-export pass cannot prove that output. This bounded prerequisite check
uses a new copy outside active Fathom; it is not integration or a new browser matrix.

- Inputs: the retained `appkit-r5-candidate.7QEdhc/src` CSS source closure, exact
  app-kit mirror `f7ab0fbd3496e9dc3da8c97b6c4290f09a71ec50`, app-owned baseline CUE
  `443461ce94d565b18ea6917631f45fed689a8248839533ec15b2dd328c5267bf`, and the unchanged
  candidate `scripts/ui-css.sh`/cache helper. Freeze inputs before execution.
- Verify every mirrored file against the archived kit revision, allowing only its
  `.vendor-commit` stamp as additional metadata. No symlinks or sibling-source reads.
- Run the actual build once in the disposable source copy, then `--check` once.
  Require the pinned Tailwind v4.3.0 darwin-arm64 checksum, existing offline module
  cache, local Go toolchain, GOWORK=off and isolated build/temp output. Deny network,
  active Fathom/app-kit source reads and writes to the existing compiler/module cache.
- Compare generated token CSS byte-for-byte with the candidate's original theme.
  Light positive must remain `oklch(0.55 0.14 155)`; no substitute kit default.
  Record exact app.css differences, if any, against the previous candidate output.
  Any difference needs explanation, not an automatic default-appearance pass.
- Keep all source/manifests unchanged, apart from the two generated CSS outputs in
  the disposable copy. Verify the original frozen source and compiler remain intact.
  The second build/check must produce identical outputs; neither build is claimed
  to be a cold dependency-closure or full Fathom application build.
- Bound each command to five minutes of observed execution; inspect the same owned
  handle if slow, never restart an unobserved process. Stop at a reproducible failure
  or completed build/check/comparison. No dependency upgrades, active consumer edits,
  daemon restart, real logs, browser acquisition, installation or deployment.

## Result

The first build reached real token generation and Tailwind v4.3.0, then exited 1:
`ui-css: required source is unreadable: cmd/fathom/ui/../observatory.go`.
It published neither generated file and removed its own temporary work/lock.
The same file subsequently reads successfully with unchanged hash
`24d96220370124c1d89b430a719a2479a937f350af904331df9b5b8fa2a664a8`,
both normally and under the unchanged sandbox. No cause is established. One unchanged
retry is admitted after this successful read recheck; a repeated failure stops the
slice. The first log, frozen source and prospective contract remain retained.

The unchanged retry completed (exit 0), and a subsequent real `--check` completed
(exit 0): `ui-css: generated CSS is current`. The first failure is retained, not
reclassified as a pass. The reported Tailwind times (70/66/67 ms) are compiler log
readings, not end-to-end build performance.

| Check | Observed result |
|---|---|
| Mirror identity | All 1,473 mirrored source files match the exact archived kit revision; `.vendor-commit` is the only extra metadata file. |
| Original frozen candidate | All 1,486 selected files still match the pre-execution archive. No original source/output was updated. |
| Disposable copy | Only `cmd/fathom/ui/app.css` differs from the selected input archive. |
| App-owned theme | Both generations reproduce the original 6,753 bytes exactly, including light positive `oklch(0.55 0.14 155)`. |
| Application CSS | 33,159 → 33,879 bytes (+720); only additive utilities/variables, no existing declaration removed or changed. |
| Isolation/cleanup | Network and active-repository reads denied during build; existing module/compiler caches read-only. Commands exited; the script's work/lock directory is empty. |

The CSS additions are three variables (`--container-4xl`, `--text-3xl`,
`--text-3xl--line-height`) and eight selectors: `.mb-6`, `.max-w-4xl`, `.gap-x-8`,
`.gap-y-6`, `.border-y`, `.py-7`, `.text-3xl`, `.sm\:grid-cols-1`. Independent
source review traced them to the mirror's account/settings/widget primitives and
settings browser fixture, which the existing Go-source scan includes. The selected
six Observatory source files do not use the new literal classes or account/settings
renderers; their widget grids select two, three or four columns rather than one.
This supports expecting no effect on those layouts, **not** visual-equivalence proof.
No scanner or frozen-mirror change was made merely to remove these bytes.

This closes the frozen candidate's missing actual CSS-rebuild prerequisite. It
does not establish active guide integration, default-appearance browser acceptance,
human usability, versioned propagation, memory/cancellation or full G3/G4 acceptance.
The copied source closure is sufficient for this CSS task, not a full Fathom build.

## Retained identity and reproduction

Private root: `/Volumes/m2ext/appkit-evidence/appkit-fathom-css.ky1jLq`. Original remains
`/Volumes/m2ext/code/geoffmilleraz/sextant/workspace/appkit-r5-candidate.7QEdhc/src`.
The archive selects `scripts/ui-css.sh`, `scripts/lib/build-cache.sh`, `cmd/fathom/ui`,
the six `observatory{,_apply,_brief,_render,_series,_story}.go` files and the exact
`third_party/app-kit` mirror. `before/` is an extraction of the pre-run archive;
`kit-reference/` is the exact Git archive, not the current sibling checkout.

| Retained artifact | SHA-256 |
|---|---|
| `candidate-css-inputs.tar` | `db8f40c558173b4b021d2439705f68e33b298aed07bf5f7960f5ae31bbc7f5df` |
| `kit-f7ab0fb.tar` | `83e2979d3e923960ed63fcc49c3d92bb40b4fc43ad818e0e1ea9b6d3d4ebfed4` |
| `build.sb` | `6ea0eafe2914321184e7ad7b614787f35af46dadbb62fd5e3b3fa167ba8c98d8` |
| `verification-01.json` | `81579427ee9b47e18ff46b54958a1dcf86e645c16b978335975abad0fe5f5234` |
| `verify.mjs` | `720be67dfab27b5e7728df65380beef10110a81974aab380d456546bde948136` |
| Original and generated `tokens/theme.css` | `e5b8a4aa803dd81d37bc18414c3415f486283d8e4a03a67d305b43b6fe82c291` |
| Original `app.css` | `0e0b5e5408c5e2484f0f2adcdba6a7c9491214d3528051ef7b43300dc52acaa1` |
| Generated `app.css` | `41b4be630db0f844e0ba47e199d7901e1731a421823c5e804621132388fc0555` |

The JSON receipt additionally pins the original/retry contracts, both build logs,
the check log and the complete CSS diff. `verify.mjs` passed after the build/check:
it compares original and copied files with the archived baseline, checks exact
mirror membership/bytes, the preserved positive token, compiler identity and cleanup.
No selected-source drift or symlink was found.

Execution used local Go 1.26.3, existing offline modules and the already cached
Tailwind v4.3.0 darwin-arm64 executable at
`/Users/geoff/Library/Caches/app-kit/tailwindcss/v4.3.0/tailwindcss-macos-arm64`, SHA-256
`56b4bbc62dbdc4614a78930d9c6986423a2ec63e4e640144a59a5d95c914322e` before and after.
From the disposable `src/`, the exact invocation was:

```sh
env FATHOM_CACHE_ROOT= GOWORK=off GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off \
  GOFLAGS=-mod=readonly GOMAXPROCS=2 \
  GOCACHE=/Volumes/m2ext/appkit-evidence/appkit-fathom-css.ky1jLq/go-cache \
  GOMODCACHE=/Users/geoff/.dev-cache/go/modules \
  TMPDIR=/Volumes/m2ext/appkit-evidence/appkit-fathom-css.ky1jLq/tmp \
  GOTMPDIR=/Volumes/m2ext/appkit-evidence/appkit-fathom-css.ky1jLq/tmp \
  /usr/bin/sandbox-exec -f /Volumes/m2ext/appkit-evidence/appkit-fathom-css.ky1jLq/build.sb \
  /bin/bash scripts/ui-css.sh
```

The check appends `--check` to the same invocation. The process-local sandbox denies
network, reads/writes under both active repository roots, and writes to the existing
compiler/module caches. It changes no global policy. Retained logs are
`build-01.log` (exit 1), `build-02.log` (exit 0) and `check-01.log` (exit 0).
No browser/server was launched for this check. Reuse this receipt for the exact frozen
candidate; when Fathom becomes available, review/rebase and verify the applied source
closure rather than assuming these results transfer to a changed consumer.
