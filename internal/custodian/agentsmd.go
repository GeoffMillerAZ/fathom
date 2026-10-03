package custodian

// agentsmd.go — AG-10 (docs/plans/agents-md.md, ADR-059): the
// agents_md_reaches_claude kind, the repository-scoped half of "will Claude
// Code load this repo's AGENTS.md". Claude Code (≥2.1.277) loads AGENTS.md
// natively under its default agents-md mode only when no CLAUDE.md,
// .claude/CLAUDE.md or CLAUDE.local.md exists; otherwise the first present
// one must import it. The kind holds when AGENTS.md exists and either no
// root CLAUDE file exists, or the first present one carries an import that
// resolves to the root AGENTS.md outside a code fence, or is that AGENTS.md
// itself (a link, so it carries AGENTS.md's text). With
// `allow_import: false` any CLAUDE file in the agent-config corpus (and a
// root CLAUDE.local.md, which git usually ignores) is a violation: one
// instruction file, no import. Test fixtures under a testdata/ directory
// are input, not instruction files, and are not counted.
//
// The half that is not the repository's — an ancestor CLAUDE file, the
// user or managed agents-md setting, the Claude Code version — is doctor's
// (cmd/fathom claudeLoadPath). File-local, so the FileChanged
// `config audit --hook` catches `/init` writing a CLAUDE.md mid-session.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// KindAgentsMDReachesClaude is the AG-10 check kind.
const KindAgentsMDReachesClaude CheckKind = "agents_md_reaches_claude"

// rootClaudeFiles are Claude Code's CLAUDE files at a repository root, in
// the precedence the first-present rule reads them.
var rootClaudeFiles = []string{"CLAUDE.md", ".claude/CLAUDE.md", "CLAUDE.local.md"}

// LinksToRootAgentsMD reports whether the file at rel (repo-relative) and
// the root AGENTS.md are one file — a symlink either way, or a hard link —
// so Claude Code reading rel reads AGENTS.md's own text with no import.
// Either side absent or unstat-able: false.
func LinksToRootAgentsMD(root, rel string) bool {
	a, err := os.Stat(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		return false
	}
	c, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return false
	}
	return os.SameFile(a, c)
}

// ImportsRootAgentsMD reports the 1-based line of the first import in
// content (the file at rel, repo-relative) that resolves to the root
// AGENTS.md outside a code fence and outside inline code; 0 when none does.
// A fathom-marked import region counts: it is an import like any other.
func ImportsRootAgentsMD(content, rel string) int {
	dir := path.Dir(filepath.ToSlash(rel))
	fenced := false
	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		for _, tok := range strings.Fields(line) {
			if !strings.HasPrefix(tok, "@") || strings.Contains(tok, "`") {
				continue
			}
			if path.Clean(path.Join(dir, strings.TrimPrefix(tok, "@"))) == "AGENTS.md" {
				return i + 1
			}
		}
	}
	return 0
}

func checkAgentsMDReachesClaude(root string, c Concern) Finding {
	f := Finding{Concern: c}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			f.Status, f.Detail, f.Evidence = StatusViolated, "no AGENTS.md: there is nothing for Claude Code to load", []string{"AGENTS.md"}
			return f
		}
		f.Status, f.Detail = StatusUnable, fmt.Sprintf("AGENTS.md: %v", err) // I38
		return f
	}

	if c.Check.ImportOnly {
		return checkAgentsMDImportOnly(root, f)
	}

	if c.Check.AllowImport != nil && !*c.Check.AllowImport {
		var claude []string
		files, err := AgentConfigFiles(root)
		if err != nil {
			f.Status, f.Detail = StatusUnable, fmt.Sprintf("agent-config discovery failed: %v", err)
			return f
		}
		for _, rel := range files {
			if path.Base(rel) == "CLAUDE.md" && !underTestdata(rel) {
				claude = append(claude, rel)
			}
		}
		if _, err := os.Stat(filepath.Join(root, "CLAUDE.local.md")); err == nil {
			claude = append(claude, "CLAUDE.local.md")
		}
		f.Files = files
		if len(claude) > 0 {
			f.Status = StatusViolated
			f.Detail = fmt.Sprintf("%d CLAUDE file(s) where AGENTS.md is the one instruction file (allow_import: false): %s", len(claude), strings.Join(claude, ", "))
			f.Evidence = claude
			return f
		}
		f.Status, f.Detail = StatusHolds, "AGENTS.md is the only instruction file; Claude Code loads it natively (≥2.1.277, default mode)"
		return f
	}

	for _, rel := range rootClaudeFiles {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		f.Files = []string{rel}
		if err == nil && LinksToRootAgentsMD(root, rel) {
			f.Status = StatusHolds
			f.Detail = fmt.Sprintf("%s is AGENTS.md itself (a link): Claude Code reads AGENTS.md through it", rel)
			f.Evidence = []string{rel}
			return f
		}
		if err != nil {
			f.Status, f.Detail = StatusUnable, fmt.Sprintf("%s: %v — whether it imports AGENTS.md cannot be read", rel, err)
			return f
		}
		if line := ImportsRootAgentsMD(string(data), rel); line > 0 {
			f.Status = StatusHolds
			f.Detail = fmt.Sprintf("%s imports AGENTS.md", rel)
			f.Evidence = []string{fmt.Sprintf("%s:%d", rel, line)}
			return f
		}
		f.Status = StatusViolated
		f.Detail = fmt.Sprintf("%s exists without an @AGENTS.md import, so Claude Code's default mode loads it instead of AGENTS.md", rel)
		f.Evidence = []string{rel}
		return f
	}
	f.Status, f.Detail = StatusHolds, "no root CLAUDE file: Claude Code loads AGENTS.md natively (≥2.1.277, default mode)"
	return f
}

// checkAgentsMDImportOnly evaluates `import_only: true` (CS-5, ADR-059
// amendment 2026-10-02): a CLAUDE.md is admitted only when its whole content
// is the `@AGENTS.md` import of the AGENTS.md beside it, so it can never
// become a second place for instructions; `.claude/CLAUDE.md` and
// `CLAUDE.local.md` are refused outright. A root CLAUDE.md puts Claude Code
// in CLAUDE.md mode (its default claude-md-or-agents-md setting reads only
// CLAUDE files once one exists in the working directory or above), where a
// nested AGENTS.md loads on demand only through a CLAUDE.md beside it — so
// each nested AGENTS.md then needs its import, or that guide silently stops
// loading. Test fixtures under testdata/ are input and are not counted.
func checkAgentsMDImportOnly(root string, f Finding) Finding {
	files, err := AgentConfigFiles(root)
	if err != nil {
		f.Status, f.Detail = StatusUnable, fmt.Sprintf("agent-config discovery failed: %v", err)
		return f
	}
	f.Files = files
	var claude, agents []string
	for _, rel := range files {
		if underTestdata(rel) {
			continue
		}
		switch path.Base(rel) {
		case "CLAUDE.md":
			claude = append(claude, rel)
		case "AGENTS.md":
			agents = append(agents, rel)
		}
	}
	var bad, evidence []string
	for _, rel := range claude {
		dir := path.Dir(rel)
		if path.Base(dir) == ".claude" {
			bad = append(bad, rel+": only a plain CLAUDE.md beside its AGENTS.md may carry the import")
			evidence = append(evidence, rel)
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			f.Status, f.Detail = StatusUnable, fmt.Sprintf("%s: %v — whether it is only the import cannot be read", rel, err) // I38
			return f
		}
		if strings.TrimSpace(string(data)) != "@AGENTS.md" {
			bad = append(bad, rel+" carries more than the @AGENTS.md import")
			evidence = append(evidence, rel)
			continue
		}
		sibling := path.Join(dir, "AGENTS.md")
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(sibling))); err != nil {
			bad = append(bad, fmt.Sprintf("%s imports %s, which does not exist", rel, sibling))
			evidence = append(evidence, rel)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "CLAUDE.local.md")); err == nil {
		bad = append(bad, "CLAUDE.local.md: a local CLAUDE file carries instructions nobody else loads")
		evidence = append(evidence, "CLAUDE.local.md")
	}
	if contains(claude, "CLAUDE.md") {
		for _, rel := range agents {
			dir := path.Dir(rel)
			if dir == "." || contains(claude, path.Join(dir, "CLAUDE.md")) {
				continue
			}
			bad = append(bad, rel+" has no CLAUDE.md beside it: under the root CLAUDE.md, Claude Code loads a nested AGENTS.md only through an @AGENTS.md import next to it")
			evidence = append(evidence, rel)
		}
	}
	if len(bad) > 0 {
		f.Status = StatusViolated
		f.Detail = fmt.Sprintf("%d problem(s) where a CLAUDE.md may be only the @AGENTS.md import (import_only): %s", len(bad), strings.Join(bad, "; "))
		f.Evidence = evidence
		return f
	}
	if len(claude) == 0 {
		f.Status, f.Detail = StatusHolds, "no CLAUDE file: Claude Code loads AGENTS.md natively (≥2.1.277, default mode)"
		return f
	}
	f.Status = StatusHolds
	f.Detail = fmt.Sprintf("%d CLAUDE file(s), each only the @AGENTS.md import of the AGENTS.md beside it (%s); every AGENTS.md reaches Claude Code through one", len(claude), strings.Join(claude, ", "))
	f.Evidence = claude
	return f
}

// underTestdata reports whether a repo-relative path sits under a testdata
// directory — test input, never an instruction file.
func underTestdata(rel string) bool {
	for _, seg := range strings.Split(rel, "/") {
		if seg == "testdata" {
			return true
		}
	}
	return false
}
