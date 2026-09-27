package main

// init_codexmcp_test.go — WF-10 (docs/plans/work-frontier.md): `fathom
// init`'s manual-steps output prints the exact Codex MCP registration
// commands (docs/adoption/codex.md §2: HTTP `codex mcp add … --url …` plus
// the stdio `serve-mcp` fallback) beside the existing `claude mcp add`
// line it already prints.

import (
	"bytes"
	"strings"
	"testing"
)

func TestInitPrintsCodexMcpAdd(t *testing.T) {
	var out bytes.Buffer
	printManualSteps(&out, "myservice", "http://127.0.0.1:7581")
	got := out.String()

	// The existing Claude Code line stays.
	if !strings.Contains(got, "claude mcp add --transport http --scope user fathom http://127.0.0.1:7581/rpc/agent") {
		t.Errorf("missing the existing claude mcp add line:\n%s", got)
	}

	// The HTTP form, per docs/adoption/codex.md §2.
	if !strings.Contains(got, "codex mcp add fathom --url http://127.0.0.1:7581/rpc/agent") {
		t.Errorf("missing the codex mcp add --url HTTP form:\n%s", got)
	}

	// The stdio fallback, per docs/adoption/codex.md §2.
	if !strings.Contains(got, "codex mcp add fathom -- ") || !strings.Contains(got, "serve-mcp") {
		t.Errorf("missing the codex mcp add stdio fallback:\n%s", got)
	}
}
