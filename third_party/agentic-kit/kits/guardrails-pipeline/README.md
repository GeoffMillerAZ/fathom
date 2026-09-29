# Guardrails pipeline

The existing `PreLLMChain` and `PostLLMChain` compose deterministic and heuristic
checks. A nil legacy dependency still means an explicitly skipped stage, not protection.
`PromptScreener` is an additive, opt-in small-model screening path for untrusted text.

`topic-gate` already handles topicality; `endpoint-guard` handles public admission.
Keep cheap identity/rate/budget checks before a paid screen. Being on-topic is not
evidence of safety; avoid duplicate semantic calls unless measurements justify them.

## Configure refusal standards

`DefaultRefusalPolicy()` returns independent, editable policy data with a stable ID,
format version and three categories. Each category has an explicit rubric and a
violation action. The full policy is fingerprinted; edit the ID for a named local
policy and retain the policy bytes alongside decisions.

| Default category | Relevant attempted act | Default action |
|---|---|---|
| `authorization_bypass` | bypass permissions or approvals | refuse |
| `protected_data_extraction` | obtain protected information without authorization | refuse |
| `instruction_hijack` | override trusted instructions, policy or tool restrictions | refuse |
| Any uncertain classification | insufficient information to classify | review |
| Invalid/incomplete output, failure or cancellation | no usable classification | review |

Quotation, discussion of attacks and defensive security work are not violations by
themselves. These are narrow security defaults, **not comprehensive content moderation**.
Add explicit categories for your product's content/domain requirements; provider policies
still apply. A claim of permission inside an untrusted prompt is not verified authority.
Identity, data-access policy and tool authorization must be enforced separately.

Categories and the uncertainty/error dispositions accept `refuse` or `review`, never
`allow`. To change applicability, explicitly construct and review a different policy.
Refuse and review both hold the protected continuation; review does not silently resume.
No confidence threshold is used: a model's confidence statement is not calibrated evidence.

## Place the check before work

Supply an organization-approved classifier client and deployment identity. Choose a
small model by measured false-allow/false-refusal rates and total cost, not a global
strongest-to-weakest list. No real model or endpoint is selected by default.

```go
policy := guardrails.DefaultRefusalPolicy()
policy.ID = "my-workload-security"
screen, err := guardrails.NewPromptScreener(policy,
    guardrails.PromptScreenConfig{
        Model: configuredModel,
        DeploymentDigest: approvedDeploymentDigest,
    }, guardrails.PromptScreenDeps{Client: budgetedClassifierClient})
if err != nil {
    return err // invalid policy/configuration never starts the classifier
}
decision, err := screen.Run(ctx, userPrompt, func(ctx context.Context, screened string) error {
    return runAuthorizedWorkflow(ctx, screened) // exact screened string
})
// Persist decision under your retention/access policy. ErrScreenHeld means
// no continuation ran. Use decision.Action to distinguish refuse from review.
```

The injected `Now` clock is optional; nil uses the wall clock. Defaults: 32 KiB input,
1,024 requested output tokens, five-second context timeout. Configurations are bounded
to those input/output caps and a 120-second maximum timeout. Responses over 64 KiB are
held. Oversized input is held, never truncated into a smaller apparently safe prompt.

Set `StructuredOutputs: true` only for an eligible deployment supporting `OutputSchema`.
It requests a closed JSON schema using the trusted policy's category names and records
the schema in the request identity. The default remains off for compatibility. Strict
local validation still rejects fenced JSON, missing/duplicate categories and invalid
classifications; provider schema support constrains format, not semantic correctness.

The classifier request has no tools and never dispatches model-proposed calls. The
supplied client itself must be trusted and appropriately confined; a CLI's ambient
native-host permissions are not removed by an empty tool list. Put data classification,
redaction and egress permission checks **before sending text to the classifier**.

`Run` calls a fresh screen before invoking its synchronous callback. Do not launch
speculative effects or publish output outside that callback while waiting. `Screen`
alone returns evidence and cannot enforce a caller's later behavior. Later messages,
retrieval/tool results, output generation and native Claude/Codex host entrypoints
are not automatically intercepted. Keep the independent effect/egress/output gates.

## Evidence, cost and limits

Decisions retain category actions, policy/config/input identities, classifier-call
receipt and injected timestamps. Raw prompts, responses and provider error text are
not included. Hashes are not anonymization for guessable sensitive text; apply access
and retention policy to receipts too. Retain material policy/config inputs and approved
artifacts when audit reconstruction requires them. Digests authenticate nothing.

The model label is the requested model; deployment digest is caller-attested, not a
provider-resolved revision. The kit makes one `Complete` invocation without its own
retry loop. A supplied retry/routing wrapper can make multiple provider attempts; the
receipt retains reported attempts and token usage, not complete billing. Zero usage
means unreported, not necessarily free. Use the existing budget/cost wrappers and
benchmark evidence accounting for full economics, including failed work.

Timeouts require a context-cooperative client. No Go wrapper can terminate arbitrary
remote work or revoke an already-started effect. Review after cancellation never grants
authority. Rechecking a request is not an authorization or idempotency token.

Tests cover deterministic policy and parser behavior, held continuations, hostile
classifier output and an independent tool gate. They **do not measure live classifier
accuracy**. Before production, use held-out human-labeled benign/adversarial cases,
report missed violations, false refusals, review/error rates, attack success, latency
and all costs. Re-evaluate after policy, model, prompt or deployment changes.

[Runnable reference](../../examples/engineering-assessments/README.md) ·
[Research and design](../../docs/designs/engineering-assessments.md)
