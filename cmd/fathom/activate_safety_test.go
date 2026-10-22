package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// GDEV-42: an upgrade playbook can retire only its generated artifacts.
func TestActivatePreservesConsumerIntent(t *testing.T) {
	body, err := renderSkill(skillsTargetClaude, "fathom-activate")
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{skillsTargetClaude, skillsTargetAgents, skillsTargetCodexPrompts} {
		b, err := renderSkill(target, "fathom-activate")
		if err != nil {
			t.Fatal(err)
		}
		if target == skillsTargetCodexPrompts && !strings.Contains(string(b), "INTENT.md") {
			continue // the deprecated prompt does not contain the retirement step
		}
		text := string(b)
		for _, want := range []string{"consumer-authored intent is never deleted", "<!-- fathom:intent-playbook begin -->", "upgrade"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s lacks retirement boundary %q", target, want)
			}
		}
		if strings.Contains(text, "delete any root") {
			t.Errorf("%s still prescribes unconditional root deletion", target)
		}
	}
	committed, err := os.ReadFile(filepath.Join("..", "..", "plugins", "fathom", "skills", "fathom-activate", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(committed), "consumer-authored intent is never deleted") || !strings.Contains(string(body), "consumer-authored intent is never deleted") {
		t.Fatal("shipped and rendered activate playbooks must preserve consumer intent")
	}
	out := t.TempDir()
	var stdout, stderr bytes.Buffer
	if code := cmdPluginRender([]string{"--out", out}, &stdout, &stderr); code != 0 {
		t.Fatalf("plugin render = %d: %s", code, stderr.String())
	}
	rendered, err := os.ReadFile(filepath.Join(out, "skills", "fathom-activate", "SKILL.md"))
	if err != nil || !bytes.Equal(rendered, committed) {
		t.Fatalf("committed playbook differs from fresh render: %v", err)
	}
}
