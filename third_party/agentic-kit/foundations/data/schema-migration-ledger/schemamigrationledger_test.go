package schemamigrationledger_test

import (
	"testing"

	schemamigrationledger "github.com/geoffmilleraz/agentic-kit/foundations/data/schema-migration-ledger"
)

// --- helpers ----------------------------------------------------------------

func rec(id, from, to string, appliedAt int64) schemamigrationledger.MigrationRecord {
	return schemamigrationledger.MigrationRecord{
		ID:        id,
		From:      from,
		To:        to,
		AppliedAt: appliedAt,
	}
}

// --- Ledger: happy path -----------------------------------------------------

func TestLedger_AppendAndApplied(t *testing.T) {
	t.Parallel()
	l := schemamigrationledger.NewLedger()

	r1 := rec("0001_init", "", "v1", 1717200000)
	r2 := rec("0002_add_col", "v1", "v2", 1717201000)

	if err := l.Append(r1); err != nil {
		t.Fatalf("Append r1: %v", err)
	}
	if err := l.Append(r2); err != nil {
		t.Fatalf("Append r2: %v", err)
	}

	applied := l.Applied()
	if len(applied) != 2 {
		t.Fatalf("expected 2 applied records, got %d", len(applied))
	}
	if applied[0] != r1 {
		t.Errorf("applied[0] = %+v, want %+v", applied[0], r1)
	}
	if applied[1] != r2 {
		t.Errorf("applied[1] = %+v, want %+v", applied[1], r2)
	}
}

func TestLedger_Current_AfterMigrations(t *testing.T) {
	t.Parallel()
	l := schemamigrationledger.NewLedger()

	if err := l.Append(rec("0001", "", "v1", 1)); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if got := l.Current(); got != "v1" {
		t.Errorf("Current() = %q, want %q", got, "v1")
	}

	if err := l.Append(rec("0002", "v1", "v2", 2)); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if got := l.Current(); got != "v2" {
		t.Errorf("Current() = %q after second migration, want %q", got, "v2")
	}
}

func TestLedger_Current_EmptyLedger(t *testing.T) {
	t.Parallel()
	l := schemamigrationledger.NewLedger()
	if got := l.Current(); got != "" {
		t.Errorf("Current() on empty ledger = %q, want empty string", got)
	}
}

func TestLedger_Applied_IsACopy(t *testing.T) {
	// Mutating the returned slice must not affect the ledger.
	t.Parallel()
	l := schemamigrationledger.NewLedger()
	if err := l.Append(rec("0001", "", "v1", 1)); err != nil {
		t.Fatalf("Append: %v", err)
	}

	snap := l.Applied()
	snap[0].ID = "tampered"

	// The ledger's internal state should be unchanged.
	fresh := l.Applied()
	if fresh[0].ID == "tampered" {
		t.Error("Applied() returned a reference to internal state; mutation leaked")
	}
}

// --- Ledger: append-only / duplicate prevention ---------------------------

func TestLedger_DuplicateIDRejected(t *testing.T) {
	t.Parallel()
	l := schemamigrationledger.NewLedger()

	r := rec("0001_init", "", "v1", 1717200000)
	if err := l.Append(r); err != nil {
		t.Fatalf("first Append: %v", err)
	}
	if err := l.Append(r); err == nil {
		t.Fatal("expected error when appending duplicate migration ID")
	}
}

func TestLedger_DifferentIDs_AllAccepted(t *testing.T) {
	t.Parallel()
	l := schemamigrationledger.NewLedger()

	for i, r := range []schemamigrationledger.MigrationRecord{
		rec("0001", "", "v1", 1),
		rec("0002", "v1", "v2", 2),
		rec("0003", "v2", "v3", 3),
	} {
		if err := l.Append(r); err != nil {
			t.Fatalf("Append[%d]: %v", i, err)
		}
	}
	if len(l.Applied()) != 3 {
		t.Errorf("expected 3 applied records, got %d", len(l.Applied()))
	}
}

// --- Ledger: validation errors on Append -----------------------------------

func TestLedger_Append_EmptyID(t *testing.T) {
	t.Parallel()
	l := schemamigrationledger.NewLedger()
	err := l.Append(rec("", "", "v1", 1))
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestLedger_Append_EmptyTo(t *testing.T) {
	t.Parallel()
	l := schemamigrationledger.NewLedger()
	err := l.Append(rec("0001", "v1", "", 1))
	if err == nil {
		t.Fatal("expected error for empty To")
	}
}

func TestLedger_Append_ZeroAppliedAt(t *testing.T) {
	t.Parallel()
	l := schemamigrationledger.NewLedger()
	err := l.Append(rec("0001", "", "v1", 0))
	if err == nil {
		t.Fatal("expected error for zero AppliedAt")
	}
}

func TestLedger_Append_NegativeAppliedAt(t *testing.T) {
	t.Parallel()
	l := schemamigrationledger.NewLedger()
	err := l.Append(rec("0001", "", "v1", -1))
	if err == nil {
		t.Fatal("expected error for negative AppliedAt")
	}
}

// --- Deterministic replay --------------------------------------------------

func TestLedger_DeterministicReplay(t *testing.T) {
	// Replaying the same sequence of Append calls on two independent ledgers
	// must produce identical Applied() snapshots.
	t.Parallel()
	migrations := []schemamigrationledger.MigrationRecord{
		rec("0001", "", "v1", 100),
		rec("0002", "v1", "v2", 200),
		rec("0003", "v2", "v3", 300),
	}

	build := func() *schemamigrationledger.Ledger {
		l := schemamigrationledger.NewLedger()
		for _, m := range migrations {
			if err := l.Append(m); err != nil {
				t.Fatalf("Append: %v", err)
			}
		}
		return l
	}

	l1 := build()
	l2 := build()

	a1 := l1.Applied()
	a2 := l2.Applied()

	if len(a1) != len(a2) {
		t.Fatalf("replay lengths differ: %d vs %d", len(a1), len(a2))
	}
	for i := range a1 {
		if a1[i] != a2[i] {
			t.Errorf("replay mismatch at index %d: %+v vs %+v", i, a1[i], a2[i])
		}
	}
}
