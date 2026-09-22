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
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// prefilterChunkSize bounds how many sections one decision.Request batches
// together for the kit's "llm" adapter (vision 32 §2 item 1: "one batched
// request per chunk of at most 20 sections") — decisionBatchSize's case 2
// (decisionbatch.go, DP-22) reads this same constant, so PrefilterSections
// and rerankWithinTiers chunk the "llm" adapter identically; a Decider this
// helper does not recognize is never chunked here either (case 3, 0 =
// unbounded), same as before decisionBatchSize existed for every Decider
// PrefilterSections previously chunked at a fixed 20 regardless of name.
const prefilterChunkSize = 20

// defaultPrefilterThreshold is the noul cutoff Compile applies when a
// Decider is configured but Compiler.PrefilterThreshold is left at its zero
// value — documented on that field.
const defaultPrefilterThreshold = 0.5

// prefilterInstructions is the fixed KindNoul question every section in a
// chunk is asked, verbatim (vision 32 §2 item 1).
const prefilterInstructions = "Does this section state a binding rule for code in this repository (something an editor of a governed file must do or must never do), rather than narrative, history, or an example?"

// PrefilterSections asks the decision port, in chunks sized by
// decisionBatchSize(d) (DP-22 — prefilterChunkSize for the kit's "llm"
// adapter, unbounded for any other recognized Decider), whether each of
// secs (already in the caller's deterministic compile order) states a
// binding rule — and returns the sections that pass threshold, in that
// same order, as keep; the rest, also in order, as skipped.
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
// earlier. decErr (DP-22, O109) is non-nil in exactly that case — data
// describing the chunk that failed (site, provider, question count,
// latency), for the caller to log via decide.Recorder.RecordError; nil on
// every other return path (identity fast paths, ErrNoProvider, success).
func PrefilterSections(ctx context.Context, d decision.Decider, threshold float64, secs []Section) (keep []Section, skipped []Section, records []decision.Record, decErr *DecisionError, err error) {
	if d == nil {
		return append([]Section(nil), secs...), nil, nil, nil, nil
	}
	if _, isNone := d.(decision.None); isNone {
		return append([]Section(nil), secs...), nil, nil, nil, nil
	}

	chunkSize := decisionBatchSize(d)
	if chunkSize <= 0 || chunkSize > len(secs) {
		// len(secs) == 0 leaves chunkSize at 0 too, but the loop below
		// never runs in that case (start := 0 < len(secs) is already
		// false), so no positive floor is needed here.
		chunkSize = len(secs)
	}

	keep = make([]Section, 0, len(secs))
	for start := 0; start < len(secs); start += chunkSize {
		end := start + chunkSize
		if end > len(secs) {
			end = len(secs)
		}
		chunk := secs[start:end]

		callStart := time.Now()
		resp, derr := d.Decide(ctx, buildPrefilterRequest(chunk))
		if derr != nil {
			if errors.Is(derr, decision.ErrNoProvider) {
				// "No provider configured" describes the Decider, not this
				// one chunk — the whole input is kept, not just the tail
				// not yet asked, and whatever this or an earlier chunk
				// already sorted into keep/skipped is discarded.
				return append([]Section(nil), secs...), nil, nil, nil, nil
			}
			decErr = &DecisionError{
				Site:      "compile_prefilter",
				Provider:  d.Name(),
				Questions: len(chunk),
				Latency:   time.Since(callStart),
				Err:       derr,
			}
			return nil, nil, nil, decErr, fmt.Errorf("semantic: compile prefilter: %w", derr)
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
	return keep, skipped, records, nil, nil
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

// toCoreDecisionError converts internal/semantic's own DecisionError (the
// kit-adjacent shape PrefilterSections/rerankWithinTiers return) into
// internal/core's dependency-free mirror, core.DecisionError — same
// rationale as toCoreDecisionRecords just above. nil in, nil out.
func toCoreDecisionError(de *DecisionError) *core.DecisionError {
	if de == nil {
		return nil
	}
	errMsg := ""
	if de.Err != nil {
		errMsg = de.Err.Error()
	}
	return &core.DecisionError{
		Site:      de.Site,
		Provider:  de.Provider,
		Questions: de.Questions,
		LatencyMS: de.Latency.Milliseconds(),
		Err:       errMsg,
	}
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
