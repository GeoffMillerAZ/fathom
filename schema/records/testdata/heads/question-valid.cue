package records

question: #QuestionHead & {
	"kind":  "question"
	"id":    "Q1"
	"title": "A question"
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
