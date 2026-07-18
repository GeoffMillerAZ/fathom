// valid.cue — a generic intent classifier rule set.
// Conforms to #RuleSet defined in schema.cue.
package intentclassifier

import "github.com/geoffmilleraz/agentic-kit/foundations/retrieval/intent-classifier:intentclassifier"

// SupportRuleSetV1 is a valid rule set that satisfies #RuleSet.
// Intents are generic domain labels — not org-specific concepts.
SupportRuleSetV1: intentclassifier.#RuleSet & {
	Name:    "generic-support-v1"
	Version: "1.0.0"
	Rules: [
		{
			Intent:   "search"
			Keywords: ["find", "search", "look up", "locate", "where is"]
		},
		{
			Intent:   "summarize"
			Keywords: ["summarize", "summary", "tldr", "brief", "overview"]
		},
		{
			Intent:   "explain"
			Keywords: ["explain", "what is", "describe", "how does", "tell me about"]
		},
		{
			Intent:   "compare"
			Keywords: ["compare", "difference", "versus", "vs", "contrast"]
		},
		{
			Intent:   "list"
			Keywords: ["list", "enumerate", "what are", "give me all", "show me all"]
		},
	]
}

// InvalidRuleSet demonstrates a value that does NOT satisfy #RuleSet because
// the Intent field is an empty string (violates #Intent constraint).
// Uncomment to observe: cue vet will reject it.
//
// BadRuleSet: intentclassifier.#RuleSet & {
// 	Name:    "bad-example"
// 	Version: "0.0.1"
// 	Rules: [
// 		{
// 			Intent:   ""          // ERROR: #Intent requires non-empty string
// 			Keywords: ["foo"]
// 		},
// 	]
// }
