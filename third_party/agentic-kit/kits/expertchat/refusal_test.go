package expertchat

import (
	"errors"
	"strings"
	"testing"
)

func TestRefusal_ImplementsError(t *testing.T) {
	var err error = &Refusal{Reason: RefusalNoEvidence, Query: "what is X?", Detail: "nothing came back"}
	if !strings.Contains(err.Error(), "no-evidence") {
		t.Fatalf("Error() = %q, want it to mention the reason", err.Error())
	}
	if !strings.Contains(err.Error(), "what is X?") {
		t.Fatalf("Error() = %q, want it to mention the query", err.Error())
	}
	if !strings.Contains(err.Error(), "nothing came back") {
		t.Fatalf("Error() = %q, want it to mention the detail", err.Error())
	}
}

func TestRefusal_ErrorsAs(t *testing.T) {
	var err error = &Refusal{Reason: RefusalBelowTierThreshold, Query: "q", Detail: "d"}
	var r *Refusal
	if !errors.As(err, &r) {
		t.Fatal("errors.As failed to unwrap *Refusal")
	}
	if r.Reason != RefusalBelowTierThreshold {
		t.Fatalf("unwrapped Reason = %v, want %v", r.Reason, RefusalBelowTierThreshold)
	}
}

func TestRefusalReason_TaxonomyIsFiveDistinctValues(t *testing.T) {
	reasons := map[RefusalReason]bool{
		RefusalNoEvidence:            true,
		RefusalBelowTierThreshold:    true,
		RefusalStaleOnlyEvidence:     true,
		RefusalFabricatedCitation:    true,
		RefusalEvidenceDoesNotAnswer: true,
	}
	if len(reasons) != 5 {
		t.Fatalf("expected 5 distinct refusal reasons, got %d: %v", len(reasons), reasons)
	}
}
