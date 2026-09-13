package scenariobench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/wiring"
)

func TestRenderArm_WiredWritesHooks(t *testing.T) {
	dir := t.TempDir()
	arm := Arm{Wired: true, Instructions: "stanza", MCP: map[string]any{"fathom": map[string]any{"type": "http"}}}
	files, err := RenderArm(arm, dir)
	if err != nil {
		t.Fatalf("RenderArm: %v", err)
	}
	if files.SettingsPath == "" {
		t.Fatal("expected a SettingsPath")
	}
	raw, err := os.ReadFile(files.SettingsPath)
	if err != nil {
		t.Fatalf("reading settings: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatalf("parsing settings: %v", err)
	}
	if _, ok := settings["hooks"]; !ok {
		t.Error("wired arm's settings.json has no \"hooks\" key")
	}
	if files.MCPConfigPath == "" {
		t.Fatal("expected an MCPConfigPath")
	}
	if files.InstructionsPath == "" {
		t.Error("expected InstructionsPath for instructions=stanza")
	}
}

func TestRenderArm_UnwiredNoHooks(t *testing.T) {
	dir := t.TempDir()
	arm := Arm{Wired: false, Instructions: "plain"}
	files, err := RenderArm(arm, dir)
	if err != nil {
		t.Fatalf("RenderArm: %v", err)
	}
	raw, err := os.ReadFile(files.SettingsPath)
	if err != nil {
		t.Fatalf("reading settings: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatalf("parsing settings: %v", err)
	}
	if _, ok := settings["hooks"]; ok {
		t.Error("unwired arm's settings.json has a \"hooks\" key")
	}
	// Even when arm.MCP is nil, an (empty) MCP config file must still be
	// written — a harness invocation always needs a --mcp-config path.
	if files.MCPConfigPath == "" {
		t.Fatal("expected an MCPConfigPath even for an empty arm.MCP")
	}
	raw2, err := os.ReadFile(files.MCPConfigPath)
	if err != nil {
		t.Fatalf("reading mcp config: %v", err)
	}
	var doc struct {
		MCPServers map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw2, &doc); err != nil {
		t.Fatalf("parsing mcp config: %v", err)
	}
	if len(doc.MCPServers) != 0 {
		t.Errorf("expected an empty mcpServers block, got %v", doc.MCPServers)
	}
}

func TestRenderArm_InstructionsNoneRemovesClaudeMD(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "CLAUDE.md"), "# hi\n")
	arm := Arm{Instructions: "none"}
	files, err := RenderArm(arm, dir)
	if err != nil {
		t.Fatalf("RenderArm: %v", err)
	}
	if files.InstructionsPath != "" {
		t.Errorf("InstructionsPath = %q, want empty", files.InstructionsPath)
	}
	if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Error("CLAUDE.md should have been removed")
	}
}

func TestRenderArm_InstructionsPlainLeavesFileUntouched(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "CLAUDE.md"), "# subject's own\n")
	arm := Arm{Instructions: "plain"}
	files, err := RenderArm(arm, dir)
	if err != nil {
		t.Fatalf("RenderArm: %v", err)
	}
	if files.InstructionsPath == "" {
		t.Error("expected InstructionsPath to be set for an existing file")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil || string(raw) != "# subject's own\n" {
		t.Errorf("CLAUDE.md was modified: %q, err=%v", raw, err)
	}
}

func TestRenderArm_StanzaRequiresWired(t *testing.T) {
	dir := t.TempDir()
	arm := Arm{Wired: false, Instructions: "stanza"}
	if _, err := RenderArm(arm, dir); err == nil {
		t.Fatal("expected an error: instructions=stanza on an unwired arm")
	}
}

func TestRenderArm_UnknownInstructions(t *testing.T) {
	dir := t.TempDir()
	arm := Arm{Instructions: "bogus"}
	if _, err := RenderArm(arm, dir); err == nil {
		t.Fatal("expected an error for an unknown instructions value")
	}
}

func TestRenderArm_NotesFileSystemAppend(t *testing.T) {
	dir := t.TempDir()
	arm := Arm{Instructions: "none", NotesFile: true}
	files, err := RenderArm(arm, dir)
	if err != nil {
		t.Fatalf("RenderArm: %v", err)
	}
	if files.SystemAppend == "" {
		t.Error("expected SystemAppend to be set when NotesFile is true")
	}

	dir2 := t.TempDir()
	arm2 := Arm{Instructions: "none", NotesFile: false}
	files2, err := RenderArm(arm2, dir2)
	if err != nil {
		t.Fatalf("RenderArm: %v", err)
	}
	if files2.SystemAppend != "" {
		t.Errorf("expected empty SystemAppend when NotesFile is false, got %q", files2.SystemAppend)
	}
}

// markedStanza returns a CLAUDE.md-shaped fathom stanza using the SAME
// marker syntax internal/wiring uses, so StripFathom's removal is tested
// against the real convention, not a stand-in.
func markedStanza(body string) string {
	return wiring.HTMLMarkers.BeginPrefix + wiring.HTMLMarkers.BeginSuffix + "\n" + body + "\n" + wiring.HTMLMarkers.End + "\n"
}

func TestStripFathom_RemovesMarkedRegionHooksAndMCPEntry(t *testing.T) {
	dir := t.TempDir()
	prose := "# Subject repo\n\nSome human-authored prose.\n\n"
	writeTestFile(t, filepath.Join(dir, "CLAUDE.md"), prose+markedStanza("fathom's stanza body")+"\nmore prose after\n")

	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(claudeDir, "settings.json"), `{"hooks":{"PostToolUse":[]},"other":"keepme"}`)
	writeTestFile(t, filepath.Join(dir, ".mcp.json"), `{"mcpServers":{"fathom":{"type":"http"},"other-server":{"type":"stdio"}}}`)

	if err := StripFathom(dir); err != nil {
		t.Fatalf("StripFathom: %v", err)
	}

	claudeMD, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("reading CLAUDE.md: %v", err)
	}
	if strings.Contains(string(claudeMD), "fathom's stanza body") {
		t.Errorf("CLAUDE.md still contains the stanza: %q", claudeMD)
	}
	if !strings.Contains(string(claudeMD), "Some human-authored prose") || !strings.Contains(string(claudeMD), "more prose after") {
		t.Errorf("StripFathom removed prose it should have left alone: %q", claudeMD)
	}

	var settings map[string]any
	raw, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatalf("parsing settings.json: %v", err)
	}
	if _, ok := settings["hooks"]; ok {
		t.Error("settings.json still has a hooks key")
	}
	if settings["other"] != "keepme" {
		t.Errorf("settings.json lost an unrelated key: %v", settings)
	}

	var mcp struct {
		MCPServers map[string]any `json:"mcpServers"`
	}
	raw2, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	if err != nil {
		t.Fatalf("reading .mcp.json: %v", err)
	}
	if err := json.Unmarshal(raw2, &mcp); err != nil {
		t.Fatalf("parsing .mcp.json: %v", err)
	}
	if _, ok := mcp.MCPServers["fathom"]; ok {
		t.Error(".mcp.json still has a fathom entry")
	}
	if _, ok := mcp.MCPServers["other-server"]; !ok {
		t.Error(".mcp.json lost an unrelated mcp server entry")
	}
}

func TestStripFathom_AbsentFilesAreNoOp(t *testing.T) {
	dir := t.TempDir()
	if err := StripFathom(dir); err != nil {
		t.Fatalf("StripFathom on a worktree with nothing to strip: %v", err)
	}
}
