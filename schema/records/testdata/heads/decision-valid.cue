package records

decision: #DecisionHead & {
	"kind":   "decision"
	"id":     "ADR-1"
	"title":  "A decision"
	"ruling": "accepted"
	"supersedes": [
		"ADR-0"
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
