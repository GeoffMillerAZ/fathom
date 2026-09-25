# Fathom guide: scoped integration and retained prerequisites

Status: authorized integration applied to Fathom's working tree; no consumer
commit, push, daemon restart or deployment. Scoped current-source tests and browser
tasks pass; broader acceptance remains open. Historical failed receipts remain
unchanged. Source patches: [review packet](fixtures/fathom-guide-only-2026-09-22/README.md).

## Original isolated scope and result

Baseline is tracked Fathom `fa8c8ed854ff319227638c4ef79a15d3c00e50b8`, clean but
27 commits ahead of origin when acquired. None of those commits was changed or
pushed. The candidate uses the exact first-party app-kit archive
`f7ab0fbd3496e9dc3da8c97b6c4290f09a71ec50`, not a sibling replacement. Every mirror
file matches that archive; its stamp is the sole addition. Agentic-kit's
`beb53a4fb901` pin and complete mirror remain byte-identical to the baseline.

The old patch is now split into three independent review units: guide/command/CUE/
CSS builder, root module prerequisite, and display-truth repair. No service JSON
fields, source readers, metrics calculations, live data or daemon routes changed.
The guide remains local, opt-in, synthetic and source-preserving; ordinary startup
does not mount it. No tests or expected values were relaxed.

| Check | Observed result |
|---|---|
| Baseline readonly dependency load | Pass. |
| Guide/exact mirror with unchanged root manifests | Fails: missing `rivo/uniseg` checksum, retained. |
| Offline private dependency resolution | Adds only root indirect `github.com/rivo/uniseg v0.4.7` and its two checksum lines. |
| Loaded Fathom package modules | Only newly loaded module is uniseg; agentic-kit version is unchanged. |
| Whole module graph | Mirror also selects MCP SDK 1.6.1 → 1.8.0 and adds `x/time v0.15.0`; neither contributes a package to this Fathom binary. Do not describe the graph as unchanged. |
| Guide-only race run | Fails the unchanged source-validity canaries: partial counts lack a lower-bound marker; failed registry/call-log/ledger reads appear as numbers or zero sessions. |
| Separate display repair + same race command | 33 top-level tests, 94 test/subtest passes, zero failures/skips. Readable zero and independent source validity stay covered. |
| Package vet / standalone build | Both pass with ordinary readonly root manifests and `GOWORK=off`. No temporary modfile is needed after applying the exact private prerequisite. |
| Synthetic CSS builder regression | 11/11 pass. |
| Actual pinned-compiler CSS rebuild | Both attempts fail required-source readability at mirrored `hitl/approval.go`; neither publishes CSS. The second reaches Tailwind v4.3.0. Subsequent direct/sandboxed hash reads succeed, but the cause is unknown. No further retry admitted. |
| Patch applicability | All three apply together to the exact tracked baseline. |

## Authorized continuation

The owner has now authorized the exact mirror/module graph and display repair,
superseding the earlier guide-only restriction. The narrower approval excludes service
JSON and agentic-kit pin changes, live data, restart, consumer commit/push and deployment.
The historical checks above did not change active consumer files or installed binaries.
The mirror's `viz.Bars` also reserves more room for long values: inspect counts of at
least 1,000 in the 256px grade chart and 560px calls chart before appearance acceptance.
The palette remains positive `oklch(0.55 0.14 155)`; this is not a theme redesign.

The continued private root is `/Volumes/m2ext/appkit-evidence/appkit-fathom-integration.SOWNalRa`.
It retains the exact mirror, agentic-kit identity, production renderers and default
palette. The adoption manifest now selects L4 / `recipe.styleguide.local` using its
reviewed experimental metadata; docs/app-kit.md, scope, changelog and command guidance
record the boundary and FATHOM-GUIDE-01 return disposition. Existing matching agent
routers already route to that contract and remain unchanged.

The bounded diagnostic in `/Volumes/m2ext/appkit-evidence/appkit-fathom-css-diagnostic.N9OwGvNq`
first failed because its harness accidentally denied writes to its own staged source.
One explicit correction restored the original sandbox. The next run completed Tailwind
but failed the post-digest `test -r` predicate at `vizframe.go`; immediate metadata
reads succeeded. That does not prove content readability or isolate a macOS cause.
A later direct hash read passed independently. Both failed harnesses/logs remain.

The builder now uses the required content-hash read as its authoritative readability
check, not an additional access predicate. Missing/non-file, symlink, containment,
digest, concurrent-change and no-publication-on-failure controls remain. Read failures
retain the generic refusal and now include the failing command's status/stderr.
Synthetic controls exercise both a false access predicate and an actual failed read.

| Current private candidate check | Result |
|---|---|
| Synthetic CSS guard suite | 12/12 pass; failed read exit77/stderr and original refusal text retained. |
| Real pinned-compiler CSS build and independent `--check` | Both exit0, no timeout/escalation. Theme byte-identical; app.css grows720 bytes to33,879. |
| Scoped race run | 35 top-level / 96 test-and-subtest passes; zero failures/skips. |
| Package vet / standalone build | Both exit0, ordinary readonly manifests, GOWORK off, no sibling source access. |
| Static adoption check | 35 passes plus one whole-catalog N/A, zero failures; not runtime/host proof. |
| Fresh-browser workflow | 61 named checks plus native ZIP/event assertions pass; light1440×1000, dark390×844, preview/refusal/repair/reset and pointer/Enter export. |
| Active delivery | Exact reviewed source closure matches the private candidate; 1,473 mirror body files match f7ab. Agentic-kit mirror, service JSON and both agent routers remain unchanged. |

Commands, outputs and lifecycle records are `run-check.mjs`, `run-css.mjs`,
`*-01.exit.json`, and matching stdout/stderr. `pre-build-source.tar` retains the
real CSS input; `final-runtime-source.tar` retains the resulting build closure.
The final synthetic suite source/harness is `ui-css-final-synthetic-harness.tar`
(SHA256 `19c6bef741dcc5a6a4383f342755f29917aab3dc27ce3c5b0802e2df86180c2a`).
Earlier RED-01 new-test bytes were not retained: its hash/logs are non-replayable
diagnostics. RED-02 source can be recovered from the original production-script archive
and identity-matched later test snapshot, not a contemporaneous bundle. Neither is
misrepresented as a complete pre-run source receipt; later passes do not upgrade them.

| Current build artifact | SHA-256 |
|---|---|
| `fathom-guide` | `52666a587cc44dfc146064cd58d5003d34d79d319592d8d16d6394c1ff454198` |
| `cmd/fathom/ui/app.css` | `41b4be630db0f844e0ba47e199d7901e1731a421823c5e804621132388fc0555` |
| `cmd/fathom/ui/tokens/theme.css` | `e5b8a4aa803dd81d37bc18414c3415f486283d8e4a03a67d305b43b6fe82c291` |

The browser run used Node26.3.0, Chrome153.0.8010.53 and Playwright1.62.1 with fresh
contexts and no signed-in profile. The prospective contract pins the executable,
source CUE, driver/helper and runtime identities. Nine document/frame asset barriers
completed. Narrow preview width/scroll width both equal356px. The two17,104-byte ZIPs
match (`6cc88b9396b211f1d590ba4a3170c3d3c55e3760bbfdd1589962e8b518d924d2`),
including the reviewed radius-only diff, source and payload/receipt hashes. The expected
422 diagnostic is retained and tied to the refusal response; no unexpected console
errors or external requests were observed. Owned server/worker/Chrome exit and final
port refusal pass. Wide preview and narrow form screenshots were inspected, not
treated as whole-screen visual certification.

`browser-harness-01.tar` retains the executed bytes (SHA256
`2512ecae977df79e125889fba45ae45bc215de9646e7f0d7d5df98fefac7eb00`);
`browser-run-01/report.json` is
`685873ae8f948c6a2b736bca38cc8d44732ef73cacd51f3bfe3676e430fcb982`.
The packet preserves the same portable harness and prospective contract.
`delivery-contract.json` binds the baseline and applied source hashes;
`active-source-equivalence.json` and `active-adoption.json` pass against the active
checkout. An initial over-broad filesystem preflight was terminated before any
delivery write; its script is retained in `delivery-harness-01.tar`. The replacement
reads only the reviewed source closure. The old mirror was moved recoverably to
`previous-active-app-kit` in the private root, not deleted. Generated app.css was
copied from the verified build; authored changes were applied as a reviewed patch.
Fathom HEAD remains `fa8c8ed854ff319227638c4ef79a15d3c00e50b8`.

The [large-value/zoom follow-up](2026-09-22-fathom-large-zoom.md) adds 118 passing scoped
geometry/keyboard observations, but 200% call-chart raster review still fails; complete
appearance acceptance remains open. Human A/B, full accessibility, memory/cancellation
and full G3/G4 acceptance remain open; no quality tier is promoted.

## Retained evidence

Private root: `/Volumes/m2ext/appkit-evidence/appkit-fathom-guide-only.v7yRsn`. Source is readonly
during Go checks; external networking, active repositories and user data are denied.
Only private build/temp output is writable; existing module/compiler caches are readonly.
Commands use Go 1.26.3, `GOWORK=off`, `GOPROXY=off`, `GOSUMDB=off`, `GOMAXPROCS=2`.
`run-check.mjs` and `run-css.mjs` retain exact commands, exit/timeout/escalation records
and logs. Both CSS failures exited without timeout; their workspace temp directory is
empty, and owned acquisition process IDs are absent after completion.

| Frozen input/output | SHA-256 |
|---|---|
| `fathom-baseline.tar` | `100b053ad0dce0b7fe2a26226a7fca8c1d38fa1815838f446bcc1cddddcb8ec4` |
| `app-kit-f7ab.tar` | `83e2979d3e923960ed63fcc49c3d92bb40b4fc43ad818e0e1ea9b6d3d4ebfed4` |
| `guide-unchanged-manifests.tar` | `3754bc982da9e19cbdbc3d85a16e4e26786aa6c646952ddcd36ef2803fc2f789` |
| `guide-minimal-manifests.tar` (failed guide-only race input) | `d750d922f5acb6adf0b4f4c979569ae6e9bddff01e94878408ca03f88997ba4c` |
| `guide-with-display-repair.tar` (passing race/vet/build input) | `7e224e3bcfe1cfb52cbf8483787ce7c2c610a8d95d62494f22aa6ff53117292c` |
| `fathom-guide` (before any fresh CSS publication) | `9155d0604a082e3ab815401ec81358cc254cb09d4fa4de3c56ae0a87d55a7959` |

`source-and-patches.json` pins all 4,922 candidate entries and restricts differences
to ten consumer files and the exact mirror. `execution-harness-01/02.tar` and
`css-harness-01/02.tar` retain executed harness revisions before modification.
The source closure is buildable, not a released or fully accepted consumer integration.
