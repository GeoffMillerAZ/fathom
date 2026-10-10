package main

// wiringdrift.go — PC-28 / PC-29 / PC-30 (docs/plans/proof-credibility.md,
// vision 26 §1.4): ONE computation of "did anything an agent could be made
// to change drift?" — hooks and stanza regions (law 15, already digested),
// MCP server entries in the files each harness descriptor names, and
// fathom-owned skill files — consumed by `fleet reconcile`, doctor's
// wiring_drift row, the custodian's wiring_current concern and the session
// attention line. Reported by kind and path, never merged.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/wiring"
)

// expandHome turns a descriptor's "~/x" into home/x; other paths are
// joined under root. Label keeps the descriptor's spelling.
func expandHome(p, root, home string) (path, label string) {
	if strings.HasPrefix(p, "~/") {
		if home == "" {
			return "", p
		}
		return filepath.Join(home, p[2:]), p
	}
	return filepath.Join(root, filepath.FromSlash(p)), p
}

// mcpFilesFor lists the MCP config files to baseline for the given harness
// descriptors (every descriptor when ids is empty), skipping descriptors
// with no config file / key path (the generic "agents" row).
func mcpFilesFor(root, home string, ids []string) []wiring.MCPFile {
	var descs []harness.Descriptor
	if len(ids) == 0 {
		descs = harness.All()
	} else {
		for _, id := range ids {
			if d, ok := harness.Lookup(id); ok {
				descs = append(descs, d)
			}
		}
	}
	var out []wiring.MCPFile
	seen := map[string]bool{}
	for _, d := range descs {
		if d.MCP.Format == "" || len(d.MCP.KeyPath) == 0 {
			continue
		}
		for _, cf := range d.MCP.ConfigFiles {
			path, label := expandHome(cf, root, home)
			if path == "" || seen[label] {
				continue
			}
			seen[label] = true
			out = append(out, wiring.MCPFile{Path: path, Label: label, Format: d.MCP.Format, KeyPath: append([]string(nil), d.MCP.KeyPath...)})
		}
	}
	return out
}

// ownedSkillFiles lists every shipped skill file that is present AND
// carries the fathom:owned marker, in the shapes installSkills writes or
// wrote (.claude/skills, an earlier real .agents/skills copy,
// ~/.codex/prompts). A present
// file without the marker is someone else's and is not fathom's to watch.
// Each carries what this binary renders for its target (renderSkill) and
// the renders earlier releases shipped (shippedSkillRenderDigests), so a
// file equal to any render fathom shipped never reads as drift (FA-24).
func ownedSkillFiles(root, home string) []wiring.SkillFile {
	var out []wiring.SkillFile
	consider := func(path, label, target, name string) {
		shipped, _ := renderSkill(target, name)
		f := wiring.SkillFile{Path: path, Label: label, Shipped: shipped, Known: shippedSkillRenderDigests()}
		data, err := os.ReadFile(path)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				// Unreadable: hand it on so DriftSkills names it (I38).
				out = append(out, f)
			}
			return
		}
		if bytes.Contains(data, []byte(skillMarker)) {
			out = append(out, f)
		}
	}
	// A `.agents/skills` that is a link (ADR-059) is one owned vehicle with
	// `.claude/skills`: reading through it would count every file twice.
	agentsIsLink, _ := wiring.LinkState(root, agentsSkillsDir)
	for _, name := range skillsdist.Names {
		consider(filepath.Join(root, ".claude", "skills", name, "SKILL.md"), ".claude/skills/"+name+"/SKILL.md", skillsTargetClaude, name)
		if !agentsIsLink {
			consider(filepath.Join(root, ".agents", "skills", name, "SKILL.md"), ".agents/skills/"+name+"/SKILL.md", skillsTargetAgents, name)
		}
		if home != "" {
			consider(filepath.Join(home, ".codex", "prompts", name+".md"), "~/.codex/prompts/"+name+".md", skillsTargetCodexPrompts, name)
		}
	}
	return out
}

// wiringDriftFor computes the full report for one repo: the law-15 plan
// over the stanza regions and the hook keys fathom owns, plus MCP entries
// and owned skills against .fathom/owned.json. harnessIDs "" means every
// descriptor; home "" skips user-scope files.
func wiringDriftFor(root, service string, harnessIDs []string, home string) wiring.DriftReport {
	var rep wiring.DriftReport
	spec := wiring.Spec{}
	svc := stanzaServiceID(root, service)
	regions, _ := instructionRegions(root, harness.ClaudeCode, svc, intentChannelWired(root))
	spec.Regions = append(spec.Regions, regions...)
	keys, _ := shimHookKeys(root)
	spec.Keys = keys
	if plan, err := wiring.PlanWiring(root, spec); err == nil {
		rep.Merge(wiring.DriftFromPlan(plan))
		if plan.LedgerNote != "" {
			rep.Unreadable = append(rep.Unreadable, ".fathom/owned.json — "+plan.LedgerNote)
		}
	} else {
		rep.Unreadable = append(rep.Unreadable, "wiring plan — "+err.Error())
	}
	rep.Merge(wiringDriftBeyondPlan(root, harnessIDs, home))
	return rep
}

// wiringDriftBeyondPlan is the drift no wiring plan can see: owned MCP
// entries and skill files changed since fathom recorded them. The wiring
// tab itemizes doctor's wiring_drift row with it (the rest of that row is
// the preview plan's BLOCKED items), so a drift is counted once.
func wiringDriftBeyondPlan(root string, harnessIDs []string, home string) wiring.DriftReport {
	var rep wiring.DriftReport
	ledger, lerr := wiring.LoadLedger(root)
	if lerr != nil {
		ledger = wiring.NewLedger()
	}
	rep.Merge(wiring.DriftMCP(ledger, mcpFilesFor(root, home, harnessIDs), nil))
	rep.Merge(wiring.DriftSkills(ledger, ownedSkillFiles(root, home), nil))
	return rep
}

// wiringDriftHome is the user home for user-scope files, or "" when none.
func wiringDriftHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// recordWiringBaselinesDetail records the MCP entries and owned skills that
// have no baseline yet, plus — only with rebaseline set — the drifted hook
// entries' digests (fleet reconcile --apply[--rebaseline]). A hook entry is
// never rewritten here; only its ledger digest moves, same as an MCP entry.
// Returns the labels recorded, sorted, and the skill files rebaseline
// refused: content no fathom release rendered stays DRIFT (FA-24,
// wiring.RecordBaselinesDetail).
func recordWiringBaselinesDetail(root string, harnessIDs []string, home, wrote string, rebaseline bool) (recorded, refused []string, err error) {
	recorded, refused, err = wiring.RecordBaselinesDetail(root, mcpFilesFor(root, home, harnessIDs), ownedSkillFiles(root, home), wrote, nil, rebaseline)
	if err != nil {
		return recorded, refused, err
	}
	hookKeys, _ := shimHookKeys(root)
	hookRecorded, herr := wiring.RecordKeyBaselines(root, hookKeys, wrote, nil, rebaseline)
	if herr != nil {
		return recorded, refused, herr
	}
	recorded = append(recorded, hookRecorded...)
	sort.Strings(recorded)
	return recorded, refused, nil
}

// wiringDriftAttention is the session attention fragment (PC-30):
// "wiring drift N" when anything fathom trusts changed, "" otherwise —
// and "" too when a file could not be read (an unreadable file is doctor's
// wiring_drift SKIP, not a nag on every session line).
func wiringDriftAttention(projDir string) string {
	rep := wiringDriftFor(projDir, "", nil, wiringDriftHome())
	if len(rep.Drifts) == 0 {
		return ""
	}
	return fmt.Sprintf("wiring drift %d", len(rep.Drifts))
}

// wiringCurrentEvaluator is what the custodian's wiring_current concern
// runs (internal/custodian must not import cmd/fathom): drift lines, the
// unreadable files, and how many entries were checked.
func wiringCurrentEvaluator(root string) (drifts, unreadable []string, checked int) {
	rep := wiringDriftFor(root, "", nil, wiringDriftHome())
	return rep.Lines(), rep.Unreadable, rep.Checked
}
