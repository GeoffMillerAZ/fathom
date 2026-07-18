// Package dadinvestigator defines CUE contracts for the DAD investigator
// pattern. DAD stands for Decompose → Analyse → Decide. Evidence items
// and Decision values are data-shaped; the state machine behaviour lives
// in types.go and impl.go.
package dadinvestigator

// #Phase enumerates the three phases of the DAD investigation loop.
// decompose → analyse → decide → (terminal: found or exhausted)
#Phase: "decompose" | "analyse" | "decide" | "found" | "exhausted"

// #Confidence enumerates the confidence bands attached to a Decision.
#Confidence: "none" | "low" | "medium" | "high"

// #EvidenceItem is a single piece of evidence fed into the investigation.
#EvidenceItem: {
	// ID is a unique identifier for this evidence item.
	ID: string & !=""

	// Content is the text body of the evidence.
	Content: string & !=""

	// Weight is the relative importance of this evidence item. Must be > 0.
	Weight: int & >0
}

// #Decision is the output of the decide phase for a single step.
#Decision: {
	// Finding is a short description of the conclusion reached in this step.
	Finding: string & !=""

	// Confidence is the confidence band attached to this decision.
	Confidence: #Confidence

	// Terminal indicates whether this decision terminates the investigation.
	Terminal: bool
}
