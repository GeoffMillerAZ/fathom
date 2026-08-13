package scope

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var fixedTime = time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)

// TestSuppressionExpiryAtReadTime is SP-1: expiry is a pure function of the
// record and the clock, judged on every read — no sweep, no daemon timer.
// The same stored record filters at one clock and not at a later one, with
// NOTHING having run in between; that is exactly the "expires while the
// daemon is down" case.
func TestSuppressionExpiryAtReadTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "suppressions.jsonl")
	exp := fixedTime.Add(24 * time.Hour)
	id, err := AppendSuppression(path, Suppression{
		Service: "svc", Pattern: "vendor/**", Reason: "third-party corpus",
		CreatedAt: fixedTime, ExpiresAt: &exp, CreatedBy: OriginCLI,
	})
	if err != nil {
		t.Fatalf("AppendSuppression: %v", err)
	}

	all, err := LoadSuppressions(path)
	if err != nil {
		t.Fatalf("LoadSuppressions: %v", err)
	}
	if len(all) != 1 || all[0].ID != id {
		t.Fatalf("loaded %+v, want the one appended record", all)
	}
	set, err := NewSet(all)
	if err != nil {
		t.Fatalf("NewSet: %v", err)
	}

	// Before expiry: active, filtering.
	if _, hidden := set.Match("svc", "vendor/lib/a.go", fixedTime); !hidden {
		t.Fatal("an unexpired suppression did not filter")
	}
	if got := SuppressionState(all[0], fixedTime); got != StateActive {
		t.Fatalf("state before expiry = %q, want active", got)
	}

	// After expiry: the SAME set, the SAME file, a later clock — inactive.
	// No sweep ran, no reload happened; the judgement is read-time.
	after := exp.Add(time.Minute)
	if _, hidden := set.Match("svc", "vendor/lib/a.go", after); hidden {
		t.Fatal("an expired suppression still filtered — expiry must be evaluated at read time")
	}
	if got := SuppressionState(all[0], after); got != StateExpired {
		t.Fatalf("state after expiry = %q, want expired", got)
	}

	// The expired record is RETAINED in the file as history (§2.3).
	if all2, err := LoadSuppressions(path); err != nil || len(all2) != 1 {
		t.Fatalf("expired record was pruned from the file (n=%d err=%v)", len(all2), err)
	}
}

func TestSuppressionRequiresReasonAndPattern(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	if _, err := AppendSuppression(path, Suppression{Pattern: "x/**", CreatedAt: fixedTime}); err == nil {
		t.Fatal("a suppression with no reason was accepted")
	}
	if _, err := AppendSuppression(path, Suppression{Reason: "r", CreatedAt: fixedTime}); err == nil {
		t.Fatal("a suppression with no pattern was accepted")
	}
	if _, err := AppendSuppression(path, Suppression{Pattern: "!x/**", Reason: "r", CreatedAt: fixedTime}); err == nil {
		t.Fatal("a re-include pattern was accepted — a suppression may only hide")
	}
	// A refused suppression writes nothing at all.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("a refused suppression still touched the file")
	}
}

func TestUnsuppressTombstoneKeepsHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	id, err := AppendSuppression(path, Suppression{
		Service: "svc", Pattern: "vendor/**", Reason: "r", CreatedAt: fixedTime, CreatedBy: OriginCLI,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := RemoveSuppression(path, id, fixedTime.Add(time.Hour)); err != nil {
		t.Fatalf("RemoveSuppression: %v", err)
	}
	all, err := LoadSuppressions(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("tombstone erased history: %d records", len(all))
	}
	if got := SuppressionState(all[0], fixedTime.Add(2*time.Hour)); got != StateRemoved {
		t.Fatalf("state = %q, want removed", got)
	}
	set, err := NewSet(all)
	if err != nil {
		t.Fatal(err)
	}
	if _, hidden := set.Match("svc", "vendor/a.go", fixedTime.Add(2*time.Hour)); hidden {
		t.Fatal("a removed suppression still filters")
	}
	// Removing twice, or removing an unknown id, is a named refusal.
	if err := RemoveSuppression(path, id, fixedTime.Add(3*time.Hour)); err == nil {
		t.Fatal("double unsuppress was accepted silently")
	}
	if err := RemoveSuppression(path, "deadbeef", fixedTime); err == nil || !strings.Contains(err.Error(), "deadbeef") {
		t.Fatalf("unknown-id refusal does not name the id: %v", err)
	}
}

func TestLoadSuppressionsMissingFileIsEmpty(t *testing.T) {
	all, err := LoadSuppressions(filepath.Join(t.TempDir(), "absent.jsonl"))
	if err != nil || all != nil {
		t.Fatalf("missing file: got (%v, %v), want (nil, nil)", all, err)
	}
	all, err = LoadSuppressions("")
	if err != nil || all != nil {
		t.Fatalf("empty path: got (%v, %v), want (nil, nil)", all, err)
	}
}

// TestScopePackageHasNoInit pins the V6 rework structurally: this package
// must never arm anything from an init() — the previous build's init read
// os.UserHomeDir() and made `task validate` depend on the developer's real
// ~/.fathom. State resolves only through explicit paths and EnvVar.
func TestScopePackageHasNoInit(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "func init(") {
			t.Errorf("%s declares func init() — scope state must resolve through an injectable path, never an init-armed global (V6)", name)
		}
	}
}
