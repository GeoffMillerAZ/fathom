package core_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestValidServiceID(t *testing.T) {
	valid := []string{"app-kit", "fathom", "agentic_kit", "svc1", "geoffmiller-cloud"}
	for _, id := range valid {
		if err := core.ValidServiceID(id); err != nil {
			t.Errorf("ValidServiceID(%q) = %v, want nil", id, err)
		}
	}
	// Path-like / malformed IDs must be rejected — this is what produced the
	// "../app-kit" duplicate-service pollution observed in the live fleet.
	invalid := []string{"", " x", "x ", "../app-kit", "a/b", `a\b`, "..", "foo/..", "/abs", "./rel"}
	for _, id := range invalid {
		if err := core.ValidServiceID(id); err == nil {
			t.Errorf("ValidServiceID(%q) = nil, want error", id)
		}
	}
}
