// valid.cue — positive fixture for the plan schema.
// This file must pass `cue vet ./...` from schema/plan.
package plan

plan: #Plan & {
	service: "my-service"
	tasks: [
		{
			id:    "T-1"
			title: "ValidateToken must exist and be tested"
			expect: {
				symbol: "ValidateToken"
			}
		},
		{
			id:      "T-2"
			title:   "US-042: charge flow covered"
			spec_ids: ["[US-042]"]
		},
	]
}
