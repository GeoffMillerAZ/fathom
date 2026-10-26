package fixtures

import "github.com/geoffmilleraz/agentic-kit/kits/catalog-kit:catalogkit"

spec: catalogkit.#Spec & {
	version: 1
	layers: [{
		name: "general"
		nodes: [{slug: "review", stable_key: "review", source: "general.cue", children: [{slug: "evidence", stable_key: "review.evidence", source: "general.cue", title: "Evidence"}]}]
	}, {
		name: "team"
		requires: ["general"]
		nodes: [{slug: "delivery", stable_key: "delivery", source: "team.cue"}]
	}]
}
policy: catalogkit.#Policy & {limits: {max_layers: 8, max_nodes: 100, max_depth: 8, max_bytes: 65536}}
profile: catalogkit.#Profile & {name: "project", roots: ["team"]}
identity: catalogkit.#DecisionIdentity & {
	provider: "fixture"
	requested_model: "configured"
	resolved_model: "recorded"
	adapter_version: "1"
	policy_version: "1"
}
