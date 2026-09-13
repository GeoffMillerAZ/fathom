package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/indexbench"
)

// stagePaymentModule stages the fauxtown fixture's payment-service — a real
// tiny Go module with Charge calling validateCharge — under t.TempDir(), so
// compare's tests exercise a real go/types truth without a network fetch or
// a hand-rolled module.
func stagePaymentModule(t *testing.T) string {
	t.Helper()
	dir, err := indexbench.StageFauxtownModule("payment-service", filepath.Join(t.TempDir(), "payment-service"))
	if err != nil {
		t.Fatalf("stage payment-service: %v", err)
	}
	return dir
}

func TestCmdCompare_MissingRepo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := cmdCompare(nil, &stdout, &stderr); code != 2 {
		t.Fatalf("exit = %d, want 2 (missing -repo)", code)
	}
}

func TestCmdCompare_MCPFlagsMustComeTogether(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cmdCompare([]string{"-repo", ".", "-mcp", "cfg.json"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (-mcp without -tool/-arm)", code)
	}
}

func TestCmdCompare_RepoOnly(t *testing.T) {
	dir := stagePaymentModule(t)
	ledger := filepath.Join(t.TempDir(), "competitors-ledger.jsonl")

	var stdout, stderr bytes.Buffer
	code := cmdCompare([]string{
		"-repo", dir, "-service", "compare-test", "-ledger", ledger, "-now", "2026-09-13",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	rows := readLedger(t, ledger)
	if len(rows) != 2 {
		t.Fatalf("got %d ledger rows, want 2 (fathom-find_callers, fathom-references): %+v", len(rows), rows)
	}
	if rows[0].Arm != "fathom-find_callers" || rows[1].Arm != "fathom-references" {
		t.Errorf("arms = %q, %q", rows[0].Arm, rows[1].Arm)
	}
	if rows[0].Date != "2026-09-13" {
		t.Errorf("date = %q, want 2026-09-13", rows[0].Date)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("fathom (find_callers)")) {
		t.Errorf("stdout missing fathom row: %s", stdout.String())
	}
}

func TestCmdCompare_Graphify(t *testing.T) {
	dir := stagePaymentModule(t)
	ledger := filepath.Join(t.TempDir(), "competitors-ledger.jsonl")

	var stdout, stderr bytes.Buffer
	code := cmdCompare([]string{
		"-repo", dir, "-service", "compare-graphify",
		"-graphify", filepath.Join("testdata", "graphify-payment.json"),
		"-version", "v0.9.61",
		"-ledger", ledger, "-now", "2026-09-13",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	rows := readLedger(t, ledger)
	if len(rows) != 2 {
		t.Fatalf("got %d ledger rows, want 2 (fathom, graphify): %+v", len(rows), rows)
	}
	if rows[0].Arm != "fathom" || rows[1].Arm != "graphify" {
		t.Fatalf("arms = %q, %q", rows[0].Arm, rows[1].Arm)
	}
	if rows[1].Version != "v0.9.61" {
		t.Errorf("graphify version = %q, want v0.9.61", rows[1].Version)
	}
	// The fixture's Charge->validateCharge "calls" edge is real: recall
	// must be > 0, not the "everything unadjudicated reads 1.0" case.
	if rows[1].Adjudicable == 0 {
		t.Errorf("graphify row adjudicable = 0, want at least one real query scored: %+v", rows[1])
	}
	if !bytes.Contains(stdout.Bytes(), []byte("graphify")) {
		t.Errorf("stdout missing graphify row: %s", stdout.String())
	}
}

func TestCmdCompare_MCP(t *testing.T) {
	dir := stagePaymentModule(t)
	ledger := filepath.Join(t.TempDir(), "competitors-ledger.jsonl")

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	if _, err := exec.LookPath(exe); err != nil {
		t.Skipf("test binary not directly executable: %v", err)
	}
	cfg := map[string]any{
		"command": exe,
		"args":    []string{"-test.run=TestFakeMCPServer"},
		"env": []string{
			"GO_WANT_HELPER_MCP_SERVER=1",
			"FAKE_MCP_TOOL=go_symbol_references",
			"FAKE_MCP_RESPONSE=Located in the file: payment.go, the symbol is located on line 24.",
		},
	}
	cfgPath := filepath.Join(t.TempDir(), "mcp-config.json")
	cfgData, _ := json.Marshal(cfg)
	if err := os.WriteFile(cfgPath, cfgData, 0o644); err != nil {
		t.Fatalf("write mcp config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := cmdCompare([]string{
		"-repo", dir, "-service", "compare-mcp",
		"-mcp", cfgPath, "-tool", "go_symbol_references", "-arm", "gopls@v0.20+",
		"-mcp-symbols", "5",
		"-ledger", ledger, "-now", "2026-09-13",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	rows := readLedger(t, ledger)
	if len(rows) != 2 {
		t.Fatalf("got %d ledger rows, want 2 (fathom, gopls): %+v", len(rows), rows)
	}
	if rows[1].Arm != "gopls" || rows[1].Version != "v0.20+" {
		t.Fatalf("competitor row = %+v, want arm=gopls version=v0.20+", rows[1])
	}
}

func TestSplitArmVersion(t *testing.T) {
	cases := map[string][2]string{
		"serena":        {"serena", ""},
		"serena@v1.7.0": {"serena", "v1.7.0"},
		"gopls@v0.20+":  {"gopls", "v0.20+"},
	}
	for in, want := range cases {
		name, version := splitArmVersion(in)
		if name != want[0] || version != want[1] {
			t.Errorf("splitArmVersion(%q) = (%q, %q), want (%q, %q)", in, name, version, want[0], want[1])
		}
	}
}

func readLedger(t *testing.T, path string) []indexbench.CompetitorLedgerRow {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open ledger %s: %v", path, err)
	}
	defer f.Close()
	var rows []indexbench.CompetitorLedgerRow
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var r indexbench.CompetitorLedgerRow
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatalf("parse ledger line %q: %v", sc.Text(), err)
		}
		rows = append(rows, r)
	}
	return rows
}

// TestFakeMCPServer mirrors internal/indexbench's helper-process pattern
// (GO_WANT_HELPER_MCP_SERVER=1) so cmdCompare's -mcp path is proven without
// any real competitor installed.
func TestFakeMCPServer(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_MCP_SERVER") != "1" {
		t.Skip("helper process, not a real test — see TestCmdCompare_MCP")
	}
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
