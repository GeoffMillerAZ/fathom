// valid.cue — positive fixtures for the support matrix schema.
// This file must pass `cue vet ./...` from schema/support.
package support

targets: #Targets & [
	{
		kind:     "client"
		id:       "cursor"
		name:     "Cursor"
		target: {"mcp-tools": true, "obligations-post-edit": true}
		plan_row: "SM-13"
		wave:     1
	},
	{
		kind:     "coverage"
		id:       "lcov"
		name:     "lcov tracefile"
		target: {parse: true, "block-join": true}
		plan_row: "test-intelligence.md#TQ-3"
		wave:     3
	},
]

reading: #Reading & {
	kind:         "client"
	id:           "claude-code"
	reading_date: "2026-09-27"
	commit:       "86d4c5e2"
	instrument:   "fathom harness verify --harness claude-code"
	metrics: {"events-observed": 7}
	met: {"mcp-tools": "excellent"}
}

ledger: #LedgerRow & {
	kind:         "language"
	id:           "typescript"
	capability:   "L1"
	level_met:    "L1"
	grade:        "good"
	reading_date: "2026-09-27"
	commit:       "86d4c5e2"
}

matrix: #Matrix & {
	schema:       "fathom.support/v1"
	generated_at: "2026-09-27T00:00:00Z"
	commit:       "86d4c5e2"
	entries: [
		{
			kind: "language", id: "typescript", name: "TypeScript", registered: true, wave: 1, engine: "tags"
			cells: [
				{capability: "L1", state: "proven-good", target: "L4", level_met: "L1", reading: "docs/bench/readings/lang/typescript.json", plan_row: "LB-Q-typescript", verdict: "open"},
				{capability: "L4", state: "tolerated", target: "L4"},
			]
		},
		{
			kind: "client", id: "cursor", name: "Cursor", registered: true
			cells: [{capability: "mcp-tools", state: "tolerated", target: true, plan_row: "SM-13", verdict: "open"}]
		},
		{
			kind: "ecosystem", id: "cargo", name: "Cargo", registered: false
			cells: [{capability: "upgrade-impact", state: "declared", target: true}]
		},
	]
}
