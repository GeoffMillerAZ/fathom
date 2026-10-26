# Examined-source support responses

ES,2026-10-26. [0048](../intent/decisions/0048-separate-examined-sources-from-premises.md)
refines the [qualified review](prose-qualified-review.md) within0045/0047. The
criterion 2 floor and85 cases are unchanged. Protocol 3's actual trial remains
failed 67/85;66 matched and 18 unexecuted, with one contract-invalid response.

## Two versioned paths

Keep `prose-lab.qualified-review-study/1` selecting
`prose-lab.support-review/3` exactly. It refuses `review_protocol`, even empty/null.
Add closed trial /2 with the same fields plus required `review_protocol`, whose
only admitted value is `prose-lab.support-review/4`. Unknown values refuse before
client creation. Both versions retain exact85 case coverage, source/floor/prior
pins, budgets, marker ordering, native replay, privacy and authority laws.

Do not change old /3 request instructions or its four-field decoder. Preserve
actual old native request hashes and the failed trial report byte-for-byte under
the new source. Protocol selection enters study, preparation, journal, marker,
response artifact and replay identity; replies cannot cross the selected codec.

## Protocol 4 wire

Provider input retains criterion 2 plus a separately versioned response instruction
and the same assertion/cited-IDs/all-typed-facts projection. No oracle labels,
reviewer bases, case/control IDs or outcome metadata enter the provider request.

Return closed JSON with five required fields:

| Field | Law |
|---|---|
| status | supported, unsupported or inconclusive |
| reason | Nonblank UTF-8,<=4096 bytes; concise source/criterion rationale |
| examined_fact_ids | Unique1..256 registered facts from the exposed case |
| premise_fact_ids | Unique0..8 IDs, all cited and examined; nonempty for Supported |
| counter_fact_ids | Unique0..256 IDs, all exposed and examined |

The latter lists may be empty for a citation/missing-premise gap. Uniqueness is
within a list; a composite fact can be examined as both premise and counter.
Examined-source metadata does not claim actual model attention. A reference is
never upgraded into a valid positive premise merely because it was examined.
Inconclusive still means unresolved meaning or material source conflict, not a
missing/invalid JSON result. Every current source/authority hold remains intact.

The instructions explicitly explain the citation-gap case. They require examined
references for every status and prohibit inventing support/counterfacts to fill a
field. This changes the response protocol, not the reference labels or criterion.
Do not generate hidden model reasoning; retain only concise answer rationale.

Record the expanded decision in private reason artifacts and full-content reports.
Ordinary reports remain source/identity-hidden and expose no claim content.
The real encoder, capture projection and offline replay use the selected same
protocol. Required/null/unknown/duplicate/Unicode/byte checks apply in original JSON.
CUE owns each version's shape and intrinsic reference/list relations; Go binds
actual source membership, cited/expected scope and exact native history.

## Acceptance

- ES-A01: Old85 native /3 preparations and the retained failed 67-case native
  report reproduce unchanged without key/provider access. Old /1 trials reject
  the new selector; new /2 requires exactly /4 and cannot consume /3 replies.
- ES-A02: Citation gap with nonempty examined IDs and empty premise/counter lists
  is representable; empty/missing/foreign examined IDs and uncited positives refuse.
- ES-A03: Premise/counter sets must be examined; overlap is valid; Supported
  cannot pass with no cited positive premise. Go/CUE actual-file parity covers
  literal/null/duplicate/multibyte/oversize and mixed-version negatives.
- ES-A04: Exact85 injected-native /4 replay, planted false support, invalid reply,
  pre-dispatch failure, accounting, output/privacy and stop/denominator laws pass.
- ES-A05: Full gate, independent code review and strict source proof precede a
  separately frozen full 85 real trial under remaining budget. It is not document
  acceptance, calibrated accuracy, source truth or UI entry.

Reject forced synthetic counterevidence, post-hoc /3 repair, new oracle labels,
discarded failed positions, a same-journal retry and a new universal evidence kit.
Extraction waits for a second concrete consumer. Cost benefit remains unmeasured.
Revisit on failed source-basis expression, critical miss, stale replay or budget.
