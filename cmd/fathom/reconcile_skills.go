package main

// reconcile_skills.go — RP-4 (docs/plans/reach-parity.md): the shipped
// skills as owned files in the wiring engine. `fleet reconcile` plans both
// repo-scoped shapes — `.claude/skills/<name>/SKILL.md` (the Claude Code
// superset) and `.agents/skills/<name>/SKILL.md` (the Agent Skills
// standard, spec-only frontmatter) — through wiring.OwnedFile: a rewrite
// happens only under the `fathom:owned` marker, a hand-authored file by a
// skill's name is BLOCKED by name, a hand edit under the marker is BLOCKED
// as drift, and every write is receipted and undoable.

import (
	"path"
	"sort"
	"strings"

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
	"github.com/geoffmilleraz/fathom/internal/wiring"
)

// skillOwnedFiles renders every shipped skill for both repo-scoped
// targets as owned files, in target then name order. root is unused for
// the rendering (the content is embedded) and kept for symmetry with the
// other spec builders.
func skillOwnedFiles(root string) []wiring.OwnedFile {
	_ = root
	var out []wiring.OwnedFile
	for _, target := range []string{skillsTargetClaude, skillsTargetAgents} {
		for _, name := range skillsdist.Names {
			body, err := renderSkill(target, name)
			if err != nil {
				continue // embedded source; cannot fail at runtime
			}
			out = append(out, wiring.OwnedFile{
				File:   skillRelPath(target, name),
				Marker: skillMarker,
				Body:   string(body),
			})
		}
	}
	return out
}

// skillRelPath is the repo-relative, slash-separated path of one skill for
// one repo-scoped target.
func skillRelPath(target, name string) string {
	if target == skillsTargetClaude {
		return path.Join(".claude", "skills", name, "SKILL.md")
	}
	return path.Join(".agents", "skills", name, "SKILL.md")
}

// skillDriftFiles buckets the skill plan for one repo: missing (absent,
// will be created), stale (marker-owned, behind the source, will be
// rewritten), blocked ("FILE — why", never written). Sorted.
func skillDriftFiles(root string) (missing, stale, blocked []string) {
	plan, err := wiring.PlanWiring(root, wiring.Spec{Files: skillOwnedFiles(root)})
	if err != nil {
		return nil, nil, nil
	}
	for _, it := range plan.Items {
		switch it.Verdict {
		case wiring.VerdictApply:
			if it.Present {
				stale = append(stale, it.File)
			} else {
				missing = append(missing, it.File)
			}
		case wiring.VerdictBlocked:
			blocked = append(blocked, it.File+" — "+it.Reason)
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	sort.Strings(blocked)
	return missing, stale, blocked
}

// skillDirsSummary names the skill directories a list of skill files spans
// (".agents/skills, .claude/skills"), for one-line dry-run reporting.
func skillDirsSummary(files []string) string {
	seen := map[string]bool{}
	var dirs []string
	for _, f := range files {
		d := path.Dir(path.Dir(f))
		if !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}
	sort.Strings(dirs)
	return strings.Join(dirs, ", ")
}
