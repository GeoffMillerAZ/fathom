package indexbench

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// TestFakeMCPServer is not a real test: it is re-executed as a subprocess
// by ProbeReferencesViaMCP's tests via the standard os/exec "helper
// process" pattern (GO_WANT_HELPER_MCP_SERVER=1). It scripts a minimal
// stdio MCP server — initialize, tools/list, tools/call — so SB-30 is
// proven without any real competitor installed (the plan's rule: no
// competitor installs on this machine).
func TestFakeMCPServer(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_MCP_SERVER") != "1" {
		t.Skip("helper process, not a real test — see ProbeReferencesViaMCP's tests")
	}
	runFakeMCPServer()
}

func runFakeMCPServer() {
	toolName := os.Getenv("FAKE_MCP_TOOL")
	response := os.Getenv("FAKE_MCP_RESPONSE")
	r := bufio.NewReader(os.Stdin)
	for {
		line, err := r.ReadString('\n')
		if len(line) > 0 {
			handleFakeMCPLine(line, toolName, response)
		}
		if err != nil {
			return
		}
	}
}

func handleFakeMCPLine(line, toolName, response string) {
	var req struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
	}
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		return
	}
	switch req.Method {
	case "initialize":
		writeFakeMCPResult(req.ID, map[string]any{"protocolVersion": "2025-11-25"})
	case "notifications/initialized":
		// no response for a notification
	case "tools/list":
		writeFakeMCPResult(req.ID, map[string]any{"tools": []map[string]any{
			{"name": toolName, "description": "fake test tool", "inputSchema": map[string]any{"type": "object"}},
		}})
	case "tools/call":
		writeFakeMCPResult(req.ID, map[string]any{
			"content": []map[string]any{{"type": "text", "text": response}},
			"isError": false,
		})
	default:
		if len(req.ID) > 0 {
			writeFakeMCPResult(req.ID, map[string]any{})
		}
	}
}

func writeFakeMCPResult(id json.RawMessage, result any) {
	resp := map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(id), "result": result}
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	fmt.Fprintf(os.Stdout, "%s\n", data)
}

// helperMCPConfig returns an MCPServerConfig that re-execs this test binary
// as the fake stdio MCP server described above, scripted to answer every
// tools/call with response regardless of arguments.
func helperMCPConfig(t *testing.T, toolName, response string) MCPServerConfig {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	// exec.LookPath confirms the test binary is directly executable before
	// we hand it to ProbeReferencesViaMCP as a bare command.
	if _, err := exec.LookPath(exe); err != nil {
		t.Skipf("test binary not directly executable, skipping helper-process test: %v", err)
	}
	return MCPServerConfig{
		Command: exe,
		Args:    []string{"-test.run=TestFakeMCPServer"},
		Env: []string{
			"GO_WANT_HELPER_MCP_SERVER=1",
			"FAKE_MCP_TOOL=" + toolName,
			"FAKE_MCP_RESPONSE=" + response,
		},
	}
}

func TestProbeReferencesViaMCP_SerenaJSON(t *testing.T) {
	cfg := helperMCPConfig(t, "find_referencing_symbols",
		`{"pkg/caller.go":{"Function":[{"name_path":"Caller","body_location":{"start_line":10,"end_line":20},"content_around_reference":"...  11:\tx := 1\n  >  12:\ty := Target(x)\n...  13:\treturn y"}]}}`)

	ctx, cancel := context.WithTimeout(context.Background(), waitSafety)
	defer cancel()

	set, err := ProbeReferencesViaMCP(ctx, cfg, "find_referencing_symbols", []TruthSymbol{
		{Name: "Target", File: "pkg/target.go"},
	})
	if err != nil {
		t.Fatalf("ProbeReferencesViaMCP: %v", err)
	}
	want := Edge{From: "pkg/caller.go:12", To: "Target", Kind: EdgeKindReference}
	if len(set.Edges) != 1 || set.Edges[0] != want {
		t.Fatalf("edges = %+v, want [%+v]", set.Edges, want)
	}
}

func TestProbeReferencesViaMCP_GoplsProse(t *testing.T) {
	cfg := helperMCPConfig(t, "go_symbol_references",
		"Found 1 reference. Located in the file: pkg/caller.go, the symbol is located on line 42.")

	ctx, cancel := context.WithTimeout(context.Background(), waitSafety)
	defer cancel()

	set, err := ProbeReferencesViaMCP(ctx, cfg, "go_symbol_references", []TruthSymbol{
		{Name: "Target", File: "pkg/target.go"},
	})
	if err != nil {
		t.Fatalf("ProbeReferencesViaMCP: %v", err)
	}
	want := Edge{From: "pkg/caller.go:42", To: "Target", Kind: EdgeKindReference}
	if len(set.Edges) != 1 || set.Edges[0] != want {
		t.Fatalf("edges = %+v, want [%+v]", set.Edges, want)
	}
}

func TestProbeReferencesViaMCP_MultipleSymbolsAndSites(t *testing.T) {
	cfg := helperMCPConfig(t, "go_symbol_references",
		"Located in the file: a.go, the symbol is located on line 1. "+
			"Located in the file: b.go, the symbol is located on line 2.")

	ctx, cancel := context.WithTimeout(context.Background(), waitSafety)
	defer cancel()

	set, err := ProbeReferencesViaMCP(ctx, cfg, "go_symbol_references", []TruthSymbol{
		{Name: "One", File: "one.go"},
		{Name: "Two", File: "two.go"},
	})
	if err != nil {
		t.Fatalf("ProbeReferencesViaMCP: %v", err)
	}
	if len(set.Edges) != 4 {
		t.Fatalf("got %d edges for 2 symbols x 2 sites each, want 4: %+v", len(set.Edges), set.Edges)
	}
}

func TestProbeReferencesViaMCP_UnknownToolRefuses(t *testing.T) {
	cfg := helperMCPConfig(t, "some_tool", "irrelevant")
	ctx, cancel := context.WithTimeout(context.Background(), waitSafety)
	defer cancel()

	if _, err := ProbeReferencesViaMCP(ctx, cfg, "no_such_tool", []TruthSymbol{{Name: "X", File: "x.go"}}); err == nil {
		t.Fatal("want an error for a tool with no registered argument mapping")
	}
}

func TestParseGoplsProse_PairsFilesAndLinesInOrder(t *testing.T) {
	text := "Located in the file: internal/foo/bar.go, the symbol is located on line 7. " +
		"Located in the file: internal/baz/qux.go, the symbol is located on line 19."
	sites := parseGoplsProse(text)
	if len(sites) != 2 {
		t.Fatalf("got %d sites, want 2: %+v", len(sites), sites)
	}
	if sites[0].file != "internal/foo/bar.go" || sites[0].line != 7 {
		t.Errorf("site 0 = %+v, want internal/foo/bar.go:7", sites[0])
	}
	if sites[1].file != "internal/baz/qux.go" || sites[1].line != 19 {
		t.Errorf("site 1 = %+v, want internal/baz/qux.go:19", sites[1])
	}
}

func TestTruthSymbolsForMCP_ResolvesDeclaringFile(t *testing.T) {
	truth := smallTruth()
	got := TruthSymbolsForMCP(truth, []string{"Foo", "Recv.Method"})
	want := []TruthSymbol{{Name: "Foo", File: "a.go"}, {Name: "Recv.Method", File: "b.go"}}
	for i, g := range got {
		if g != want[i] {
			t.Errorf("symbol %d = %+v, want %+v", i, g, want[i])
		}
	}
}
