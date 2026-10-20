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
// DP-22 (O109): the batch is chunked at decisionBatchPlan(d, ...) questions
// per Decide call (see decisionbatch.go) and the chunks run CONCURRENTLY,
// bounded at rerankConcurrency — a ~160-claim batch sent as one request to
// the kit's "llm" adapter came back truncated (not valid JSON), the whole
// Decide call errored, and nothing ranked. A chunk that still fails is
// never allowed to drop or reorder its own questions' rules — they simply
// keep their pre-rerank (lexical) position, exactly as an unanswered
// question already did before chunking existed — and is reported back as a
// DecisionError for the caller to log, rather than vanishing silently.
//
// DP-26 (O110): decisionBatchPlan also bounds each chunk's summed state
// BYTES, not only its question count — every claim's own rule text
// (rerankCriteria) sizes its item, though in practice a rerank batch stays
// far under the byte budget long before it would hit it (a claim summary
// is short; O109 measured calls up to 248 questions succeed against the
// SAME unbounded-by-count typesafe/llm defaults that failed the compile
// pre-filter's 72-full-section batch) — see rerankWithinTiers' own doc
// comment for why this plan is expected to stay count-bound in practice.
//
// DP-24 (O109): chunking alone still asked the Decider about EVERY
// scope-eligible claim — obligationsCore measured ~160 candidates on a
// 6-rule served cap, so 154 of those questions (8 chunks, ~28s wall) were
// spent on rules the cap could never seat regardless of how they ranked.
// rerankEligible narrows the batch to only the governing-scope tiers
// (governingscope.go) that intersect the served window [0, max) in the
// ALREADY-SORTED matched order obligationsCore hands this file (its own
// less(), run before rerankWithinTiers is ever called) — a tier wholly
// inside the window is asked (its own internal order still matters for
// reading), a tier straddling the cut is asked (ranking decides who
// survives), a tier wholly below the cut is never asked (no ranking of it
// can change what is served). A claim this excludes is never asked and so
// never answered — it falls through to the EXACT same "unusable, unmoved"
// path an unanswered chunk already used before DP-24 existed.
//
// DP-25 (O109): a chunk's own Decide call — genuinely slow (a live LLM
// lane) or simply hanging — no longer has an unbounded ctx to run under.
// rerankWithinTiers derives one context.WithTimeout(ctx, timeout) shared by
// every chunk in the call (RerankTimeout, [decide] rerank_timeout_ms,
// default defaultRerankTimeout); a chunk still in flight when that deadline
// fires is cancelled through ctx (a well-behaved Decider — any real
// HTTP-backed adapter that threads ctx into its request — returns promptly
// once ctx.Done() fires, so wg.Wait() below returns at the deadline rather
// than blocking on it) and is treated exactly like any other hard-failed
// chunk: its rules keep their lexical position, and a DecisionError is
// reported. The served order and the response itself are never held
// hostage to a slow or unresponsive decider.

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// defaultRerankTimeout (DP-25, O109) is rerankWithinTiers' own ceiling when
// the caller leaves its timeout argument at zero — a caller that builds
// ObligationsOptions directly (a test, or any future caller that never
// threads [decide] rerank_timeout_ms through pkg/client.WithDecision) still
// gets SOME bound rather than an unbounded wait. config.DefaultDecide.
// RerankTimeoutMS (internal/config, always supplied for any REAL decision
// port via pkg/client's decisionOpts) carries its own, tighter, deliberately
// chosen default (1500ms) — see docs/configuration.md's "[decide]" table for
// why that number specifically: it is kept under the PostToolUse edit
// hook's own 2000ms HTTP client timeout (cmd/fathom/edithook.go's
// fetchObligations call) with margin, so a daemon that exhausts the rerank
// budget still returns before that client gives up and renders nothing at
// all. This constant is the generic, config-unaware fallback only.
const defaultRerankTimeout = 2 * time.Second

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
	// CandidatesTotal (DP-24, O109) is len(matched) as this call received
	// it — the total scope-eligible claim count the served window was
	// computed from, BEFORE DP-24 narrowed which of them ever became a
	// decision.Question. 0 only when matched was empty (the same early
	// return that leaves ChunksTotal/ChunksFailed at 0).
	CandidatesTotal int
	// RankedCount (DP-24, O109) is how many of those candidates the
	// served-window tier filter actually sent to the Decider as a
	// decision.Question — regardless of whether that question's own chunk
	// later succeeded or failed (a chunk that timed out still ASKED; it
	// simply never got an answer). The gap between RankedCount and
	// CandidatesTotal is DP-24's whole point: O109 measured 154 of ~160
	// candidates asked about for no reason a 6-rule cap could ever use.
	// DP-26 (O110): a claim decisionBatchPlan reports Oversized (its own
	// rerankCriteria text alone exceeds the byte budget) is ALSO never
	// asked, so it is excluded from RankedCount the same as a
	// window-excluded claim — RankedCount always means "actually sent",
	// never merely "eligible to be sent."
	RankedCount int
	// TimedOut (DP-25, O109) is true when this call's shared rerank budget
	// (the timeout argument, or defaultRerankTimeout) was exceeded — at
	// least one chunk was still in flight when its context.WithTimeout
	// deadline fired and was cancelled rather than answered. The caller's
	// RankNote names the budget explicitly in this case
	// (rerankTimeoutNote) rather than the ordinary per-chunk-failure text
	// rerankFailureNote renders, so a reader can tell "the decider was too
	// slow for this call's budget" apart from an ordinary hard failure
	// (a truncated reply, say).
	TimedOut bool
	// Timeout is the budget this call actually ran under — the timeout
	// argument, or defaultRerankTimeout when the caller left it at zero —
	// carried only so rerankTimeoutNote can name it without re-deriving
	// the default; on a path with a ceiling, obligationsCore passes the
	// ceiling less its joins when that is shorter (KS-10). Zero only when
	// matched was empty (this function's own early return, which never
	// reaches the timeout logic at all) or the re-rank was refused before
	// it asked (rerankRefused).
	Timeout time.Duration
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

// rerankWithinTiers asks d one batched decision.KindPredicate question per
// ELIGIBLE matched claim — "does this rule govern the edit described in the
// state" — where eligible (DP-24) means: matched's own governing-scope tier
// (scored.governing.tier) has at least one member inside the served
// window's first max positions of matched, which the caller hands this
// function ALREADY SORTED (obligationsCore's own less(), run before this is
// ever called) — see rerankEligible's own doc comment for the tier-window
// rule in full. A claim outside that window is never asked and so never
// answered; it falls straight through to the same "no answer, unmoved"
// path an unanswered chunk already used before DP-24 existed. Asking is
// chunked by decisionBatchPlan(d, ...) (DP-22/DP-26 — a question-count cap
// AND a state-byte budget, the byte budget sized off each claim's own
// rerankCriteria text — an item whose criteria alone exceeds the byte
// budget is never asked at all, exactly like a DP-24 window-excluded claim,
// and is recorded with a reason instead), the chunks run concurrently
// (bounded at rerankConcurrency, each its own Decide call sharing the same
// rendered State) under one shared context.WithTimeout(ctx, timeout)
// (DP-25 — see the timeout parameter's own doc below) — and
// rerankWithinTiers returns matched (the FULL input,
// eligible or not) re-ordered ONLY inside each rule's own governing-scope
// tier: a rule never crosses a tier and never drops (every input claim is
// present in the output, exactly once). Within a tier, a rule whose noul
// answer clears I42's unsure-band floor (usableNoul, decision.UnsureBelow)
// is ordered by descending noul, stable on ties; every other rule in that
// tier (never asked at all — DP-24's own window exclusion, or DP-25's own
// timeout — including every rule in a chunk whose Decide call itself
// failed, or an answer below the floor) keeps its EXACT original position —
// I42: "a decision in the unsure band never changes served content,"
// extended by DP-22 to "a decision that was never successfully asked never
// changes served content" either, and by DP-24/DP-25 to "a decision that
// was never even ASKED" the same way.
//
// max is the served window's own cap (obligationsCore's own max, already
// clamped to [1, MaxObligations] by the time it reaches here) — the boundary
// rerankEligible computes tiers against. timeout is this call's shared
// rerank budget; <= 0 means defaultRerankTimeout.
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
// chunk whose Decide call itself errored (including one cancelled by the
// DP-25 deadline), for the caller to log as an error row instead (I42/
// ADR-042 decision 3 apply the same way per chunk that the old single-call
// path applied to the whole batch: a provider error is swallowed into an
// untouched result FOR THAT CHUNK'S OWN QUESTIONS, never surfaced as a
// fatal error from this function — the caller decides how to report it).
func rerankWithinTiers(ctx context.Context, d decision.Decider, file, hunk string, matched []scored, max int, timeout time.Duration) rerankOutcome {
	if d == nil || len(matched) == 0 {
		return rerankOutcome{Matched: matched}
	}
	if timeout <= 0 {
		timeout = defaultRerankTimeout
	}
	rctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	eligible := rerankEligible(matched, max)

	state := rerankState(file, hunk)

	items := make([]decisionPlanItem, len(eligible))
	for i, m := range eligible {
		items[i] = decisionPlanItem{QuestionID: m.cl.ClaimID, StateBytes: len(rerankCriteria(m.cl))}
	}
	plan := decisionBatchPlan(d, items)
	oversizedReason := make(map[int]string, len(plan.Oversized))
	for _, o := range plan.Oversized {
		oversizedReason[o.Index] = o.Reason
	}

	var chunks [][]scored
	var oversizedRecords []decision.Record
	for _, idxs := range plan.Chunks {
		if len(idxs) == 1 {
			if reason, ok := oversizedReason[idxs[0]]; ok {
				// DP-26 (O110): this ONE claim's own rule text alone
				// exceeds the byte budget — never ask about it (exactly
				// like a DP-24 window-excluded claim: never asked, never
				// moves from its lexical position), but say why via a
				// synthetic Record (Confidence 0 — decision.BandOf reads
				// that as unsure, so a Recorder logs it applied:false,
				// matching "this was never actually answered").
				m := eligible[idxs[0]]
				oversizedRecords = append(oversizedRecords, decision.Record{
					Site:       "obligations",
					Provider:   d.Name(),
					QuestionID: m.cl.ClaimID,
					Kind:       decision.KindPredicate,
					Choice:     reason,
				})
				continue
			}
		}
		grp := make([]scored, len(idxs))
		for j, idx := range idxs {
			grp[j] = eligible[idx]
		}
		chunks = append(chunks, grp)
	}

	results := make([]rerankChunkResult, len(chunks))
	sem := make(chan struct{}, rerankConcurrency)
	var wg sync.WaitGroup
	for i, chunk := range chunks {
		wg.Add(1)
		sem <- struct{}{}
		//fathom:tenancy-allow bare-go bounded fan-out: a semaphore caps the goroutines and a WaitGroup joins them
		go func(i int, chunk []scored) {
			defer wg.Done()
			defer func() { <-sem }()

			req := decision.Request{State: state, Questions: make([]decision.Question, 0, len(chunk))}
			for _, m := range chunk {
				req.Questions = append(req.Questions, decision.Question{
					ID:           m.cl.ClaimID,
					Kind:         decision.KindPredicate,
					Instructions: "Does this rule govern the edit described in the state — would an editor of this file have to follow it here?",
					TrueCriteria: rerankCriteria(m.cl),
				})
			}
			start := time.Now()
			resp, err := d.Decide(decide.WithSite(rctx, decide.SiteObligationsRerank), req)
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
			if decision.CanonicalKind(a.Kind) == decision.KindPredicate {
				relevance[id] = a.Noul
			}
		}
		records = append(records, res.resp.Records("obligations")...)
	}
	records = append(records, oversizedRecords...)
	sort.Slice(records, func(i, j int) bool { return records[i].QuestionID < records[j].QuestionID })

	// DP-25: rctx's own deadline, not the parent ctx's — a caller-side
	// cancellation of ctx itself would also show up here (context.Canceled,
	// not DeadlineExceeded via errors.Is), which this deliberately does NOT
	// call a "budget" timeout; TimedOut only fires for this call's OWN
	// rerank_timeout_ms budget actually being exhausted.
	timedOut := errors.Is(rctx.Err(), context.DeadlineExceeded)

	return rerankOutcome{
		Matched:         reorderByNoulWithinTier(matched, relevance),
		Relevance:       relevance,
		Records:         records,
		Failures:        failures,
		Ranked:          ranked,
		ChunksTotal:     len(chunks),
		ChunksFailed:    len(failures),
		CandidatesTotal: len(matched),
		RankedCount:     len(eligible) - len(plan.Oversized),
		TimedOut:        timedOut,
		Timeout:         timeout,
	}
}

// rerankRefused is the outcome of a re-rank refused before it asked (KS-10:
// the joins spent the path's ceiling): matched unmoved, nothing asked, and
// err, the refusal, as the one failure naming how many questions it kept.
func rerankRefused(d decision.Decider, matched []scored, max int, err error) rerankOutcome {
	return rerankOutcome{
		Matched:         matched,
		Failures:        []DecisionError{{Site: "obligations", Provider: d.Name(), Questions: len(rerankEligible(matched, max)), Err: err}},
		ChunksTotal:     1,
		ChunksFailed:    1,
		CandidatesTotal: len(matched),
	}
}

// rerankEligible (DP-24, O109) narrows matched to the claims rerankWithinTiers
// should actually ask the Decider about. DP-7's tier law (governingscope.go)
// means only a claim whose OWN governing-scope tier has at least one member
// inside the served window's first max positions of the already-sorted
// matched slice can possibly change what gets served:
//
//   - a tier wholly inside the window (every member already within
//     [0, max)) is served in full regardless of ranking, but its own
//     INTERNAL reading order still benefits from a rerank, so it stays
//     eligible and is asked about in full;
//   - a tier straddling the cut (some members inside [0, max), some
//     outside) is exactly where a rerank's answer decides which of its own
//     members make the cut — eligible, and asked about IN FULL (not only
//     the members currently inside the window: the within-tier permutation
//     needs every member of a straddling tier to place them correctly);
//   - a tier wholly below the cut (no member reaches a position < max)
//     cannot be served no matter how it ranks — asking the Decider about it
//     spends a real, billed round trip on an answer the served payload can
//     never act on (O109 measured 154 of ~160 candidates on a 6-rule cap).
//     Never asked; its members keep their exact pre-rerank (lexical)
//     position, the same "never asked, never moves" contract an unanswered
//     chunk already carried before DP-24 existed.
//
// A root-scope call (every matched claim carries noGoverningScope, tier -1)
// has exactly one implicit tier, so every claim is always eligible — DP-24
// changes nothing there. max <= 0 (or >= len(matched)) also changes
// nothing: the window covers the whole slice, so every tier intersects it.
func rerankEligible(matched []scored, max int) []scored {
	limit := max
	if limit <= 0 || limit > len(matched) {
		limit = len(matched)
	}
	tiers := make(map[int]bool, limit)
	for i := 0; i < limit; i++ {
		tiers[matched[i].governing.tier] = true
	}
	eligible := make([]scored, 0, len(matched))
	for _, m := range matched {
		if tiers[m.governing.tier] {
			eligible = append(eligible, m)
		}
	}
	return eligible
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

// rerankTimeoutNote renders RankNote's text for a rerank whose shared
// rerank_timeout_ms budget was exhausted (rerankOutcome.TimedOut) — named
// distinctly from rerankFailureNote's "N of M chunks failed: <err>" so a
// reader can tell "the decider was too slow for this call's budget" apart
// from an ordinary hard failure (a truncated reply, say): the budget itself
// is disclosed, not a provider error string (there may not even be one yet
// for the chunks still in flight when the deadline fired — DecisionErrors
// carries context.DeadlineExceeded for those, not a provider-specific
// message).
func rerankTimeoutNote(timeout time.Duration, failed, total int) string {
	return fmt.Sprintf("decision budget %dms exceeded: %d of %d chunks unanswered", timeout.Milliseconds(), failed, total)
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
