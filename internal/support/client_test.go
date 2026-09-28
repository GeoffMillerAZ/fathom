package support

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// TestClientRegistryFromHarnessDescriptors proves SM-5: the client registry
// is built from every harness descriptor, with the FULL harness Capability
// list as columns.
func TestClientRegistryFromHarnessDescriptors(t *testing.T) {
	entries := clientRegistry()

	want := harness.All()
	if len(entries) != len(want) {
		t.Fatalf("clientRegistry() returned %d entries, want %d (one per harness.All() descriptor)", len(entries), len(want))
	}

	byID := make(map[string]RegistryEntry, len(entries))
	for _, e := range entries {
		if e.Kind != KindClient {
			t.Errorf("entry %s: kind = %q, want %q", e.ID, e.Kind, KindClient)
		}
		byID[e.ID] = e
	}

	wantCaps := make([]string, 0, len(harness.Capabilities))
	for _, c := range harness.Capabilities {
		wantCaps = append(wantCaps, string(c))
	}

	for _, d := range want {
		e, ok := byID[d.ID]
		if !ok {
			t.Errorf("missing registry entry for descriptor %s", d.ID)
			continue
		}
		if e.Name != d.Display {
			t.Errorf("entry %s: name = %q, want %q", d.ID, e.Name, d.Display)
		}
		if !equalStrings(e.Capabilities, wantCaps) {
			t.Errorf("entry %s: capabilities = %v, want the full harness.Capabilities list %v", d.ID, e.Capabilities, wantCaps)
		}
	}

	if _, ok := byID[harness.ClaudeCode]; !ok {
		t.Error("clientRegistry() has no entry for claude-code")
	}
	if _, ok := byID[harness.Codex]; !ok {
		t.Error("clientRegistry() has no entry for codex")
	}
}
