package llm

import decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"

// llmMaxOptions is the most Options LLM declares for a KindChoice question:
// the contract it emulates allows 255, and Validate enforces the same bound.
const llmMaxOptions = 255

// Capabilities declares what LLM answers. It emulates every kind in ONE
// completion per Request, so batching questions per State saves calls, and
// declares no Score level limit of its own (it renders any Levels into the
// prompt; Validate's structural rules still apply). Its probabilities are
// self-reported: a generative model writes them as text, so they are
// calibrated separately from any native provider (docs/fast-judgment.md
// section 1).
//
// Latency and Cost are unknown: the wrapped llmclient.LLMClient may be any
// model, local or hosted, and llmclient.DescribeProvider's ProviderInfo
// carries no cost or latency field to read them from. Capabilities makes no
// call and reads no field, so it is safe on a nil *LLM.
func (l *LLM) Capabilities() decision.Capabilities {
	return decision.Capabilities{
		Declared:            true,
		Kinds:               []decision.Kind{decision.KindChoice, decision.KindScore, decision.KindPredicate},
		MaxOptions:          llmMaxOptions,
		SingleRoundTrip:     true,
		Latency:             decision.LatencyUnknown,
		Cost:                decision.CostUnknown,
		ConfidenceSemantics: decision.ConfidenceSelfReported,
	}
}
