package semantic

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

func twoSections() []Section {
	return []Section{
		{Path: "docs/a.md", Subject: "docs/a.md#x", StartLine: 1, EndLine: 2, Text: "# X\ncontent x"},
		{Path: "docs/a.md", Subject: "docs/a.md#y", StartLine: 3, EndLine: 4, Text: "# Y\ncontent y"},
	}
}

// TestPrefilterSections_NoneIsIdentity: nil, decision.None{}, and a Decider
// whose Decide answers decision.ErrNoProvider all keep every section,
// skip nothing, produce no Record, and return no error — a nil Decider and
// decision.None make ZERO calls (detected without invoking Decide at all);
// a Decider that answers ErrNoProvider is called once (that is how the
// error is discovered) and then treated identically for the WHOLE input.
func TestPrefilterSections_NoneIsIdentity(t *testing.T) {
	ctx := context.Background()
	secs := twoSections()

	assertIdentity := func(t *testing.T, keep, skipped []Section, records []decision.Record, decErr *DecisionError, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if decErr != nil {
			t.Errorf("decErr = %+v, want nil", decErr)
		}
		if !reflect.DeepEqual(keep, secs) {
			t.Errorf("keep = %+v, want every section (%+v)", keep, secs)
		}
		if len(skipped) != 0 {
			t.Errorf("skipped = %+v, want none", skipped)
		}
		if len(records) != 0 {
			t.Errorf("records = %+v, want none", records)
		}
	}

	t.Run("nil decider, zero calls", func(t *testing.T) {
		keep, skipped, records, decErr, err := PrefilterSections(ctx, nil, 0.5, secs)
		assertIdentity(t, keep, skipped, records, decErr, err)
	})

	t.Run("decision.None, zero calls", func(t *testing.T) {
		keep, skipped, records, decErr, err := PrefilterSections(ctx, decision.None{}, 0.5, secs)
		assertIdentity(t, keep, skipped, records, decErr, err)
	})

	t.Run("ErrNoProvider from a generic Decider, one call then stop", func(t *testing.T) {
		md := decision.NewMockDecider("mock-no-provider", decision.MockEntry{Err: decision.ErrNoProvider})
		keep, skipped, records, decErr, err := PrefilterSections(ctx, md, 0.5, secs)
		assertIdentity(t, keep, skipped, records, decErr, err)
		if calls := md.Calls(); len(calls) != 1 {
			t.Errorf("Decide called %d times, want exactly 1 (the call that discovered ErrNoProvider)", len(calls))
		}
	})
}

// TestPrefilterSections_Batches: 45 sections chunk into 3 requests of
// 20/20/5 under a Decider NAMED "llm" (decisionBatchSize's case 2, DP-22 —
// the same rule rerankWithinTiers' own chunking test pins), and every
// section's stable Subject survives as its Question ID through to the
// matching Answer.
func TestPrefilterSections_Batches(t *testing.T) {
	ctx := context.Background()

	const total = 45
	secs := make([]Section, total)
	for i := range secs {
		id := fmt.Sprintf("docs/b.md#s%02d", i)
		secs[i] = Section{Path: "docs/b.md", Subject: id, StartLine: i + 1, EndLine: i + 1, Text: fmt.Sprintf("# S%02d\nrule %d", i, i)}
	}

	chunkSizes := []int{20, 20, 5}
	entries := make([]decision.MockEntry, len(chunkSizes))
	idx := 0
	for ci, n := range chunkSizes {
		answers := make(map[string]decision.Answer, n)
		for j := 0; j < n; j++ {
			sec := secs[idx]
			answers[sec.Subject] = decision.Answer{ID: sec.Subject, Kind: decision.KindPredicate, Noul: 0.9, Confidence: 0.9}
			idx++
		}
		entries[ci] = decision.MockEntry{Response: decision.Response{Provider: "llm", Answers: answers}}
	}
	md := decision.NewMockDecider("llm", entries...)

	keep, skipped, records, decErr, err := PrefilterSections(ctx, md, 0.5, secs)
	if err != nil {
		t.Fatalf("PrefilterSections: %v", err)
	}
	if decErr != nil {
		t.Errorf("decErr = %+v, want nil on success", decErr)
	}
	if len(skipped) != 0 {
		t.Errorf("skipped = %+v, want none (every mocked answer was 0.9 >= threshold 0.5)", skipped)
	}
	if !reflect.DeepEqual(keep, secs) {
		t.Errorf("keep did not preserve order/identity of all %d sections", total)
	}
	if len(records) != total {
		t.Errorf("records = %d, want %d (one per section, across all chunks)", len(records), total)
	}

	calls := md.Calls()
	if len(calls) != len(chunkSizes) {
		t.Fatalf("Decide called %d times, want %d (45 sections at chunk size 20)", len(calls), len(chunkSizes))
	}
	idx = 0
	for ci, want := range chunkSizes {
		qs := calls[ci].Questions
		if len(qs) != want {
			t.Errorf("chunk %d: %d questions, want %d", ci, len(qs), want)
		}
		for j, q := range qs {
			wantID := secs[idx].Subject
			if q.ID != wantID {
				t.Errorf("chunk %d question %d: ID = %q, want %q (ids not preserved in order)", ci, j, q.ID, wantID)
			}
			if q.Kind != decision.KindPredicate {
				t.Errorf("chunk %d question %d: Kind = %q, want %q", ci, j, q.Kind, decision.KindPredicate)
			}
			idx++
		}
	}
}

// TestPrefilterSections_UnrecognizedDeciderIsUnbounded: a Decider whose
// Name() is neither "llm" nor implements MaxQuestions()/MaxStateBytes()
// (decisionBatchCaps' default case, DP-26) still gets defaultBatchQuestions
// (64) and defaultBatchStateBytes (96 KiB) — no longer literally unbounded
// since DP-26 (O110), but 45 tiny sections stay well under BOTH caps, so
// they still go in ONE Decide call, not the 3
// TestPrefilterSections_Batches' "llm"-named mock gets chunked into (its
// count cap is the tighter prefilterChunkSize, 20). See
// TestPrefilterSections_DefaultCapsChunkByCount/ByBytes below for a case
// that actually exceeds one of the new default caps.
func TestPrefilterSections_UnrecognizedDeciderIsUnbounded(t *testing.T) {
	ctx := context.Background()

	const total = 45
	secs := make([]Section, total)
	answers := make(map[string]decision.Answer, total)
	for i := range secs {
		id := fmt.Sprintf("docs/c.md#s%02d", i)
		secs[i] = Section{Path: "docs/c.md", Subject: id, StartLine: i + 1, EndLine: i + 1, Text: fmt.Sprintf("# S%02d\nrule %d", i, i)}
		answers[id] = decision.Answer{ID: id, Kind: decision.KindPredicate, Noul: 0.9, Confidence: 0.9}
	}
	md := decision.NewMockDecider("mock-generic", decision.MockEntry{
		Response: decision.Response{Provider: "mock-generic", Answers: answers},
	})

	keep, _, _, decErr, err := PrefilterSections(ctx, md, 0.5, secs)
	if err != nil {
		t.Fatalf("PrefilterSections: %v", err)
	}
	if decErr != nil {
		t.Errorf("decErr = %+v, want nil on success", decErr)
	}
	if !reflect.DeepEqual(keep, secs) {
		t.Errorf("keep did not preserve all %d sections", total)
	}
	if calls := md.Calls(); len(calls) != 1 {
		t.Fatalf("Decide called %d times, want exactly 1 (an unrecognized Decider is never chunked)", len(calls))
	} else if len(calls[0].Questions) != total {
		t.Errorf("single call carried %d questions, want %d (the whole batch)", len(calls[0].Questions), total)
	}
}

// TestPrefilterSections_HardFailureReturnsDecisionError (DP-22, O109): a
// real Decider error (not ErrNoProvider) still aborts the whole call — the
// pre-DP-22 contract — but now also returns a *DecisionError naming the
// site, provider, question count and latency of the chunk that failed, so
// the caller (compile.go, pkg/client) can log it instead of it vanishing
// along with the aborted run.
func TestPrefilterSections_HardFailureReturnsDecisionError(t *testing.T) {
	ctx := context.Background()
	secs := twoSections()
	boom := errors.New("boom: provider exploded")
	md := decision.NewMockDecider("llm", decision.MockEntry{Err: boom})

	keep, skipped, records, decErr, err := PrefilterSections(ctx, md, 0.5, secs)
	if err == nil {
		t.Fatal("err = nil, want the wrapped provider error")
	}
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want it to wrap %v", err, boom)
	}
	if keep != nil || skipped != nil || records != nil {
		t.Errorf("keep/skipped/records = %+v/%+v/%+v, want all nil on hard failure", keep, skipped, records)
	}
	if decErr == nil {
		t.Fatal("decErr = nil, want a populated DecisionError")
	}
	if decErr.Site != "compile_prefilter" {
		t.Errorf("decErr.Site = %q, want %q", decErr.Site, "compile_prefilter")
	}
	if decErr.Provider != "llm" {
		t.Errorf("decErr.Provider = %q, want %q", decErr.Provider, "llm")
	}
	if decErr.Questions != len(secs) {
		t.Errorf("decErr.Questions = %d, want %d (both sections fit one chunk)", decErr.Questions, len(secs))
	}
	if decErr.Latency < 0 {
		t.Errorf("decErr.Latency = %v, want >= 0", decErr.Latency)
	}
	if decErr.Err == nil || !strings.Contains(decErr.Err.Error(), boom.Error()) {
		t.Errorf("decErr.Err = %v, want it to name %v", decErr.Err, boom)
	}
}

// TestPrefilterSections_DefaultCapsChunkByBytes is DP-26's own acceptance
// test (O110): under a Decider that is neither "llm" nor an opt-in
// (decisionBatchCaps' default case — the shape typesafe/Jev takes), 72
// sections whose own rendered block is ~2KiB each fit well under the
// 64-question count cap by count alone, but their SUM (~144KiB) is well
// over the 96KiB byte budget — so this must chunk by BYTES, not merely go
// out as one call the way the pre-DP-26 "unbounded unless named llm" rule
// would have. This is O110's exact failure shape, scaled down: a
// typesafe-named Decider handed every section's full text in one request.
// Every call's rendered State must fit the byte budget, every call's
// question count must fit the count cap, and every section must appear in
// exactly one call, each answered and kept.
func TestPrefilterSections_DefaultCapsChunkByBytes(t *testing.T) {
	ctx := context.Background()

	const total = 72
	const bodyBytes = 2000 // plus a small id/doc/heading/text preamble, ~2KiB/section block
	secs := make([]Section, total)
	answers := make(map[string]decision.Answer, total)
	for i := range secs {
		id := fmt.Sprintf("docs/big.md#s%03d", i)
		text := fmt.Sprintf("# S%03d\n%s", i, strings.Repeat("x", bodyBytes))
		secs[i] = Section{Path: "docs/big.md", Subject: id, StartLine: i + 1, EndLine: i + 1, Text: text}
		answers[id] = decision.Answer{ID: id, Kind: decision.KindPredicate, Noul: 0.9, Confidence: 0.9}
	}
	resp := decision.Response{Provider: "typesafe", Answers: answers}
	md := decision.NewMockDecider("typesafe",
		decision.MockEntry{Response: resp}, decision.MockEntry{Response: resp},
		decision.MockEntry{Response: resp}, decision.MockEntry{Response: resp},
		decision.MockEntry{Response: resp}, decision.MockEntry{Response: resp},
	)

	keep, skipped, _, decErr, err := PrefilterSections(ctx, md, 0.5, secs)
	if err != nil {
		t.Fatalf("PrefilterSections: %v", err)
	}
	if decErr != nil {
		t.Errorf("decErr = %+v, want nil on success", decErr)
	}
	if len(skipped) != 0 {
		t.Errorf("skipped = %+v, want none (every mocked answer was 0.9 >= threshold 0.5)", skipped)
	}
	if !reflect.DeepEqual(keep, secs) {
		t.Errorf("keep did not preserve order/identity of all %d sections", total)
	}

	calls := md.Calls()
	if len(calls) < 2 {
		t.Fatalf("Decide called %d times, want > 1 (72 ~2KiB sections must not fit one 96KiB-budget call)", len(calls))
	}
	const budget = 96 * 1024
	const countCap = 64
	totalQuestions := 0
	seen := make(map[string]bool, total)
	for i, c := range calls {
		if len(c.State) > budget {
			t.Errorf("call %d: State = %d bytes, want <= %d (byte budget)", i, len(c.State), budget)
		}
		if len(c.Questions) > countCap {
			t.Errorf("call %d: %d questions, want <= %d (count cap)", i, len(c.Questions), countCap)
		}
		totalQuestions += len(c.Questions)
		for _, q := range c.Questions {
			if seen[q.ID] {
				t.Errorf("question %q asked more than once across calls", q.ID)
			}
			seen[q.ID] = true
		}
	}
	if totalQuestions != total {
		t.Errorf("total questions across %d calls = %d, want %d", len(calls), totalQuestions, total)
	}
}

// TestPrefilterSections_OversizedSectionNeverSent is DP-26's own acceptance
// test (O110) for a single section too big to fit even alone: a 200KiB
// section is skipped, never appears in any Decide call's State or as a
// Question, and its skip reason is recorded via a synthetic decision.Record
// (Choice carries the reason, Confidence 0 — never actually answered);
// normal-sized sections around it are still asked about and kept.
func TestPrefilterSections_OversizedSectionNeverSent(t *testing.T) {
	ctx := context.Background()

	huge := Section{Path: "docs/huge.md", Subject: "docs/huge.md#giant", StartLine: 1, EndLine: 1, Text: "# Giant\n" + strings.Repeat("y", 200*1024)}
	secs := []Section{
		{Path: "docs/n.md", Subject: "docs/n.md#a", StartLine: 1, EndLine: 1, Text: "# A\nrule a"},
		huge,
		{Path: "docs/n.md", Subject: "docs/n.md#b", StartLine: 2, EndLine: 2, Text: "# B\nrule b"},
	}
	answers := map[string]decision.Answer{
		"docs/n.md#a": {ID: "docs/n.md#a", Kind: decision.KindPredicate, Noul: 0.9, Confidence: 0.9},
		"docs/n.md#b": {ID: "docs/n.md#b", Kind: decision.KindPredicate, Noul: 0.9, Confidence: 0.9},
	}
	resp := decision.Response{Provider: "typesafe", Answers: answers}
	md := decision.NewMockDecider("typesafe", decision.MockEntry{Response: resp}, decision.MockEntry{Response: resp})

	keep, skipped, records, decErr, err := PrefilterSections(ctx, md, 0.5, secs)
	if err != nil {
		t.Fatalf("PrefilterSections: %v", err)
	}
	if decErr != nil {
		t.Errorf("decErr = %+v, want nil (an oversized item is skipped, not a hard failure)", decErr)
	}
	if len(skipped) != 1 || skipped[0].Subject != huge.Subject {
		t.Fatalf("skipped = %+v, want exactly [%s]", skipped, huge.Subject)
	}
	if len(keep) != 2 || keep[0].Subject != "docs/n.md#a" || keep[1].Subject != "docs/n.md#b" {
		t.Errorf("keep = %+v, want the two normal sections, in order", keep)
	}

	for _, c := range md.Calls() {
		if strings.Contains(c.State, "Giant") {
			t.Error("the oversized section's text reached a Decide call's State")
		}
		for _, q := range c.Questions {
			if q.ID == huge.Subject {
				t.Error("the oversized section was asked about as a Question")
			}
		}
	}

	var reasonRec *decision.Record
	for i := range records {
		if records[i].QuestionID == huge.Subject {
			reasonRec = &records[i]
		}
	}
	if reasonRec == nil {
		t.Fatal("records carries no entry for the oversized section")
	}
	if reasonRec.Choice == "" {
		t.Error("oversized record's Choice (reason) is empty")
	}
	if !strings.Contains(reasonRec.Choice, "budget") {
		t.Errorf("oversized record's reason = %q, want it to name the byte budget", reasonRec.Choice)
	}
	if reasonRec.Confidence != 0 {
		t.Errorf("oversized record's Confidence = %v, want 0 (never actually answered)", reasonRec.Confidence)
	}
}
