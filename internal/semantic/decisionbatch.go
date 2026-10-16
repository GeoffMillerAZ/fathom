package semantic

// decisionbatch.go — DP-26 (O110, docs/plans/decision-port.md): how a batch
// of decision.Question is split into decision.Request-sized chunks — sized
// by BOTH a per-call question-count cap and a per-call state-BYTE budget,
// never by count alone — plus the data-only failure record both chunked
// call sites (PrefilterSections, rerankWithinTiers) use to report a hard
// Decide error to the caller — internal/semantic stays free of the
// decide.Recorder type itself (as today; see decisionrank.go's own doc
// comment on the same discipline), so pkg/client turns each DecisionError
// into one decide.Recorder.RecordError row.
//
// O109's live finding (DP-22): the obligations re-rank sent ONE batched
// noul request of ~160 candidate claims against the kit's "llm" adapter
// (OpenRouter, openai/gpt-6-luna); the reply truncated before it parsed as
// JSON, the whole batch's Decide call errored, and nothing ranked — the
// original decisionBatchSize (count-only, retired by this file) fixed that
// by chunking a Decider named "llm" at prefilterChunkSize (20) questions
// per call and leaving every other Decider fully unbounded.
//
// O110's live finding (DP-26): unbounded was the wrong default. Wiring the
// REAL typesafe (Jev) adapter for the first time sent it the compile
// pre-filter's full 72-section batch — every section's complete markdown
// text as State — in ONE request, and Jev refused it outright
// (max_tokens_exceeded) before answering a single question. The obligations
// re-rank's own unbounded batches never hit this because a rerank
// question's content is a short claim summary, not a whole document
// section; the pre-filter's is the section's FULL TEXT. A rule keyed only
// on question COUNT cannot see that difference — decisionBatchPlan packs
// against a per-item BYTE size the caller supplies (whatever field on the
// wire actually carries that item's content: PrefilterSections' chunk
// State, rerankWithinTiers' per-question TrueCriteria) as well as count.
import (
	"fmt"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// maxQuestionser is the optional interface a decision.Decider MAY implement
// to name its own per-call question-count ceiling directly: MaxQuestions()
// returns the largest number of decision.Question one Decide call should
// carry at once, 0 meaning unbounded on this dimension (decisionBatchPlan
// still applies the byte budget below, unless maxStateByteser is ALSO
// implemented and also returns 0). The kit defines no shipped Decider that
// implements this as of this writing — decisionBatchPlan is written
// against the interface, not any concrete kit type, so the day a real
// adapter grows a genuine MaxQuestions() method, decisionBatchCaps picks it
// up automatically with NO CHANGE to this file.
type maxQuestionser interface {
	MaxQuestions() int
}

// maxStateByteser is maxQuestionser's byte-budget sibling (DP-26, O110): a
// decision.Decider MAY implement MaxStateBytes() to name its own per-call
// state-byte ceiling directly, 0 meaning unbounded on this dimension. As
// with maxQuestionser, no shipped kit Decider implements this yet;
// decisionBatchCaps picks it up automatically the day one does.
type maxStateByteser interface {
	MaxStateBytes() int
}

const (
	// defaultBatchQuestions is decisionBatchCaps' question-count cap for
	// typesafe and any Decider it does not specially recognize (DP-26,
	// O110) — a generous count chosen only to bound per-call latency and
	// blast radius; defaultBatchStateBytes is what actually protects a
	// large-state caller like the compile pre-filter.
	defaultBatchQuestions = 64

	// defaultBatchStateBytes is decisionBatchCaps' state-byte budget for
	// the "llm" adapter, typesafe, and any Decider it does not specially
	// recognize (DP-26, O110): 96 KiB. At a rule-of-thumb 4 bytes per
	// token, that is roughly 24,000 tokens of state per Decide call —
	// comfortably under Jev (typesafe/jev-1.13)'s measured 32k-token state
	// limit (O110's own compile --dry-run failure), leaving margin for the
	// instructions/schema overhead this byte count does not itself
	// capture. This is the fix for O110's root cause: the pre-DP-26
	// default for any Decider but "llm" was 0 (fully unbounded), which is
	// exactly what let PrefilterSections hand typesafe all 72 sections'
	// full text — many times this budget — in one request.
	defaultBatchStateBytes = 96 * 1024
)

// decisionBatchCaps resolves the (questionCap, stateByteBudget) pair
// decisionBatchPlan packs items against for Decider d — the two caps both
// PrefilterSections (the compile pre-filter) and rerankWithinTiers (the
// obligations re-rank) chunk their batches against, so the two sites share
// ONE rule rather than drifting independently. Either cap may be 0
// (unbounded on that dimension only); a Decider opting out of both never
// gets chunked at all, exactly like the pre-DP-26 decisionBatchSize's own
// case 3.
//
// Resolution, per cap independently:
//
//  1. d implements the matching optional interface (maxQuestionser for the
//     question cap, maxStateByteser for the byte cap): its own method
//     decides that cap, 0 meaning unbounded on that dimension — see each
//     interface's own doc comment.
//  2. d.Name() == "llm": the question cap is prefilterChunkSize (20) — the
//     kit's LLM adapter renders a whole batch into one prompt and one
//     completion; O109 measured a ~160-question batch fail to come back as
//     valid JSON at all, and the compile pre-filter already chunked this
//     same adapter at 20 without incident before decisionBatchPlan
//     existed. Its byte cap is the generic default below (defaultBatchStateBytes)
//     — nothing before DP-26 bounded the llm adapter's bytes either, and
//     O109/O110 never observed it as the source of a real failure, but
//     there is no reason to leave it truly unbounded now that every other
//     Decider gets a byte budget.
//  3. Otherwise (typesafe, decision.None, any test double, or a future
//     unrecognized Decider): BOTH caps default to
//     defaultBatchQuestions/defaultBatchStateBytes. This is DP-26's actual
//     fix: O110's root cause was exactly this case falling through to
//     fully unbounded (0, 0) — the compile pre-filter's 72-section, full-
//     text batch sailed straight through as ONE request against the real
//     typesafe (Jev) adapter and was refused outright
//     (max_tokens_exceeded) before answering a single question.
func decisionBatchCaps(d decision.Decider) (maxQuestions, maxStateBytes int) {
	maxQuestions = defaultBatchQuestions
	maxStateBytes = defaultBatchStateBytes
	if d.Name() == "llm" {
		maxQuestions = prefilterChunkSize
	}
	if mq, ok := d.(maxQuestionser); ok {
		maxQuestions = mq.MaxQuestions()
	}
	if mb, ok := d.(maxStateByteser); ok {
		maxStateBytes = mb.MaxStateBytes()
	}
	return maxQuestions, maxStateBytes
}

// decisionPlanItem is one decision.Question decisionBatchPlan may place
// into a chunk, paired with how many bytes of request content it would add
// to whichever chunk it lands in. StateBytes is computed by the CALLER from
// whatever field on the wire actually carries that item's bulk content —
// PrefilterSections' chunk State (every section in a chunk is rendered,
// full text, into one shared decision.Request.State) and
// rerankWithinTiers' per-question TrueCriteria (rerankWithinTiers' own
// State — the edited file path and an optional hunk — is small and shared
// by the whole call, not itemized here) — decisionBatchPlan itself does
// not know or care which field that is.
type decisionPlanItem struct {
	// QuestionID identifies the item for decisionOversizedItem's own
	// Reason text; decisionBatchPlan does not otherwise interpret it, and
	// does not require it to be unique.
	QuestionID string
	// StateBytes is how many bytes this ONE item would add to a chunk's
	// request payload if included in it.
	StateBytes int
}

// decisionOversizedItem names one item decisionBatchPlan could not fit into
// ANY chunk within the byte budget, even entirely alone — Index is the
// item's own index into the items slice decisionBatchPlan was called with;
// Reason is a human-readable explanation naming the item's own byte count
// and the budget it exceeded, suited to carrying in a decision.Record's
// Choice field (see PrefilterSections'/rerankWithinTiers' own use) or a log
// line. An oversized item still occupies its own singleton chunk in
// decisionChunkPlan.Chunks (a plan is a total partition of items: every
// item appears in exactly one chunk), but the caller must never actually
// send that chunk to Decide — the whole point of Oversized is "this cannot
// possibly fit even alone."
type decisionOversizedItem struct {
	Index  int
	Reason string
}

// decisionChunkPlan is decisionBatchPlan's result: Chunks holds each
// chunk's item indices (into the items slice given to decisionBatchPlan),
// in order, covering every item exactly once and never splitting one item
// across two chunks; Oversized names, by index, every item whose own
// StateBytes alone exceeds the byte budget (see decisionOversizedItem).
type decisionChunkPlan struct {
	Chunks    [][]int
	Oversized []decisionOversizedItem
}

// decisionBatchPlan packs items (in the order given) into decisionChunkPlan
// chunks bounded by decisionBatchCaps(d): a chunk never carries more than
// the question-count cap, and never carries more summed StateBytes than
// the byte budget, whichever cap is finite. Packing is greedy and
// order-preserving — items go into the chunk currently being filled until
// adding the next one would exceed either finite cap, at which point the
// current chunk is closed and a new one starts with that item — so a
// caller that already produced items in its own deterministic order
// (PrefilterSections' compile order, rerankWithinTiers' tier-sorted order)
// gets chunks in that same order, never reshuffled.
//
// An item whose own StateBytes exceeds the byte budget cannot fit into any
// chunk, even alone: it is given its own singleton chunk (so decisionChunkPlan
// remains a total partition — every input item is present in Chunks exactly
// once) AND named in Oversized. A caller must check Oversized before
// sending a chunk: an oversized item's singleton chunk must never actually
// reach Decide (see PrefilterSections/rerankWithinTiers, which mark that
// one question as never-asked with Oversized's own Reason rather than
// sending it and having the whole call refused the way O110's unbounded
// 72-section pre-filter batch was).
//
// Both caps 0 (unbounded on both dimensions) reproduces a single chunk
// holding every item — the same "never chunk a Decider this helper knows
// nothing about, and a caller with a small set pays no extra round trips
// either way" contract the pre-DP-26 decisionBatchSize's case 3 had, just
// resolved per-dimension now instead of as one combined number.
func decisionBatchPlan(d decision.Decider, items []decisionPlanItem) decisionChunkPlan {
	maxQuestions, maxStateBytes := decisionBatchCaps(d)

	var plan decisionChunkPlan
	var cur []int
	curBytes := 0

	flush := func() {
		if len(cur) > 0 {
			plan.Chunks = append(plan.Chunks, cur)
			cur = nil
			curBytes = 0
		}
	}

	for i, it := range items {
		if maxStateBytes > 0 && it.StateBytes > maxStateBytes {
			// Cannot fit even alone: close whatever chunk is in progress
			// (its own items are unaffected by this one), give the
			// oversized item its own singleton chunk so every item is
			// still accounted for exactly once, and name it in Oversized
			// so the caller never actually sends that chunk.
			flush()
			plan.Chunks = append(plan.Chunks, []int{i})
			plan.Oversized = append(plan.Oversized, decisionOversizedItem{
				Index: i,
				Reason: fmt.Sprintf(
					"question %q needs %d bytes of state, over the %d-byte budget for provider %q",
					it.QuestionID, it.StateBytes, maxStateBytes, d.Name(),
				),
			})
			continue
		}

		exceedsCount := maxQuestions > 0 && len(cur)+1 > maxQuestions
		exceedsBytes := maxStateBytes > 0 && curBytes+it.StateBytes > maxStateBytes
		if len(cur) > 0 && (exceedsCount || exceedsBytes) {
			flush()
		}
		cur = append(cur, i)
		curBytes += it.StateBytes
	}
	flush()

	return plan
}

// DecisionError (O109, DP-22) is data describing one decision-port Decide
// call that hard-failed: no answer, no decision.Record, just an error —
// data only, so internal/semantic stays free of the decide.Recorder type
// (as today). The caller (pkg/client, which owns the Recorder) turns each
// of these into one decide.Recorder.RecordError row, so a failed decision
// leaves a trace in decisions.jsonl instead of pure silence.
type DecisionError struct {
	// Site identifies the call site ("compile_prefilter", "obligations") —
	// the same string Response.Records' own site argument uses.
	Site string
	// Provider is the failing Decider's own Name().
	Provider string
	// Questions is how many decision.Question this one failed call
	// carried (a chunk's own size, not the whole batch's).
	Questions int
	// Latency is how long the call took before it failed.
	Latency time.Duration
	// Err is the underlying error, verbatim — never a secret (the kit's
	// own adapters already keep API keys and raw provider bodies out of
	// their returned errors).
	Err error
}
