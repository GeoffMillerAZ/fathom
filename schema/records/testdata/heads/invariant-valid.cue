package records

invariant: #InvariantHead & {
	"kind":      "invariant"
	"id":        "I1"
	"statement": "Preserve evidence"
	"enforced_by": [
		"test:TestEvidence"
	]
	"conditions": [
		{
			"label":     "Revisit when"
			"kind":      "record"
			"text":      "Linked record changes"
			"review_by": "2027-01-02"
			"target":    "ADR-1"
			"want":      "accepted"
		}
	]
}
