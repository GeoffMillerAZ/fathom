package embeddingprovenance_test

import (
	"testing"

	embeddingprovenance "github.com/geoffmilleraz/agentic-kit/foundations/data/embedding-provenance"
)

// --- helpers ----------------------------------------------------------------

func validEP() embeddingprovenance.EmbeddingProvenance {
	return embeddingprovenance.EmbeddingProvenance{
		VectorID:  "vec-001",
		Model:     "text-embedding-ada-002",
		SourceID:  "doc-abc123",
		CreatedAt: 1717200000,
		Dims:      1536,
	}
}

// --- Validate: happy path ---------------------------------------------------

func TestValidate_Valid(t *testing.T) {
	t.Parallel()
	if err := validEP().Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_MinimalPositiveValues(t *testing.T) {
	t.Parallel()
	ep := embeddingprovenance.EmbeddingProvenance{
		VectorID:  "v",
		Model:     "m",
		SourceID:  "s",
		CreatedAt: 1,
		Dims:      1,
	}
	if err := ep.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Validate: field presence errors ----------------------------------------

func TestValidate_EmptyVectorID(t *testing.T) {
	t.Parallel()
	ep := validEP()
	ep.VectorID = ""
	if err := ep.Validate(); err == nil {
		t.Fatal("expected error for empty VectorID")
	}
}

func TestValidate_EmptyModel(t *testing.T) {
	t.Parallel()
	ep := validEP()
	ep.Model = ""
	if err := ep.Validate(); err == nil {
		t.Fatal("expected error for empty Model")
	}
}

func TestValidate_EmptySourceID(t *testing.T) {
	t.Parallel()
	ep := validEP()
	ep.SourceID = ""
	if err := ep.Validate(); err == nil {
		t.Fatal("expected error for empty SourceID")
	}
}

func TestValidate_ZeroCreatedAt(t *testing.T) {
	t.Parallel()
	ep := validEP()
	ep.CreatedAt = 0
	if err := ep.Validate(); err == nil {
		t.Fatal("expected error for zero CreatedAt")
	}
}

func TestValidate_NegativeCreatedAt(t *testing.T) {
	t.Parallel()
	ep := validEP()
	ep.CreatedAt = -1
	if err := ep.Validate(); err == nil {
		t.Fatal("expected error for negative CreatedAt")
	}
}

func TestValidate_ZeroDims(t *testing.T) {
	t.Parallel()
	ep := validEP()
	ep.Dims = 0
	if err := ep.Validate(); err == nil {
		t.Fatal("expected error for zero Dims")
	}
}

func TestValidate_NegativeDims(t *testing.T) {
	t.Parallel()
	ep := validEP()
	ep.Dims = -768
	if err := ep.Validate(); err == nil {
		t.Fatal("expected error for negative Dims")
	}
}

// --- Tracker: basic operations ----------------------------------------------

func TestTracker_RecordAndLookup(t *testing.T) {
	t.Parallel()
	tr := embeddingprovenance.NewTracker()
	ep := validEP()

	if err := tr.Record(ep); err != nil {
		t.Fatalf("Record: %v", err)
	}

	got, ok := tr.Lookup(ep.VectorID)
	if !ok {
		t.Fatal("Lookup returned not-found for recorded entry")
	}
	if got != ep {
		t.Errorf("Lookup returned %+v, want %+v", got, ep)
	}
}

func TestTracker_LookupMissing(t *testing.T) {
	t.Parallel()
	tr := embeddingprovenance.NewTracker()
	_, ok := tr.Lookup("nonexistent")
	if ok {
		t.Fatal("expected not-found for unknown VectorID")
	}
}

func TestTracker_IdempotentRecord(t *testing.T) {
	t.Parallel()
	tr := embeddingprovenance.NewTracker()
	ep := validEP()

	if err := tr.Record(ep); err != nil {
		t.Fatalf("first Record: %v", err)
	}
	// Recording the exact same entry again is idempotent.
	if err := tr.Record(ep); err != nil {
		t.Fatalf("idempotent re-Record: %v", err)
	}
}

func TestTracker_ConflictingRecord(t *testing.T) {
	t.Parallel()
	tr := embeddingprovenance.NewTracker()
	ep1 := validEP()

	ep2 := validEP()
	ep2.Model = "different-model" // same VectorID, different data

	if err := tr.Record(ep1); err != nil {
		t.Fatalf("first Record: %v", err)
	}
	if err := tr.Record(ep2); err == nil {
		t.Fatal("expected conflict error when re-recording VectorID with different data")
	}
}

func TestTracker_RecordInvalidEntry(t *testing.T) {
	t.Parallel()
	tr := embeddingprovenance.NewTracker()
	bad := embeddingprovenance.EmbeddingProvenance{} // all zero — invalid
	if err := tr.Record(bad); err == nil {
		t.Fatal("expected error when recording invalid provenance")
	}
}

func TestTracker_All(t *testing.T) {
	t.Parallel()
	tr := embeddingprovenance.NewTracker()

	ep1 := validEP()
	ep2 := embeddingprovenance.EmbeddingProvenance{
		VectorID:  "vec-002",
		Model:     "bge-base-en-v1.5",
		SourceID:  "chunk-xyz",
		CreatedAt: 1717201000,
		Dims:      768,
	}

	if err := tr.Record(ep1); err != nil {
		t.Fatalf("Record ep1: %v", err)
	}
	if err := tr.Record(ep2); err != nil {
		t.Fatalf("Record ep2: %v", err)
	}

	all := tr.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 entries from All(), got %d", len(all))
	}
}

func TestTracker_All_Empty(t *testing.T) {
	t.Parallel()
	tr := embeddingprovenance.NewTracker()
	if all := tr.All(); len(all) != 0 {
		t.Fatalf("expected empty All() on new tracker, got %d", len(all))
	}
}
