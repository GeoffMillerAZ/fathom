package queryplanner

// valid_gmail_query is a minimal valid email-targeting query.
valid_gmail_query: #PlannedQuery & {
	Provider:  "gmail"
	Account:   ""
	Query:     "(from:alice@example.com OR to:alice@example.com)"
	Rationale: "emails involving entity Alice"
	IsAI:      false
	Warnings: []
}

// valid_drive_query is a Drive full-text query.
valid_drive_query: #PlannedQuery & {
	Provider:  "drive"
	Account:   ""
	Query:     "fullText contains 'Alice'"
	Rationale: "documents mentioning entity Alice"
	IsAI:      false
	Warnings: []
}

// valid_ai_query is an LLM-refined query with a warning.
valid_ai_query: #PlannedQuery & {
	Provider:  "gmail"
	Account:   ""
	Query:     "deposit agreement"
	Rationale: "LLM suggested lease-related terms"
	IsAI:      true
	Warnings: []
}

// valid_brief is a well-formed investigation brief.
valid_brief: #Brief & {
	ID:   "brief-001"
	Text: "Find all communications about the lease agreement."
}
