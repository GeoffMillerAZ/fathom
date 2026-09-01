package expertchat

import (
	"context"
	"errors"
	"testing"
)

func TestFakeRetriever_ReturnsScriptedItems(t *testing.T) {
	items := []EvidenceItem{
		{Module: "m", ClaimID: "c1", Tier: "verified", Status: StatusFresh},
		{Module: "m", ClaimID: "c2", Tier: "verified", Status: StatusFresh},
	}
	f := &FakeRetriever{Items: items}

	got, err := f.Retrieve(context.Background(), "q1", 0)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d items, want 2", len(got))
	}

	if _, err := f.Retrieve(context.Background(), "q2", 1); err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if qs := f.Queries(); len(qs) != 2 || qs[0] != "q1" || qs[1] != "q2" {
		t.Fatalf("Queries() = %v, want [q1 q2]", qs)
	}
}

func TestFakeRetriever_RespectsLimit(t *testing.T) {
	f := &FakeRetriever{Items: []EvidenceItem{
		{ClaimID: "c1"}, {ClaimID: "c2"}, {ClaimID: "c3"},
	}}
	got, err := f.Retrieve(context.Background(), "q", 2)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d items, want 2 (limit applied)", len(got))
	}
}

func TestFakeRetriever_ReturnsScriptedError(t *testing.T) {
	wantErr := errors.New("boom")
	f := &FakeRetriever{Err: wantErr}
	_, err := f.Retrieve(context.Background(), "q", 0)
	if !errors.Is(err, wantErr) {
		t.Fatalf("got err %v, want %v", err, wantErr)
	}
}

func TestFakeRetriever_MutatingReturnedSliceDoesNotAffectFake(t *testing.T) {
	f := &FakeRetriever{Items: []EvidenceItem{{ClaimID: "c1"}}}
	got, _ := f.Retrieve(context.Background(), "q", 0)
	got[0].ClaimID = "mutated"
	got2, _ := f.Retrieve(context.Background(), "q", 0)
	if got2[0].ClaimID != "c1" {
		t.Fatalf("mutating a returned slice leaked into the fake's internal state: %q", got2[0].ClaimID)
	}
}
