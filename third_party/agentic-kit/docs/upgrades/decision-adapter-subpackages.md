# Decision port: the adapters move to `decision/llm` and `decision/typesafe`

`foundations/decision` is the port: the kinds, `Request`, `Response`, `Decider`,
`Validate`, `Confidence`, the decorators and `None`. The two provider adapters now live
in their own packages, which import the port; the port imports neither of them, nor
`foundations/llmclient` or `net/http` (plan row DP-14,
[decision 0017](../intent/decisions/0017-fast-judgment-is-optional-and-calibrated.md)).
A consumer that never builds an adapter no longer links a completion client or an HTTP
transport through the port, and a better or different provider plugs in as one more
package beside them.

## What changed

- Import paths. `LLM` moved to `github.com/geoffmilleraz/agentic-kit/foundations/decision/llm`
  (package `llm`) and `TypeSafe` to
  `github.com/geoffmilleraz/agentic-kit/foundations/decision/typesafe` (package `typesafe`).
- Constructor and type names are unchanged; only the qualifier moves, so the names
  stutter on purpose:

  | Before | After |
  |---|---|
  | `decision.NewLLM`, `decision.LLM`, `decision.LLMOption` | `llm.NewLLM`, `llm.LLM`, `llm.LLMOption` |
  | `decision.WithMaxTokens`, `decision.WithNow`, `decision.DefaultMaxTokens` | `llm.WithMaxTokens`, `llm.WithNow`, `llm.DefaultMaxTokens` |
  | `decision.NewTypeSafe`, `decision.TypeSafe`, `decision.TypeSafeConfig` | `typesafe.NewTypeSafe`, `typesafe.TypeSafe`, `typesafe.TypeSafeConfig` |
  | `decision.TypeSafeError` | `typesafe.TypeSafeError` |
  | `decision.DefaultTypeSafeBaseURL`, `decision.DefaultTypeSafeModel` | `typesafe.DefaultTypeSafeBaseURL`, `typesafe.DefaultTypeSafeModel` |

- `KindNoul` is removed. It was a deprecated constant equal to `KindPredicate` since
  DP-12 ([note](decision-kind-predicate.md)); write `decision.KindPredicate`. The decode
  alias is not removed: `ParseKind("noul")`, `Kind.UnmarshalText`, CUE `#Kind` and a raw
  `Kind("noul")` still mean `KindPredicate`.
- Two exported functions are new in the core, for adapters written in their own
  packages: `decision.CanonicalKind(k)` folds the `"noul"` alias into `KindPredicate`,
  and `decision.OutsideDeclaredLimits(provider, caps, q)` is the declared-limit refusal
  `TypeSafe` uses. Both wrap helpers the core already had.
- `foundations/decision/typesafe` carries a `schema.cue` with `#TypeSafeConfig` (the
  data fields of `TypeSafeConfig` and `NewTypeSafe`'s defaults).

## What did not change

- The wires. The `LLM` prompt and OutputSchema and the `TypeSafe` request body are
  rendered by the same code; the three golden fixtures (`llm/testdata/prompt.golden`,
  `llm/testdata/schema.golden`, `typesafe/testdata/typesafe_request.golden`) moved
  byte-identical, so no prompt calibration needs re-measuring.
- `Decider`, `Request`, `Question`, `Answer`, `Response`, `Record`, `Capabilities` and
  every decorator. A `*llm.LLM` or `*typesafe.TypeSafe` is a `decision.Decider` and a
  `decision.CapabilityDeclarer` exactly as before, and its declared Capabilities are the
  same.
- Behavior and errors. Error strings, `TypeSafeError.Retryable`, `TypeSafe.Ping`, and the
  rule that `NewTypeSafe` refuses an empty key are unchanged.
- Both adapters still pass `decisiontest.RunConformance` (`TestLLMPassesConformance`,
  `TestTypeSafePassesConformance`, now in their own packages).

## What to do

1. Add the import for each adapter you construct:

   ```go
   import (
       decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
       "github.com/geoffmilleraz/agentic-kit/foundations/decision/llm"
       "github.com/geoffmilleraz/agentic-kit/foundations/decision/typesafe"
   )
   ```

2. Change the qualifier at each construction and type use: `decision.NewLLM(client, model)`
   becomes `llm.NewLLM(client, model)`, `decision.NewTypeSafe(decision.TypeSafeConfig{...})`
   becomes `typesafe.NewTypeSafe(typesafe.TypeSafeConfig{...})`, and an
   `errors.As(err, &tse)` target becomes `*typesafe.TypeSafeError`. Code that holds the
   result as a `decision.Decider` needs no other change.
3. Rename a local variable called `llm` or `typesafe` if it now shadows the package.
4. Replace any remaining `decision.KindNoul` with `decision.KindPredicate`.
5. A Decider you wrote yourself that copied the core's alias folding or limit check can
   call `decision.CanonicalKind` and `decision.OutsideDeclaredLimits` instead.
