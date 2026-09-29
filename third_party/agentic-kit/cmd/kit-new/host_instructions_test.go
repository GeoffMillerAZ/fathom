package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// One instruction file: Claude Code reads AGENTS.md natively only while no
// CLAUDE.md exists (docs/project-intent/integration.md section 2), so the
// installer must never create a CLAUDE.md, and must import AGENTS.md into one
// the adopter already has.

func changedPaths(plan hostPlan) []string {
	paths := make([]string, 0, len(plan.Changes))
	for _, c := range plan.Changes {
		paths = append(paths, c.Path)
	}
	return paths
}

func readOrEmpty(t *testing.T, f hostFixture, path string) (string, bool) {
	t.Helper()
	raw, exists, err := hostRead(f.root, path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw), exists
}

func TestHostPathsBothHostsUseAgentsMd(t *testing.T) {
	for _, host := range []string{"claude", "codex"} {
		if _, instructions, _, _ := hostPaths(host); instructions != "AGENTS.md" {
			t.Fatalf("%s instruction file = %q, want AGENTS.md", host, instructions)
		}
	}
}

func TestHostClaudeInstallWritesAgentsMdAndNeverCreatesClaudeMd(t *testing.T) {
	f := newHostFixture(t, "claude")
	plan := f.install(t)
	got := changedPaths(plan)
	want := []string{".agentic-kit/hosts/claude/ownership.json", ".agentic-kit/hosts/claude/policy.json", "AGENTS.md", ".claude/settings.json"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("plan paths = %v, want %v", got, want)
	}
	if _, exists := readOrEmpty(t, f, "CLAUDE.md"); exists {
		t.Fatal("installer created CLAUDE.md, which hides AGENTS.md from Claude Code")
	}
	text, exists := readOrEmpty(t, f, "AGENTS.md")
	if !exists || !strings.Contains(text, "agentic-kit:host:claude:begin") {
		t.Fatalf("AGENTS.md lacks the stanza: %q", text)
	}
	if _, _, err := validateInstalledHost(f.root, f.dir, "claude"); err != nil {
		t.Fatalf("doctor rejects a fresh install: %v", err)
	}
}

func TestHostClaudeInstallImportsAgentsMdIntoExistingClaudeMd(t *testing.T) {
	f := newHostFixture(t, "claude")
	claude := "Claude-only notes\n"
	f.write(t, "CLAUDE.md", []byte(claude))
	f.write(t, "AGENTS.md", []byte("Shared instructions\n"))
	plan, err := planHostInstall(f.root, f.dir, "claude", f.binary, f.policy)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".agentic-kit/hosts/claude/ownership.json", ".agentic-kit/hosts/claude/policy.json", "AGENTS.md", "CLAUDE.md", ".claude/settings.json"}
	if got := changedPaths(plan); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("plan paths = %v, want %v (hooks last)", got, want)
	}
	if err := applyHostPlan(f.root, plan); err != nil {
		t.Fatal(err)
	}
	after, _ := readOrEmpty(t, f, "CLAUDE.md")
	if !strings.HasPrefix(after, claude) || !hostImportsAgents([]byte(after)) {
		t.Fatalf("CLAUDE.md must keep its text and gain the import: %q", after)
	}
	if strings.Contains(after, "Agentic-kit host policy") {
		t.Fatalf("CLAUDE.md must hold the import only, never a second copy of the stanza: %q", after)
	}
	if _, _, err := validateInstalledHost(f.root, f.dir, "claude"); err != nil {
		t.Fatalf("doctor rejects the install: %v", err)
	}
	if again, err := planHostInstall(f.root, f.dir, "claude", f.binary, f.policy); err != nil || len(again.Changes) != 0 {
		t.Fatalf("not idempotent: %v %+v", err, again)
	}

	// Text the adopter adds later survives; uninstall removes exactly what was generated.
	edited := strings.Replace(after, "Claude-only notes\n", "Claude-only notes\nAdded later\n", 1)
	f.write(t, "CLAUDE.md", []byte(edited))
	uninstall, err := planHostUninstall(f.root, f.dir, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if err := applyHostPlan(f.root, uninstall); err != nil {
		t.Fatal(err)
	}
	if got, _ := readOrEmpty(t, f, "CLAUDE.md"); got != "Claude-only notes\nAdded later\n" {
		t.Fatalf("CLAUDE.md after uninstall = %q", got)
	}
	if got, _ := readOrEmpty(t, f, "AGENTS.md"); got != "Shared instructions\n" {
		t.Fatalf("AGENTS.md after uninstall = %q", got)
	}
}

func TestHostClaudeInstallSkipsImportWhenClaudeMdAlreadyImports(t *testing.T) {
	f := newHostFixture(t, "claude")
	claude := "# Project\n\n@AGENTS.md\n"
	f.write(t, "CLAUDE.md", []byte(claude))
	plan := f.install(t)
	if got := changedPaths(plan); strings.Contains(strings.Join(got, ","), "CLAUDE.md") {
		t.Fatalf("plan touches an already-importing CLAUDE.md: %v", got)
	}
	uninstall, err := planHostUninstall(f.root, f.dir, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if err := applyHostPlan(f.root, uninstall); err != nil {
		t.Fatal(err)
	}
	if got, _ := readOrEmpty(t, f, "CLAUDE.md"); got != claude {
		t.Fatalf("uninstall changed the adopter's own import: %q", got)
	}
}

func TestHostImportsAgents(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"empty", "", false},
		{"standalone", "@AGENTS.md\n", true},
		{"standalone with relative form", "notes\n@./AGENTS.md\n", true},
		{"indented and padded", "  @AGENTS.md  \r\n", true},
		{"prose mention is not an import line", "see @AGENTS.md for more\n", false},
		{"inside a fenced block is not loaded", "```\n@AGENTS.md\n```\n", false},
		{"after a closed fence", "```\ncode\n```\n@AGENTS.md\n", true},
		{"another file", "@README.md\n", false},
		{"nested path", "@docs/AGENTS.md\n", false},
	}
	for _, tc := range cases {
		if got := hostImportsAgents([]byte(tc.text)); got != tc.want {
			t.Errorf("%s: hostImportsAgents(%q) = %v, want %v", tc.name, tc.text, got, tc.want)
		}
	}
}

func TestHostClaudeFencedImportDoesNotCount(t *testing.T) {
	f := newHostFixture(t, "claude")
	f.write(t, "CLAUDE.md", []byte("Example:\n```\n@AGENTS.md\n```\n"))
	plan, err := planHostInstall(f.root, f.dir, "claude", f.binary, f.policy)
	if err != nil {
		t.Fatal(err)
	}
	if got := changedPaths(plan); !strings.Contains(strings.Join(got, ","), "CLAUDE.md") {
		t.Fatalf("an import inside a code fence is not loaded, so the plan must add a real one: %v", got)
	}
}

func TestHostDoctorFailsWhenClaudeMdAppearsWithoutImport(t *testing.T) {
	f := newHostFixture(t, "claude")
	f.install(t)
	if _, _, err := validateInstalledHost(f.root, f.dir, "claude"); err != nil {
		t.Fatal(err)
	}
	f.write(t, "CLAUDE.md", []byte("A later tool wrote this\n"))
	_, _, err := validateInstalledHost(f.root, f.dir, "claude")
	if err == nil || !strings.Contains(err.Error(), "CLAUDE.md") || !strings.Contains(err.Error(), "@AGENTS.md") {
		t.Fatalf("doctor must name the hidden AGENTS.md, got %v", err)
	}
	f.write(t, "CLAUDE.md", []byte("A later tool wrote this\n@AGENTS.md\n"))
	if _, _, err := validateInstalledHost(f.root, f.dir, "claude"); err != nil {
		t.Fatalf("doctor still rejects after the import was added: %v", err)
	}
}

func TestHostUninstallRefusesChangedOwnedImport(t *testing.T) {
	f := newHostFixture(t, "claude")
	f.write(t, "CLAUDE.md", []byte("notes\n"))
	f.install(t)
	after, _ := readOrEmpty(t, f, "CLAUDE.md")
	f.write(t, "CLAUDE.md", []byte(strings.Replace(after, "@AGENTS.md", "@OTHER.md", 1)))
	if _, err := planHostUninstall(f.root, f.dir, "claude"); err == nil || !strings.Contains(err.Error(), "import") {
		t.Fatalf("uninstall must refuse a changed owned import block, got %v", err)
	}
}

func TestHostClaudeConfigDigestCoversClaudeMd(t *testing.T) {
	f := newHostFixture(t, "claude")
	f.install(t)
	before, err := hostConfigDigest(f.root, "claude")
	if err != nil {
		t.Fatal(err)
	}
	f.write(t, "CLAUDE.md", []byte("@AGENTS.md\n"))
	after, err := hostConfigDigest(f.root, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("evidence digest ignores CLAUDE.md, which decides whether Claude Code loads the stanza")
	}
}

// An install written before the one-instruction-file change recorded no
// instruction path and kept a Claude stanza in CLAUDE.md. It must still
// validate and uninstall cleanly, so an upgrade never orphans generated text.
func TestHostLegacyClaudeInstallStillValidatesAndUninstalls(t *testing.T) {
	f := newHostFixture(t, "claude")
	f.write(t, "CLAUDE.md", []byte("User notes\n"))
	f.install(t)

	// Rewrite the fresh install into the legacy layout.
	ownRaw, _, _ := hostRead(f.root, ".agentic-kit/hosts/claude/ownership.json")
	var own hostOwnership
	if err := hostDecode(ownRaw, &own); err != nil {
		t.Fatal(err)
	}
	agents, _ := readOrEmpty(t, f, "AGENTS.md")
	if strings.Count(agents, own.Stanza) != 1 {
		t.Fatalf("fixture: stanza missing from AGENTS.md")
	}
	f.write(t, "CLAUDE.md", []byte("User notes\n"+own.Stanza))
	if err := f.root.Remove("AGENTS.md"); err != nil {
		t.Fatal(err)
	}
	own.InstructionsPath, own.ImportPath, own.Import = "", "", ""
	f.write(t, ".agentic-kit/hosts/claude/ownership.json", hostJSON(own))
	if strings.Contains(string(hostJSON(own)), "instructions_path") {
		t.Fatal("legacy record must omit the new field")
	}

	if _, _, err := validateInstalledHost(f.root, f.dir, "claude"); err != nil {
		t.Fatalf("legacy install no longer validates: %v", err)
	}
	uninstall, err := planHostUninstall(f.root, f.dir, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if err := applyHostPlan(f.root, uninstall); err != nil {
		t.Fatal(err)
	}
	if got, _ := readOrEmpty(t, f, "CLAUDE.md"); got != "User notes\n" {
		t.Fatalf("legacy stanza not removed from CLAUDE.md: %q", got)
	}
}

func TestHostOwnershipRejectsUnknownInstructionPath(t *testing.T) {
	f := newHostFixture(t, "claude")
	f.install(t)
	raw, _, _ := hostRead(f.root, ".agentic-kit/hosts/claude/ownership.json")
	var own hostOwnership
	if err := hostDecode(raw, &own); err != nil {
		t.Fatal(err)
	}
	if own.InstructionsPath != "AGENTS.md" {
		t.Fatalf("ownership records instructions_path %q, want AGENTS.md", own.InstructionsPath)
	}
	own.InstructionsPath = "../elsewhere.md"
	if err := validateHostOwnership(own, "claude", f.dir); err == nil {
		t.Fatal("ownership naming an arbitrary instruction path must be invalid")
	}
	own.InstructionsPath = "AGENTS.md"
	own.ImportPath = "AGENTS.md"
	own.Import = "x"
	if err := validateHostOwnership(own, "claude", f.dir); err == nil {
		t.Fatal("an import recorded outside CLAUDE.md must be invalid")
	}
	// The record stays plain JSON with the documented keys.
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		t.Fatal(err)
	}
	if _, ok := keys["instructions_path"]; !ok {
		t.Fatal("instructions_path key missing from the ownership record")
	}
}
