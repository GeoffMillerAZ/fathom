package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
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
// what makes `--skills[=claude|codex|all]`'s bracket-optional syntax work
// with the standard library's flag package.
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

// resolveSkillsMode normalizes --skills' raw flag value to "" (flag
// omitted — skip), "claude", "codex", or "all". A bare `--skills` reaches
// here as "true" (optionalStringFlag.IsBoolFlag's default), which this maps
// to "all" — the useful default when a caller doesn't know or care which
// harness is in play, rather than pretending to detect an environment this
// command has no reliable signal for.
func resolveSkillsMode(raw string) (string, error) {
	switch raw {
	case "":
		return "", nil
	case "true":
		return "all", nil
	case "claude", "codex", "all":
		return raw, nil
	default:
		return "", fmt.Errorf("invalid --skills value %q (want claude, codex, or all)", raw)
	}
}

// installSkills renders the shipped dist/agents/skills/ sources into the
// selected harness location(s). mode is a value already normalized by
// resolveSkillsMode ("" is a no-op). root is the target repo (used for the
// Claude Code shape, .claude/skills/<name>/SKILL.md); the Codex shape is
// user-global regardless of root — ~/.codex/prompts/<name>.md is the only
// location Codex's custom-prompts convention documents (no project-scoped
// variant; see any shipped codex-prompt.md's own header for the newer
// .agents/skills/ mechanism this falls back to documenting, not writing,
// since it isn't reliably verifiable from this repo).
func installSkills(root, mode string, stdout io.Writer) error {
	if mode == "" {
		return nil
	}
	if mode == "claude" || mode == "all" {
		for _, name := range skillsdist.Names {
			content, err := skillsdist.FS.ReadFile(name + "/SKILL.md")
			if err != nil {
				return fmt.Errorf("reading embedded skill %q: %w", name, err)
			}
			dest := filepath.Join(root, ".claude", "skills", name, "SKILL.md")
			if err := installSkillFile(dest, content, stdout); err != nil {
				return err
			}
		}
	}
	if mode == "codex" || mode == "all" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolving home directory for Codex prompts: %w", err)
		}
		for _, name := range skillsdist.Names {
			content, err := skillsdist.FS.ReadFile(name + "/codex-prompt.md")
			if err != nil {
				return fmt.Errorf("reading embedded codex prompt %q: %w", name, err)
			}
			dest := filepath.Join(home, ".codex", "prompts", name+".md")
			if err := installSkillFile(dest, content, stdout); err != nil {
				return err
			}
		}
	}
	return nil
}

// installSkillFile writes content to destPath, honoring the fathom-owned
// marker convention: create if absent, overwrite if the existing file
// carries skillMarker (no-op if the bytes are already identical), and
// refuse loudly — an error, not a silent skip — if a different,
// non-fathom-owned file already lives there.
func installSkillFile(destPath string, content []byte, stdout io.Writer) error {
	existing, err := os.ReadFile(destPath)
	switch {
	case err == nil:
		if !strings.Contains(string(existing), skillMarker) {
			return fmt.Errorf("refusing to overwrite %s: a file already exists there and is not fathom-owned (no %q marker) — rename/remove it, or this skill name collides with one of yours", destPath, skillMarker)
		}
		if string(existing) == string(content) {
			fmt.Fprintf(stdout, "ok %s (already current)\n", destPath)
			return nil
		}
		if err := os.WriteFile(destPath, content, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "updated %s\n", destPath)
		return nil
	case os.IsNotExist(err):
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(destPath, content, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "created %s\n", destPath)
		return nil
	default:
		return err
	}
}
