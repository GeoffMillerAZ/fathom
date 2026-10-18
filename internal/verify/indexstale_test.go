package verify

import "testing"

func staleRow(id, detail string) TaskVerdict {
	return TaskVerdict{TaskID: id, Claimed: true, Status: Unknown, Label: "gone-unknown",
		Criteria: []Criterion{{Name: goneCriterion, Status: Unknown, Detail: detail}}}
}

func TestIndexStaleOf_AggregatesAndParsesPath(t *testing.T) {
	d := goneUnprovenPrefix + "the working tree has uncommitted changes since the index was ingested" + DirtyPathMarker + "a/b.txt) " + IndexStaleMarker
	reps := []Report{{Tasks: []TaskVerdict{staleRow("A", d), staleRow("B", d)}}}
	agg, ok := IndexStaleOf(reps)
	if !ok || agg.Rows != 2 || agg.FirstPath != "a/b.txt" || agg.Remedy != IndexStaleRemedy {
		t.Fatalf("agg = %+v ok=%v", agg, ok)
	}
	if !OperationalShortfall(reps, 2, true) {
		t.Error("all failures stale: want shortfall")
	}
	if OperationalShortfall(reps, 3, true) {
		t.Error("an extra failure must not read as stale")
	}
	if OperationalShortfall(reps, 2, false) {
		t.Error("non-strict never fails on Unknown")
	}
}

func TestIndexStaleOf_IgnoresOtherUnknown(t *testing.T) {
	r := staleRow("A", goneUnprovenPrefix+"the typed pass failed for x")
	if _, ok := IndexStaleOf([]Report{{Tasks: []TaskVerdict{r}}}); ok {
		t.Error("typed-pass failure is not index-stale")
	}
}
