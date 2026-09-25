# ADR-0067: Prove isolated warm-cache lineage without claiming observed warm bytes

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-16
- **Extends:** ADR-0023 and ADR-0042; no existing receipt or oracle is superseded

## Context

P01's v1 performance experiment requires every response body, including cached fonts.
[Eight browser instruments](../assessments/2026-09-16-cached-font-protocol.md) observe
the exact cold font but cannot expose warm bytes while preserving actual caching.
The measurements are not a v1 pass. Repeating those instruments adds no evidence.
Two independent reviews identify a narrower proof by construction for an owned,
identity-encoded server and one fresh cold-to-warm cache partition.

## Decision

Admit a separate prospective P01 v2 performance contract with identical numeric
budgets, workload and16 observations. Preserve v1 code, contracts, failed receipts
and direct-body requirements. No retrospective relabeling, baseline replacement or
general cache-identity fallback follows. The [v2 guide](../assessments/fixtures/hero-adapter/CACHE-LINEAGE.md)
owns the frozen acquisition contract and implementation state.
The [first full acquisition](../assessments/2026-09-16-hero-cache-lineage.md) cannot
satisfy observed warm request-header equality and has unmatched loading failures.
The method remains unproven on that instrument; its rejection conditions stay intact.

V2 may accept explicitly derived `isolated-cache-lineage` only for its selected font:
one owned browser/context, a unique loopback origin per pair, freshly cleared cache,
exactly one admitted cold seed and two consecutive fresh navigations. Bind request
IDs, method, URL, frame, distinct current document loaders and ordered events. Reject
redirects,304/ranges, cookies/authentication, Vary, content encoding, interception,
service-worker/prefetch sourcing, missing font requests and unrelated/extra requests.

Observe the exact cold font body and match immutable source/server manifests before
and after collection. Bind cold/warm representation headers, including a strong
body-hash ETag, MIME, length and cache policy, plus request headers. Require an actual
request-bound warm HTTP-cache event and zero warm static-server requests. Preserve
the exact unavailable-body error; never substitute source or cold bytes as observed
warm bytes. Paired PNG/geometry and actual custom-font use must agree independently.

Report the derived method and `directWarmBodyObserved:false` distinctly. Uncached
body totals remain derived only from directly observed uncached responses. Missing
evidence or any counterexample refuses this method; plausible authored observations
are not authenticated facts. The v2 performance gate additionally needs its entire
matrix, unchanged task checks/budgets, negative controls and clean owned cleanup.

## Consequences and alternatives

The representation-bound fixture adds validator headers; it is a new delivery
experiment, not a claim that the prior server supplied them. Freeze collector,
checker, contract and controls before acquisition; independently review oracle changes.
Retain original-asset payload failures and test cache/provenance/rendering mutations.
No consumer write, new runtime API, catalog promotion or complete target certification.

Repeating unsupported body extraction is rejected without a new instrument/version.
Pixels alone, stored font files or a generic cache boolean do not prove lineage.
Disabling caching to obtain bodies measures a different workload and is rejected.
P10's supplemental validator-linkage evidence remains separately scoped and unchanged.

## Revisit when

- A declared cache/representation/rendering mutation passes the refusal checker.
- The controlled server gains negotiation, compression, redirects or app scripts.
- A changed browser supplies direct warm bodies or changes recorded cache semantics.
- Another source/profile needs this method: earn its own prospective applicability.
