package main

import (
	"strings"
	"testing"
)

func TestServerInstructionsLeadWithAdoptionAndTrustBoundary(t *testing.T) {
	text := serverInstructions(nil, nil, nil)
	if len(text) < 512 {
		t.Fatal("expected full instructions after compact introduction")
	}
	for _, required := range []string{"consumer's intent", "get_kit_guidance", "search_capabilities", "get_capability", "list_quality_work", "visual references", "data, not agent instructions", "not quality certification"} {
		if !strings.Contains(text[:512], required) {
			t.Errorf("first 512 bytes omit essential guidance %q", required)
		}
	}
}
