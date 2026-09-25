// First-party synthetic facts and independent expected outcomes. No vendor bodies.
package receipts

#Amount: null | int & >=0 & <=1000000000
#Fact: {
	product: "atlas" | "beacon" | "cedar"
	receipts_cents: #Amount
	refunds_cents: #Amount
	missing_reason: string
}
#Snapshot: {
	period: =~"^2024-(08|09|10|11)$"
	coverage_end: string
	observed_at: string
	facts: [#Fact, #Fact, #Fact]
}

fixture: {
	now: "2024-12-05T12:00:00Z"
	stale_observed_at: "2024-12-03T12:00:00Z"
	products: [
		{id: "atlas", label: "Atlas", channel: "app"},
		{id: "beacon", label: "Beacon", channel: "mobile"},
		{id: "cedar", label: "Cedar", channel: "app"},
	]
	snapshots: [...#Snapshot] & [
		{
			period: "2024-08", coverage_end: "2024-09-01T00:00:00Z"
			observed_at: "2024-12-05T11:00:00Z"
			facts: [
				{product: "atlas", receipts_cents: 150000, refunds_cents: 10000, missing_reason: ""},
				{product: "beacon", receipts_cents: 90000, refunds_cents: 0, missing_reason: ""},
				{product: "cedar", receipts_cents: 20000, refunds_cents: 20000, missing_reason: ""},
			]
		},
		{
			period: "2024-09", coverage_end: "2024-10-01T00:00:00Z"
			observed_at: "2024-12-05T11:00:00Z"
			facts: [
				{product: "atlas", receipts_cents: 100000, refunds_cents: 20000, missing_reason: ""},
				{product: "beacon", receipts_cents: 95000, refunds_cents: 5000, missing_reason: ""},
				{product: "cedar", receipts_cents: 10000, refunds_cents: 25000, missing_reason: ""},
			]
		},
		{
			period: "2024-10", coverage_end: "2024-11-01T00:00:00Z"
			observed_at: "2024-12-05T11:00:00Z"
			facts: [
				{product: "atlas", receipts_cents: 0, refunds_cents: 0, missing_reason: ""},
				{product: "beacon", receipts_cents: null, refunds_cents: null, missing_reason: "Receipt and refund source unavailable"},
				{product: "cedar", receipts_cents: 5000, refunds_cents: null, missing_reason: "Refund source unavailable"},
			]
		},
		{
			period: "2024-11", coverage_end: "2024-11-21T00:00:00Z"
			observed_at: "2024-12-05T11:00:00Z"
			facts: [
				{product: "atlas", receipts_cents: 20000, refunds_cents: 30000, missing_reason: ""},
				{product: "beacon", receipts_cents: 40000, refunds_cents: 5000, missing_reason: ""},
				{product: "cedar", receipts_cents: 5000, refunds_cents: 0, missing_reason: ""},
			]
		},
	]
}

// Literal oracle, not expressions derived from fixture facts or projection code.
expected: {
	freshness: {fresh_age_seconds: 3600, boundary_age_seconds: 86400, stale_age_seconds: 172800}
	facts: {
		"2024-08": [
			{product: "atlas", receipts_cents: 150000, refunds_cents: 10000, missing_reason: ""},
			{product: "beacon", receipts_cents: 90000, refunds_cents: 0, missing_reason: ""},
			{product: "cedar", receipts_cents: 20000, refunds_cents: 20000, missing_reason: ""},
		]
		"2024-09": [
			{product: "atlas", receipts_cents: 100000, refunds_cents: 20000, missing_reason: ""},
			{product: "beacon", receipts_cents: 95000, refunds_cents: 5000, missing_reason: ""},
			{product: "cedar", receipts_cents: 10000, refunds_cents: 25000, missing_reason: ""},
		]
		"2024-10": [
			{product: "atlas", receipts_cents: 0, refunds_cents: 0, missing_reason: ""},
			{product: "beacon", receipts_cents: null, refunds_cents: null, missing_reason: "Receipt and refund source unavailable"},
			{product: "cedar", receipts_cents: 5000, refunds_cents: null, missing_reason: "Refund source unavailable"},
		]
		"2024-11": [
			{product: "atlas", receipts_cents: 20000, refunds_cents: 30000, missing_reason: ""},
			{product: "beacon", receipts_cents: 40000, refunds_cents: 5000, missing_reason: ""},
			{product: "cedar", receipts_cents: 5000, refunds_cents: 0, missing_reason: ""},
		]
	}
	september: {
		all: {current: 155000, previous: 230000, delta: -75000, largest_decline: "atlas"}
		app: {current: 65000, previous: 140000, delta: -75000, largest_decline: "atlas"}
		mobile: {current: 90000, previous: 90000, delta: 0, largest_decline: ""}
		rows: [
			{id: "atlas", current: 80000, previous: 140000, delta: -60000},
			{id: "beacon", current: 90000, previous: 90000, delta: 0},
			{id: "cedar", current: -15000, previous: 0, delta: -15000},
		]
	}
	october: {known_subtotal: 0, known: 1, selected: 3, exact_total: null, delta: null}
	november: {known_subtotal: 30000, known: 3, selected: 3, exact_total: null, delta: null}
}
