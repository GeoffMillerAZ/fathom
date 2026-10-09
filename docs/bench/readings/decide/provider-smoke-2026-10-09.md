# Decision providers, live smoke: OpenAI Decisions and Jev over OpenRouter (2026-10-09)

## Verdict, plainly

**Jev over OpenRouter works through fathom's decision port as wired: 8 of 8 calls
answered, every answer parsed, 256-516 ms per call, US$0.00017 for all eight.**
**OpenAI's Decisions API could not be read: the endpoint and the key are real, but
the account has no credit, so all 8 calls came back `429 insufficient_quota` and no
answer shape was seen.** Nothing here says either model is right often enough to
trust; this is a wiring and wire-shape check, not a calibration reading. Keep both
providers optional (`provider = "none"` stays the default). Pair Jev with the sites'
deterministic paths, which still decide whenever it answers nothing; re-run the
OpenAI half once the account has credit before turning it on anywhere.

One fix came out of it, landed in agentic-kit `9ecaf13` and pinned here:
OpenAI's quota-exhausted 429 had read as a retryable rate limit
(`OpenAIError.Retryable()` true), so a caller backing off would retry a request that
cannot succeed. It is now `QuotaExhausted()`, not retryable, and fathom's degrade line
says `quota exhausted`, not `rate limited`.

## What ran

`internal/decide/live_smoke_test.go` (build tag `decidelive`, also guarded by
`FATHOM_DECIDE_LIVE=1`, so never part of the gate), through `decide.Open` — the same
constructor every fathom entry point uses — then the adapter `Open` built:

```sh
FATHOM_DECIDE_LIVE=1 FATHOM_DECIDE_LIVE_OUT=<file.json> \
  FATHOM_DECIDE_LIVE_OPENROUTER_KEY_FILE="$HOME/secrets/<provider>/token" \
  go test -tags decidelive -run TestLiveProviderSmoke -count=1 -v ./internal/decide
```

| | OpenAI | Jev over OpenRouter |
|---|---|---|
| `[decide]` | `provider = "openai"`, `api_key_env = "OPENAI_API_KEY"` | `provider = "typesafe"`, `transport = "openrouter"`, `api_key_file` (chmod 600) |
| Endpoint | `POST https://api.openai.com/v1/decisions` | `POST https://openrouter.ai/api/alpha/decisions` |
| Model asked | `gpt-6-luna` | `typesafe/jev-1.13` |
| Calls (cap 20 each, owner-approved) | 8 | 8, plus 1 `fathom doctor` ping |
| Answered | 0 (all `429 insufficient_quota`) | 8 |
| Input / output tokens | none billed | 4,074 / 504 |
| Cost | US$0 | US$0.000171 (OpenRouter's own `usage.cost`, summed) |
| Latency per call | 103-394 ms to the 429 (1,155 ms for the first, with TLS setup) | 256-516 ms (`Response.Latency` and wall clock agree within 3 ms) |

The prompts are fathom-shaped, two sets of three questions, asked singly and then as
one batch of three over a combined state: a predicate (does this obligation govern
this diff), a choice over the route verb set (`localize`, `find_symbol`, `references`,
`find_callers`, `obligations`, `staleness_report`, `search`, `none`), and a score of
test strength on four levels (no assertion, smoke, one value, values and a failure
path). Set 1 should read true / `find_symbol` / 3; set 2 false / `localize` / 0.

One more OpenRouter call (nine in all) checked the product entry point: `fathom doctor`
in a scratch repository whose `.fathom/config.toml` holds the OpenRouter block above
printed (the key file's path shown as the placeholder the docs use)

```
decision_provider    PASS    [decide] provider=typesafe model=typesafe/jev-1.13 transport=openrouter endpoint=https://openrouter.ai/api/alpha/decisions key=key file "~/secrets/<provider>/token" (present, mode 0600) ping ok (sites off: route); its thresholds need a calibration reading (decision-port-calibration-*.md) before they mean anything for this provider
```

The raw exchanges are beside this file: `provider-smoke-2026-10-09.openai.json` and
`provider-smoke-2026-10-09.jev-openrouter.json` — request and reply bodies, statuses
and timings only. No header was recorded, every body was passed through a key-shape
redactor, and neither key was read by the test (the adapters read the variable and the
file).

## Side by side

OpenAI has no column of answers: every call was refused for credit before a model ran.

| Question | Expected | Jev, asked alone | Jev, in the batch of three |
|---|---|---|---|
| set 1 predicate: "symbol: anchors name functions only" vs a diff adding `symbol: Decide` (a struct) | true | 0.42 (confidence 0.58) | 0.51 (0.51) |
| set 1 choice: "where is decisionProviderVerdict defined?" | `find_symbol` | `find_symbol` (1.00) | `find_symbol` (1.00) |
| set 1 score: a test asserting a FAIL, a `chmod 600` remedy and no key in the reason | 3 | 2.99 (0.99) | 2.97 (0.97) |
| set 2 predicate: "deploy is task deploy, never cp" vs a diff to `decidekey.go` | false | 0.02 (0.98) | 0.03 (0.97) |
| set 2 choice: "TestDecideDegradesToOneDisclosedLine fails with ..." | `localize` | `localize` (0.99) | `localize` (0.92) |
| set 2 score: a test that calls `decide.Open` and asserts nothing | 0 | 0.13 (0.83) | 0.60 (0.37) |

Ten of twelve answers land where expected. The set 1 predicate is the hard one (the
diff breaks the rule by naming a struct, which needs the reader to know `Decide` is a
type) and Jev sits at the fence, which its own confidence says. In the batch the weak
test drifts from 0.13 to 0.60 with confidence 0.37: one shared state that mixes an
obligation, a prompt and a test makes "the test" ambiguous, so a site should batch
questions about one state, not unrelated states glued together. Six answers per
column is not a calibration; it only shows the wire works and the answers are not
noise.

## Shapes against the spec

**Jev over OpenRouter.** Request as the kit sends it: `{model, state: string,
questions: {<id>: {type: noul|choice|score, instructions, criteria}}}` — `criteria` is
`{true, false}` for a noul, `{<option>: <description>}` for a choice, and the ordered
level list for a score; `instructions` is sent on every question (OpenRouter requires
it). Reply: `{model, answers: {<id>: ...}, usage: {input_tokens, output_tokens, cost},
id, provider}`, with noul `{type, noul}`, choice `{type, choice, probabilities:
{<option>: p}, confidence}`, score `{type, score, legend: {<index>: <level>},
probabilities: {<index>: p}, confidence}`. Drift from what the adapter reads, all
additive and ignored by it: top-level `id` (`gen-dec-...`) and `provider`
(`"TypeSafe"`), `usage.cost` in US dollars, and a score's `legend`. The model echoed is
the dated build, `typesafe/jev-1.13-20260917`, which fathom's decision log records. A
string `state` was accepted although OpenRouter's SDK examples pass an object. Nothing
the adapter needs was missing or renamed: no adapter change.

**OpenAI.** Request as the guide specifies and the kit sends it: `{model, input:
string, questions: [{type: predicate|choice|score, name, instructions, choices:
[{value, description}] | levels: [{label}]}]}` (a predicate's true and false criteria
are folded into `instructions`). Reply: `429 {"error": {"message": "You have no credits
remaining. ...", "type": "insufficient_quota", "param": null, "code":
"credit_balance_exhausted"}}` on every call — the endpoint exists (no 404) and the key
authenticated (no 401). The answer shapes (`answers[]` of predicate `{probability}`,
choice `{choice, probabilities[], confidence}`, score `{score, probabilities[],
confidence}`, refusal `{type: "refusal", name}`) remain as the guide documents them and
unverified live. The one live shape seen, the quota 429, differed from the adapter's
reading of it; that is the kit fix above.

## Costs, from these numbers

Jev over OpenRouter billed US$0.042 per million input tokens and nothing for output
(171.1 micro-dollars / 4,074 input tokens). A single-question call carried 374-477
input tokens for states of a few hundred characters, so a fixed overhead of roughly
350 tokens per call dominates small questions: the batch of three cost 780 tokens
against 1,296 for the same three asked alone. OpenAI lists US$0.10 per million input
tokens and no output charge; nothing was billed here.

## What is not claimed

No accuracy, calibration or threshold claim for either provider (ADR-056 R3: that needs
a reading on labelled cases); no OpenAI answer shape; no latency under load. The
OpenAI run made all 8 of its calls before the smoke's recorder knew a quota 429 as a
refusal (it now stops a provider at the first one, and at a 402, as it already did at a
401, 403 or 404); OpenAI was not called again. Both providers stayed under the 20-call cap.
