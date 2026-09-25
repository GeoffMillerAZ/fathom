# Jev adoption assessment

Date: 2026-09-22  
Status: research recommendation; no app-kit provider integration is implemented or admitted

## Decision summary

Keep app-kit's catalog search, MCP responses, adoption checks and cursors fully
deterministic. Jev may be worth a small, opt-in experiment as a second-stage judgment
over an already eligible candidate set, but current evidence does not justify adding a
provider, changing default ranking or creating a new source of catalog truth.

[ADR-0062](../adr/0062-search-declared-capability-tasks.md) remains authoritative:
normalized lexical matching against IDs, names and authored positive task phrases owns
recall and refusal. A model cannot rescue zero candidates without a separately reviewed
recall design. Vocabulary repair remains the first response to a missed supported task.

Applicable intent is I8/I15/I18: unknown evidence stays explicit, a proposal is not a
pass, and catalog declarations do not become runtime or adoption proof. No current
future, compatibility promise or provider availability claim is promoted here.

## Verified public capability and transport

TypeSafe documents Jev as a text-only System One model: it evaluates string or
structured-text state and returns typed decisions rather than prose. Its primitives are
`Noul` (probability of yes), `Choice` (one option plus a probability distribution) and
`Score` (a rubric-weighted value). TypeSafe advises atomic questions recombined by
deterministic application code; calibration over groups does not guarantee an individual
answer. Sources: [System One](https://docs.typesafe.ai/concepts/system-one),
[API](https://docs.typesafe.ai/api), and [models](https://docs.typesafe.ai/models).

OpenRouter publicly lists `typesafe/jev-1.13` as a text-to-decisions model. Its official
[`ai-sdk-provider` README](https://github.com/OpenRouterTeam/ai-sdk-provider/blob/main/README.md#evaluation-jev-with-ai-sdk-through-openrouter)
documents shipped `decisionModel` / `evaluationModel` support over the Decisions base
`https://openrouter.ai/api/alpha` for AI SDK 7.0.103 and later. This remains an alpha
transport, not the Chat Completions endpoint, and is evidence of availability rather
than a direction to add its Node 22/ESM runtime or dependencies to this Go kit.

The [OpenRouter model page](https://openrouter.ai/typesafe/jev-1.13) listed, on this
assessment date, 32K context and $0.042 per million input tokens with output listed as
free. That is dated vendor-list pricing, not an app-kit bill, benchmark or measured cost.
No authenticated request, paid inference or credential read was performed for this
assessment. `typesafe-jev` is not an installed app-kit feature.

## Bounded opportunity

Two uses have modest plausible value:

1. Re-rank a strict ID allowlist returned by existing catalog search for a declared task.
   Preserve the same maturity/native/reference groups; a model may only order candidates
   inside its allowed group.
2. Triage an adoption question into bounded labels such as likely fit, insufficient
   evidence or needs human review, using only approved first-party catalog metadata.

Do not use Jev as a screenshot judge, converter, compatibility oracle, accessibility or
security certifier, maturity authority, provenance source, adoption proof, or permission
to mutate a consumer. It must not see licensed source, private design assets, live logs,
consumer data, credentials or raw secrets.

## Required fallback and receipt contract

- Provider mode is off by default. Default search results, serialization and cursors are
  byte-identical to today's behavior.
- Candidate IDs are produced and validated locally. Returned IDs outside the allowlist
  are rejected; no model-created capability can enter results.
- Unavailable access, timeout, malformed output, unknown answer, budget exhaustion or
  confidence below a calibrated threshold leaves the baseline order unchanged and emits
  an explicit optional-provider status. There is no silent model-to-model fallback.
- Pin both the requested model and the versioned returned model. Bind receipts to the
  catalog revision, query, candidate IDs, question policy, endpoint, actual upstream
  provider, fallback/routing controls and price version.
- The official adapter rounds probabilities and scores to two decimals; preserve the
  returned precision and do not imply finer confidence or calibration.
- Credentials and egress policy are operator-owned external configuration. Repository
  content cannot choose an endpoint, credential variable or broader source scope.
- Treat provider calls as external disclosure. TypeSafe's [data handling](https://docs.typesafe.ai/models#data-handling)
  points to zero data retention for enterprise customers; it does not establish
  ordinary-account ZDR, so verify the applicable OpenRouter and upstream endpoint policy.

## Reuse boundary

Fathom's current in-tree agentic-kit mirror is stamped
`beb53a4fb901d919c19670877e8c09abacbc0531` and contains the public package
`github.com/geoffmilleraz/agentic-kit/foundations/decision`: `Decider`, typed
request/question/answer records, `None`, generic LLM and direct TypeSafe adapters,
test seams and explicit no-provider errors. Fathom already consumes that seam.
The canonical agentic-kit checkout inspected for this
assessment was on another dirty branch at `fac9fe7` and did not contain the package in
its working tree, although the stamped commit exists on its local `main`. Therefore this
is committed shared work, not a currently adopted app-kit dependency.
The updated port accepts explicit ordered score levels; direct TypeSafe and OpenRouter
remain different transports. Review answer/distribution and confidence provenance before
reuse; absent probability evidence must not become synthetic certainty.

Review Kit's working-tree `kits/review-kit/internal/judgment` adds a bounded OpenRouter
transport, ledger reservation, exact-source receipts and returned-model checks. It is an
`internal` package in dirty cross-repo work, so app-kit cannot depend on it. It is a
promotion candidate if a shared provider adapter is later authorized. Do not copy it or
automatically extract cross-repo code. Any promotion needs its own owner, dependency
direction review and clean evidence.

A later read-only refresh inspected Review Kit's
[live synthetic receipt](../../../platform-engineering/kits/review-kit/docs/assessments/2026-09-22-openrouter-jev-smoke.md).
That other authorized effort made two requests: one refused the returned model identity,
then an explicitly pinned request produced an advisory report. Crucially, both responses
called an uninformative, author-labeled **unknown** case satisfied (probability0.59).
Its archived regression preserves that error. This is transport evidence, not an app-kit
trial or quality win; missing-evidence/abstention cases must remain in the holdout.
Reuse the public port if admitted; promote the reviewed OpenRouter policy/billing/receipt
controls into a shared package before integration rather than duplicate the transport.
No app-kit credential read, remote call or provider integration occurred in this refresh.

## Small evaluation before promotion

Use 40 synthetic, metadata-only cases with held-out, independently adjudicated labels:
supported fits, close alternatives, insufficient evidence and prohibited/invented tasks.
Compare deterministic baseline versus optional reranking on top-1/top-3 relevance,
refusal preservation, abstention, repeated-answer stability, latency and failures.
Replay the current selection/refusal corpus (73 tasks after the signed-value additions)
and require no safety regression.

Bound the whole experiment to at most 250,000 input tokens, a $0.05 estimated envelope
at the dated list price, a two-second request deadline, five-minute run cap, no retry,
and retained request/model/usage
receipts without secrets. This is an estimate ceiling, not prepaid or measured spend.

Promote only if held-out benefit materially beats a reviewed task-vocabulary fix while
all default/refusal/privacy/budget contracts remain intact. Kill the experiment if gains
are absent, unstable or explainable by vocabulary repair, or if transport, model identity,
privacy, egress or cost cannot be bounded. Until that gate passes, documentation may name
the experiment, but MCP and catalog surfaces must not advertise Jev availability.
