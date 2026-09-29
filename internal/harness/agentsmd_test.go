package harness

// agentsmd_test.go — AG-1 (docs/plans/agents-md.md, ADR-059): the
// descriptors say where the stanza goes and what a CLAUDE file carries,
// as data. Claude Code reads AGENTS.md natively since 2.1.277 and discovers
// skills only in .claude/skills; every harness gets the stanza in AGENTS.md.

import (
	"reflect"
	"strings"
	"testing"
)

func TestClaudeCodeReadsAgentsMDNativelyAndImportsFromCLAUDE(t *testing.T) {
	d, _ := Lookup(ClaudeCode)
	modes := map[string]string{}
	for _, f := range d.Instructions {
		modes[f.Path] = f.Mode
	}
	if modes["AGENTS.md"] != "native" {
		t.Errorf("claude-code AGENTS.md mode = %q, want native (Claude Code ≥2.1.277)", modes["AGENTS.md"])
	}
	for _, f := range []string{"CLAUDE.md", ".claude/CLAUDE.md", "CLAUDE.local.md"} {
		if modes[f] != "import" {
			t.Errorf("claude-code %s mode = %q, want import", f, modes[f])
		}
	}
	if got, want := d.ImportFiles(), []string{"CLAUDE.md", ".claude/CLAUDE.md", "CLAUDE.local.md"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ImportFiles = %v, want %v (Claude Code's precedence order)", got, want)
	}
	if got, want := d.Skills.RepoDirs, []string{".claude/skills/{{name}}/SKILL.md"}; !reflect.DeepEqual(got, want) {
		t.Errorf("claude-code skills dirs = %v, want %v — Claude Code does not discover .agents/skills", got, want)
	}
	if !strings.Contains(d.VerifiedAgainst, "2.1.284") {
		t.Errorf("claude-code verified_against %q does not name 2.1.284", d.VerifiedAgainst)
	}
	if note := d.Notes[CapStanzaLoaded]; !strings.Contains(note, "AGENTS.md") {
		t.Errorf("stanza-loaded note %q does not say InstructionsLoaded is silent for an AGENTS.md loaded natively", note)
	}
}

func TestEveryHarnessGetsTheStanzaInAgentsMDOnly(t *testing.T) {
	for _, d := range All() {
		if got := d.StanzaFiles(); !reflect.DeepEqual(got, []string{"AGENTS.md"}) {
			t.Errorf("%s: StanzaFiles = %v, want [AGENTS.md] (ADR-059)", d.ID, got)
		}
		if d.ID != ClaudeCode && len(d.ImportFiles()) != 0 {
			t.Errorf("%s: ImportFiles = %v, want none", d.ID, d.ImportFiles())
		}
	}
}

func TestGeminiReadsAgentsMDThroughTheContextFileNameSetting(t *testing.T) {
	d, _ := Lookup(GeminiCLI)
	var agentsMD *InstructionFile
	for i := range d.Instructions {
		if d.Instructions[i].Path == "AGENTS.md" {
			agentsMD = &d.Instructions[i]
		}
	}
	if agentsMD == nil || agentsMD.Mode != "settings-key" || agentsMD.Setting == nil {
		t.Fatalf("gemini-cli AGENTS.md = %+v, want a settings-key file with its setting", agentsMD)
	}
	s := agentsMD.Setting
	if s.File != ".gemini/settings.json" || !reflect.DeepEqual(s.Key, []string{"context", "fileName"}) || !reflect.DeepEqual(s.Value, []string{"AGENTS.md", "GEMINI.md"}) {
		t.Errorf("gemini-cli setting = %+v, want .gemini/settings.json context.fileName [AGENTS.md GEMINI.md] (the key replaces the default list)", s)
	}
}
