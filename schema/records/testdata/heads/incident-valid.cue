package records

incident: #IncidentHead & {
	"kind":  "incident"
	"id":    "INC-1"
	"title": "An incident"
	"outcome": [
		"I1"
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
