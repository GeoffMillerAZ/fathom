package records

future: #FutureHead & {
	"kind":       "future"
	"id":         "F1"
	"title":      "A future"
	"confidence": "likely"
	"horizon":    "next"
	"value": {
		"level": "high"
		"why":   "Useful evidence"
	}
	"basis": {
		"kind": "measured"
		"cite": "reading.md"
	}
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
