# Theme-guide browser workload: theme-guide-browser-v1

Prospective contract for the missing browser part of `core.themes-style-guide`.
It supplements [request costs](README.md), not the whole converted-core standard or
real-consumer acceptance. Freeze this file, harness, selected server/source/CSS and
tool identities before acquisition. No production optimization is presumed necessary.

## Fixed task and identity

Use Gallery's actual `NewThemeLab`, the existing `geoff-synthwave` source and all
three real synthetic specimens: dashboard, components and forms. Use both schemes
at1440×1000 and390×844 CSS pixels. Preserve empty iframe sandbox, native forms,
same-origin POST, no-store editor/preview policy, acknowledged ZIP export and immutable
source. Use only the approved installed Chrome/Playwright with a fresh private profile;
no signed-in browser, external assets, consumer writes, dependency installation or
real data. Page evaluation is read-only diagnostic instrumentation, not authored JS.

The selected server's page and exported receipt must report its actual executable
SHA-256. Record Node/Go/browser/Playwright identities, OS/hardware, source and complete
asset closure. A read/measurement failure is not zero, a pass or a silent omission.

One task starts at a new guide document, edits sans to `system-serif`, medium radius
to `.75rem` and light accent to `#ff7edb`, submits Preview candidate natively, checks
the selected specimen/scheme and computed candidate values, downloads the acknowledged
ZIP and resets to useful baseline. Preview creates acknowledgement; do not invent a
second review control. ZIP members, source, diff and response hash must agree.

## Timing and delivery matrix

Run five independent samples per viewport. Within each sample exercise all six
specimen/scheme combinations in each lane:

1. Fresh-context first visit, ordinary loopback transport.
2. Same-context repeat of the same task, ordinary loopback transport.
3. Fresh-context degraded visit:100ms configured request latency,200,000 bytes/s
   download and64,000 bytes/s upload. No CPU/device emulation is implied.

Do not call a repeat a warm-cache pass. Existing Playwright routing disables HTTP
caching; such an acquisition must say so. A separate cache-preserving lane is required
to assess any genuinely cacheable asset: record the actual cache/revalidation events,
prove containment independently, and distinguish intentionally no-store documents.
Absent cache provenance keeps that dimension incomplete, not N/A by convenience.

Start monotonic timers immediately before native navigation, Enter or download action.
End initial/preview/reset timing only after the expected document and iframe content
are visible, selected candidate/baseline styles are computed, and expected digest and
export-button state agree. `networkidle`, response headers or an iframe load alone
do not prove usefulness. Export completes only after saved ZIP bytes are readable and
verified. Record all samples and maxima; five samples do not estimate a population p95.

| Per-task metric | Ordinary ceiling | Degraded ceiling |
|---|---:|---:|
| Navigation → useful selected view | 1,000ms | 3,000ms |
| Native preview → useful acknowledged candidate | 500ms | 2,500ms |
| Native export → verified saved ZIP | 500ms | 2,500ms |
| Native reset → useful source view | 1,000ms | 3,000ms |
| Complete decoded bodies through each useful view | 256KiB | 256KiB |
| Candidate ZIP | 1MiB | 1MiB |

These budgets target responsive local authoring with the existing bounded specimens,
not a network SLA or claims about mobile hardware. Record wire/transfer observations
separately from decoded bodies; a cached zero transfer still needs body identity.
Count every response in the selected closure. Failed, aborted, unexpected, duplicate,
unaccounted or out-of-scope traffic must be reported, never counted as successful zero.
Prove degraded shaping applies to iframe/assets as well as the top document before
accepting its results. A configured CDP command alone is not coverage evidence.

## Memory and lifecycle

Measure memory in a separate lane to avoid contaminating timing with collection.
Record renderer JS heap/DOM counts and owned-browser process RSS, identifying precisely
which processes/targets are included. Sample owned renderer RSS at least every200ms
during20 preview/export/reset cycles; report gaps and the maximum observed sample,
not an unsampled absolute peak. Sum renderer RSS at most256MiB. After forced collection
and five idle seconds, renderer JS retained growth at most4MiB and renderer RSS growth
at most32MiB relative to the same warmed workload; no extra live guide frames remain.
Browser fixed overhead and server RSS are recorded separately, not disguised as JS heap.

Abort an in-flight native preview using the browser's stop/cancel mechanism in a
separate controlled-delay lane. Prove no download/source write occurs, local edits stay
recoverable, and an unchanged retry completes the expected candidate/export/reset task.
This does not prove cancellation of arbitrary trusted Go callback CPU or I/O. Separate
cooperative-callback/context checks must identify that boundary; do not claim preemption.

## Receipt and failure policy

Retain exact executed harnesses/configuration, raw response/event identities, complete
measurements, expected and observed postconditions, source before/after hashes and
cleanup. Every fresh process/profile is owned and cleaned up; original failures remain
in immutable packets. Independent checker tests must reject missing samples, invalid
numbers, over-budget values, incomplete closures, wrong identities and failed task states.
No baseline, oracle, budget or application policy changes to turn a failure green.

Each lane can advance independently, but incomplete warm-cache, degraded coverage,
memory or cancellation evidence prevents a complete browser-performance pass. This
contract does not replace real-consumer delivery, accessibility or theme acceptance.
