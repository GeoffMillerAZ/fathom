package mcp

import (
	"context"
	"testing"
)

func TestStalenessDescriptionExplainsEvidence(t *testing.T) {
	const original = "Staleness summary: source files, last-ingested state, git-commit drift. Reach for it when an answer looks wrong. Required: service."
	const compact = "Source/commit drift. Reach for it when an answer looks stale. Required: service."
	s := NewServer(nil)
	for _, tool := range s.visibleToolsFor("") {
		if tool.Name != "staleness_report" {
			continue
		}
		if tool.Description == original {
			return
		}
		if tool.Description != compact {
			t.Fatalf("unreviewed agent-facing description: %q", tool.Description)
		}
		// F10 permits the compact description only when the original cannot
		// fit the existing schema ceiling. Both strings are plain ASCII.
		originalBytes := s.surfaceSchemaBytes(WithRequestScope(context.Background(), "", true)) + len(original) - len(compact)
		if originalBytes <= fullSurfaceSchemaByteCeiling {
			t.Fatalf("restore original description: it fits at %d bytes", originalBytes)
		}
		t.Logf("F10 schema exception: original %d bytes exceeds unchanged ceiling %d", originalBytes, fullSurfaceSchemaByteCeiling)
		return
	}
	t.Fatal("staleness_report not registered")
}
