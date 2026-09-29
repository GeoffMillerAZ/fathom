package semantic

// agentsmd.go — ADR-059 (docs/plans/agents-md.md AG-13..AG-15): the
// obligations layer reads AGENTS.md as a first-class instruction file.
//
//   - fathom's own stanza region (<!-- fathom:begin … --> … <!-- fathom:end
//     -->) is never compiled as a rule: it is fathom's text, and its truth is
//     the custodian's stanza_tools_served check, not an LLM restating it.
//   - A repository still carrying CLAUDE.md beside AGENTS.md (the two-copy
//     layout) compiles a byte-identical section once, keeping the AGENTS.md
//     copy — never two claim sets for one rule.
//   - `.claude/AGENTS.md` and `.claude/CLAUDE.md` are ROOT guides (Claude
//     Code reads them as the project's own), not a floor to `.claude/`; the
//     connector prunes `.claude/` (it holds worktrees and settings), so they
//     are read by one bounded stat each (RootGuideExtras).

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// fathom's own region markers, as the wiring engine writes them.
const (
	fathomRegionBeginPrefix = "<!-- fathom:begin"
	fathomRegionEnd         = "<!-- fathom:end -->"
)

// rootGuideExtraCap bounds one RootGuideExtras read.
const rootGuideExtraCap = 1 << 20

// isRootGuidePath reports whether a repo-relative path is a ROOT
// instruction file: a root CLAUDE.md, AGENTS.md, GEMINI.md or BUGBOT.md, or
// `.claude/CLAUDE.md` / `.claude/AGENTS.md`. scopeFloorDir, docCompileTier
// and isRootLevelSource share it, so the three cannot disagree about which
// guides govern the whole repository.
func isRootGuidePath(p string) bool {
	p = path.Clean(filepath.ToSlash(p))
	if instructionFileBases[p] {
		return true
	}
	dir, base := path.Split(p)
	return dir == ".claude/" && (base == "CLAUDE.md" || base == "AGENTS.md")
}

// blankFathomRegions returns lines with fathom's own well-formed region
// (begin marker line through end marker line) replaced by empty lines, so
// line numbers — document coordinates for every citation — are unchanged
// while no section carries the region's text. A corrupted pair is left as
// written: fathom does not guess at a broken region.
func blankFathomRegions(lines []string) []string {
	var out []string
	for i := 0; i < len(lines); i++ {
		if !strings.HasPrefix(strings.TrimSpace(lines[i]), fathomRegionBeginPrefix) {
			continue
		}
		end := -1
		for j := i + 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) == fathomRegionEnd {
				end = j
				break
			}
		}
		if end < 0 {
			return lines
		}
		if out == nil {
			out = append([]string(nil), lines...)
		}
		for k := i; k <= end; k++ {
			out[k] = ""
		}
		i = end
	}
	if out == nil {
		return lines
	}
	return out
}

// dedupeTwinGuideSections drops, per directory holding both a CLAUDE.md and
// an AGENTS.md, every CLAUDE.md section whose text is byte-identical to a
// section of the AGENTS.md beside it: the rule compiles once, from the
// AGENTS.md copy (ADR-059). Sections that differ are both kept.
func dedupeTwinGuideSections(all []docSections) []docSections {
	agentsDigests := map[string]map[string]bool{} // dir -> section text digests
	for _, ds := range all {
		dir, base := path.Split(path.Clean(ds.doc.Path))
		if base != "AGENTS.md" {
			continue
		}
		set := agentsDigests[dir]
		if set == nil {
			set = map[string]bool{}
			agentsDigests[dir] = set
		}
		for _, sec := range ds.sections {
			set[sec.Digest] = true
		}
	}
	if len(agentsDigests) == 0 {
		return all
	}
	out := make([]docSections, 0, len(all))
	for _, ds := range all {
		dir, base := path.Split(path.Clean(ds.doc.Path))
		twin := agentsDigests[dir]
		if base != "CLAUDE.md" || twin == nil {
			out = append(out, ds)
			continue
		}
		kept := make([]Section, 0, len(ds.sections))
		for _, sec := range ds.sections {
			if !twin[sec.Digest] {
				kept = append(kept, sec)
			}
		}
		out = append(out, docSections{doc: ds.doc, sections: kept})
	}
	return out
}

// corpusSections splits every doc of a governing corpus, in path order, and
// drops CLAUDE.md sections an AGENTS.md twin already carries — the one
// split the compile, the raw fallback and layer coverage all read, so the
// three count the same sections.
func corpusSections(corpus []core.SourceFile) []docSections {
	docs := append([]core.SourceFile(nil), corpus...)
	sort.Slice(docs, func(i, j int) bool { return docs[i].Path < docs[j].Path })
	all := make([]docSections, 0, len(docs))
	for _, doc := range docs {
		all = append(all, docSections{doc: doc, sections: SplitMarkdownSections(doc.Path, doc.Content)})
	}
	return dedupeTwinGuideSections(all)
}

// RootGuideExtras reads the corpus's literal include entries that sit under
// `.claude/` — `.claude/AGENTS.md`, `.claude/CLAUDE.md` — which the
// connector never indexes (it prunes `.claude/`), by one stat and one
// bounded read each from root. Entries already in have, excluded ones,
// absent or unreadable files, and files over the cap are skipped: this is
// a reader of guides, never a walk.
func RootGuideExtras(root string, include, exclude []string, have map[string]bool) []core.SourceFile {
	if root == "" {
		return nil
	}
	var out []core.SourceFile
	for _, inc := range include {
		if !strings.HasPrefix(inc, ".claude/") || strings.ContainsAny(inc, "*?[") || have[inc] {
			continue
		}
		if len(SelectCorpus([]core.SourceFile{{Path: inc}}, include, exclude)) == 0 {
			continue
		}
		abs := filepath.Join(root, filepath.FromSlash(inc))
		fi, err := os.Stat(abs)
		if err != nil || !fi.Mode().IsRegular() || fi.Size() > rootGuideExtraCap {
			continue
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		out = append(out, core.SourceFile{Path: inc, Content: data, Lang: connector.LanguageForPath(inc)})
	}
	return out
}
