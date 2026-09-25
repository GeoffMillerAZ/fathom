# Gallery CSS revalidation: a useful change with an explicit preview limit

Scope: Gallery's immutable embedded `/static/app.css`, not candidate CSS, private
references, consumer upgrades or the complete theme-guide performance contract.

## Implemented behavior

`apps/gallery/static_css.go` snapshots the stylesheet and computes its exact SHA-256
ETag once per handler. GET/HEAD responses use `public, max-age=0, must-revalidate`:
reusers must check freshness, with no fresh-cache interval. `net/http.ServeContent`
handles conditional GET/HEAD, weak/list/wildcard validators and body-free 304 replies.
Different bytes have a different validator; no modification timestamp is invented.
Historical non-read status/body behavior stays intact with explicit `no-store`.
The CSS bytes, token defaults, theme editor/preview `no-store`, empty iframe sandbox,
source immutability and native review/export workflow are unchanged.

This is Gallery-local implementation, not a new exported cache framework or automatic
consumer migration. Existing shell assets already use this standard-library pattern.

## Retained red/green proof

Private root: `/Volumes/m2ext/appkit-evidence/appkit-css-revalidation.s5h3UgUQ/`.
`verify.mjs` freezes itself, selected Go/Node executables and repository-local
Go/test/embed inputs before execution. It uses existing local module/build caches;
this is not a hermetic dependency rebuild or a published release.

- `red/`: the pre-change route fails all eleven validator/method cases; the
  theme-document isolation regression already passes. Original test/source/log bytes
  remain retained. Receipt SHA-256:
  `91701af6e1272637dadea8b628c2f4ba3baf0449708161d8bcb37aef0c43146c`.
- `green/`: full race suites for `apps/gallery`, `apps/styleguide` and
  `apps/gallery/cmd/gallery` pass. Added replacement/snapshot coverage proves that a
  changed stylesheet cannot validate an old body and caller-slice mutation cannot
  change the captured body. A fresh Gallery binary is built from the selected inputs.
  Receipt SHA-256:
  `4d93e4dbc10e1ce393e3fc698e25342253e88ee712c2697d73122e59c5b50992`.
- Candidate binary SHA-256:
  `5331b8e5dd5e268ae16bb358893969914602d0ab93f7807ec27bc2979aaf056b`.
- Unchanged stylesheet:51,373 bytes, SHA-256
  `94589496d9aa1e0e72c629b316df2c2629104af3dec2dda27e5a73726179a01b`.

## Actual browser outcomes

Chrome152.0.7977.84, selected Node26.3.0/Playwright1.63, fresh private profiles and
application-target CDP containment; no signed-in profile, external assets or real data.
The independent controls are not the same workload as the theme-guide matrix.
Internal acquisition root: `/private/tmp/appkit-css-revalidation.JuY50B4Z/`.

| Acquisition | Observed result | Claim boundary |
|---|---|---|
| `cache-gallery-pa8WCB` |120/120 native theme tasks complete;360 CSS responses remain200, no cache hits or304s | Candidate does **not** complete theme-guide cache acceptance |
| `cache-control-1KmuJr` | First top-level CSS200, next CSS304; recorder incorrectly tries reading a304 entity and fails | Failed harness retained, not upgraded |
| `cache-control-BQU0jV` | `/forms` → `/components` → `/forms` at390×844; cold51,373-byte CSS then two body-free304s | Ordinary Gallery stylesheet revalidation only |

The corrected top-level control records304 as having no entity, rather than claiming
to read warm body bytes. Each raw session/request pair binds the exact conditional
request and returned validator to the retained cold body. Both repeated visits have
zero CSS response-body transfer and identical applied CSSOM/computed ink. That avoids
102,746 CSS body bytes across the two repeated visits, excluding HTTP headers.
This is not a latency improvement measurement, complete page-traffic certification,
or a theme-preview cache pass. An ordinary favicon404 remains visible in raw events.

Successful control probe SHA-256:
`115d603bdeaed556194a3902fbd3d8f8f731fc84abbfb14a0b546ac678d6af71`;
owner receipt SHA-256:
`09935ccd00ea10db77c3befab151832f62a635b90386cdbb1eab110ed59750c2`.
The theme-task probe SHA-256 is
`f79b503ad8f06d71954670eece033b22353f73d724ab8ded66dad1d12875129b`;
owner receipt SHA-256:
`5000fc7dc7942db8d85a57db90fc123c10330e32c7933f9830e6cec142415fbb`.
All three acquisitions retain executed producer/launcher inputs and raw observations;
their owned Chrome/Gallery processes exit0, listening ports refuse connections and
selected inputs remain unchanged. Failed acquisitions stay failed despite clean cleanup.

The independent transport checker passes48 synthetic positive/negative canaries,
including causal same-context CSS lineage, exact validators, historical no-validator
degraded receipts and refusals of unbound/future/cross-context/no-store cache claims.
`transport-check-DLctSc/` independently accepts the candidate's task/body/timing/ZIP
and owner-cleanup evidence, but fails with exactly `repeat cache provenance absent`.
Maxima are190.603ms navigation,181.693ms preview,100.676ms export,156.195ms reset,
141,623 decoded view bytes and16,340 ZIP bytes, against the unchanged ordinary limits.
These do not erase the failed cache dimension or imply a latency improvement.

`verify-0zjfNl/` passes58 phase/checker tests, the styleguide race suite, qualityplan/MCP
checks, documentation links and selected diff checks; scope remains144 lines.
The earlier `green/` packet supplies the complete Gallery/command race suites.
`internal-evidence-WN8kHS/` retains byte-identical copies of all three internal
acquisitions (215 files,124,013,403 bytes), excluding ephemeral Chrome profiles only.
Its manifest maps original paths to retained paths; originals were not removed.
Later evidence-pointer prose does not change the selected runtime or test bytes.

## Why the preview result needs a separate explanation

The theme CSS requests occur inside newly created opaque-origin preview frames;
ordinary Gallery requests do not. Cache partitioning is therefore a hypothesis worth
testing, not a reason to relax the sandbox or silently call the gate N/A.
[Chrome's cache-partitioning explanation](https://developer.chrome.com/blog/http-cache-partitioning)
describes top-level/current-frame partition keys. A
[pinned Chromium source revision](https://chromium.googlesource.com/chromium/src/+/4c4be495bbf5e19a73586b704e13be07c4b22f13/net/base/network_isolation_key.h)
also describes multiple key modes with different opaque-frame sharing behavior.
Neither source proves which mode this installed Chrome selected. No browser feature
flags or application isolation policy were changed to manufacture a hit.

## Next evidence, unchanged goals

Keep the negative preview cache result. Use a controlled synthetic comparison to
isolate the frame/cache behavior before selecting any further delivery change.
No timing/body budget or historical oracle is relaxed by this candidate.

Memory acquisition remains a separate workload: one warmed native review/export/reset
cycle, a baseline after forced GC and five idle seconds, then20 verified cycles with
owned renderer RSS samples no more than200ms apart, followed by the same settled
endpoint. Record owned browser/server PIDs and start identities, raw browser-reported
process IDs, target topology and unique JS isolate IDs; do not double-count heaps or
pretend target IDs map one-to-one to OS PIDs. Failed samples are not zeros. Preserve
the256MiB maximum sampled renderer RSS,4MiB retained-heap growth and32MiB RSS-growth
ceilings from the [fixed contract](fixtures/styleguide-performance/browser-contract.md).
This protocol preparation is not executed memory evidence. The subsequent
[memory investigation](2026-09-17-theme-guide-memory.md) retains actual failed
acquisitions and a separate Chrome-overhead control; it does not change these budgets.

Browser stop/retry, consumer propagation, full accessibility and other core gates
remain open. G1/G4 remain partial; this is not a catalog target certification.
