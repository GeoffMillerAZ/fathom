package semantic

// decisionrank.go — DP-7 (docs/plans/decision-port.md, ADR-042 decision 2,
// vision 32 §2 item 2): the obligations engine's decision-port re-rank.
// ADR-035's governing-scope tier law (governingscope.go) stays untouchable —
// "an exact-file rule outranks a package rule outranks a root rule, and no
// model may change that" (vision 32 §2). A decision.Decider may only
// re-order rules STRICTLY INSIDE one tier, by how confidently it answers
// "does this rule govern this edit" for each — never across tiers, never by
// dropping one.
//
// DP-22 (O109): the batch is chunked at decisionBatchSize(d) questions per
// Decide call (see decisionbatch.go) and the chunks run CONCURRENTLY,
// bounded at rerankConcurrency — a ~160-claim batch sent as one request to
// the kit's "llm" adapter came back truncated (not valid JSON), the whole
// Decide call errored, and nothing ranked. A chunk that still fails is
// never allowed to drop or reorder its own questions' rules — they simply
// keep their pre-rerank (lexical) position, exactly as an unanswered
// question already did before chunking existed — and is reported back as a
// DecisionError for the caller to log, rather than vanishing silently.

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// rerankConcurrency bounds how many chunk Decide calls rerankWithinTiers
// runs at once (DP-22). Chunking alone still pays N round trips — this is
// what keeps that N round trips in PARALLEL rather than trading "one slow,
// unreliable call" for "N slow calls in series".
const rerankConcurrency = 4

// rerankOutcome is rerankWithinTiers' return value (DP-22 grew this from
// four bare returns once a batch could produce a PARTIAL result — some
// chunks answered, some hard-failed).
type rerankOutcome struct {
	// Matched is the caller's matched, reordered by whichever chunks
	// answered (reorderByNoulWithinTier) — byte-identical in content and
	// order to the input when every chunk failed (nothing to reorder by).
	Matched []scored
	// Relevance carries every claim ID a SUCCEEDING chunk answered — nil
	// when no chunk succeeded (Ranked is false), matching the pre-DP-22
	// contract that a failed rerank never populates it.
	Relevance map[string]float64
	// Records is every succeeding chunk's Response.Records("obligations"),
	// merged across chunks and sorted by QuestionID — the same order
	// decision.Response.Records itself already guarantees for one call —
	// so concurrent chunk completion order never leaks into the served or
	// logged order.
	Records []decision.Record
	// Failures is one DecisionError per chunk whose Decide call itself
	// errored, in chunk order — for the caller to log via
	// decide.Recorder.RecordError. A chunk that answered validly but left
	// some questions unanswered (Invalid, or simply absent from Answers)
	// is a normal, successful Response and is never a Failure.
	Failures []DecisionError
	// Ranked is true iff at least one chunk succeeded. The caller sets
	// RankedBy to the Decider's Name() only when Ranked — when false
	// (every chunk failed), the payload stays exactly as untouched as the
	// single-call, pre-DP-22 error path always left it.
	Ranked bool
	// ChunksTotal and ChunksFailed feed the caller's "N of M chunks
	// failed" disclosure. Both 0 only when matched was empty — this
	// function's own early return, which never reaches the chunking logic
	// at all.
	ChunksTotal, ChunksFailed int
}

// rerankChunkResult is one chunk's Decide outcome, written into
// results[i] by exactly one goroutine (index i, never shared) and read back
// only after every goroutine has returned (sync.WaitGroup.Wait) — safe
// under -race with no further synchronization.
type rerankChunkResult struct {
	chunk []scored
	resp  decision.Response
	err   error
	lat   time.Duration
}

// rerankWithinTiers asks d one batched decision.KindNoul question per
// matched claim — "does this rule govern the edit described in the state" —
// chunked at decisionBatchSize(d) questions per Decide call (DP-22), the
// chunks run concurrently (bounded at rerankConcurrency, each its own
// Decide call sharing the same rendered State) — and returns matched
// re-ordered ONLY inside each rule's own governing-scope tier
// (scored.governing.tier): a rule never crosses a tier and never drops
// (every input claim is present in the output, exactly once). Within a
// tier, a rule whose noul answer clears I42's unsure-band floor
// (usableNoul, decision.UnsureBelow) is ordered by descending noul, stable
// on ties; every other rule in that tier (no answer at all — including
// every rule in a chunk whose Decide call itself failed — or an answer
// below the floor) keeps its EXACT original position — I42: "a decision in
// the unsure band never changes served content," extended by DP-22 to "a
// decision that was never successfully asked never changes served
// content" either.
//
// d is assumed non-nil and RankWithinTier-gated by the caller
// (obligationsCore never calls this otherwise); a nil d (or an empty
// matched) still degrades safely — matched returned unchanged, no call, no
// chunks — so a direct unit test may pass one without special-casing it.
//
// relevance carries every claim ID a succeeding chunk answered (its RAW
// noul value, regardless of the unsure-band floor — that floor gates
// ORDERING only, never whether the served payload discloses the answer)
// for obligationsCore to attach as each served Obligation.Relevance.
// Records is every succeeding chunk's resp.Records("obligations"), merged
// and sorted, for the caller to log; Failures is one DecisionError per
// chunk whose Decide call itself errored, for the caller to log as an
// error row instead (I42/ADR-042 decision 3 apply the same way per chunk
// that the old single-call path applied to the whole batch: a provider
// error is swallowed into an untouched result FOR THAT CHUNK'S OWN
// QUESTIONS, never surfaced as a fatal error from this function — the
// caller decides how to report it).
func rerankWithinTiers(ctx context.Context, d decision.Decider, file, hunk string, matched []scored) rerankOutcome {
	if d == nil || len(matched) == 0 {
		return rerankOutcome{Matched: matched}
	}

	state := rerankState(file, hunk)
	size := decisionBatchSize(d)
	if size <= 0 || size > len(matched) {
		size = len(matched)
	}

	var chunks [][]scored
	for start := 0; start < len(matched); start += size {
		end := start + size
		if end > len(matched) {
			end = len(matched)
		}
		chunks = append(chunks, matched[start:end])
	}

	results := make([]rerankChunkResult, len(chunks))
	sem := make(chan struct{}, rerankConcurrency)
	var wg sync.WaitGroup
	for i, chunk := range chunks {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, chunk []scored) {
			defer wg.Done()
			defer func() { <-sem }()

			req := decision.Request{State: state, Questions: make([]decision.Question, 0, len(chunk))}
			for _, m := range chunk {
				req.Questions = append(req.Questions, decision.Question{
					ID:           m.cl.ClaimID,
					Kind:         decision.KindNoul,
					Instructions: "Does this rule govern the edit described in the state — would an editor of this file have to follow it here?",
					TrueCriteria: rerankCriteria(m.cl),
				})
			}
			start := time.Now()
			resp, err := d.Decide(ctx, req)
			results[i] = rerankChunkResult{chunk: chunk, resp: resp, err: err, lat: time.Since(start)}
		}(i, chunk)
	}
	wg.Wait()

	relevance := make(map[string]float64)
	var records []decision.Record
	var failures []DecisionError
	ranked := false
	for _, res := range results {
		if res.err != nil {
			failures = append(failures, DecisionError{
				Site:      "obligations",
				Provider:  d.Name(),
				Questions: len(res.chunk),
				Latency:   res.lat,
				Err:       res.err,
			})
			continue
		}
		ranked = true
		for id, a := range res.resp.Answers {
			if a.Kind == decision.KindNoul {
				relevance[id] = a.Noul
			}
		}
		records = append(records, res.resp.Records("obligations")...)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].QuestionID < records[j].QuestionID })

	return rerankOutcome{
		Matched:      reorderByNoulWithinTier(matched, relevance),
		Relevance:    relevance,
		Records:      records,
		Failures:     failures,
		Ranked:       ranked,
		ChunksTotal:  len(chunks),
		ChunksFailed: len(failures),
	}
}

// rerankFailureNote renders RankNote's text for a rerank whose Decide
// call(s) hard-failed (rerankOutcome.Failures non-empty). total == 1 (no
// real chunking occurred — the whole matched set fit in one chunk, or the
// Decider was never split) reproduces the exact pre-DP-22 wording, a bare
// error string, so every caller written against the single-error era keeps
// working unchanged. total > 1 names the chunk tally FIRST ("N of M chunks
// failed") — "how many failed" is the fact a human or agent needs at a
// glance — then the first failure's own error text, so the underlying
// cause (O109's truncated-JSON failure, say) is still one read away, never
// only a bare count.
func rerankFailureNote(failures []DecisionError, total int) string {
	if len(failures) == 0 {
		return ""
	}
	if total <= 1 {
		return failures[0].Err.Error()
	}
	return fmt.Sprintf("%d of %d chunks failed: %v", len(failures), total, failures[0].Err)
}

// rerankState renders the State text a rerank Request's questions are asked
// about: the edited path, plus the hunk when the hook supplied one.
func rerankState(file, hunk string) string {
	if hunk == "" {
		return "file: " + file
	}
	return "file: " + file + "\n\nhunk:\n" + hunk
}

// rerankCriteria is one claim's noul-question TrueCriteria: the rule text
// and its scope, so the Decider judges against the actual governing text
// rather than the claim ID alone.
func rerankCriteria(cl core.SemanticClaim) string {
	return "Rule: " + cl.Rule + "\nScope: " + strings.Join(cl.Scope, ", ")
}

// usableNoul reports id's noul answer from relevance and whether it clears
// I42's unsure-band floor (decision.UnsureBelow, 0.5) — the only answers
// rerankWithinTiers' own reorder (and obligationsCore's less()) ever act
// on. An absent answer, or one below the floor, always reports ok=false:
// the caller's claim never moves because of it.
func usableNoul(id string, relevance map[string]float64) (float64, bool) {
	v, ok := relevance[id]
	if !ok || v < decision.UnsureBelow {
		return 0, false
	}
	return v, true
}

// reorderByNoulWithinTier permutes matched ONLY among index positions that
// share the same governing.tier, and only among the USABLE-noul subset of
// that tier: every unusable-noul rule (no answer, or an unsure-band
// answer) is left at its EXACT original index; the usable rules are
// stable-sorted by descending noul and written back into the (usable-only)
// positions they already occupied, in that new order.
//
// This is deliberately NOT a single whole-tier comparator sort: mixing
// "no meaningful answer, never move me" items into an ordinary pairwise
// comparator produces an ill-defined (non-transitive) relation once a
// tier holds more than one usable item interleaved with an unusable one.
// Extracting the usable subsequence and sorting only among itself keeps
// this a plain, well-defined total order over floats.
func reorderByNoulWithinTier(matched []scored, relevance map[string]float64) []scored {
	out := append([]scored(nil), matched...)

	byTier := map[int][]int{}
	var tierOrder []int
	for i, m := range out {
		t := m.governing.tier
		if _, seen := byTier[t]; !seen {
			tierOrder = append(tierOrder, t)
		}
		byTier[t] = append(byTier[t], i)
	}

	for _, t := range tierOrder {
		idxs := byTier[t]
		var usableIdxs []int
		var usableItems []scored
		for _, idx := range idxs {
			if _, ok := usableNoul(out[idx].cl.ClaimID, relevance); ok {
				usableIdxs = append(usableIdxs, idx)
				usableItems = append(usableItems, out[idx])
			}
		}
		if len(usableItems) < 2 {
			continue // 0 or 1 usable rule in this tier: nothing to reorder.
		}
		sort.SliceStable(usableItems, func(i, j int) bool {
			ni, _ := usableNoul(usableItems[i].cl.ClaimID, relevance)
			nj, _ := usableNoul(usableItems[j].cl.ClaimID, relevance)
			return ni > nj
		})
		for k, idx := range usableIdxs {
			out[idx] = usableItems[k]
		}
	}
	return out
}
