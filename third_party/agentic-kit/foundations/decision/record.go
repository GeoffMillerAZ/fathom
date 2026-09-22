package decision

import (
	"sort"
	"time"
)

// Record is one flattened, storage-friendly view of a single answered
// question from a Response — one per Decide call site (Site) and per
// question, suited to an append-only log or metrics sink.
type Record struct {
	// Site identifies the call site that produced this Record — the string
	// passed to Response.Records.
	Site string
	// Provider is the Response's Provider.
	Provider string
	// Model is the Response's Model.
	Model string
	// QuestionID is the answered Question's ID.
	QuestionID string
	// Kind is the answered Question's Kind.
	Kind Kind

	// Choice is the Answer's Choice (KindChoice only; empty otherwise).
	Choice string
	// Confidence is the Answer's Confidence.
	Confidence float64
	// Score is the Answer's Score (KindScore only; zero otherwise).
	Score float64
	// Noul is the Answer's Noul (KindNoul only; zero otherwise).
	Noul float64

	// Latency is the Response's Latency, copied onto every Record.
	Latency time.Duration
	// InputTokens is the Response's Usage.InputTokens, copied onto every
	// Record.
	InputTokens int
	// OutputTokens is the Response's Usage.OutputTokens, copied onto every
	// Record.
	OutputTokens int
}

// Records returns one Record per Question r.Answers holds validly (a
// Question named in r.Invalid produces no Record), with Site, r.Provider,
// r.Model, r.Usage, and r.Latency copied onto each, sorted by QuestionID
// for deterministic output regardless of map iteration order.
func (r Response) Records(site string) []Record {
	recs := make([]Record, 0, len(r.Answers))
	for id, a := range r.Answers {
		recs = append(recs, Record{
			Site:         site,
			Provider:     r.Provider,
			Model:        r.Model,
			QuestionID:   id,
			Kind:         a.Kind,
			Choice:       a.Choice,
			Confidence:   a.Confidence,
			Score:        a.Score,
			Noul:         a.Noul,
			Latency:      r.Latency,
			InputTokens:  r.Usage.InputTokens,
			OutputTokens: r.Usage.OutputTokens,
		})
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].QuestionID < recs[j].QuestionID })
	return recs
}
