// Positive CUE fixture for the self-contained local prose-check envelope.
package kitworkflow

import workflow "github.com/geoffmilleraz/agentic-kit/cmd/kit-workflow:kitworkflow"

valid: workflow.#Input & {
	version: "kit-workflow/prose/v1"
	sources: [{
		ref: {id: "source-a", version: "v1", digest: "6b51d431df5d7f141cbececcf79edf3dd861c3b4069f0b11661a3eefacbba918"}
		content: "12"
		media_type: "text/plain"
		acquired_by: "example"
		acquired_at: "2026-09-13T09:00:00Z"
		access: "public"
		retain_until: "2026-09-13T11:00:00Z"
		revoked: false
	}]
	current: [{id: "source-a", ref: {id: "source-a", version: "v1", digest: "6b51d431df5d7f141cbececcf79edf3dd861c3b4069f0b11661a3eefacbba918"}}]
	request: {
		principal: "reader"
		topic: "example"
		facts: [{
			span: {
				ID: "fact-a", EntityID: "entity-a", Field: "result", Value: "12", Confidence: 1
				SourceID: "source-a", RecordID: "v1", RecordHash: "6b51d431df5d7f141cbececcf79edf3dd861c3b4069f0b11661a3eefacbba918"
				OccurredAt: "2026-09-13T08:00:00Z", ObservedAt: "2026-09-13T09:00:00Z"
				Locator: {Kind: "byte-range", Value: "source-a", Start: 0, End: 1}
				Author: {Kind: "human", Human: {ID: "reviewer"}}
				Grade: {Tier: "documentary", Confidence: 1}
			}
			source: {id: "source-a", version: "v1", digest: "6b51d431df5d7f141cbececcf79edf3dd861c3b4069f0b11661a3eefacbba918"}
			unit: "items"
			historical: false
		}]
	}
	draft: {claims: [{id: "claim-a", assertion: "12", fact_ids: ["fact-a"], contradictions: []}], units: [{id: "unit-a", text: "12", claims: [{claim_id: "claim-a", start: 0, end: 2}]}]}
	config: {policy_version: "policy/v1", prompt_version: "prompt/v1", model: "supplied-draft", limits: {max_facts: 256, max_units: 256, max_claims: 512, max_bytes: 1048576, max_tokens: 8192}}
}
