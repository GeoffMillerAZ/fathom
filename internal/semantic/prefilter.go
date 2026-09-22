package semantic

// prefilter.go — the compile section pre-filter (ADR-042 decision 2, vision
// 32 §2 item 1): before a section is sent to the (slow, costly) compile
// model, one batched decision-port `noul` question asks whether it states a
// binding rule for code in this repository at all. A section the port says
// no to (or cannot answer) is never compiled, but it is never silently
// dropped either — the caller (Compile) counts it and folds it into
// UncompiledScopes exactly like a budget-stopped section, so ADR-037's raw
// fallback still serves it.
//
// Off by default (ADR-042 decision 2/4): with no Decider configured,
// PrefilterSections is the identity and makes no call — this file adds no
// behavior at all until an operator wires `[decide] provider = llm`.
import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// prefilterChunkSize bounds how many sections one decision.Request batches
// together (vision 32 §2 item 1: "one batched request per chunk of at most
// 20 sections").
const prefilterChunkSize = 20

// defaultPrefilterThreshold is the noul cutoff Compile applies when a
// Decider is configured but Compiler.PrefilterThreshold is left at its zero
// value — documented on that field.
const defaultPrefilterThreshold = 0.5

// prefilterInstructions is the fixed KindNoul question every section in a
// chunk is asked, verbatim (vision 32 §2 item 1).
const prefilterInstructions = "Does this section state a binding rule for code in this repository (something an editor of a governed file must do or must never do), rather than narrative, history, or an example?"

// PrefilterSections asks the decision port, in chunks of at most
// prefilterChunkSize, whether each of secs (already in the caller's
// deterministic compile order) states a binding rule — and returns the
// sections that pass threshold, in that same order, as keep; the rest, also
// in order, as skipped.
//
// Identity fast paths — every section is kept, nothing is skipped, no
// Record is produced, and no call is made:
//
//   - d is nil (no Decider configured at all);
//   - d is a decision.None (the kit's own zero-configuration stub, which
//     never calls out by its own contract — detected without invoking
//     Decide, so neither case pays for a call);
//   - any other Decider's Decide answers decision.ErrNoProvider on a chunk
//     (a misconfigured or intentionally-disabled adapter) — the WHOLE
//     input is then treated as kept, not just the chunks not yet asked,
//     since "no provider configured" describes the Decider, not one chunk
//     of it.
//
// Otherwise, one decision.Request is sent per chunk: State is the chunk's
// sections rendered deterministically (id, doc path, heading, text) and
// Questions is one KindNoul question per section, ID = the section's
// stable Subject, Instructions = prefilterInstructions. A section whose
// answer is absent from the Response (named in Invalid, or simply never
// answered) or whose Noul falls strictly below threshold is skipped; a tie
// AT threshold is kept. records accumulates every chunk's
// Response.Records("compile_prefilter"), in chunk order, for the caller to
// log.
//
// A real Decider error (anything but ErrNoProvider) aborts and is returned
// to the caller — the same "poison the whole run, don't guess" posture
// Compile's own LLM call already takes at the section level, just one step
// earlier.
func PrefilterSections(ctx context.Context, d decision.Decider, threshold float64, secs []Section) (keep []Section, skipped []Section, records []decision.Record, err error) {
	if d == nil {
		return append([]Section(nil), secs...), nil, nil, nil
	}
	if _, isNone := d.(decision.None); isNone {
		return append([]Section(nil), secs...), nil, nil, nil
	}

	keep = make([]Section, 0, len(secs))
	for start := 0; start < len(secs); start += prefilterChunkSize {
		end := start + prefilterChunkSize
		if end > len(secs) {
			end = len(secs)
		}
		chunk := secs[start:end]

		resp, derr := d.Decide(ctx, buildPrefilterRequest(chunk))
		if derr != nil {
			if errors.Is(derr, decision.ErrNoProvider) {
				// "No provider configured" describes the Decider, not this
				// one chunk — the whole input is kept, not just the tail
				// not yet asked, and whatever this or an earlier chunk
				// already sorted into keep/skipped is discarded.
				return append([]Section(nil), secs...), nil, nil, nil
			}
			return nil, nil, nil, fmt.Errorf("semantic: compile prefilter: %w", derr)
		}

		for _, sec := range chunk {
			ans, ok := resp.Answers[sec.Subject]
			if !ok || ans.Noul < threshold {
				skipped = append(skipped, sec)
				continue
			}
			keep = append(keep, sec)
		}
		records = append(records, resp.Records("compile_prefilter")...)
	}
	return keep, skipped, records, nil
}

// buildPrefilterRequest renders one chunk's decision.Request: State is
// every section's id/doc/heading/text, deterministically ordered (chunk
// order, which is secs' own order); Questions is one KindNoul question per
// section keyed by its stable Subject.
func buildPrefilterRequest(chunk []Section) decision.Request {
	qs := make([]decision.Question, len(chunk))
	for i, sec := range chunk {
		qs[i] = decision.Question{
			ID:           sec.Subject,
			Kind:         decision.KindNoul,
			Instructions: prefilterInstructions,
		}
	}
	return decision.Request{
		State:     renderPrefilterState(chunk),
		Questions: qs,
	}
}

// toCoreDecisionRecords converts the kit's decision.Record (agentic-kit
// foundations/decision) into internal/core's dependency-free mirror,
// core.DecisionRecord — see that type's doc comment for why internal/core
// cannot import the kit package directly. nil in, nil out.
func toCoreDecisionRecords(recs []decision.Record) []core.DecisionRecord {
	if len(recs) == 0 {
		return nil
	}
	out := make([]core.DecisionRecord, len(recs))
	for i, r := range recs {
		out[i] = core.DecisionRecord{
			Site:         r.Site,
			Provider:     r.Provider,
			Model:        r.Model,
			QuestionID:   r.QuestionID,
			Kind:         string(r.Kind),
			Choice:       r.Choice,
			Confidence:   r.Confidence,
			Score:        r.Score,
			Noul:         r.Noul,
			LatencyMS:    r.Latency.Milliseconds(),
			InputTokens:  r.InputTokens,
			OutputTokens: r.OutputTokens,
		}
	}
	return out
}

// renderPrefilterState renders a chunk's sections deterministically: one
// block per section (id, doc path, heading, full text), sections separated
// by a blank line, in the caller's own order.
func renderPrefilterState(chunk []Section) string {
	var b strings.Builder
	for i, sec := range chunk {
		if i > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "id: %s\ndoc: %s\nheading: %s\ntext:\n%s",
			sec.Subject, sec.Path, firstLine(sec.Text), sec.Text)
	}
	return b.String()
}
