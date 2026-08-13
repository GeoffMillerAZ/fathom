package scope

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestSourceSeesNewSuppressionOnNextSnapshot is V5's mechanism at its own
// level: a Source created (and read) before the file exists reloads on the
// next Snapshot after the file changes — no reopen, no restart.
func TestSourceSeesNewSuppressionOnNextSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "suppressions.jsonl")
	src := NewSource(path, func() time.Time { return fixedTime })

	if set, _ := src.Snapshot(); !set.Empty() {
		t.Fatal("a missing file yielded a non-empty set")
	}

	if _, err := AppendSuppression(path, Suppression{
		Service: "svc", Pattern: "secrets/**", Reason: "r", CreatedAt: fixedTime, CreatedBy: OriginCLI,
	}); err != nil {
		t.Fatal(err)
	}

	set, now := src.Snapshot()
	if set.Empty() {
		t.Fatal("the snapshot after an append is still empty — the source is not re-reading state per call (V5)")
	}
	if _, hidden := set.Match("svc", "secrets/token.go", now); !hidden {
		t.Fatal("the reloaded set does not filter the new pattern")
	}
}

// TestSourceFailsOpenOnMalformedFile: a corrupt suppressions file must
// never make the index unreadable — the set is empty (fail open) and Err
// carries the parse error for the loud surfaces (scope list, doctor).
func TestSourceFailsOpenOnMalformedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "suppressions.jsonl")
	if err := os.WriteFile(path, []byte("{not json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	src := NewSource(path, func() time.Time { return fixedTime })
	set, _ := src.Snapshot()
	if !set.Empty() {
		t.Fatal("a malformed file produced a filtering set")
	}
	if src.Err() == nil {
		t.Fatal("the parse error was swallowed — fail-open must still be loud somewhere")
	}
}

// TestSourceIsRaceSafe: the daemon calls Snapshot from many request
// goroutines while a CLI appends — run it under -race.
func TestSourceIsRaceSafe(t *testing.T) {
	path := filepath.Join(t.TempDir(), "suppressions.jsonl")
	src := NewSource(path, nil)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				set, now := src.Snapshot()
				_, _ = set.Match("svc", "a/b.go", now)
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 10; j++ {
			_, _ = AppendSuppression(path, Suppression{
				Service: "svc", Pattern: "vendor/**", Reason: "r",
				CreatedAt: time.Now().UTC().Add(time.Duration(j) * time.Millisecond), CreatedBy: OriginCLI,
			})
		}
	}()
	wg.Wait()
}

// TestSourceEmptyPathIsInert: an empty path (no FATHOM_SUPPRESSIONS) is a
// permanently disarmed source — the hermetic default.
func TestSourceEmptyPathIsInert(t *testing.T) {
	src := NewSource("", nil)
	if set, _ := src.Snapshot(); !set.Empty() {
		t.Fatal("an empty-path source filtered something")
	}
	if src.Err() != nil {
		t.Fatal("an empty-path source reported an error")
	}
}
