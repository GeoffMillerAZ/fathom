package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
	"github.com/geoffmilleraz/fathom/internal/wiring"
)

// skillMarker identifies a file `fathom init --skills` owns: a re-run
// overwrites ONLY files carrying this marker (a previous fathom render, or
// bytes already identical to the current one); a file at the same path
// WITHOUT it is a pre-existing, human-authored file that happens to share a
// skill's name, and installSkillFile refuses to touch it (AK-2,
// docs/plans/pack-taxonomy-adoption.md). Mirrors the CLAUDE.md/AGENTS.md
// stanza markers' spirit (fathomStanzaBegin/End, init.go) one level down:
// a single marker line rather than a wrapped block, since a whole skill
// file is fathom's or it isn't — there's no partial section to replace.
const skillMarker = "<!-- fathom:owned"

// optionalStringFlag is a flag.Value that also implements the informal
// "boolFlag" interface the flag package checks for (IsBoolFlag), so a bare
// `--skills` (no `=value`) is accepted and calls Set("true") instead of
// erroring "flag needs an argument" or consuming the next positional arg.
// `--skills=codex` etc. still calls Set with the literal value. This is
// what makes `--skills[=claude|codex|agents|all|codex-prompts]`'s
// bracket-optional syntax work with the standard library's flag package.
type optionalStringFlag struct {
	value string
	set   bool
}

func (f *optionalStringFlag) String() string { return f.value }
func (f *optionalStringFlag) Set(v string) error {
	f.set = true
	f.value = v
	return nil
}
func (f *optionalStringFlag) IsBoolFlag() bool { return true }

// skillsModes is the --skills vocabulary, in help order.
const skillsModes = "claude, codex, agents, all, or codex-prompts"

// resolveSkillsMode normalizes the --skills flag state to "" (flag
// omitted — skip) or one of the skillsModes. A bare `--skills` reaches
// here as value "true" (optionalStringFlag.IsBoolFlag's default), which this
// maps to "all" — the useful default when a caller doesn't know or care
// which harness is in play, rather than pretending to detect an environment
// this command has no reliable signal for.
//
// An explicit empty value (`--skills=`) is REFUSED, not treated as "all":
// set distinguishes it from the flag being omitted entirely (both reach
// here with value ""). Someone typing `--skills=` was mid-thought about
// which harness to name — silently installing both is the same class of
// surprise as the positional trap cmdInit guards against, so the ambiguity
// errors loudly instead.
func resolveSkillsMode(value string, set bool) (string, error) {
	if !set {
		return "", nil
	}
	switch value {
	case "true":
		return "all", nil
	case "claude", "codex", "agents", "all", "codex-prompts":
		return value, nil
	case "":
		return "", fmt.Errorf("empty --skills= value (want --skills=claude, --skills=codex, --skills=agents, --skills=all, --skills=codex-prompts, or bare --skills for all)")
	default:
		return "", fmt.Errorf("invalid --skills value %q (want %s)", value, skillsModes)
	}
}

// Skill render targets (RP-1, docs/plans/reach-parity.md). Each is one
// on-disk shape of the shipped skill pack; a --skills mode selects a fixed
// list of them.
const (
	// skillsTargetClaude is `.claude/skills/<name>/SKILL.md` — the Claude
	// Code superset frontmatter, verbatim from the embedded source.
	skillsTargetClaude = "claude"
	// skillsTargetAgents is `.agents/skills` — the Agent Skills standard
	// path (agentskills.io), the one Codex, Gemini CLI, Cursor, Copilot and
	// the rest read. Since ADR-059 init writes it as a link to
	// `.claude/skills` (agentsSkillsLink); renderSkill's spec-only rendering
	// (agentsVariant) for it still serves `fathom plugin render` and
	// recognizes an earlier fathom's real copy there.
	skillsTargetAgents = "agents"
	// skillsTargetCodexPrompts is `~/.codex/prompts/<name>.md` — Codex's
	// DEPRECATED custom-prompts twin. The only user-global write, and only
	// behind the explicit `--skills=codex-prompts`.
	skillsTargetCodexPrompts = "codex-prompts"
)

// skillsTargetsFor maps a --skills mode (already normalized by
// resolveSkillsMode) to the render targets it writes, in a fixed order.
// `.claude/skills` holds the ONE real copy for every repo-scoped mode
// (ADR-059); skillsTargetAgents is `.agents/skills` as a relative link to
// it (agentsSkillsLink), planned only for a harness that reads
// `.agents/skills` and not `.claude/skills`:
//
//   - bare --skills / "all": `.claude/skills` and the `.agents/skills`
//     link — the pair covers 14 of the 15 surveyed harnesses (vision 25 §1),
//     and both are repo-scoped.
//   - "codex": `.claude/skills` and the link — `.agents/skills` is the path
//     Codex documents for skills. The legacy `~/.codex/prompts` twin is NOT
//     written here any more (0.8.0 Breaking:
//     docs/upgrades/0.8.0-skills-on-the-standard.md).
//   - "agents": `.claude/skills` and the link.
//   - "claude": `.claude/skills` only — Claude Code discovers nothing else.
//   - "codex-prompts": the deprecated `~/.codex/prompts` twin, explicit only.
//   - "": nothing.
func skillsTargetsFor(mode string) []string {
	switch mode {
	case "all", "codex", "agents":
		return []string{skillsTargetClaude, skillsTargetAgents}
	case "claude":
		return []string{skillsTargetClaude}
	case "codex-prompts":
		return []string{skillsTargetCodexPrompts}
	default:
		return nil
	}
}

// agentsSkillsDir is where the Agent Skills standard path lives, and
// agentsSkillsLinkTarget what fathom's link there points at: relative, so
// a clone or a moved checkout keeps it.
const (
	agentsSkillsDir        = ".agents/skills"
	agentsSkillsLinkTarget = "../.claude/skills"
)

// agentsSkillsLink is the owned link `.agents/skills -> ../.claude/skills`
// (ADR-059, AG-8). Owned lists what an earlier fathom rendered there — the
// spec-only copy of every shipped skill under its marker — so a real
// directory holding only those files, none edited, is replaced by the link
// and anything else blocks it by name.
func agentsSkillsLink() wiring.Link {
	l := wiring.Link{Path: agentsSkillsDir, Target: agentsSkillsLinkTarget}
	for _, name := range skillsdist.Names {
		body, err := renderSkill(skillsTargetAgents, name)
		if err != nil {
			continue // embedded source; cannot fail at runtime
		}
		l.Owned = append(l.Owned, wiring.OwnedFile{File: skillRelPath(skillsTargetAgents, name), Marker: skillMarker, Body: string(body), Known: shippedSkillRenderDigests()})
	}
	return l
}

// skillDest is the on-disk path of one skill for one target. root is the
// target repo; home is the user's home (only the codex-prompts target uses
// it, and it is the caller's job to resolve it so tests never touch the
// real one).
func skillDest(root, home, target, name string) string {
	switch target {
	case skillsTargetClaude:
		return filepath.Join(root, ".claude", "skills", name, "SKILL.md")
	case skillsTargetAgents:
		return filepath.Join(root, ".agents", "skills", name, "SKILL.md")
	case skillsTargetCodexPrompts:
		return filepath.Join(home, ".codex", "prompts", name+".md")
	}
	return ""
}

// renderSkill returns the bytes one target receives for one shipped skill:
// the embedded SKILL.md verbatim for Claude Code, the spec-only
// agentsVariant for `.agents/skills`, and the codex-prompt.md twin for the
// deprecated prompts path.
func renderSkill(target, name string) ([]byte, error) {
	switch target {
	case skillsTargetCodexPrompts:
		content, err := skillsdist.FS.ReadFile(name + "/codex-prompt.md")
		if err != nil {
			return nil, fmt.Errorf("reading embedded codex prompt %q: %w", name, err)
		}
		return content, nil
	case skillsTargetClaude, skillsTargetAgents:
		content, err := skillsdist.FS.ReadFile(name + "/SKILL.md")
		if err != nil {
			return nil, fmt.Errorf("reading embedded skill %q: %w", name, err)
		}
		if target == skillsTargetAgents {
			return agentsVariant(name, content)
		}
		return content, nil
	}
	return nil, fmt.Errorf("unknown skills target %q", target)
}

// agentSkillsFields is the Agent Skills specification's closed frontmatter
// field set (agentskills.io/specification, source 2026-08-04; the same six
// the skills-ref validator and Claude's spec-only surfaces accept). Any
// other key is a Claude Code extension (`paths`, `hooks`, `context`, …)
// and is a hard error on a spec-only reader, so the `.agents/skills`
// rendering never carries one. schema/skills/schema.cue is the same set as
// a vetted schema.
//
//fathom:tenancy-allow pkg-map read-only lookup table of constants, never written after init: not a cache
var agentSkillsFields = map[string]bool{
	"name": true, "description": true, "license": true,
	"compatibility": true, "metadata": true, "allowed-tools": true,
}

var frontmatterKeyLine = regexp.MustCompile(`^([A-Za-z0-9_-]+):`)

// agentsVariant renders the `.agents/skills` shape of one shipped skill
// (RP-2): the YAML frontmatter is reduced to the Agent Skills spec fields —
// a top-level key outside agentSkillsFields is dropped with its
// continuation lines — and `name` is held equal to the directory name,
// which the spec requires ("must match the parent directory name"). The
// body is untouched, marker line included. A file without a leading
// frontmatter block is an error: there is no spec-conformant rendering of
// it.
func agentsVariant(dir string, content []byte) ([]byte, error) {
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], "\r") != "---" {
		return nil, fmt.Errorf("skill %q: SKILL.md does not start with a --- frontmatter line", dir)
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r") == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return nil, fmt.Errorf("skill %q: SKILL.md frontmatter is not closed with ---", dir)
	}
	var kept []string
	keep := false
	sawName := false
	for _, line := range lines[1:end] {
		if m := frontmatterKeyLine.FindStringSubmatch(line); m != nil {
			key := m[1]
			keep = agentSkillsFields[key]
			if key == "name" {
				sawName = true
				line = "name: " + dir
			}
		}
		if keep {
			kept = append(kept, line)
		}
	}
	if !sawName {
		kept = append([]string{"name: " + dir}, kept...)
	}
	var out bytes.Buffer
	out.WriteString("---\n")
	for _, l := range kept {
		out.WriteString(l)
		out.WriteString("\n")
	}
	out.WriteString("---\n")
	out.WriteString(strings.Join(lines[end+1:], "\n"))
	return out.Bytes(), nil
}

// skillsHomeDir resolves the home directory the codex-prompts target writes
// under. Only consulted when that target is selected, so a repo-scoped
// install never fails on an unresolvable home.
var skillsHomeDir = os.UserHomeDir

// installSkills renders the shipped dist/agents/skills/ sources into the
// targets skillsTargetsFor(mode) names. mode is a value already normalized
// by resolveSkillsMode ("" is a no-op). root is the target repo; the
// codex-prompts target is user-global regardless of root.
func installSkills(root, mode string, stdout io.Writer) error {
	_, err := installSkillsReport(root, mode, stdout)
	return err
}

// skillWrite is one file installSkills touched or left alone — the data
// `fathom init --json` reports (RP-15).
type skillWrite struct {
	Target string `json:"target"`
	Path   string `json:"path"`
	Action string `json:"action"` // created | updated | current | linked
}

// installSkillsReport is installSkills returning every write as data.
// The `.agents/skills` target is the owned link, planned through the wiring
// engine (receipted, `fleet undo` removes it) after the real copy exists;
// nothing is ever written through it.
//
// FA-23: every skill file it leaves at the shipped render — created,
// updated or already current — has its baselines recorded in root's
// .fathom/owned.json from the bytes on disk, after the writes and also when
// a refusal stops the loop part-way: this writer used to record none, so a
// later release's reconcile read fathom's own render as a hand edit.
func installSkillsReport(root, mode string, stdout io.Writer) (writes []skillWrite, err error) {
	var written []wiring.SkillFile
	defer func() {
		if len(written) == 0 {
			return
		}
		recorded, rerr := wiring.RecordWritten(root, written, time.Now().UTC().Format(time.RFC3339))
		if rerr != nil && err == nil {
			err = fmt.Errorf("recording the skills' baselines in .fathom/owned.json: %w", rerr)
		}
		if len(recorded) > 0 {
			fmt.Fprintf(stdout, "recorded baselines for %d skill file(s) in .fathom/owned.json\n", len(recorded))
		}
	}()
	for _, target := range skillsTargetsFor(mode) {
		if target == skillsTargetAgents {
			w, err := installAgentsSkillsLink(root, stdout)
			if err != nil {
				return writes, err
			}
			writes = append(writes, w)
			continue
		}
		home := ""
		if target == skillsTargetCodexPrompts {
			h, err := skillsHomeDir()
			if err != nil {
				return writes, fmt.Errorf("resolving home directory for Codex prompts: %w", err)
			}
			home = h
		}
		for _, name := range skillsdist.Names {
			content, err := renderSkill(target, name)
			if err != nil {
				return writes, err
			}
			dest := skillDest(root, home, target, name)
			action, err := installSkillFile(dest, content, stdout)
			if err != nil {
				return writes, err
			}
			writes = append(writes, skillWrite{Target: target, Path: dest, Action: action})
			label := skillRelPath(target, name)
			if target == skillsTargetCodexPrompts {
				label = "~/.codex/prompts/" + name + ".md"
			}
			written = append(written, wiring.SkillFile{Path: dest, Label: label, Shipped: content})
		}
	}
	return writes, nil
}

// installAgentsSkillsLink plans and applies the `.agents/skills` link. A
// BLOCKED link (a real directory with entries fathom did not write, a link
// to elsewhere, a regular file) is refused loudly, exactly as a colliding
// skill file is.
func installAgentsSkillsLink(root string, stdout io.Writer) (skillWrite, error) {
	dest := filepath.Join(root, filepath.FromSlash(agentsSkillsDir))
	res, err := wiring.ApplyWiring(root, wiring.Spec{Links: []wiring.Link{agentsSkillsLink()}}, wiring.ApplyOptions{Now: time.Now})
	if err != nil {
		return skillWrite{}, fmt.Errorf("linking %s: %w", dest, err)
	}
	if len(res.Blocked) > 0 {
		return skillWrite{}, fmt.Errorf("refusing to link %s -> %s: %s", dest, agentsSkillsLinkTarget, res.Blocked[0].Reason)
	}
	if len(res.Applied) > 0 {
		fmt.Fprintf(stdout, "linked %s (-> %s)\n", dest, agentsSkillsLinkTarget)
		return skillWrite{Target: skillsTargetAgents, Path: dest, Action: "linked"}, nil
	}
	fmt.Fprintf(stdout, "ok %s (already a link to %s)\n", dest, agentsSkillsLinkTarget)
	return skillWrite{Target: skillsTargetAgents, Path: dest, Action: "current"}, nil
}

// installSkillFile writes content to destPath, honoring the fathom-owned
// marker convention: create if absent, overwrite if the existing file
// carries skillMarker (no-op if the bytes are already identical), and
// refuse loudly — an error, not a silent skip — if a different,
// non-fathom-owned file already lives there. It returns the action taken
// (created | updated | current).
func installSkillFile(destPath string, content []byte, stdout io.Writer) (string, error) {
	existing, err := os.ReadFile(destPath)
	switch {
	case err == nil:
		if !strings.Contains(string(existing), skillMarker) {
			return "", fmt.Errorf("refusing to overwrite %s: a file already exists there and is not fathom-owned (no %q marker) — rename/remove it, or this skill name collides with one of yours", destPath, skillMarker)
		}
		if string(existing) == string(content) {
			fmt.Fprintf(stdout, "ok %s (already current)\n", destPath)
			return "current", nil
		}
		if err := os.WriteFile(destPath, content, 0o644); err != nil {
			return "", err
		}
		fmt.Fprintf(stdout, "updated %s\n", destPath)
		return "updated", nil
	case os.IsNotExist(err):
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(destPath, content, 0o644); err != nil {
			return "", err
		}
		fmt.Fprintf(stdout, "created %s\n", destPath)
		return "created", nil
	default:
		return "", err
	}
}
