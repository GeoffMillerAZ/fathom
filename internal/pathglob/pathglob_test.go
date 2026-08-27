package pathglob_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/pathglob"
)

// TestPathGlobDoubleStar (EP-3) pins the semantics the three private copies
// shared, so promoting them to one package cannot quietly change behaviour.
func TestPathGlobDoubleStar(t *testing.T) {
	for _, tc := range []struct {
		glob, ref string
		want      bool
	}{
		{"internal/store/**", "internal/store/sqlite.go", true},
		{"internal/store/**", "internal/store/sub/deep.go", true},
		{"internal/store/**", "internal/storeother/x.go", false},
		{"internal/store/**", "internal/query/engine.go", false},
		{"cmd/fathom/*hook*.go", "cmd/fathom/edithook.go", true},
		{"cmd/fathom/*hook*.go", "cmd/fathom/verify.go", false},
		{"**/*.cue", "schema/plan/schema.cue", true},
		{"**/*.cue", "schema/plan/schema.go", false},
		{"docs/intent/**", "docs/intent/invariants.md", true},
		{"ADR.md", "ADR.md", true},
		{"ADR.md", "docs/ADR.md", false},
		{"[", "anything", false}, // malformed matches nothing, never panics
		{"internal/**/*_test.go", "internal/mcp/surface_test.go", true},
	} {
		if got := pathglob.Match(tc.glob, tc.ref); got != tc.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tc.glob, tc.ref, got, tc.want)
		}
	}
}

func TestMatchAny(t *testing.T) {
	globs := []string{"docs/intent/**", "ADR.md"}
	if !pathglob.MatchAny(globs, "ADR.md") {
		t.Error("MatchAny must match the second glob")
	}
	if pathglob.MatchAny(globs, "cmd/fathom/main.go") {
		t.Error("MatchAny matched a path no glob covers")
	}
	if pathglob.MatchAny(nil, "anything") {
		t.Error("no globs must match nothing")
	}
}
