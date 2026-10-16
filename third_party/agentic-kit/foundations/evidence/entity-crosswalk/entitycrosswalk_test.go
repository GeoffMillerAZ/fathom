package entitycrosswalk_test

import (
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	ec "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/entity-crosswalk"
	ap "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/provenance/ai-provenance"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// fixedClock returns a clock that always returns t.
func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

var (
	t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 = time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	// authBot is a well-formed AI author with full reproduction key.
	authBot = ap.NewAI(
		"claude-sonnet-4-6",
		"20260608",
		"sha256-promptABC123",
		"sha256-paramsXYZ",
		"sha256-inputDEF456",
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	)

	// authHum is a well-formed human author.
	authHum = ap.NewHuman("analyst-1")
)

// newCW constructs a Crosswalk with a fixed clock for deterministic tests.
func newCW(clock func() time.Time) *ec.Crosswalk {
	return ec.New(clock)
}

// ---------------------------------------------------------------------------
// Finding: empty/self-alias validation (medium + low)
// ---------------------------------------------------------------------------

// TestAddRejectsEmptyCanonical verifies Add rejects a blank canonical ID.
func TestAddRejectsEmptyCanonical(t *testing.T) {
	cw := newCW(fixedClock(t0))
	err := cw.Add("", "alias-x", authBot)
	if err == nil {
		t.Fatal("Add(\"\", alias) should return error, got nil")
	}
}

// TestAddRejectsEmptyAlias verifies Add rejects a blank alias.
func TestAddRejectsEmptyAlias(t *testing.T) {
	cw := newCW(fixedClock(t0))
	err := cw.Add("svc", "", authBot)
	if err == nil {
		t.Fatal("Add(svc, \"\") should return error, got nil")
	}
}

// TestAddRejectsSelfAlias verifies Add("x","x") is rejected.
func TestAddRejectsSelfAlias(t *testing.T) {
	cw := newCW(fixedClock(t0))
	err := cw.Add("svc", "svc", authBot)
	if err == nil {
		t.Fatal("Add(svc, svc) should return error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Finding: basic correctness (preserved from original 7 tests, updated sig)
// ---------------------------------------------------------------------------

// TestAddAndCanonical verifies Canonical returns the registered canonical ID.
func TestAddAndCanonical(t *testing.T) {
	cw := newCW(fixedClock(t0))
	mustAdd(t, cw, "service-payments", "repo-payments", authBot)
	mustAdd(t, cw, "service-payments", "workload-payments", authBot)

	if got := cw.Canonical("repo-payments"); got != "service-payments" {
		t.Errorf("Canonical(repo-payments) = %q, want %q", got, "service-payments")
	}
	if got := cw.Canonical("workload-payments"); got != "service-payments" {
		t.Errorf("Canonical(workload-payments) = %q, want %q", got, "service-payments")
	}
}

// TestAliasesOrdering verifies Aliases returns lexicographic order.
func TestAliasesOrdering(t *testing.T) {
	cw := newCW(fixedClock(t0))
	mustAdd(t, cw, "svc", "alias-c", authBot)
	mustAdd(t, cw, "svc", "alias-a", authBot)
	mustAdd(t, cw, "svc", "alias-b", authBot)

	got := cw.Aliases("svc")
	want := []string{"alias-a", "alias-b", "alias-c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Aliases(svc): got %v, want %v", got, want)
	}
}

// TestCanonicalUnknown verifies Canonical returns "" for unknown aliases.
func TestCanonicalUnknown(t *testing.T) {
	cw := newCW(fixedClock(t0))
	if got := cw.Canonical("nope"); got != "" {
		t.Errorf("expected empty string for unknown alias, got %q", got)
	}
}

// TestAliasesUnknown verifies Aliases returns nil for unknown canonical IDs.
func TestAliasesUnknown(t *testing.T) {
	cw := newCW(fixedClock(t0))
	if got := cw.Aliases("nope"); got != nil {
		t.Errorf("expected nil for unknown canonical, got %v", got)
	}
}

// TestAddIdempotent verifies adding the same pair twice doesn't duplicate.
func TestAddIdempotent(t *testing.T) {
	cw := newCW(fixedClock(t0))
	mustAdd(t, cw, "svc", "alias-a", authBot)
	mustAdd(t, cw, "svc", "alias-a", authBot)

	aliases := cw.Aliases("svc")
	if len(aliases) != 1 {
		t.Errorf("expected 1 alias after duplicate Add, got %d: %v", len(aliases), aliases)
	}
}

// TestNewCrosswalkEmpty verifies New returns an empty, functional Crosswalk.
func TestNewCrosswalkEmpty(t *testing.T) {
	cw := newCW(fixedClock(t0))
	if got := cw.Canonical("x"); got != "" {
		t.Errorf("new Crosswalk: Canonical should return empty string, got %q", got)
	}
	if got := cw.Aliases("x"); got != nil {
		t.Errorf("new Crosswalk: Aliases should return nil, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// Finding: append-only / alias reassignment (high)
// ---------------------------------------------------------------------------

// TestAliasReassignmentReturnsError verifies that remapping an alias to a new
// canonical returns ErrAliasConflict and preserves the old mapping intact.
func TestAliasReassignmentReturnsError(t *testing.T) {
	cw := newCW(fixedClock(t0))
	mustAdd(t, cw, "svc-old", "alias-x", authBot)

	err := cw.Add("svc-new", "alias-x", authBot)
	if !errors.Is(err, ec.ErrAliasConflict) {
		t.Fatalf("expected ErrAliasConflict, got %v", err)
	}

	// Old mapping must still be intact (append-only — prior fact preserved).
	if got := cw.Canonical("alias-x"); got != "svc-old" {
		t.Errorf("Canonical(alias-x) = %q after conflict, want svc-old", got)
	}
}

// TestHistoryRecordsEntries verifies History records all successful Add calls.
func TestHistoryRecordsEntries(t *testing.T) {
	cw := newCW(fixedClock(t0))
	mustAdd(t, cw, "svc", "alias-a", authHum)
	mustAdd(t, cw, "svc", "alias-b", authBot)

	h := cw.History()
	if len(h) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(h))
	}
	if h[0].CanonicalID != "svc" || h[0].AliasID != "alias-a" {
		t.Errorf("history[0] unexpected: %+v", h[0])
	}
	if h[1].AliasID != "alias-b" {
		t.Errorf("history[1] unexpected: %+v", h[1])
	}
}

// TestHistoryIsDefensiveCopy verifies that mutating the History slice does
// not affect internal state.
func TestHistoryIsDefensiveCopy(t *testing.T) {
	cw := newCW(fixedClock(t0))
	mustAdd(t, cw, "svc", "alias-a", authBot)

	h := cw.History()
	h[0].CanonicalID = "tampered"

	h2 := cw.History()
	if h2[0].CanonicalID == "tampered" {
		t.Error("History() is not returning a defensive copy")
	}
}

// TestAliasesIsDefensiveCopy verifies Aliases returns a fresh slice each call.
func TestAliasesIsDefensiveCopy(t *testing.T) {
	cw := newCW(fixedClock(t0))
	mustAdd(t, cw, "svc", "alias-a", authBot)

	got := cw.Aliases("svc")
	got[0] = "tampered"

	got2 := cw.Aliases("svc")
	if got2[0] == "tampered" {
		t.Error("Aliases() is not returning a defensive copy")
	}
}

// ---------------------------------------------------------------------------
// Finding: provenance on every Add (low)
// ---------------------------------------------------------------------------

// TestProvenanceTimestamp verifies entries carry the injected clock time.
func TestProvenanceTimestamp(t *testing.T) {
	cw := newCW(fixedClock(t1))
	mustAdd(t, cw, "svc", "alias-a", authHum)

	h := cw.History()
	if len(h) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(h))
	}
	if !h[0].RecordedAt.Equal(t1) {
		t.Errorf("RecordedAt = %v, want %v", h[0].RecordedAt, t1)
	}
	if h[0].Author.ReproKey() != authHum.ReproKey() {
		t.Errorf("Author = %+v, want %+v", h[0].Author, authHum)
	}
}

// TestAuthorKindRequired verifies Add rejects an Author with no Kind set.
func TestAuthorKindRequired(t *testing.T) {
	cw := newCW(fixedClock(t0))
	// Zero-value Author has empty Kind — must be rejected.
	err := cw.Add("svc", "alias-a", ec.Author{})
	if err == nil {
		t.Fatal("Add with zero Author should return error, got nil")
	}
}

// TestAIAuthorReproKeyPreserved verifies that the full AI reproduction key is
// stored on the history entry — this is the invariant #1 check.
func TestAIAuthorReproKeyPreserved(t *testing.T) {
	cw := newCW(fixedClock(t0))
	mustAdd(t, cw, "svc", "alias-a", authBot)

	h := cw.History()
	if len(h) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(h))
	}
	if !h[0].Author.IsAI() {
		t.Fatal("expected AI author on history entry")
	}
	if h[0].Author.AI == nil {
		t.Fatal("AI field is nil on stored AI author")
	}
	if h[0].Author.AI.ModelID != "claude-sonnet-4-6" {
		t.Errorf("ModelID = %q, want claude-sonnet-4-6", h[0].Author.AI.ModelID)
	}
	if h[0].Author.AI.PromptHash == "" {
		t.Error("PromptHash must not be empty on stored AI author")
	}
	if h[0].Author.AI.InputDigest == "" {
		t.Error("InputDigest must not be empty on stored AI author")
	}
}

// TestAIAuthorMissingFieldsRejected verifies Add rejects an AI author that
// lacks required reproduction-key fields (ModelID, PromptHash, InputDigest).
func TestAIAuthorMissingFieldsRejected(t *testing.T) {
	cw := newCW(fixedClock(t0))
	// AI author with missing required fields.
	incomplete := ap.Author{
		Kind: ap.KindAI,
		AI: &ap.AIAuthor{
			ModelID: "some-model",
			// ModelVersion, PromptHash, InputDigest, At all zero/empty — invalid.
		},
	}
	err := cw.Add("svc", "alias-a", incomplete)
	if err == nil {
		t.Fatal("Add with incomplete AI author should return error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Finding: transitive resolution (medium)
// ---------------------------------------------------------------------------

// TestCanonicalTransitive verifies Canonical resolves multi-hop chains to root.
// Scenario: alias-b -> entity-A -> entity-C.
// Canonical("alias-b") must return "entity-C", not "entity-A".
func TestCanonicalTransitive(t *testing.T) {
	cw := newCW(fixedClock(t0))
	// entity-A is an alias of entity-C.
	mustAdd(t, cw, "entity-C", "entity-A", authBot)
	// alias-b is an alias of entity-A.
	mustAdd(t, cw, "entity-A", "alias-b", authBot)

	got := cw.Canonical("alias-b")
	if got != "entity-C" {
		t.Errorf("Canonical(alias-b) = %q, want entity-C (transitive)", got)
	}
}

// TestCanonicalTransitiveMultiHop verifies deeper chains (3 hops).
func TestCanonicalTransitiveMultiHop(t *testing.T) {
	cw := newCW(fixedClock(t0))
	mustAdd(t, cw, "root", "mid", authBot)
	mustAdd(t, cw, "mid", "leaf", authBot)
	mustAdd(t, cw, "leaf", "deepleaf", authBot)

	if got := cw.Canonical("deepleaf"); got != "root" {
		t.Errorf("Canonical(deepleaf) = %q, want root", got)
	}
}

// ---------------------------------------------------------------------------
// Finding: concurrency safety (high)
// ---------------------------------------------------------------------------

// TestConcurrencySafe verifies no data races under concurrent Add + Canonical.
// The race detector will catch problems here when run with -race.
func TestConcurrencySafe(t *testing.T) {
	cw := newCW(fixedClock(t0))
	const goroutines = 20

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			canonical := "canonical-" + itoa(i)
			alias := "alias-" + itoa(i)
			_ = cw.Add(canonical, alias, authBot)
		}()
		go func() {
			defer wg.Done()
			_ = cw.Canonical("alias-" + itoa(i))
		}()
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustAdd(t *testing.T, cw *ec.Crosswalk, canonical, alias string, author ec.Author) {
	t.Helper()
	if err := cw.Add(canonical, alias, author); err != nil {
		t.Fatalf("Add(%q, %q) unexpected error: %v", canonical, alias, err)
	}
}

// itoa converts an int to a decimal string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
