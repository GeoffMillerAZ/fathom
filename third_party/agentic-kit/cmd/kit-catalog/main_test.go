package main

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/cmd/internal/catalog"
)

// TestRealRepoSmoke is a thin smoke test that the binary's main() path
// (which delegates to cmd/internal/catalog) can collect the real repo.
// The full golden/SkipDir/SectionKey tests live in cmd/internal/catalog.
func TestRealRepoSmoke(t *testing.T) {
	entries, err := catalog.Collect("../..")
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(entries) <= 100 {
		t.Fatalf("found %d packages, want >100", len(entries))
	}
	md := catalog.Render(entries)
	for _, want := range []string{
		"- `kits/agent-harness` (`agentharness`)",
		"- `cmd/kit-catalog` (`main`)",
		"## kits (",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("catalog missing expected line %q", want)
		}
	}
}
