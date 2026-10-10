package mcp

// localize_logtemplate.go — TL-4 (F67; ADR-064; docs/design/
// telemetry-links.md "Log templates (TL-4)"): a pasted log line, given as
// localize's existing `error` input, reaches the logger call whose
// template wrote it. The service's LogTemplate nodes (stored at ingest,
// internal/logtemplate) are matched against each line of the text; a line
// some template matches yields a typed log_template seed per best template
// and is consumed: it seeds nothing else, so no hop argument, why-line or
// answer carries any of it beyond the matched template's own text (ADR-058
// decision 3, as a payload's message fields are never read). No new tool,
// schema field or surface (law 4).

import (
	"fmt"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/logtemplate"
	"github.com/geoffmilleraz/fathom/internal/stackframe"
)

// localizeSeedLogTemplate is a seed read from a log line: the template a
// logger call writes it with (File, Line and API the call's).
const localizeSeedLogTemplate localizeSeedKind = "log_template"

// localizeLogTemplateScore weights the enclosing symbol of the call whose
// template a pasted line matches. Measured by the TL-4 replay
// (docs/bench/readings/telemetry/tl4-replay.md): above everything a log
// line alone can add to another candidate (a quoted literal's 5.0 grep hop,
// a name's 8, graph convergence's few points), below a trace's throw site
// (a frame's 10 plus its 20), so a pasted trace still names the code that
// threw.
const localizeLogTemplateScore = 25.0

// Bounds on the text matched: lines, and bytes per line.
const (
	localizeMaxLogLines     = 200
	localizeMaxLogLineBytes = 8192
)

// localizeLogTemplates compiles a service's LogTemplate nodes.
func localizeLogTemplates(nodes []core.SymbolNode) []*logtemplate.Template {
	var out []*logtemplate.Template
	for _, n := range nodes {
		f, ok := logtemplate.FromNode(n)
		if !ok {
			continue
		}
		if t, ok := logtemplate.Compile(f); ok {
			out = append(out, t)
		}
	}
	return out
}

// withoutLogTemplates drops LogTemplate nodes: a frame, a grep hit or a
// template resolves to the code symbol around it, never to a template.
func withoutLogTemplates(nodes []core.SymbolNode) []core.SymbolNode {
	out := make([]core.SymbolNode, 0, len(nodes))
	for _, n := range nodes {
		if n.Kind != logtemplate.Kind {
			out = append(out, n)
		}
	}
	return out
}

// localizeIsShapedPayload reports whether text is a payload of a
// registered shape (TL-1): its fields are frame tuples only and never read
// as log lines.
func localizeIsShapedPayload(text string) bool {
	r, ok := stackframe.ExtractPayload(text)
	return ok && len(r.Shapes) > 0
}

// localizeLogTemplateSeeds matches each line of text against ts. A line
// with a best match yields a seed per best template (ties all seeded, in
// file and line order) and is dropped from rest, the text the other seeds
// are read from. matched counts the consumed lines.
func localizeLogTemplateSeeds(text string, ts []*logtemplate.Template) (seeds []localizeSeed, rest string, matched int) {
	lines := strings.Split(text, "\n")
	var kept []string
	for i, ln := range lines {
		line := strings.TrimRight(ln, "\r")
		if i >= localizeMaxLogLines || len(line) > localizeMaxLogLineBytes || strings.TrimSpace(line) == "" {
			kept = append(kept, ln)
			continue
		}
		best, _ := logtemplate.Best(line, ts)
		if len(best) == 0 {
			kept = append(kept, ln)
			continue
		}
		matched++
		for _, t := range best {
			seeds = append(seeds, localizeSeed{Kind: localizeSeedLogTemplate, Value: t.Text, File: t.File, Line: t.Line, API: t.API})
		}
	}
	return dedupSeeds(seeds), strings.Join(kept, "\n"), matched
}

// localizeLogTemplateArg is a log_template hop's argument and why-line
// tail: the template's own text, its API and its call site. Never the line.
func localizeLogTemplateArg(s localizeSeed) string {
	return fmt.Sprintf("%q (%s) at %s:%d", s.Value, s.API, s.File, s.Line)
}

// localizeLogTemplateHop resolves a log_template seed to the symbol around
// its call.
func localizeLogTemplateHop(item localizeSeed, byFile map[string][]core.SymbolNode, upsert func(file, symbol, tier string, score float64, why string, distance int), addSeedKey func(string)) int {
	arg := localizeLogTemplateArg(item)
	file, syms, ok := findFileByPathSuffix(byFile, item.File)
	if !ok {
		file = item.File
	}
	if sym, found := enclosingSymbol(syms, item.Line, nil); ok && found {
		upsert(file, sym.Name, "typed", localizeLogTemplateScore, "log template "+arg, 0)
		addSeedKey(sym.Key())
		return 1
	}
	upsert(file, "", "heuristic", localizeLogTemplateScore, "log template "+arg+" (file; no enclosing symbol at that line)", 0)
	return 1
}
