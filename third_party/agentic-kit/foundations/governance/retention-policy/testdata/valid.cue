// valid.cue — concrete values that satisfy the retentionpolicy schema.
// Run `cue vet ./...` from the kit root to validate these against schema.cue.
package retentionpolicy

import "github.com/geoffmilleraz/agentic-kit/foundations/governance/retention-policy:retentionpolicy"

// pii7Year is a standard 7-year PII shred policy.
pii7Year: retentionpolicy.#Policy & {
	Name:            "pii-7-year"
	NamespacePrefix: "case-"
	Class:           "pii"
	MaxAgeSec:       220752000 // 7 years in seconds
	Action:          "shred"
}

// financialFlagAnnual flags financial-class namespaces older than 1 year for review.
financialFlagAnnual: retentionpolicy.#Policy & {
	Name:            "financial-flag-annual"
	NamespacePrefix: ""
	Class:           "financial"
	MaxAgeSec:       31536000 // 1 year in seconds
	Action:          "flag"
}

// auditReport matches all namespaces and triggers a report every 90 days.
auditReport: retentionpolicy.#Policy & {
	Name:            "quarterly-report"
	NamespacePrefix: ""
	Class:           ""
	MaxAgeSec:       7776000 // 90 days in seconds
	Action:          "report"
}

// caseNsEntry is a typical PII namespace entry for a legal case.
caseNsEntry: retentionpolicy.#NamespaceEntry & {
	Namespace:        "case-00123"
	Class:            "pii"
	CreatedAtRFC3339: "2016-01-01T00:00:00Z"
}

// financialNsEntry is a financial-class namespace.
financialNsEntry: retentionpolicy.#NamespaceEntry & {
	Namespace:        "acme-billing-2024"
	Class:            "financial"
	CreatedAtRFC3339: "2024-06-15T00:00:00Z"
}
