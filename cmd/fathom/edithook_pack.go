package main

// edithook_pack.go -- FD-14 / FD-15: the pre-edit hook serves discovered
// tech packs' guards and the generated-file line below the repository's own
// rules (docs/design/fathom-dir.md sections 7.4 and 7.5). Data only: a
// pack's `regenerate` is rendered text and nothing here executes anything
// (I56 no fetch, I68 a pack never governs). Every failure is silence.

import (
	"fmt"
	"os"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/techpack"
)

// packLinesMax is how many lines one pack may serve at one edit (7.4).
const packLinesMax = 2

// packGroup is one source's candidate lines at one edit: the project's own
// generated declaration (tag "") or one discovered pack (tag name@version).
type packGroup struct {
	tag   string
	lines []string
	// over counts lines the per-pack cap already dropped.
	over int
}

// generatedServe is a generated line a pack or the project declared for the
// edited file, with its filled sources (the FD-22 join).
type generatedServe struct {
	output  string
	sources []string
}

// packExtras reads the project's generated declarations and every discovered
// pack's passing sections for the file being edited. nil groups: nothing to
// say, which is every repository without a pack or a declaration.
func packExtras(projDir, filePath string) (rel string, groups []packGroup, gen *generatedServe) {
	root := ""
	for _, cand := range serviceCandidates(projDir) {
		if r, ok := relativeToRoot(cand.root, filePath); ok {
			root, rel = cand.root, r
			break
		}
	}
	if root == "" {
		return "", nil, nil
	}
	if decls := projectGenerated(root); len(decls) > 0 {
		if line, srcs, ok := techpack.GeneratedLineFor(decls, rel); ok {
			groups = append(groups, packGroup{lines: []string{line}})
			gen = &generatedServe{output: rel, sources: srcs}
		}
	}
	d, err := techpack.Served(root)
	if err != nil {
		return rel, groups, gen
	}
	for _, pk := range d.Packs {
		if len(pk.Guards) == 0 && len(pk.Generated) == 0 {
			continue
		}
		reading, rerr := techpack.RunFixtures(pk.Dir)
		if rerr != nil {
			continue
		}
		served := map[string]bool{}
		for _, s := range reading.Sections {
			served[s.Section] = !s.Withheld
		}
		var lines []string
		if served[techpack.SectionGenerated] {
			if line, srcs, ok := techpack.GeneratedLineFor(pk.Generated, rel); ok {
				lines = append(lines, line)
				if gen == nil {
					gen = &generatedServe{output: rel, sources: srcs}
				}
			}
		}
		if served[techpack.SectionGuards] {
			for _, g := range pk.Guards {
				if packGuardCovers(g, rel) && strings.TrimSpace(g.Note) != "" {
					lines = append(lines, strings.TrimSpace(capWords("", strings.Fields(g.Note), " ", "word(s)")))
				}
			}
		}
		if len(lines) == 0 {
			continue
		}
		over := 0
		if len(lines) > packLinesMax {
			over, lines = len(lines)-packLinesMax, lines[:packLinesMax]
		}
		groups = append(groups, packGroup{tag: pk.Tag(), lines: lines, over: over})
	}
	return rel, groups, gen
}

func packGuardCovers(g techpack.Guard, rel string) bool {
	for _, s := range g.Scopes {
		if techpack.GlobMatch(s, rel) {
			return true
		}
	}
	return false
}

// projectGenerated is the repository's own `generated` declarations; a
// repository without a .fathom/ pays one stat.
func projectGenerated(root string) []techpack.Generated {
	if _, err := os.Stat(projectdir.Under(root)); err != nil {
		return nil
	}
	proj, err := projectdir.Load(root)
	if err != nil {
		return nil
	}
	k, ok := proj.Key("generated")
	if !ok || k.Err != nil {
		return nil
	}
	var gs []techpack.Generated
	if k.Value.Decode(&gs) != nil {
		return nil
	}
	return gs
}

// fitPackLines places the groups' lines into the slots I35 leaves after the
// repository's lines and the bound module rules, in group order. Lines that
// do not fit are counted per source.
func fitPackLines(groups []packGroup, slots int) (bullets []string, displaced []string) {
	if slots < 0 {
		slots = 0
	}
	for _, g := range groups {
		src := g.tag
		if src == "" {
			src = "project"
		}
		lost := g.over
		for _, l := range g.lines {
			if slots == 0 {
				lost++
				continue
			}
			slots--
			if g.tag != "" {
				l = "[" + g.tag + "] " + l
			}
			bullets = append(bullets, "• "+l)
		}
		if lost > 0 {
			displaced = append(displaced, fmt.Sprintf("%s %d", src, lost))
		}
	}
	return bullets, displaced
}

// displacedLine is the one cap-disclosure line: the repository's own count
// (semantic rules the daemon displaced) and the pack lines dropped, per source.
func displacedLine(rules int, packs []string) string {
	line := ""
	if rules > 0 {
		line = fmt.Sprintf("%d more eligible rule(s) not shown (cap)", rules)
	}
	if len(packs) > 0 {
		part := "pack line(s) not shown (cap): " + strings.Join(packs, ", ")
		if line != "" {
			line += "; "
		}
		line += part
	}
	return line
}

// packSlotsLeft is how many of I35's slots remain for pack lines.
func packSlotsLeft(intentLines, rules int) int {
	return semantic.MaxObligations - intentLines - rules
}

// servedGenerated is the generated declaration only when its line was among
// the bullets actually shown (a displaced line was not served).
func servedGenerated(bullets []string, gen *generatedServe) *generatedServe {
	if gen == nil {
		return nil
	}
	for _, b := range bullets {
		if strings.Contains(b, "`"+gen.output+"` is generated from ") {
			return gen
		}
	}
	return nil
}
