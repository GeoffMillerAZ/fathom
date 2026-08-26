package custodian

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidFixtureLoads(t *testing.T) {
	cs, err := LoadFile(filepath.Join("testdata", "valid.cue"), ScopeUser)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != len(KnownKinds) {
		t.Fatalf("valid fixture should carry every kind (%d), got %d concerns", len(KnownKinds), len(cs))
	}
}

// The negative fixture ships fully commented (cue-vet never evaluates it);
// this test uncomments it and asserts the refusal — negatives are asserted
// by Go tests, not live-vetted (repo CUE convention).
func TestInvalidFixtureRefused(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "invalid.cue"))
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, ln := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimPrefix(strings.TrimPrefix(ln, "// "), "//")
		b.WriteString(trimmed + "\n")
	}
	p := filepath.Join(t.TempDir(), "concerns.cue")
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = LoadFile(p, ScopeUser)
	if err == nil || !strings.Contains(err.Error(), "feels_right") {
		t.Fatalf("uncommented negative fixture must be refused naming the kind, got %v", err)
	}
}
