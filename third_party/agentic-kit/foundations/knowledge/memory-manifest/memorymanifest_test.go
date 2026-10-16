package memorymanifest_test

import (
	"testing"
	"time"

	mm "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/knowledge/memory-manifest"
)

// fixedTime is a deterministic reference instant used throughout the tests.
var fixedTime = time.Date(2024, 11, 1, 9, 0, 0, 0, time.UTC)

// makeEntry returns a ManifestEntry with all fields populated.
func makeEntry(cardID, hash string) mm.ManifestEntry {
	return mm.ManifestEntry{
		CardID:    cardID,
		Hash:      hash,
		CreatedAt: fixedTime,
	}
}

// TestAddAndList verifies that added entries are returned by List in the same order.
func TestAddAndList(t *testing.T) {
	var m mm.Manifest

	e1 := makeEntry("card-001", "hash-aaa")
	e2 := makeEntry("card-002", "hash-bbb")
	e3 := makeEntry("card-003", "hash-ccc")

	m.Add(e1)
	m.Add(e2)
	m.Add(e3)

	got := m.List()
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got))
	}
	if got[0].CardID != "card-001" || got[1].CardID != "card-002" || got[2].CardID != "card-003" {
		t.Errorf("wrong order: %v", []string{got[0].CardID, got[1].CardID, got[2].CardID})
	}
}

// TestListReturnsEmptySliceWhenNoEntries verifies that List on a fresh
// Manifest returns a non-nil, empty slice.
func TestListReturnsEmptySliceWhenNoEntries(t *testing.T) {
	var m mm.Manifest
	got := m.List()
	if got == nil {
		t.Error("List should return non-nil slice, got nil")
	}
	if len(got) != 0 {
		t.Errorf("expected 0 entries, got %d", len(got))
	}
}

// TestListReturnsCopy verifies that mutating the returned slice does not
// affect subsequent calls to List (append-only invariant).
func TestListReturnsCopy(t *testing.T) {
	var m mm.Manifest
	m.Add(makeEntry("card-001", "hash-aaa"))

	snap1 := m.List()
	// Overwrite the first element in the snapshot.
	snap1[0] = mm.ManifestEntry{CardID: "MUTATED", Hash: "x", CreatedAt: fixedTime}

	snap2 := m.List()
	if snap2[0].CardID != "card-001" {
		t.Errorf("manifest was mutated via List slice; got CardID=%q", snap2[0].CardID)
	}
}

// TestContainsReturnsTrueForPresentEntry verifies Contains finds a known card.
func TestContainsReturnsTrueForPresentEntry(t *testing.T) {
	var m mm.Manifest
	m.Add(makeEntry("card-alpha", "h1"))
	m.Add(makeEntry("card-beta", "h2"))

	if !m.Contains("card-alpha") {
		t.Error("Contains should return true for card-alpha")
	}
	if !m.Contains("card-beta") {
		t.Error("Contains should return true for card-beta")
	}
}

// TestContainsReturnsFalseForAbsentEntry verifies Contains returns false for
// a card that was never added.
func TestContainsReturnsFalseForAbsentEntry(t *testing.T) {
	var m mm.Manifest
	m.Add(makeEntry("card-001", "h1"))

	if m.Contains("card-999") {
		t.Error("Contains should return false for card-999")
	}
}

// TestContainsEmptyManifest verifies Contains on an empty manifest returns false.
func TestContainsEmptyManifest(t *testing.T) {
	var m mm.Manifest
	if m.Contains("anything") {
		t.Error("Contains on empty manifest should return false")
	}
}

// TestAddSetsCreatedAtWhenZero verifies that Add fills in CreatedAt when
// the entry is provided with the zero value.
func TestAddSetsCreatedAtWhenZero(t *testing.T) {
	var m mm.Manifest
	e := mm.ManifestEntry{CardID: "card-zero", Hash: "h1"} // CreatedAt is zero
	before := time.Now().UTC()
	m.Add(e)
	after := time.Now().UTC()

	got := m.List()
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	ts := got[0].CreatedAt
	if ts.IsZero() {
		t.Error("CreatedAt should have been set by Add")
	}
	if ts.Before(before) || ts.After(after) {
		t.Errorf("CreatedAt %v outside expected window [%v, %v]", ts, before, after)
	}
}

// TestAddPreservesExplicitCreatedAt verifies that Add does not overwrite an
// explicitly provided CreatedAt timestamp.
func TestAddPreservesExplicitCreatedAt(t *testing.T) {
	var m mm.Manifest
	e := makeEntry("card-ts", "h1") // CreatedAt = fixedTime
	m.Add(e)

	got := m.List()
	if got[0].CreatedAt != fixedTime {
		t.Errorf("CreatedAt was overwritten: got %v, want %v", got[0].CreatedAt, fixedTime)
	}
}

// TestAddDoesNotMutatePastEntries verifies the append-only guarantee:
// adding a new entry does not change the data of any existing entry.
func TestAddDoesNotMutatePastEntries(t *testing.T) {
	var m mm.Manifest
	m.Add(makeEntry("card-001", "hash-aaa"))

	snap1 := m.List()
	m.Add(makeEntry("card-002", "hash-bbb"))
	snap2 := m.List()

	if snap2[0].CardID != "card-001" || snap2[0].Hash != "hash-aaa" {
		t.Errorf("first entry was mutated after second add: %+v", snap2[0])
	}
	if snap1[0].CardID != "card-001" {
		t.Errorf("first snapshot was mutated: %+v", snap1[0])
	}
}

// TestManifestIsAppendOnly verifies that the manifest grows monotonically.
func TestManifestIsAppendOnly(t *testing.T) {
	var m mm.Manifest

	for i := 0; i < 5; i++ {
		id := "card-" + string(rune('a'+i))
		m.Add(makeEntry(id, "h"))
		if len(m.List()) != i+1 {
			t.Fatalf("after %d adds, expected %d entries, got %d", i+1, i+1, len(m.List()))
		}
	}
}

// TestAddUsesInjectedClock pins the determinism seam: a zero CreatedAt is
// stamped from the manifest's injected clock, never the wall clock.
func TestAddUsesInjectedClock(t *testing.T) {
	fixed := time.Date(2026, 6, 10, 12, 0, 0, 0, time.FixedZone("MST", -7*3600))
	m := mm.Manifest{Now: func() time.Time { return fixed }}
	m.Add(mm.ManifestEntry{CardID: "card-1", Hash: "abc"})
	got := m.List()[0].CreatedAt
	if !got.Equal(fixed) {
		t.Fatalf("CreatedAt = %v, want injected clock %v", got, fixed)
	}
	if got.Location() != time.UTC {
		t.Fatalf("CreatedAt location = %v, want UTC", got.Location())
	}
	// An explicit CreatedAt is never overwritten.
	explicit := fixed.Add(-time.Hour)
	m.Add(mm.ManifestEntry{CardID: "card-2", Hash: "def", CreatedAt: explicit})
	if got := m.List()[1].CreatedAt; !got.Equal(explicit) {
		t.Fatalf("explicit CreatedAt overwritten: got %v, want %v", got, explicit)
	}
}
