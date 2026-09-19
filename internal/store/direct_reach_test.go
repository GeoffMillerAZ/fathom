package store

import (
	"context"
	"path/filepath"
	"testing"
)

// The Store interface contract suite exercises these through the interface;
// these two tests reach the concrete methods directly so the plan rows SU-1
// and SU-2 are provable by static test reach.
func TestEnsureIndexes_DirectReach(t *testing.T) {
	st, err := NewSQLiteStore(filepath.Join(t.TempDir(), "s.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.EnsureIndexes(context.Background()); err != nil {
		t.Fatalf("EnsureIndexes: %v", err)
	}
}

func TestClaimsForServe_DirectReach(t *testing.T) {
	st, err := NewSQLiteStore(filepath.Join(t.TempDir(), "s.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	got, err := st.ClaimsForServe(context.Background(), "svc", nil, nil, nil)
	if err != nil {
		t.Fatalf("ClaimsForServe: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("empty store served %d claims", len(got))
	}
}
