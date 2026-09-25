# P06 prospective input and contrast proof

## Result

The final collector revision passes **24/24 cases twice**, with zero browser errors or
forbidden requests and 24/24 byte-identical repeat screenshot pairs. Independent Go
semantic checks pass, including rejection canaries; target status remains `not-verified`.
The earlier 115-check original/reference regression still records both 40px source
target limitations. Legacy totals remain **439/657 compiler/DOM, five typed candidates,
zero certified targets**. No renderer or source adaptation changed in this work.

Visual review sampled narrow light/dark 200% focus and wide dark focus: labels remain
readable, current-page underline and keyboard outline are distinct, and wrapping is
usable. This is not a full screen-reader or arbitrary-content accessibility audit.
Observed minimum target dimension is 48.390625px; sampled minimum text ratios are
4.8364 light and 5.6692 dark, outline ratios 6.4646 light and 5.6692 dark.

## Scope and contract

This extends [P06](2026-09-14-navigation-typed-adapter.md) with a source-specific
`internal/navigationproof` CUE/Go fact checker and a separate private browser collector.
It does not promote a target, replace the original or finish the nine conversion gates.
The collector freezes its contract before launching Chrome or activating the task;
the expected digest is taken from that pre-run console output, not the report.

The frozen matrix is 24 cases: requested viewport 390/1440, light/dark,
keyboard/pointer/emulated touch and actual browser zoom 100%/200%. Every case moves
page 3 → page 1 → page 2 → page 3 → page 4, then browser Back → page 3. App JavaScript
is disabled. The checker derives route/current/HTTP/focus/target/contrast outcomes
from observed facts; it does not accept a collector-authored pass flag.

Requirements: four ordered visible links, nonblank bounded labels, target dimensions
at least 44px, no page overflow, no label overflow beyond 1px integer rounding, normal/
hover/focus text contrast at least 4.5, and a solid focus outline at least 2px with
contrast at least 3 against its surrounding background. Keyboard activation uses
natural Tab then Enter; pointer clicks and emulated taps retain distinct input lanes.
Every forward navigation must return 200 with exactly one truthful current page.
Back may use a 200 response or browser cache; it must restore page 3.

## Pre-run measurement correction

The first collection (`quality-OxAzar`) is retained as **nonqualifying**. Its contract
bound an earlier checker with a flattened observation shape, while the producer emitted
nested `spec`. A literal wire-shape canary now covers that integration boundary.
The old report was never a semantic pass and was not rewritten/rebound afterward.

Review also found the proposed `outer_width == spec.width` rule measured the wrong
axis. `spec.width` means the requested Playwright viewport, not Chrome's outer window.
An independent local probe read both the configured viewport and CDP window bounds:

| Requested width | Browser zoom | Inner width | Outer/CDP width | DPR | Visual scale |
|---:|---:|---:|---:|---:|---:|
| 390 | 1 | 390 | 500 | 1 | 1 |
| 390 | 2 | 195 | 500 | 2 | 1 |
| 1440 | 1 | 1440 | 1440 | 1 | 1 |
| 1440 | 2 | 720 | 1440 | 2 | 1 |

Chrome 152.0.7977.84, Playwright 1.63.0, macOS/arm64; the probe's fresh profile was
removed after browser close. This observation is not a universal Chrome minimum claim.
Independent review approved checking exact inner=requested/zoom, DPR=zoom and visual
scale 1; positive outer width must cover inner width and remain identical across each
zoom pair. Inner width halves and DPR doubles across the pair. Actual 500 values stay
in reports; no floor is hardcoded and no product/style change makes this check pass.
Canaries reject outer-under-inner, changed outer, wrong inner/DPR and pinch-scale drift.
The task matrix uses separate profiles for the two zoom values: its paired outer-width
check proves cross-context consistency, not an in-place transition of one native window.
Only the separate controlled probe above also collected CDP window bounds.

Outline contrast is measured against ancestor backgrounds, separately from the link's
own fill. The collector preserves computed colors and alpha composition; the Go checker
independently computes contrast. This assumes the specimen's simple solid backgrounds,
not arbitrary gradients, overlapping content or a general pixel-contrast audit.

## Reproduction

Use the [private preparation instructions](fixtures/navigation-adapter/README.md), then:

```sh
APPKIT_PLAYWRIGHT_MODULE=/absolute/isolated/playwright/index.mjs \
  node docs/assessments/fixtures/navigation-adapter/quality.mjs "$PWD" /private/prepared-directory
APPKIT_NAVIGATION_PROOF_ROOT=/private/printed-quality-directory \
  APPKIT_NAVIGATION_CONTRACT_SHA=the-pre-run-console-digest \
  GOWORK=off GOPROXY=off go test -race -count=1 ./internal/navigationproof
```

Neither environment variable means an explicit private-test skip; specifying only one,
an invalid directory or a stale contract fails. Contracts/reports have canonical typed
JSON and a 256KiB bound. Missing/duplicate/extra cases, bad bindings and planted task
failures cannot become passes. Successful scoped facts still return `not-verified`.
The collector rehashes frozen source/artifact/CSS/binary/preparation/runner/oracle files
after its tasks. Unexpected requests, windows, dialogs, server exits and cleanup failures
are recorded. Six fresh browser profiles are removed only after successful browser close.

## Retained failures and exact receipts

All runs below live under the private temporary preparation directory
`/var/folders/2r/2zrkv4fd549bnlc5vwfzd4b00000gn/T/appkit-p06-DPv8SY`.
Each has `contract.json`, `report.json`, screenshot hashes, cleanup metadata and server log.

| Run suffix | Contract SHA-256 | Report SHA-256 | Disposition |
|---|---|---|---|
| `OxAzar` | `f88bcbf7d9fd9ee961ddeb2e521b6a39ea6f62df3529a2866ed95c4599374559` | `aebe0ca63b5c23a71d9bd50a5dd1b37036f41a10646fa88c60e284545613cdc7` | Nonqualifying old schema/oracle |
| `DGyqYq` | `15dd21150788392b8e584987e5daf747ace5418dc1fe8bd0027a4f152c1e548b` | `cdf7c6817d3d9d723901659df8efe2daae144fe2ec1ad7aa8fe7ac3872ac28ba` | Earlier 24-case semantic pass |
| `tdNvHc` | `d0618e5bead810808ecb8631210419fe0d9a59e67e7f839e0eb1984de0298995` | `9a8621cb69d00d9144e4fc3cf5319f06dd317842a066122e00a3b38a4340b889` | Browser errors; semantic rejection confirmed |
| `nMlP9N` | `2b80a741e47e6ca79701c8896e28eb8a6546ab96ddc6a65ad5f27fa65ff993af` | `c671dfc015ecae6b38515c4596077f2b8f8db63c147cd4a25d1f34cc9f1e351f` | Intermediate collection only |
| `iuUg6O` | `a71f773c129bd9a0771ceb8a5fc3f19d152c364bb46b2d69fcf8d2783b91a724` | `3cefc01b12b6b04e5987234f79b3fbef873c6c15dc1ed0170f341f441ebf1c33` | Final revision, 24-case semantic pass |
| `2DrLVH` | `5430b2afce1cb923f965bfc564a77277e9f7439a095e30fb1c082ecbdb6489f7` | `f5ae995657019f198da75e76209e902296479efa98df51e834c830fbf11bb5c1` | Same final revision, repeat semantic pass |

The failed repeat reported `createElement` and Trusted Types `createHTML` callbacks
that were no longer runnable. Disabling scripts while Chrome settings remained active
was the suspected harness lifecycle cause, not an established component defect. The
reviewed fix installs error diagnosis before configuration, verifies zoom, navigates to
exact `about:blank`, then disables app scripts before any specimen navigation. No error
is filtered or excused. The final two runs pass with that sequencing; this does not
guarantee all future browser teardown races are solved. All six fixture ports are closed
and all temporary Chrome profiles removed; private evidence is retained.

| Frozen input | SHA-256 |
|---|---|
| Source | `01345180cee4744b57e50a3ceca9461566fdfd7ab5d8f629113654bac052ca31` |
| Artifact | `8ab0e692df0b1521097ac04f3847c68ddf3c5e9acaca13c13e8aa0289d78bea6` |
| Preparation | `b47ecca856d578e11513c209e83025d70b1fb648f0f3379ef83ba1cfd0e7b85f` |
| Preview binary | `17df9d2d92bfcfeefa2d6a882c0a179cbe0c66f6ddebbc964b93c0ca85aec878` |
| Embedded-font CSS, 484717 bytes | `8f9f11836291945e9b08fb92f82e94a6a2704ca9fa0c49ea92388aa5cd615cf3` |
| Final collector | `a0d74566e4de909a77bcab96b5f65641a0421cde65e733c11108347c163954c7` |
| Go checker | `a542761f716dd8c64d99e793884d4684788922aad0f8bdd926e137a12831c04b` |
| CUE schema | `36f5bf3f3a822bf6ad94c367d446e7e8d5796320b94689087a9c2b8447b5e4cc` |
| Oracle: checker + NUL + schema | `e32c107f884449d59e8d814124dc192ed3cd5ac7459076b8b462a4181038d6ac` |

Final full package race test with the repeat's explicit private pins passes (12.458s).
The first final live check passes separately (1.537s). Source-only integrity and full
MCP document/protocol race tests pass (2.666s/23.232s); absent private integrity data
is an explicit skip there, not a new retained-file run. Independent code/oracle review
found no blocking issue. Scoped intent budgets, 42 ADR shapes/index, stanza parity and
207 relative file links pass; host/skill invocation and link anchors were not checked.
Separate process checks confirm explicit skip when unrequested and failure for a missing
pin, missing directory or stale pin. [INC-26](../intent/incidents.md) records the lifecycle failure.

## Remaining boundary

Full input/dependency closure, authenticated observation/review, screen readers, physical
touch hardware, other browsers/native hosts, full visual fidelity and lifecycle/security
gates, performance workloads and consumer promotion remain open. These task observations
are not proof against a forged report with invented plausible facts. No full gate is
automatically marked passed; the integrity layer remains separate. Temporary private
screenshots/source bytes are not backed up by Git. No consumer, dependency, release,
global configuration or deployed runtime is changed by this maintainer-only proof.

A later [private retention step](2026-09-14-private-evidence-catalog.md) copies the final
repeat's contract/report, oracle and 24 images into an immutable 41-artifact packet.
That historical aggregate and its ten-artifact predecessor are now discoverable through
CLI/MCP/Gallery. All nine full gates remain not-run; the store does not execute this checker.
