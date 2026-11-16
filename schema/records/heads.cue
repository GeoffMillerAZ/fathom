package records

// Parsed authored heads only. Go consumers enforce semantic date and condition laws.
#Text:   string & !=""
#Review: string & (=~"^[0-9]{4}-[0-9]{2}-[0-9]{2}$" | "never")
#Condition: close({
	label:     "Done when" | "Promote when" | "Drop if" | "Revisit when" | "If false"
	kind:      "date" | "record" | "reading" | "manual"
	text:      #Text
	review_by: #Review
	target?:   #Text
	want?:     #Text
})
#DecisionHead: close({
	kind:   "decision", id: =~"^ADR-[0-9]+$", title: #Text
	ruling: "proposed" | "accepted" | "rejected" | "deferred" | "withdrawn"
	supersedes: [...#Text], conditions: [...#Condition]
})
#InvariantHead: close({
	kind: "invariant", id: =~"^I[0-9]+$", statement: #Text
	enforced_by: [...#Text], conditions: [...#Condition]
})
#GoalHead: close({
	kind:   "goal", id:        =~"^G[0-9]+$", title: #Text
	bet:    #Text, instrument: #Text, threshold:     #Text
	review: #Review, conditions: [...#Condition]
})
#FutureHead: close({
	kind:       "future", id: =~"^F[0-9]+$", title: #Text
	confidence: "likely" | "plausible" | "speculative"
	horizon:    "next" | "later" | "someday"
	value?: close({level: "low" | "medium" | "high" | "very-high", why: #Text})
	basis?: close({kind: "measured" | "argued" | "guess", cite?: #Text})
	conditions: [...#Condition]
})
#QuestionHead: close({
	kind: "question", id: =~"^Q[0-9]+$", title: #Text
	conditions: [...#Condition]
})
#IncidentHead: close({
	kind: "incident", id: =~"^INC-[0-9]+$", title: #Text
	outcome: [...#Text], conditions: [...#Condition]
})
#ObservationHead: close({
	kind:     "observation", id: =~"^O[0-9]+$", title: #Text
	verdict?: "fix" | "wontfix" | "duplicate" | "ceiling"
	outcome: [...#Text], conditions: [...#Condition]
})
#AskHead: close({
	kind: "ask", id: #Text, title: #Text, conditions: [...#Condition]
})
