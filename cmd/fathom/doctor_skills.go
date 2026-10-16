package main

// doctor_skills.go — the `skills_installed` row on the standard (RP-3,
// docs/plans/reach-parity.md): Claude Code's skills live in
// `.claude/skills`; every other registry harness (codex, gemini-cli,
// agents, …) discovers the Agent Skills path `.agents/skills`. Since
// ADR-059 that path is fathom's link to `.claude/skills` (AG-8, AG-12): the
// row resolves the link and compares the one real copy; an earlier real
// `.agents/skills` copy is compared against the spec-only rendering
// (agentsVariant) and named as the link-to-be; a link anywhere else fails.
// A missing skill is NAMED in the reason, never folded into a count.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/wiring"
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
	var linkNote string
	if target == skillsTargetAgents {
		switch isLink, to := wiring.LinkState(root, agentsSkillsDir); {
		case isLink && to == agentsSkillsLinkTarget:
			// The link is the vehicle: the bytes a reader sees through it are
			// the one real copy's, spec-conformant by test (AG-9).
			target = skillsTargetClaude
			linkNote = " (.agents/skills links to .claude/skills)"
		case isLink:
			return doctorCheck{Name: name, Status: doctorFail,
				Reason: fmt.Sprintf("%s is a link to %s, not fathom's link to %s — the skills a reader finds there are not fathom's", filepath.Join(root, filepath.FromSlash(agentsSkillsDir)), to, agentsSkillsLinkTarget),
				Remedy: "move the link aside, then " + remedy}
		default:
			if fi, err := os.Lstat(filepath.Join(root, filepath.FromSlash(agentsSkillsDir))); err == nil && fi.IsDir() {
				linkNote = " (a real directory: `" + remedy + "` replaces it with a link to .claude/skills)"
			}
		}
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
	if target == skillsTargetClaude && linkNote == "" {
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
	c.Reason += linkNote
	if d, ok := harness.Lookup(doctorHarnessID(harnessLabel)); ok && !d.Skills.Verified {
		c.Reason += " (discovery of .agents/skills by " + d.Display + " is unverified on this machine)"
	}
	return c
}
