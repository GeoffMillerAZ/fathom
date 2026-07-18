package recordfingerprint_test

import (
	"strings"
	"testing"
	"time"

	rf "github.com/geoffmilleraz/agentic-kit/foundations/evidence/record-fingerprint"
)

// advancingClock returns a clock backed by a mutable tick pointer so tests can
// advance time between observations.
func advancingClock(tick *time.Time) func() time.Time {
	return func() time.Time { return *tick }
}

// validHash is a canonical 64-char lowercase hex SHA-256 digest used in tests
// where the exact content does not matter.
var validHash = rf.HashContent([]byte("test-content"))

// TestLifecycle covers new → unchanged → changed for a single record ID,
// using an advancing clock to prove SeenAt semantics.
func TestLifecycle(t *testing.T) {
	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)

	tick := t1
	s := rf.NewStore(advancingClock(&tick))

	content1 := []byte("version one")
	content2 := []byte("version two")

	// First observe: new; SeenAt == t1.
	kind, fp, err := s.Observe("rec-1", content1)
	if err != nil {
		t.Fatalf("first observe: unexpected error: %v", err)
	}
	if kind != rf.ChangeNew {
		t.Fatalf("first observe: want ChangeNew, got %q", kind)
	}
	if fp.RecordID != "rec-1" {
		t.Fatalf("fp.RecordID: want rec-1, got %q", fp.RecordID)
	}
	if fp.ContentHash == "" {
		t.Fatal("fp.ContentHash must not be empty")
	}
	if !fp.SeenAt.Equal(t1) {
		t.Fatalf("SeenAt: want %v, got %v", t1, fp.SeenAt)
	}

	// Advance the clock to t2 so that a broken "always refresh SeenAt"
	// implementation would now produce t2, making the next assertion fail.
	tick = t2

	// Same content, clock now at t2: must be unchanged and SeenAt must still be t1.
	kind2, fp2, err2 := s.Observe("rec-1", content1)
	if err2 != nil {
		t.Fatalf("second observe: unexpected error: %v", err2)
	}
	if kind2 != rf.ChangeUnchanged {
		t.Fatalf("second observe (same): want ChangeUnchanged, got %q", kind2)
	}
	if fp2.ContentHash != fp.ContentHash {
		t.Fatal("unchanged: ContentHash must not change")
	}
	// This assertion now proves the invariant: if a broken implementation
	// refreshed SeenAt on every observe, it would return t2 and fail here.
	if !fp2.SeenAt.Equal(t1) {
		t.Fatalf("unchanged: SeenAt must be preserved from first observation; want %v, got %v", t1, fp2.SeenAt)
	}

	// Different content, clock still at t2: changed; SeenAt updated to t2.
	kind3, fp3, err3 := s.Observe("rec-1", content2)
	if err3 != nil {
		t.Fatalf("third observe: unexpected error: %v", err3)
	}
	if kind3 != rf.ChangeChanged {
		t.Fatalf("third observe (changed): want ChangeChanged, got %q", kind3)
	}
	if fp3.ContentHash == fp.ContentHash {
		t.Fatal("changed: ContentHash must differ from original")
	}
	// SeenAt must be t2 (the current clock value at the time of the change).
	if !fp3.SeenAt.Equal(t2) {
		t.Fatalf("changed: SeenAt want %v, got %v", t2, fp3.SeenAt)
	}
}

// TestHashContentDeterminism verifies sha256 output is stable and input-sensitive.
func TestHashContentDeterminism(t *testing.T) {
	a := rf.HashContent([]byte("hello"))
	b := rf.HashContent([]byte("hello"))
	c := rf.HashContent([]byte("world"))

	if a != b {
		t.Fatalf("same input: hashes differ: %q vs %q", a, b)
	}
	if a == c {
		t.Fatal("different input: hashes must differ")
	}
	// sha256 hex is always 64 lowercase hex chars.
	if len(a) != 64 {
		t.Fatalf("HashContent length: want 64, got %d", len(a))
	}
	if strings.ToLower(a) != a {
		t.Fatalf("HashContent must be lowercase hex, got %q", a)
	}
}

// TestObserveHash uses the precomputed-hash path and verifies it mirrors Observe.
func TestObserveHash(t *testing.T) {
	s := rf.NewStore(nil)
	hash := rf.HashContent([]byte("data"))

	kind, fp, err := s.ObserveHash("rec-h", hash)
	if err != nil {
		t.Fatalf("first ObserveHash: unexpected error: %v", err)
	}
	if kind != rf.ChangeNew {
		t.Fatalf("first ObserveHash: want ChangeNew, got %q", kind)
	}
	if fp.ContentHash != hash {
		t.Fatalf("ContentHash mismatch: want %q, got %q", hash, fp.ContentHash)
	}

	kind2, _, err2 := s.ObserveHash("rec-h", hash)
	if err2 != nil {
		t.Fatalf("second ObserveHash: unexpected error: %v", err2)
	}
	if kind2 != rf.ChangeUnchanged {
		t.Fatalf("second ObserveHash same hash: want ChangeUnchanged, got %q", kind2)
	}

	different := rf.HashContent([]byte("other"))
	kind3, _, err3 := s.ObserveHash("rec-h", different)
	if err3 != nil {
		t.Fatalf("third ObserveHash: unexpected error: %v", err3)
	}
	if kind3 != rf.ChangeChanged {
		t.Fatalf("ObserveHash different hash: want ChangeChanged, got %q", kind3)
	}
}

// TestObserveHashRejectsInvalidInputs verifies that ObserveHash returns an
// error for an empty RecordID or a ContentHash that is not a 64-char
// lowercase hex SHA-256 digest.
func TestObserveHashRejectsInvalidInputs(t *testing.T) {
	cases := []struct {
		name        string
		recordID    string
		contentHash string
	}{
		{"empty-RecordID", "", validHash},
		{"empty-ContentHash", "rec-1", ""},
		{"short-ContentHash", "rec-1", "abc123"},
		{"md5-length-hash", "rec-1", strings.Repeat("a", 32)},
		{"uppercase-hex", "rec-1", strings.ToUpper(validHash)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := rf.NewStore(nil)
			_, _, err := s.ObserveHash(tc.recordID, tc.contentHash)
			if err == nil {
				t.Errorf("%s: expected error, got nil", tc.name)
			}
		})
	}
}

// TestObserveHashValidInputsSucceed verifies the happy-path three-return signature.
func TestObserveHashValidInputsSucceed(t *testing.T) {
	s := rf.NewStore(nil)
	kind, fp, err := s.ObserveHash("rec-ok", validHash)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kind != rf.ChangeNew {
		t.Fatalf("want ChangeNew, got %q", kind)
	}
	if fp.RecordID != "rec-ok" {
		t.Fatalf("RecordID: want rec-ok, got %q", fp.RecordID)
	}
}

// TestObserveRejectsEmptyRecordID verifies Observe propagates the error from
// ObserveHash for empty RecordID.
func TestObserveRejectsEmptyRecordID(t *testing.T) {
	s := rf.NewStore(nil)
	_, _, err := s.Observe("", []byte("content"))
	if err == nil {
		t.Fatal("Observe with empty RecordID: expected error, got nil")
	}
}

// TestAllSorted verifies All returns fingerprints sorted by RecordID.
func TestAllSorted(t *testing.T) {
	s := rf.NewStore(nil)
	ids := []string{"zebra", "alpha", "middle", "beta"}
	for _, id := range ids {
		if _, _, err := s.Observe(id, []byte(id)); err != nil {
			t.Fatalf("Observe(%q): unexpected error: %v", id, err)
		}
	}

	all := s.All()
	if len(all) != len(ids) {
		t.Fatalf("All: want %d items, got %d", len(ids), len(all))
	}
	for i := 1; i < len(all); i++ {
		if all[i].RecordID < all[i-1].RecordID {
			t.Fatalf("All not sorted at index %d: %q < %q", i, all[i].RecordID, all[i-1].RecordID)
		}
	}
	if all[0].RecordID != "alpha" {
		t.Fatalf("All[0]: want alpha, got %q", all[0].RecordID)
	}
}

// TestGetHitMiss exercises Get for present and absent record IDs.
func TestGetHitMiss(t *testing.T) {
	s := rf.NewStore(nil)
	if _, _, err := s.Observe("exists", []byte("payload")); err != nil {
		t.Fatalf("Observe: unexpected error: %v", err)
	}

	fp, ok := s.Get("exists")
	if !ok {
		t.Fatal("Get(exists): want true, got false")
	}
	if fp.RecordID != "exists" {
		t.Fatalf("Get(exists): RecordID want exists, got %q", fp.RecordID)
	}

	_, ok2 := s.Get("missing")
	if ok2 {
		t.Fatal("Get(missing): want false, got true")
	}
}

// TestLen verifies Len tracks distinct IDs.
func TestLen(t *testing.T) {
	s := rf.NewStore(nil)
	if s.Len() != 0 {
		t.Fatalf("initial Len: want 0, got %d", s.Len())
	}
	if _, _, err := s.Observe("a", []byte("1")); err != nil {
		t.Fatalf("Observe(a): unexpected error: %v", err)
	}
	if _, _, err := s.Observe("b", []byte("2")); err != nil {
		t.Fatalf("Observe(b): unexpected error: %v", err)
	}
	if _, _, err := s.Observe("a", []byte("1")); err != nil { // re-observe same ID
		t.Fatalf("re-Observe(a): unexpected error: %v", err)
	}
	if s.Len() != 2 {
		t.Fatalf("Len after 2 distinct IDs: want 2, got %d", s.Len())
	}
}

// TestValidateRejectsEmpty ensures Fingerprint.Validate catches empty and
// malformed fields.
func TestValidateRejectsEmpty(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		fp   rf.Fingerprint
	}{
		{"empty-RecordID", rf.Fingerprint{RecordID: "", ContentHash: validHash, SeenAt: now}},
		{"empty-ContentHash", rf.Fingerprint{RecordID: "r1", ContentHash: "", SeenAt: now}},
		{"short-ContentHash", rf.Fingerprint{RecordID: "r1", ContentHash: "abc", SeenAt: now}},
		{"uppercase-ContentHash", rf.Fingerprint{RecordID: "r1", ContentHash: strings.ToUpper(validHash), SeenAt: now}},
		{"zero-SeenAt", rf.Fingerprint{RecordID: "r1", ContentHash: validHash, SeenAt: time.Time{}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.fp.Validate(); err == nil {
				t.Errorf("%s: expected validation error, got nil", tc.name)
			}
		})
	}
}

// TestValidateHappyPath ensures a well-formed Fingerprint passes Validate.
func TestValidateHappyPath(t *testing.T) {
	fp := rf.Fingerprint{RecordID: "r1", ContentHash: validHash, SeenAt: time.Now()}
	if err := fp.Validate(); err != nil {
		t.Fatalf("valid Fingerprint: unexpected error: %v", err)
	}
}
