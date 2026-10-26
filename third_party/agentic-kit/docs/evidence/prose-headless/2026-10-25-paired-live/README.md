# Paired support-review trial: qualification failed

Source: 9036e94175f6fae611cf6f91338f9274a75a3948. This single frozen
public-synthetic trial stopped at execution108 of140, exactly at its first
candidate mismatch. No retry, fallback, dropped position or retrospective label
change occurred. The candidate is not adopted. The three scope-regression
extensions were not reached, so their improvement hypothesis is untested.

| Observation | Baseline | Candidate |
|---|---:|---:|
| Planned | 70 | 70 |
| Known completions | 54 | 54 |
| Matches to fixed labels | 54 | 53 |
| False holds against fixed labels | 0 | 1 |
| False support / inconclusive / invalid / unknown | 0 | 0 |
| Unexecuted | 16 | 16 |
| Input tokens | 59,699 | 66,989 |
| Output tokens | 5,062 | 5,028 |
| Usage-priced microUSD | 170,018 | 184,258 |
| Native latency, summed seconds | 82.591 | 144.760 |

These are54 paired logical controls, not108 independent examples. The candidate
cost14,240microUSD more on this observed prefix; neither cost nor latency is a
population estimate. The longer prompt added135 input tokens per attempted pair.
Cache usage was zero. None of the three added development cases ran.

## Stop and semantic limit

The fixed positive operational-runbook--p-03 says that an operator may restart one
stalled worker only after confirming its lease has expired. It cites f-06, which
allows restart after confirmation. The candidate returned unsupported; the
baseline returned supported. Capture and report exit1; the supervising driver
exit2 preserves the failed qualification.

The candidate's ordinary reason confuses a necessary condition with the sole
condition when invoking other checks. A blind independent agent and owner review
also found a genuine oracle ambiguity: permissive after wording is a reasonable
operational prerequisite, but does not unambiguously establish only-after under
strict entailment. Context f-07 prohibits active-lease restarts without separately
settling expired-but-unconfirmed leases. Neither model opinion establishes source
truth. The historical label and failed result remain unchanged. Report false_hold
means disagreement with the fixed label, not an independently proven model error.

[0047](../../../intent/decisions/0047-qualify-oracle-semantics-before-prompt-tuning.md)
and the [oracle contract](../../../designs/prose-oracle-qualification.md) require
source/semantic adjudication before a new calibrated site or document study.

## Evidence and custody

- [Frozen protocol](protocol.json), [study](study.json) and
  [140 preparations](preparations.json) identify the prospective selection.
- [Report](report.json) retains all140 positions, including32 missing after stop.
- [Native audit](native-audit.json) retains request/response digests, observed
  statuses, usage, cost and latency for every attempted position.
- [Receipt](receipt.json) binds exact source, clean binary, private driver,
  prospective owner labels, independent source/offline review and actual accounting.

Owner and independent agent audited all108 native responses: selected model,
HTTP200, end_turn, standard service tier, global inference and standard speed.
Requests matched frozen digests; each pair differed only in the system prompt.
Ordinary answers, private reason files and receipts agreed. No hidden-thinking
block occurred. Raw native responses and rationales remain private. Digests prove
byte identity, not authenticated provider/source custody.

126,688 input tokens at2microUSD plus10,090 output tokens at10microUSD equal
354,276microUSD ($0.354276). Prior424,414 plus354,276 equals778,690microUSD
($0.778690) and197 known native calls across the retained direct-stage ledger.
These are local usage-priced estimates, not invoices or account balances.
Reservations were4,040,960microUSD before each dispatch; each settled before the
next. New cap5,000,000 and direct-stage cap10,000,000 were preserved.

Twelve owner and twelve independent offline runner tests passed before dispatch.
Those tests prove fake/source controls. This trial separately observes actual
provider behavior. Full local publication gate is recorded in the receipt.

## Remaining gates

HP-17 qualification, HP-18 generation guidance, the required1200..1600-field long
artifact, complete document matrix, independent sufficiency, actual human goals,
HITL and UI remain open. Original failed studies stay failed. No acceptance,
publication, source authentication or model-site adoption follows.
