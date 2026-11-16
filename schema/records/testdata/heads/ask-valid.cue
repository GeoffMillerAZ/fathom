package records

ask: #AskHead & {
	"kind":  "ask"
	"id":    "existing-key"
	"title": "An ask"
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
