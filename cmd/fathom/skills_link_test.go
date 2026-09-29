package main

// skills_link_test.go — ADR-059 AG-8, AG-9, AG-12 (docs/plans/agents-md.md):
// `.claude/skills` holds the one real copy; `.agents/skills` is fathom's
// relative link to it, planned only for a harness that reads `.agents/skills`
// and not `.claude/skills`; no writer writes through the link; an earlier
// real copy is replaced only when every file in it is fathom's, unedited.

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
	"github.com/geoffmilleraz/fathom/internal/wiring"
)

func TestSkillsLinkOnlyForAgentsSkillsReaders(t *testing.T) {
	if got := skillsTargetsFor("claude"); !reflect.DeepEqual(got, []string{skillsTargetClaude}) {
		t.Errorf("claude targets = %v, want .claude/skills only", got)
	}
	for _, mode := range []string{"all", "codex", "agents"} {
		if got := skillsTargetsFor(mode); !reflect.DeepEqual(got, []string{skillsTargetClaude, skillsTargetAgents}) {
			t.Errorf("%s targets = %v, want .claude/skills then the .agents/skills link", mode, got)
		}
	}

	for _, mode := range []string{"claude", "codex", "agents", "all"} {
		root := t.TempDir()
		var out bytes.Buffer
		writes, err := installSkillsReport(root, mode, &out)
		if err != nil {
			t.Fatalf("%s: %v", mode, err)
		}
		for _, name := range skillsdist.Names {
			fi, err := os.Lstat(filepath.Join(root, ".claude", "skills", name, "SKILL.md"))
			if err != nil || !fi.Mode().IsRegular() {
				t.Fatalf("%s: .claude/skills/%s/SKILL.md is not a real file (err=%v)", mode, name, err)
			}
		}
		isLink, target := wiring.LinkState(root, agentsSkillsDir)
		if mode == "claude" {
			if _, err := os.Lstat(filepath.Join(root, ".agents")); !os.IsNotExist(err) {
				t.Errorf("claude: .agents exists (err=%v); Claude Code reads .claude/skills only", err)
			}
			continue
		}
		if !isLink || target != agentsSkillsLinkTarget {
			t.Fatalf("%s: .agents/skills link = %v %q, want a link to %s", mode, isLink, target, agentsSkillsLinkTarget)
		}
		through, _ := os.ReadFile(filepath.Join(root, ".agents", "skills", "fathom-plan", "SKILL.md"))
		real, _ := os.ReadFile(filepath.Join(root, ".claude", "skills", "fathom-plan", "SKILL.md"))
		if len(through) == 0 || !bytes.Equal(through, real) {
			t.Errorf("%s: the link does not resolve to the one real copy", mode)
		}
		last := writes[len(writes)-1]
		if last.Target != skillsTargetAgents || last.Action != "linked" {
			t.Errorf("%s: last write = %+v, want the link", mode, last)
		}
		// Idempotent: the second run links nothing and rewrites nothing.
		out.Reset()
		again, err := installSkillsReport(root, mode, &out)
		if err != nil {
			t.Fatal(err)
		}
		if a := again[len(again)-1]; a.Action != "current" {
			t.Errorf("%s: second run link action = %q, want current", mode, a.Action)
		}
	}
}

// TestSkillsLinkReplacesAnEarlierFathomCopyAndBlocksAForeignOne pins the
// migration and its refusal.
func TestSkillsLinkReplacesAnEarlierFathomCopyAndBlocksAForeignOne(t *testing.T) {
	root := t.TempDir()
	// An earlier fathom's two-copy layout: the spec-only render under the
	// marker in a real .agents/skills.
	for _, name := range skillsdist.Names {
		body, err := renderSkill(skillsTargetAgents, name)
		if err != nil {
			t.Fatal(err)
		}
		writeRepoFile(t, root, ".agents/skills/"+name+"/SKILL.md", string(body))
	}
	var out bytes.Buffer
	if _, err := installSkillsReport(root, "codex", &out); err != nil {
		t.Fatalf("migration: %v\n%s", err, out.String())
	}
	if isLink, _ := wiring.LinkState(root, agentsSkillsDir); !isLink {
		t.Fatal("fathom's own earlier copy was not replaced by the link")
	}

	foreign := t.TempDir()
	writeRepoFile(t, foreign, ".agents/skills/fathom-plan/SKILL.md", mustRender(t, skillsTargetAgents, "fathom-plan"))
	writeRepoFile(t, foreign, ".agents/skills/house-skill/SKILL.md", "---\nname: house-skill\ndescription: ours\n---\n")
	_, err := installSkillsReport(foreign, "agents", &out)
	if err == nil || !strings.Contains(err.Error(), ".agents/skills/house-skill/SKILL.md") {
		t.Fatalf("a foreign entry must block the link by name; err = %v", err)
	}
	if got := readRepoFile(t, foreign, ".agents/skills/house-skill/SKILL.md"); !strings.Contains(got, "ours") {
		t.Fatal("the foreign skill was touched")
	}
	if isLink, _ := wiring.LinkState(foreign, agentsSkillsDir); isLink {
		t.Fatal("a directory holding a foreign skill was replaced")
	}
}

func mustRender(t *testing.T, target, name string) string {
	t.Helper()
	b, err := renderSkill(target, name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestShippedSkillsCarryOnlyAgentSkillsFields pins AG-9: every shipped
// SKILL.md is already its own spec-only rendering — only Agent Skills
// frontmatter fields, name equal to its directory — so the bytes a
// spec-only reader finds through the `.agents/skills` link are valid.
func TestShippedSkillsCarryOnlyAgentSkillsFields(t *testing.T) {
	for _, name := range skillsdist.Names {
		raw, err := skillsdist.FS.ReadFile(name + "/SKILL.md")
		if err != nil {
			t.Fatal(err)
		}
		spec, err := agentsVariant(name, raw)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !bytes.Equal(spec, raw) {
			t.Errorf("%s: SKILL.md carries frontmatter beyond the Agent Skills fields, or a name other than its directory — the linked copy would be invalid for a spec-only reader", name)
		}
		for _, line := range strings.Split(frontmatterOf(t, raw), "\n") {
			if m := frontmatterKeyLine.FindStringSubmatch(line); m != nil && !agentSkillsFields[m[1]] {
				t.Errorf("%s: frontmatter key %q is not an Agent Skills field", name, m[1])
			}
		}
	}
}

// TestReconcileNeverWritesThroughTheSkillsLink: with the link in place the
// reconcile plan owns .claude/skills files and the link, never a file
// under .agents/skills, and drift counts each skill once.
func TestReconcileNeverWritesThroughTheSkillsLink(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	if _, err := installSkillsReport(root, "all", &out); err != nil {
		t.Fatal(err)
	}
	for _, f := range skillOwnedFiles(root) {
		if strings.HasPrefix(f.File, agentsSkillsDir+"/") {
			t.Errorf("reconcile would write %s through the link", f.File)
		}
	}
	if missing, stale, blocked := skillDriftFiles(root); len(missing)+len(stale)+len(blocked) != 0 {
		t.Errorf("a fresh install reads as drift: missing=%v stale=%v blocked=%v", missing, stale, blocked)
	}
	owned := ownedSkillFiles(root, "")
	if len(owned) != len(skillsdist.Names) {
		t.Errorf("ownedSkillFiles = %d entries, want %d (one per skill; the link is not a second copy)", len(owned), len(skillsdist.Names))
	}
	// A repo with no .agents/skills never gets one from reconcile.
	bare := t.TempDir()
	if links := skillLinks(bare); len(links) != 0 {
		t.Errorf("reconcile planned a link init never chose: %+v", links)
	}
}

// TestCheckAgentsSkillsResolvesTheLink pins AG-12's skills half.
func TestCheckAgentsSkillsResolvesTheLink(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	if _, err := installSkillsReport(root, "codex", &out); err != nil {
		t.Fatal(err)
	}
	c := checkAgentsSkills(root, "codex")
	if c.Status != doctorPass || !strings.Contains(c.Reason, "links to .claude/skills") {
		t.Fatalf("codex skills through the link = %+v", c)
	}
	other := t.TempDir()
	if err := os.MkdirAll(filepath.Join(other, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../elsewhere", filepath.Join(other, ".agents", "skills")); err != nil {
		t.Fatal(err)
	}
	if c := checkAgentsSkills(other, "codex"); c.Status != doctorFail || !strings.Contains(c.Reason, "not fathom's link") {
		t.Errorf("a link elsewhere = %+v, want FAIL naming it", c)
	}
}
