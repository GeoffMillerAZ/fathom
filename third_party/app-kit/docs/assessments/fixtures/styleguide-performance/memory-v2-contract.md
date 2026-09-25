# Theme-guide memory v2: prospective metrics and attribution contract

Contract ID: `appkit-theme-guide-memory-v2`. Decision:
[ADR-0073](../../../adr/0073-separate-memory-metrics-from-attribution.md).
This does not supersede the [v1 browser contract](browser-contract.md), rewrite its
failed receipts or change production code. No Gallery acquisition has qualified under v2.
The [first synthetic control](../../2026-09-22-theme-memory-controls.md) failed at
bootstrap on an unadmitted background page; no fixture cycle or retention run occurred.
A separate one-switch startup-only run passes50 bounded samples, not lifecycle or
memory qualification; that manifest admits no app/control workload.
The separately admitted control profile reaches20 native cycles and structural lifecycle
checks, but fails process classification and shutdown/capture. Retention remains unrun.
One separately frozen shutdown-only control passes explicit owned-context disposal and
unforced complete capture after20 native actions without memory sampling. It neither
explains the prior failure nor qualifies the positive control or memory acceptance.
A later component-update-disabled positive still fails unavailable process commands
and a short idle, despite clean context/process capture. That profile is not maintained.
The collector now rechecks its minimum idle deadline; frozen tests pass, not a new
browser qualification. All numeric limits and the strict metric checker stay unchanged.

## Selected implementation boundary

`memory-v2-metrics.mjs` exports `evaluateMemoryV2Metrics(lane, owners)` as a pure
subcheck. `owners` supplies exact browser/server PID and start identity plus origin.
Input uses the existing memory lane's raw measurements and explicit `contract_id`.
Do not relabel a v1 receipt as a new acquisition. The marker selects a parser; it
does not authenticate a run. The caller must bind frozen source, tools, contract,
task/guard evidence and owner cleanup before claiming even a scoped runtime result.

The result distinguishes `metrics_valid` from `budgets_passed` (null when invalid).
It always returns `attribution: "not-verified"`, `memory_complete: false` and
`full_contract_complete: false`. A metric-only pass never promotes catalog readiness.
It is not integrated into `transport.mjs` or `transport-check.mjs`; those remain v1.

## Fixed workload and limits

Use Gallery's actual Synthwave guide and the same source/candidate/export/reset
postconditions, exact body/ZIP checks, native form actions and policy as v1. No page
script injection, sandbox relaxation, paid content, real app data or consumer writes.
Each cell: one warm task, baseline, twenty successful full cycles, final endpoint.
Full v2 acquisition still requires dashboard/components/forms × light/dark ×
1440×1000 and390×844; a diagnostic cell cannot stand in for the other eleven.
Memory instrumentation is not useful-view latency evidence.

Both endpoints use the same post-reset UI state. Collect each unique isolate once,
then wait at least5000ms without network activity or a target change before reading
its heap and diagnostic DOM counters. Keep raw values and failures. The two endpoints
may have different target/session/isolate IDs after native navigation; stability is
required within each idle window, not across the entire task.

| Measurement | Prospective treatment |
|---|---|
| Sum of `usedSize` once per observed isolate | Final minus baseline ≤4MiB |
| Sum of `embedderHeapUsedSize` once per observed isolate | Independently ≤4MiB growth |
| Sum of `backingStorageSize` once per observed isolate | Independently ≤4MiB growth |
| `totalSize` | Retain; used JS heap must not exceed it; not another growth budget |
| `Memory.getDOMCounters` | Retain by session; do not sum or assume per-frame scope |
| All owned Chrome process RSS | Report baseline, final, growth and sampled peak |
| Renderer and non-renderer RSS | Report separately, including internal/spare processes |
| Owned server RSS | Report separately; never add to Chrome or attribute to app JS |
| Process samples |100ms target; ≤200ms acquisition duration and actual coverage gaps |

Each domain gets the existing v1 JS-growth allowance independently: a bounded
transient authoring task should not accumulate more than4MiB in any measured domain
over twenty repetitions. These are prospective product limits, not empirically tuned
to a v2 run, a per-cycle leak allowance or an arbitrary-consumer SLA. Domains may
overlap; adding them would assert unsupported accounting. Negative growth is retained.

Raw heap fields, byte/count values and process identities must be valid; missing data,
unsafe integers, duplicate IDs, inconsistent session/isolate assignments, PID reuse,
unowned/omitted process rows, sample errors and coverage gaps invalidate the subcheck.
Unknown costs never become zero. Invalid output carries null derived measurements.
RSS is a sum of process resident sets, not unique physical memory. It has no v2
ceiling and is never blank-baseline-adjusted. Unexplained RSS growth still prevents
a broader stability claim even when the three scoped heap budgets pass.
Unavailable parenthesized `ps` command placeholders are not affirmative non-renderer
evidence. Each child needs one explicit process-type argument before classification;
missing or ambiguous type evidence invalidates metrics instead of dropping a process.

## Attribution qualification still required

Before a future runtime checker can change attribution from unverified, retain and
independently join raw target/context IDs, guarded attachment/policy/resume/detach
times, page frame tree, parent/frame/loader IDs and per-isolate membership. Establish
one guide page with its opaque preview; same URLs alone are not ownership evidence.
Every live application target must be accounted for. Shared-isolate members cannot
be silently excluded. Replaced frames need causal creation/navigation/destruction
evidence; detachment alone is not proof that a target or its memory was destroyed.
Unknown targets, missing lifecycle events and incomplete scope remain unqualified.

CDP's live-isolate values cannot by themselves exclude retained detached isolates or
native/image/GPU allocations. The future acceptance checker must expose that limit
and require a separately reviewed lifetime/retention method before claiming bounded
whole-app memory. An active-target disappearance is never a negative allocation.
The opt-in [lifecycle instrument](../../2026-09-22-theme-lifecycle-instrument.md)
records raw protocol events and bounded guide checkpoints. The separate
[independent join](../../2026-09-22-theme-lifecycle-join.md) checks structural lifecycle
relationships from that raw evidence. Neither establishes retention or complete
attribution/acceptance; runtime controls and lifetime coverage remain required.

## Independent controls and bounded continuation

Pure controls must prove shared-isolate deduplication, changed native IDs, each exact
4MiB boundary and one-byte excess, high Chrome RSS without app attribution, malformed
or absent values, process identity/refusal and200ms sample gaps. A producer's `ok`
cannot decide metric validity or budget acceptance. V1 checks must remain unchanged.

Before the first v2 browser run, freeze the exact acquisition/checker/contract, selected
source/server/browser/Node/Playwright bytes and launch flags. Retain a benign native
navigation control and a deliberately retained >4MiB allocation control in a separate
synthetic instrumentation page; never inject it into the actual guide or relax its
policy. Controls must also expose missing/shared/retired ownership as unqualified.
Control success qualifies the instrument only, not the twelve-cell app workload.

Next bounded task: address the retained incomplete-process evidence and orderly
shutdown hypotheses with separate controls before **one** native guide cell. Both failed
positive attempts stay retained. Cap a separately selected qualification at one positive and one deliberate retention run;
stop on unknown-target/identity/containment failure, retain it and select a concrete
R2 hypothesis before another run. Do not launch a full matrix until that method is
qualified. Fix limits before execution; preserve unsuccessful bytes before repairs.
No consumer rollout, human A/B replacement or reopened full-catalog campaign follows.

## Instrument semantics checked locally

The selected Playwright CDP `protocol.d.ts` declares heap use isolate-wide, defines
the three heap fields separately, and gives no frame-local scope for DOM counters.
Observed local source: `/Users/geoff/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright-core/types/protocol.d.ts`,
SHA-256 `b836eed98b79fabb54f1df73252a0ce178b8df4a89794251ade2fa0c5e52b678`,
lines22725–22747 and10194–10203. This records inspected instrument documentation,
not browser execution. Freeze the actual selected protocol/runtime again for a run.
