package schemas_test

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/toolchains/schemas"
)

// TestRegistry_NotEmpty verifies that the Registry slice contains at least one
// schema package path.
func TestRegistry_NotEmpty(t *testing.T) {
	reg := schemas.Registry
	if len(reg) == 0 {
		t.Fatal("Registry must not be empty")
	}
}

// TestRegistry_AllPathsNonEmpty verifies that no Registry entry is a blank
// string (which would be a misconfiguration).
func TestRegistry_AllPathsNonEmpty(t *testing.T) {
	for i, path := range schemas.Registry {
		if strings.TrimSpace(path) == "" {
			t.Errorf("Registry[%d] is blank", i)
		}
	}
}

// TestRegistry_ContainsSchemas verifies the canonical schemas package itself
// is listed in the Registry.
func TestRegistry_ContainsSchemas(t *testing.T) {
	found := false
	for _, path := range schemas.Registry {
		if path == "toolchains/schemas" {
			found = true
			break
		}
	}
	if !found {
		t.Error(`Registry does not contain "toolchains/schemas"`)
	}
}

// TestKnownNodeKinds_AllValuesPresent verifies that all expected NodeKind
// values are returned by KnownNodeKinds.
func TestKnownNodeKinds_AllValuesPresent(t *testing.T) {
	kinds := schemas.KnownNodeKinds()
	want := map[schemas.NodeKind]bool{
		schemas.NodeKindService:  true,
		schemas.NodeKindFinding:  true,
		schemas.NodeKindEvidence: true,
		schemas.NodeKindAgent:    true,
	}
	for _, k := range kinds {
		delete(want, k)
	}
	if len(want) > 0 {
		t.Errorf("KnownNodeKinds() is missing: %v", want)
	}
}

// TestKnownNodeKinds_NoDuplicates verifies there are no duplicate entries in
// the KnownNodeKinds slice.
func TestKnownNodeKinds_NoDuplicates(t *testing.T) {
	seen := make(map[schemas.NodeKind]int)
	for i, k := range schemas.KnownNodeKinds() {
		if prev, ok := seen[k]; ok {
			t.Errorf("duplicate NodeKind %q at indices %d and %d", k, prev, i)
		}
		seen[k] = i
	}
}

// TestArchetypeKind_Values verifies the ArchetypeKind constants have the
// expected string values (mirrors the CUE enum).
func TestArchetypeKind_Values(t *testing.T) {
	cases := []struct {
		kind schemas.ArchetypeKind
		want string
	}{
		{schemas.ArchetypeKindNode, "node"},
		{schemas.ArchetypeKindEdge, "edge"},
		{schemas.ArchetypeKindEvent, "event"},
	}
	for _, tc := range cases {
		if string(tc.kind) != tc.want {
			t.Errorf("ArchetypeKind %v: got %q, want %q", tc.kind, tc.kind, tc.want)
		}
	}
}

// TestArchetype_ZeroValue verifies that a zero-value Archetype is safely
// inspectable (no panics, reasonable zero state).
func TestArchetype_ZeroValue(t *testing.T) {
	var a schemas.Archetype
	if a.ID != "" || a.Label != "" || a.Kind != "" {
		t.Errorf("unexpected non-zero zero-value Archetype: %+v", a)
	}
}
