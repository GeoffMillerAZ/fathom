# 0035: Command safety starts with offline qualification

Status: accepted
Type: security
Date: 2026-10-23

Request: "lets start building (but not yet installing) tooling for advanced classifiers and safety and governance decision tooling. we can also audit our most common bash commands as bash is likely the most dangerous tool."

## Context

The request also covers typed replacements, secret checks, optional local/advanced
classifiers, layered policy, optional Fathom, scoped hook maintenance, harness intent
and catalog/review skills. [Coverage and research](../../assessments/2026-10-23-command-governance.md)
retain these clauses and today's daily-brief addendum. Existing 0030-0034 preserve
domain authority and optional integration. `agent-hooks` is economic fail-open guidance;
bounded-exec has ambient host authority. Neither is a safety authorization boundary.
Exploratory command discovery exceeded its declared envelope (INC-20).

## Decision

- Amend W1's second-slot offline reference under 0032/0033 with `cmd/command-lab`
  audit and typed-operation assessment, [CG-1..7](../../designs/command-governance.md).
  Retain five engineering days, zero provider spend, <=64 MiB selected input,
  maintainer ownership and review 2026-11-21 or next release. No third slot.
- Implement bounded explicit-source intake, closed-vocabulary aggregate output,
  independently declared expected layers, restrictive composition and a command-local
  classifier seam over the existing decision port. No live models or commands run.
- Go is the default: stdlib fits the current module and existing contracts. Adopt
  Rust only for a separately measured unmet requirement, not an assumed speed benefit.
- Specify typed execution, independent authorization, scanner/evidence adapters,
  governed hook maintenance and harness review under CG-8..16. Keep these queued;
  no daemon, execution broker, global setup or mandatory external service is admitted.
- Mandatory controls and publication gates cannot be suspended through advisory
  tuning. A future single approval binds a scoped finite maintenance transaction;
  it is not blanket permission to turn off every hook or weaken org floors.
- Keep Policy Kit constructive floors/exceptions, Review Kit independent scope,
  Catalog Kit identity/tailoring and Fathom source evidence in their existing owners.
  Candidate config is reviewed data, not its own trusted authority. First-slice
  layer simulation does not authenticate organizations or enforce OS permissions.
- Add a project-local audit skill and explicit source/capability/coverage receipts.
  Preserve Pages unchanged. The brief-source trigger fired on 2026-10-23: native
  Ollama decision models, specialized triage and local scanners inform CG-10/11;
  each remains a qualification candidate, never an unattended permission gate.

## Rejected alternatives

- A model, regex or AST declares arbitrary Bash safe: cannot establish its effects
  or authority. Wins if: never as sole authorization; bounded analysis can inform review.
- Expand quality-gate or agent-hooks silently into a universal security executor:
  incompatible trust/fault contracts. Wins if: a separately reviewed migration proves
  controlled-path coverage and preserves existing behavior and policy authority.
- Blanket global hook-disable, denial-loop safety release or environment bypass:
  can erase mandatory controls. Wins if: never for this safety path; an independently
  authorized owner action outside this tool remains outside its guarantee.
- Ordinary precedence lets personal/project config weaken org floors: loses authority.
  Wins if: an explicit trusted delegation/exception permits that exact bounded change.
- Build a shell interpreter, new general policy engine, sandbox or provider framework:
  existing ports and external systems suffice. Wins if: two named consumers establish
  the same missing contract and bounded admission proves net value.

## Revisit when

Discovery exceeds a cap; output leaks untrusted data; a
missing layer or classifier failure improves eligibility; a maintenance lease extends
itself; or a named consumer admits execution/installation with independent negative
proofs. Review date still applies. Broader AE/G15 goals remain open; no exported API,
global/consumer configuration, gate or compatibility-tier change is authorized here.
