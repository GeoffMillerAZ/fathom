package consumer

import "github.com/geoffmilleraz/agentic-kit/kits/catalog-kit:catalogkit"

spec: catalogkit.#Spec & {
	version: 1
	layers: [{
		name: "base"
		nodes: [{slug: "review", stable_key: "review", source: "base.cue", children: [{slug: "evidence", stable_key: "review.evidence", source: "base.cue", title: "Evidence"}]}]
	}, {
		name: "project"
		requires: ["base"]
		nodes: [{slug: "release", stable_key: "release", source: "project.cue", title: "Release"}]
	}]
}

profile: catalogkit.#Profile & {name: "consumer", roots: ["project"]}
policy: catalogkit.#Policy & {limits: {max_layers: 4, max_nodes: 20, max_depth: 4, max_bytes: 65536}}
identity: catalogkit.#DecisionIdentity & {
	provider: "offline-fixture"
	requested_model: "example"
	resolved_model: "example"
	adapter_version: "1"
	policy_version: "1"
}
