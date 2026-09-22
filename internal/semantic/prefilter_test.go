package semantic

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/decision"
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

	assertIdentity := func(t *testing.T, keep, skipped []Section, records []decision.Record, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
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
		keep, skipped, records, err := PrefilterSections(ctx, nil, 0.5, secs)
		assertIdentity(t, keep, skipped, records, err)
	})

	t.Run("decision.None, zero calls", func(t *testing.T) {
		keep, skipped, records, err := PrefilterSections(ctx, decision.None{}, 0.5, secs)
		assertIdentity(t, keep, skipped, records, err)
	})

	t.Run("ErrNoProvider from a generic Decider, one call then stop", func(t *testing.T) {
		md := decision.NewMockDecider("mock-no-provider", decision.MockEntry{Err: decision.ErrNoProvider})
		keep, skipped, records, err := PrefilterSections(ctx, md, 0.5, secs)
		assertIdentity(t, keep, skipped, records, err)
		if calls := md.Calls(); len(calls) != 1 {
			t.Errorf("Decide called %d times, want exactly 1 (the call that discovered ErrNoProvider)", len(calls))
		}
	})
}

// TestPrefilterSections_Batches: 45 sections chunk into 3 requests of
// 20/20/5, and every section's stable Subject survives as its Question ID
// through to the matching Answer.
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
			answers[sec.Subject] = decision.Answer{ID: sec.Subject, Kind: decision.KindNoul, Noul: 0.9, Confidence: 0.9}
			idx++
		}
		entries[ci] = decision.MockEntry{Response: decision.Response{Provider: "mock-batch", Answers: answers}}
	}
	md := decision.NewMockDecider("mock-batch", entries...)

	keep, skipped, records, err := PrefilterSections(ctx, md, 0.5, secs)
	if err != nil {
		t.Fatalf("PrefilterSections: %v", err)
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
			if q.Kind != decision.KindNoul {
				t.Errorf("chunk %d question %d: Kind = %q, want %q", ci, j, q.Kind, decision.KindNoul)
			}
			idx++
		}
	}
}
