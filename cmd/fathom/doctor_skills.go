package main

// doctor_skills.go — the `skills_installed` row on the standard (RP-3,
// docs/plans/reach-parity.md): Claude Code's skills live in
// `.claude/skills`; every other registry harness (codex, gemini-cli,
// agents, …) discovers the Agent Skills path `.agents/skills`, and the
// on-disk bytes there are compared against the spec-only rendering
// (agentsVariant) — the same bytes `fathom init --skills` writes. A missing
// skill is NAMED in the reason, never folded into a count.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
	"github.com/geoffmilleraz/fathom/internal/harness"
)

// checkAgentsSkills is doctor's skills_installed row. harnessLabel is
// doctor's label ("claude" for Claude Code; a registry id otherwise).
func checkAgentsSkills(root, harnessLabel string) doctorCheck {
	const name = "skills_installed"

	target := skillsTargetAgents
	remedy := "fathom init --skills=agents --path " + root
	if harnessLabel == "claude" {
		target = skillsTargetClaude
		remedy = "fathom init --skills=claude --path " + root
	}

	var missing, stale, notOwned []string
	current := 0
	for _, skillName := range skillsdist.Names {
		want, err := renderSkill(target, skillName)
		if err != nil {
			return doctorCheck{Name: name, Status: doctorSkipped, Reason: "embedded skill source unreadable: " + err.Error()}
		}
		dest := skillDest(root, "", target, skillName)
		got, err := os.ReadFile(dest)
		switch {
		case err != nil:
			missing = append(missing, skillName)
		case !strings.Contains(string(got), skillMarker):
			notOwned = append(notOwned, dest)
		case string(got) != string(want):
			stale = append(stale, dest)
		default:
			current++
		}
	}

	dir := filepath.Join(root, ".agents", "skills")
	if target == skillsTargetClaude {
		dir = filepath.Join(root, ".claude", "skills")
	}
	var c doctorCheck
	switch {
	case len(missing) > 0 || len(stale) > 0 || len(notOwned) > 0:
		var parts []string
		if len(missing) > 0 {
			parts = append(parts, fmt.Sprintf("missing from %s: %s", dir, strings.Join(missing, ", ")))
		}
		if len(stale) > 0 {
			parts = append(parts, "stale (differs from the shipped source): "+strings.Join(stale, ", "))
		}
		if len(notOwned) > 0 {
			parts = append(parts, "not fathom-owned (no marker; init refuses to touch): "+strings.Join(notOwned, ", "))
		}
		c = doctorCheck{Name: name, Status: doctorFail, Reason: strings.Join(parts, "; "), Remedy: remedy}
	default:
		c = doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("all %d shipped skills current in %s for %s", current, dir, harnessLabel)}
	}
	if d, ok := harness.Lookup(doctorHarnessID(harnessLabel)); ok && !d.Skills.Verified {
		c.Reason += " (discovery of .agents/skills by " + d.Display + " is unverified on this machine)"
	}
	return c
}
