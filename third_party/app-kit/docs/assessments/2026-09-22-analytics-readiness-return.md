# Analytics adoption connected to readiness

Starting revision: `41dd47d`. This updates app-kit's authored readiness record, not
RAC's source, dependencies or installed artifact. Conjure/Fathom remain untouched;
human A/B and the full G3/G4 requirements stay open.

## Real scoped return

`core.analytics` now records `resume-as-code` as `verified-scoped`, linking the
[existing integration receipt](2026-09-22-analytics-adoption-integration.md) and
consumer-owned `RAC-ADOPT-04` contribution. It stays a `starter-area`, not a converted
component. All seven standards and nine gates remain partial; none is accepted.

The selected artifact is the executed synthetic `browser-03.test`, SHA-256
`c727501e305a091802070e18761d6c8c0ceea8b9efe627be63edb79a86c1e831`, not a shipped app.
Its retained browser report still hashes to
`4ad37ad301db9259480e56c59900354d62505d27dab949ba7af9c3afe2f1bf97`.
Both were rehashed before recording the claim. The receipt is pinned as
`1a80513a401bf38181d4ff6ea0f41736171ccbc7b7a7fc31b797ef9217bedbdc`;
its four source identities, failed runs and closure limits remain authoritative.

RAC uses app-owned `ops.ChannelYield` and kit visualization, not the public
`analytics.RenderBars` recipe. Its real renderer/export task uses synthetic data:
all four channel rows, five truth states, native download and bounded pure-render
measurements. This is originating-app evidence and a returned testing lesson,
not two-app propagation, live snapshot consistency, auth/hosted acceptance or release.
The [contribution guide](../design/contribution-loop.md) now names this real example
separately from the fictional JIT rehearsal.

Newly built Gallery/MCP binaries expose this record through existing quality detail
surfaces. Installed binaries are not refreshed here; their embedded snapshots can be
older. These surfaces report reviewed historical claims, not live evidence validation.

## INC-57: stale rehearsal pin caught and prevented at push

The first full quality package run failed `TestJITContributionPathRehearsal` because
`f905a33` added the duplicate-consumer rule to the standards document but left its
rehearsal pin at `5730f6b9…`. Earlier reported tests do not establish a pass for those
final combined bytes. Source review confirms only that documented reader rule changed.
The fixture now explicitly pins the current `e79c8777…` document. Neither contract,
hash comparison nor any acceptance assertion was weakened; old packets stay unchanged.

Repository `lefthook.yml` now runs `TestInventoryCompilesAndTracksIndependentFacts`
and `TestJITContributionPathRehearsal` before push, independently of static Fathom
verification. Both agent guides explain pin review after documentation changes.
The exact new hook was invoked with the old pin: it exited1 for that hash mismatch.
After the pin-only correction, the same command exited0. This is local hook evidence,
not proof of installation in every clone or a remote CI check.

Private evidence: `/Volumes/m2ext/appkit-evidence/appkit-analytics-readiness.sK0lJ6`.

| Retained input/output | SHA-256 |
|---|---|
| `inputs-01.tar` | `cea1f7eecca7ff03b0fa789762ad27c898038ab84af018d67075600b88e36919` |
| `pin-red-inputs.tar` | `f7e1df75fd57ff1d5058d65774253d5779004d40f2bdc5cda5a31d4ca071260c` |
| `pin-green-inputs.tar` | `e1164375ce65dbe46562f5265d770e18c3bc23cc1c412470442062165d4b27d3` |
| `gate-red-02.log` | `4369c87c9ebda1a819b388dc7c9b77600d1e1c0d980fa0b4da77ebbab4147bae` |
| `gate-green.log` | `1dc0bfc7a3f63dfa7baa88509bfbed07ec9e9772baa92e5428d1a7536eb9175a` |

Exact command: `lefthook run pre-push --commands quality-evidence --no-auto-install
--no-tty --force`. Before the forced pair, a temporary external-volume read/write
failure prevented launch; the unchanged retry's `gate-red.log` then skipped because
there were no committed push files. That skipped invocation is not a gate pass.
`--force` exercises the working-tree hook for this controlled red/green comparison;
ordinary pushes use the repository's normal installed pre-push entrypoint.

## Verification

Pinned-CUE validation, independent review, identical agent guides and the controlled
hook red/green checks pass. The first Gallery test then exposed a stale empty-consumer
expectation. Its empty-state assertion is preserved on an explicit empty fixture,
not on the now-populated analytics item.

Source review also found consumer receipt/reviewer fields omitted from HTML even
though JSON/MCP expose them. A new synthetic metadata fixture first fails on three
missing strings; the native disclosure repair passes all quality tests. Hostile
reference/reviewer text remains escaped plain text, not a file-serving link.

Prospective browser scope: two root-kit views,1440/light and390/dark, JavaScript off;
native Tab reaches the new disclosure, Enter opens/closes it, the exact receipt and
reviewer are visible, focus persists, the target owns44px and the page does not
overflow. One acquisition, with at most one retained harness correction if needed.
Use the existing isolated loopback fixture and `quality-consumer.mjs`; no consumer
execution, external assets, global configuration or broader analytics matrix.
The unchanged driver passes32/32 checks in Chrome153.0.8010.53, Playwright1.62.1,
Node26.3.0. Both viewport screenshots were inspected; this is consumer-metadata
inspection, not whole-page accessibility or an analytics usability comparison.
No page/network errors or external requests were recorded. Fixture shutdown reports
success, the Go process exits0 and its port49261 has no remaining listener.

| Additional retained input/output | SHA-256 |
|---|---|
| `consumer-red-inputs.tar` | `47d4df5442a94fa3b720e51be38a989950dc14277d572ffc5ead7083ffc17aaf` |
| `consumer-green-inputs.tar` | `86e7d82c1642a1adc90821d562f0201297e8fb7385df1562f759de7cd755bf62` |
| `consumer-red-results.txt` | `e206e9886f7a44bc005786eb51bf25cc56d53d5e5a1717d8a1bd54ea4ecc83f8` |
| `browser-inputs-01.tar` | `6d61a49b741192ec14fc75a1a25e264e1425ba8974485207612de65e2c16b798` |
| `gallery.test` | `b01cb9354e598faf15c1e3464e54810f042d0d4d85c335018e90a617b6ae7f60` |
| `browser-run-01/report.json` | `43fe10a26851c4337962abb35457bd17d80d32ded41f12b44b02aaf9c9234dea` |

An initial Node startup failed with `EPERM: process.cwd`; it never ran the driver or
opened a browser. One unchanged retry used the already-running ready fixture and
passed, without restarting it or relaxing an assertion. A Go launch also failed to
read `go.mod`; the unchanged retry of `go test ./internal/qualityplan ./apps/gallery
./cmd/appkit-mcp -count=1` passed all three packages, including existing MCP/Gallery
JSON parity and pin/missing-evidence refusal tests. No permissions were changed.
The browser archive retains production source, fixture, CSS, inventory and driver;
all earlier failing/skipped acquisitions remain distinct.
