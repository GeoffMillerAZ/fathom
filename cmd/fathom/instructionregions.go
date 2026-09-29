package main

// instructionregions.go — ADR-059 (docs/plans/agents-md.md AG-2..AG-4): the
// one seam every writer of instruction files plans through — `fathom init`
// for every --harness, `fleet reconcile --apply`, the drift report and the
// console's apply.
//
//   - The stanza region goes into AGENTS.md, for every harness. AGENTS.md is
//     created when absent. No path creates a CLAUDE.md.
//   - The first present root CLAUDE file (CLAUDE.md, .claude/CLAUDE.md,
//     CLAUDE.local.md — Claude Code's own order) carries at most one
//     fathom-marked region whose body is the import line: `@AGENTS.md`, or
//     `@../AGENTS.md` from .claude/, because Claude Code resolves an import
//     relative to the importing file. Its presence would otherwise switch
//     AGENTS.md off under Claude Code's default agents-md mode.
//   - A CLAUDE file whose region still holds the old stanza is rewritten in
//     place to the import (the wiring engine's ordinary out-of-date
//     upgrade, receipted, `fleet undo` restores it; a hand edit blocks).
//   - When an import line the adopter wrote already sits outside fathom's
//     region, fathom plans the REMOVAL of its own region (Region.Remove) —
//     one import, never two — and writes none where it has none.

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/wiring"
)

// agentsMDFile is the one file the stanza region goes into (ADR-059).
const agentsMDFile = harness.AgentsMD

// claudeFiles are the root CLAUDE files in Claude Code's precedence order —
// the claude-code descriptor's import files.
func claudeFiles() []string {
	d, _ := harness.Lookup(harness.ClaudeCode)
	return d.ImportFiles()
}

// rootClaudeFile is the first present root CLAUDE file, "" when none is.
// A path that exists but cannot be stat'ed as a file counts as present:
// fathom will not guess past a file it cannot read (the plan names it).
func rootClaudeFile(root string) string {
	for _, f := range claudeFiles() {
		fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(f)))
		if err == nil && !fi.IsDir() {
			return f
		}
		if err != nil && !os.IsNotExist(err) {
			return f
		}
	}
	return ""
}

// claudeImportLine is the import a CLAUDE file at file (repo-relative)
// carries to reach the root AGENTS.md.
func claudeImportLine(file string) string {
	dir := path.Dir(file)
	if dir == "." {
		return "@" + agentsMDFile
	}
	up := strings.Repeat("../", strings.Count(dir, "/")+1)
	return "@" + up + agentsMDFile
}

// hasUnfencedImport reports whether content (the CLAUDE file at file)
// carries, outside fathom's region and outside code fences and inline code,
// an import token that resolves to the root AGENTS.md.
func hasUnfencedImport(content, file string) bool {
	body := content
	if start := strings.Index(body, fathomStanzaBeginPrefix); start >= 0 {
		if end := strings.Index(body[start:], fathomStanzaEnd); end >= 0 {
			body = body[:start] + body[start+end+len(fathomStanzaEnd):]
		}
	}
	dir := path.Dir(file)
	fenced := false
	for _, line := range strings.Split(body, "\n") {
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
			if path.Clean(path.Join(dir, strings.TrimPrefix(tok, "@"))) == agentsMDFile {
				return true
			}
		}
	}
	return false
}

// claudeImportRegion is the region the first present root CLAUDE file
// gets: the import body, or — when an unfenced import already sits outside
// fathom's region — the removal of fathom's own region. ok is false when
// no root CLAUDE file exists: fathom never creates one.
func claudeImportRegion(root string) (wiring.Region, bool) {
	file := rootClaudeFile(root)
	if file == "" {
		return wiring.Region{}, false
	}
	r := wiring.Region{
		File:           file,
		Syntax:         wiring.HTMLMarkers,
		Version:        fathomStanzaVersion,
		Body:           claudeImportLine(file),
		AdoptUntracked: true,
	}
	if data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file))); err == nil && hasUnfencedImport(string(data), file) {
		r.Remove = true
	}
	return r, true
}

// instructionRegions plans the instruction regions for one repository: the
// stanza region in AGENTS.md (its body fitted to any router_thin budget on
// AGENTS.md, with id's hook sentences) and, when a root CLAUDE file
// exists, that file's import region. notes names what the fit trimmed.
func instructionRegions(root, id, service string, intentOn bool) ([]wiring.Region, []string) {
	regions, notes := stanzaRegionsFit(root, []string{agentsMDFile}, id, service, intentOn)
	if r, ok := claudeImportRegion(root); ok {
		regions = append(regions, r)
	}
	return regions, notes
}

// isStanzaBody reports whether a region body is a fathom stanza (as
// opposed to the one-line import): the template's own heading line.
func isStanzaBody(body string) bool {
	return strings.Contains(body, "## fathom code intelligence")
}

// isImportBody reports whether a region body is the one-line import (and
// not a stanza, or any other body an older fathom left in a CLAUDE file).
func isImportBody(body string) bool {
	b := strings.TrimSpace(body)
	return !strings.Contains(b, "\n") && strings.HasPrefix(b, "@") && strings.HasSuffix(b, agentsMDFile)
}
