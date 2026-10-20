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

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// prefilterChunkSize bounds how many sections one decision.Request batches
// together for the kit's "llm" adapter (vision 32 §2 item 1: "one batched
// request per chunk of at most 20 sections") — decisionBatchCaps' own "llm"
// case (decisionbatch.go, DP-22/DP-26) reads this same constant, so
// PrefilterSections and rerankWithinTiers chunk the "llm" adapter's
// question count identically; typesafe and any other Decider get their own
// default question/byte caps instead (DP-26, O110) — never truly unbounded
// any more, see decisionBatchCaps' own doc comment for why.
const prefilterChunkSize = 20

// defaultPrefilterThreshold is the noul cutoff Compile applies when a
// Decider is configured but Compiler.PrefilterThreshold is left at its zero
// value — documented on that field.
const defaultPrefilterThreshold = 0.5

// sizingEstimateKey marks a dry run's context as a sizing estimate (EW-77).
type sizingEstimateKey struct{}

// WithSizingEstimate marks ctx as a sizing estimate (EW-77, G12): a dry run
// under it does not ask the decision port's compile prefilter, so a
// spending compile that sizes its client timeout from a dry run first pays
// the prefilter once per section (in the spend), not twice. Its
// sections_compiled counts the sections before the prefilter, an upper bound
// for the spend. Compile honors it only on a dry run; a spending compile
// always prefilters.
func WithSizingEstimate(ctx context.Context) context.Context {
	return context.WithValue(ctx, sizingEstimateKey{}, true)
}

// sizingEstimate reports whether ctx carries WithSizingEstimate.
func sizingEstimate(ctx context.Context) bool {
	v, _ := ctx.Value(sizingEstimateKey{}).(bool)
	return v
}

// prefilterInstructions is the fixed KindPredicate question every section in a
// chunk is asked, verbatim (vision 32 §2 item 1).
const prefilterInstructions = "Does this section state a binding rule for code in this repository (something an editor of a governed file must do or must never do), rather than narrative, history, or an example?"

// PrefilterSections asks the decision port, in chunks planned by
// decisionBatchPlan(d, ...) (DP-22/DP-26 — prefilterChunkSize questions for
// the kit's "llm" adapter, defaultBatchQuestions otherwise, EVERY Decider
// also byte-budgeted at its own decisionBatchCaps state-byte cap since
// DP-26/O110), whether each of secs (already in the caller's deterministic
// compile order) states a binding rule — and returns the sections that
// pass threshold, in that same order, as keep; the rest, also in order, as
// skipped. A section whose own rendered state block alone exceeds the byte
// budget (decisionBatchPlan's Oversized) is never sent at all — it is
// added to skipped and recorded with a reason (see the chunk loop below),
// the DP-26 fix for O110's `max_tokens_exceeded` failure: the old count-
// only chunking sent typesafe (Jev) all 72 sections' full text in ONE
// request and the whole run was refused outright before answering
// anything; a byte-oversized single section now degrades that ONE section
// to "uncompiled" (ADR-037's raw fallback still serves it) instead of
// poisoning the entire compile run.
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
// Questions is one KindPredicate question per section, ID = the section's
// stable Subject, Instructions = prefilterInstructions. A section whose
// answer is absent from the Response (named in Invalid, or simply never
// answered) or whose Noul falls strictly below threshold is skipped; a tie
// AT threshold is kept. records accumulates every chunk's
// Response.Records("compile_prefilter"), in chunk order, for the caller to
// log.
//
// A provider failure through internal/decide's port is ErrNoProvider (JF-6:
// the port's fallback to decision.None discloses one line), so it keeps
// every section like no provider at all. A real Decider error (anything
// but ErrNoProvider) aborts and is returned to the caller — the same "poison the whole run, don't guess" posture
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

	plan := decisionBatchPlan(d, prefilterPlanItems(secs))
	oversized := make(map[int]string, len(plan.Oversized))
	for _, o := range plan.Oversized {
		oversized[o.Index] = o.Reason
	}

	keep = make([]Section, 0, len(secs))
	for _, idxs := range plan.Chunks {
		if len(idxs) == 1 {
			if reason, ok := oversized[idxs[0]]; ok {
				// DP-26 (O110): this ONE section's own rendered state
				// alone exceeds the byte budget — never send it (that is
				// exactly the shape of O110's max_tokens_exceeded
				// failure, just narrowed from all 72 sections to one).
				// It is skipped like any Invalid/below-threshold section,
				// and reason travels as a synthetic decision.Record's
				// Choice (Confidence 0 — decision.BandOf classifies it
				// unsure, so a Recorder logs it applied:false, "asked and
				// answered nothing" being exactly wrong here — "never
				// asked" is the true state, which is why NO Decide call
				// is made for it at all).
				sec := secs[idxs[0]]
				skipped = append(skipped, sec)
				records = append(records, decision.Record{
					Site:       "compile_prefilter",
					Provider:   d.Name(),
					QuestionID: sec.Subject,
					Kind:       decision.KindPredicate,
					Choice:     reason,
				})
				continue
			}
		}

		chunk := make([]Section, len(idxs))
		for j, idx := range idxs {
			chunk[j] = secs[idx]
		}

		callStart := time.Now()
		resp, derr := d.Decide(decide.WithSite(ctx, decide.SiteCompilePrefilter), buildPrefilterRequest(chunk))
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

// prefilterPlanItems builds decisionBatchPlan's input for secs: each
// item's StateBytes is the length of that ONE section's own rendered block
// (prefilterSectionBlock) — the actual bytes it would add to a chunk's
// shared decision.Request.State, since a prefilter chunk's whole batch of
// section text rides on State, not on any per-Question field (contrast
// rerankWithinTiers' items, whose bulk content rides on each Question's
// own TrueCriteria instead — see that function's own doc comment).
func prefilterPlanItems(secs []Section) []decisionPlanItem {
	items := make([]decisionPlanItem, len(secs))
	for i, sec := range secs {
		items[i] = decisionPlanItem{QuestionID: sec.Subject, StateBytes: len(prefilterSectionBlock(sec))}
	}
	return items
}

// buildPrefilterRequest renders one chunk's decision.Request: State is
// every section's id/doc/heading/text, deterministically ordered (chunk
// order, which is secs' own order); Questions is one KindPredicate question per
// section keyed by its stable Subject.
func buildPrefilterRequest(chunk []Section) decision.Request {
	qs := make([]decision.Question, len(chunk))
	for i, sec := range chunk {
		qs[i] = decision.Question{
			ID:           sec.Subject,
			Kind:         decision.KindPredicate,
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
		b.WriteString(prefilterSectionBlock(sec))
	}
	return b.String()
}

// prefilterSectionBlock renders ONE section's own block exactly as
// renderPrefilterState writes it into a chunk's shared State (id, doc
// path, heading, full text) — factored out so prefilterPlanItems can size
// decisionBatchPlan's byte budget per section without rendering a whole
// chunk just to measure one member of it.
func prefilterSectionBlock(sec Section) string {
	return fmt.Sprintf("id: %s\ndoc: %s\nheading: %s\ntext:\n%s",
		sec.Subject, sec.Path, firstLine(sec.Text), sec.Text)
}
