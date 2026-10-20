package langbench

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// TrackerMarker is where the generated section goes in the template.
const TrackerMarker = "{{LANGBENCH_GENERATED}}"

// trackerTemplate is docs/languages.md's hand-kept part (design §12: the
// preamble and the typed-edge prose are kept as a template).
//
//go:embed languages.tmpl.md
var trackerTemplate string

// Registration says a queue item's language or format is registered today.
type Registration struct {
	Extractor string // the extractor or dialect directory that reads it
	LangTag   string // connector language tag, for the typed-edge producer
}

// TrackerInput is everything the tracker renders from.
type TrackerInput struct {
	Template   string
	Queue      []QueueEntry
	Readings   map[string]Reading
	Registered map[string]Registration
	// Refused are the queue items whose plan row, or its -rest sibling, is
	// [-] refused (design §14.2): queue id -> "<row>: <reason>". A refused
	// item claims no level whatever its reading measured, and its dialect,
	// even when built, is not dispatched: the language stays disclosed.
	Refused    map[string]string
	TypedEdges func(langTag string) string
	// Dispatch is fusion's per-key extractor dispatch (fusion.DispatchTable):
	// extension or basename -> the label of the extractor serving it. The
	// template's "Engine / dispatch" cells are `{{dispatch <key>...}}`
	// placeholders expanded from it (LB-F356), so no cell hand-types which
	// extractor handles a language.
	Dispatch map[string]string
}

// Dispatched is Registered without the refused items: the extractors and
// dialects ingest actually routes files to, which is what the
// unsupported-language table (LB-F25) is rendered against.
func (in TrackerInput) Dispatched() map[string]Registration {
	out := make(map[string]Registration, len(in.Registered))
	for id, r := range in.Registered {
		if _, refused := in.Refused[id]; !refused {
			out[id] = r
		}
	}
	return out
}

// RefusedItems reads the refused queue items from the plan (design §14.2):
// an item is refused when its own row or its LB-Q-<id>-rest sibling is [-]
// with disposition refused. The value names the row and its reason.
func RefusedItems(q []QueueEntry, plan verify.Plan) map[string]string {
	rows := planRows(plan)
	out := map[string]string{}
	for _, e := range q {
		for _, id := range []string{e.PlanRow, e.PlanRow + "-rest"} {
			if t, ok := rows[id]; ok && t.Skipped && t.Disposition == "refused" {
				out[e.ID] = id + ": " + t.Reason
				break
			}
		}
	}
	return out
}

// registeredFormats are the tool formats and languages registered today
// whose extractors have no incumbent bench spec (no tier-B oracle exists
// for them): queue id -> extractor.
var registeredFormats = map[string]Registration{
	"kubernetes":     {Extractor: "k8sextract.go", LangTag: "yaml"},
	"helm":           {Extractor: "helmextract.go", LangTag: "yaml"},
	"markdown":       {Extractor: "mdextract.go", LangTag: "md"},
	"templ":          {Extractor: "templextract.go", LangTag: "templ"},
	"json-yaml-toml": {Extractor: "jsonextract.go, tomlextract.go", LangTag: "json"},
}

// TrackerInputFromRepo gathers the tracker's inputs from a checkout: the
// queue, every committed reading (docs/bench/readings/lang/<id>.json), and
// the registered dialects (incumbents, registered formats, and dialect
// directories).
func TrackerInputFromRepo(root string) (TrackerInput, error) {
	q, err := LoadQueue(filepath.Join(root, "docs", "plans", "language-breadth.queue.json"))
	if err != nil {
		return TrackerInput{}, err
	}
	// The plan carries the refusals (§14.2); a checkout without it (a
	// fixture) has none to report.
	plan, err := LoadBreadthPlan(root)
	if err != nil {
		return TrackerInput{}, err
	}
	in := TrackerInput{Template: trackerTemplate, Queue: q, Readings: map[string]Reading{},
		Registered: map[string]Registration{}, Refused: RefusedItems(q, plan),
		TypedEdges: connector.TypedEdgeProducer, Dispatch: fusion.DispatchTable()}
	for id, reg := range registeredFormats {
		in.Registered[id] = reg
	}
	specs, err := Incumbents()
	if err != nil {
		return TrackerInput{}, err
	}
	for _, s := range specs {
		in.Registered[s.Manifest.ID] = Registration{Extractor: "incumbent", LangTag: s.Manifest.LangTag}
	}
	manifests, _ := filepath.Glob(filepath.Join(root, "internal", "extractor", "dialects", "*", "dialect.json"))
	for _, p := range manifests {
		b, err := os.ReadFile(p)
		if err != nil {
			return TrackerInput{}, err
		}
		m, err := ParseManifest(b)
		if err != nil {
			return TrackerInput{}, fmt.Errorf("%s: %w", p, err)
		}
		in.Registered[m.ID] = Registration{Extractor: "dialects/" + m.ID, LangTag: m.LangTag}
	}
	readings, _ := filepath.Glob(filepath.Join(root, "docs", "bench", "readings", "lang", "*.json"))
	for _, p := range readings {
		r, err := LoadReading(p)
		if err != nil {
			return TrackerInput{}, err
		}
		in.Readings[r.Dialect] = r
	}
	return in, nil
}

// LoadReading reads a committed reading strictly and validates it.
func LoadReading(path string) (Reading, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Reading{}, err
	}
	var r Reading
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return Reading{}, fmt.Errorf("%s: %w", path, err)
	}
	if err := r.Validate(); err != nil {
		return Reading{}, fmt.Errorf("%s: %w", path, err)
	}
	if want := strings.TrimSuffix(filepath.Base(path), ".json"); r.Dialect != want {
		return Reading{}, fmt.Errorf("%s: holds dialect %q", path, r.Dialect)
	}
	return r, nil
}

func levelIndex(level string) int { return slices.Index(Levels, level) } // -1 for none

func cell(s string) string {
	if s == "" {
		return "—"
	}
	return strings.ReplaceAll(s, "|", "\\|")
}

// levelCell renders a reading's level with every met level's grade.
func levelCell(r Reading) string {
	if r.LevelMet == LevelNone {
		return "none (measured)"
	}
	var grades []string
	for _, l := range Levels[:levelIndex(r.LevelMet)+1] {
		grades = append(grades, l+" "+r.GradeAt(l))
	}
	return fmt.Sprintf("%s (%s)", r.LevelMet, strings.Join(grades, ", "))
}

// RenderTracker renders docs/languages.md: the template with the generated
// excellence board and the support-level section in place of TrackerMarker
// (design §12).
func RenderTracker(in TrackerInput) string {
	var b strings.Builder
	b.WriteString(RenderBoard(in) + "\n")
	b.WriteString("## Support levels (generated)\n\n")
	b.WriteString("Generated by `go run ./cmd/indexbench lang-tracker > docs/languages.md` from the queue,\n")
	b.WriteString("the registered dialects and the committed readings; never hand-edit it. A level is the\n")
	b.WriteString("reading's computed `level_met` with each met level's grade; `tolerated` is registered with\n")
	b.WriteString("no reading yet; `unsupported (disclosed)` is queued, not registered, and named per service\n")
	b.WriteString("by `list_services`; `refused` is refused by its plan row (design §14.2): it claims no level,\n")
	b.WriteString("is not dispatched and stays disclosed, whatever its reading measured.\n\n")
	b.WriteString(MilestoneLine(CountMilestones(in)) + "\n\n")
	b.WriteString("| Language or format | Kind | Engine | Grammar / parser | Target | Level met | Oracles | Typed edges | Reading | Next |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|\n")
	for _, e := range in.Queue {
		kind := "language"
		if e.Kind == "tooling" {
			kind = "tool format"
		}
		level, oracles, reading, next := "unsupported (disclosed)", "", "", e.PlanRow
		reg, registered := in.Registered[e.ID]
		if registered {
			level = "tolerated"
		}
		why, refused := in.Refused[e.ID]
		if r, ok := in.Readings[e.ID]; ok {
			level = levelCell(r)
			if refused {
				level = "refused; measured " + level
			}
			var os []string
			for _, o := range r.Oracles {
				if o.Status == "" {
					os = append(os, strings.TrimSpace(o.Tier+" "+o.Tool+" "+o.Version))
				}
			}
			oracles = strings.Join(os, "; ")
			reading = r.MeasuredAt
			if levelIndex(r.LevelMet) >= levelIndex(e.TargetLevel) && !refused {
				next = "at target (" + e.PlanRow + ")"
			}
		} else if refused {
			level = "refused"
		}
		if refused {
			row, _, _ := strings.Cut(why, ":")
			next = "re-plan (" + row + " refused)"
		}
		typed := ""
		if registered && !refused && reg.LangTag != "" && in.TypedEdges != nil {
			typed = in.TypedEdges(reg.LangTag)
		}
		grammar := e.GrammarSource
		if e.Licence != "" {
			grammar += " (" + e.Licence + ")"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			cell(e.Name), kind, cell(e.Engine), cell(grammar), e.TargetLevel, level, cell(oracles), cell(typed), cell(reading), cell(next))
	}
	return strings.Replace(ExpandDispatch(in.Template, in.Dispatch), TrackerMarker, strings.TrimRight(b.String(), "\n"), 1)
}

// dispatchRE matches a template cell's placeholder: {{dispatch .java .jsx}}.
var dispatchRE = regexp.MustCompile(`\{\{dispatch ([^}]+)\}\}`)

// DispatchCell renders the dispatch cell for keys (extensions or basenames):
// the one label when every key shares it, else each label with its keys. A
// key the registry does not serve reads "no extractor registered".
func DispatchCell(table map[string]string, keys []string) string {
	var order []string
	byLabel := map[string][]string{}
	for _, k := range keys {
		l, ok := table[k]
		if !ok {
			l = "no extractor registered"
		}
		if _, seen := byLabel[l]; !seen {
			order = append(order, l)
		}
		byLabel[l] = append(byLabel[l], k)
	}
	if len(order) == 1 {
		return order[0]
	}
	parts := make([]string, 0, len(order))
	for _, l := range order {
		ks := make([]string, len(byLabel[l]))
		for i, k := range byLabel[l] {
			ks[i] = "`" + k + "`"
		}
		parts = append(parts, strings.Join(ks, " ")+" "+l)
	}
	return strings.Join(parts, "; ")
}

// ExpandDispatch replaces every {{dispatch <key>...}} placeholder in tmpl with
// its DispatchCell.
func ExpandDispatch(tmpl string, table map[string]string) string {
	return dispatchRE.ReplaceAllStringFunc(tmpl, func(m string) string {
		return DispatchCell(table, strings.Fields(dispatchRE.FindStringSubmatch(m)[1]))
	})
}

// MilestoneCounts are the vision 37 §5 milestone quantities.
type MilestoneCounts struct {
	L1Excellent, L1, L2, L3, L4, ToolsAtTarget int
}

// CountMilestones counts readings toward the milestones: languages by
// level met (L1 excellent needs the L1 grade), tool formats at target.
func CountMilestones(in TrackerInput) MilestoneCounts {
	var c MilestoneCounts
	for _, e := range in.Queue {
		r, ok := in.Readings[e.ID]
		if _, refused := in.Refused[e.ID]; !ok || refused {
			continue // no reading, or refused: no level claimed (§14.2)
		}
		lvl := levelIndex(r.LevelMet)
		if e.Kind == "tooling" {
			if lvl >= 0 && lvl >= levelIndex(e.TargetLevel) {
				c.ToolsAtTarget++
			}
			continue
		}
		if lvl >= 0 {
			c.L1++
			if r.Grade.L1 == GradeExcellent {
				c.L1Excellent++
			}
		}
		if lvl >= 1 {
			c.L2++
		}
		if lvl >= 2 {
			c.L3++
		}
		if lvl >= 3 {
			c.L4++
		}
	}
	return c
}

type milestone struct {
	name                                 string
	l1Excellent, l1, l2, l3, l4, toolsAt int
}

// milestones are vision 37 §5's table.
var milestones = []milestone{
	{"M1", 20, 0, 12, 8, 6, 15},
	{"M2", 35, 45, 25, 14, 10, 30},
	{"M3", 0, 60, 35, 20, 12, 40},
}

func (m milestone) reached(c MilestoneCounts) bool {
	return c.L1Excellent >= m.l1Excellent && c.L1 >= m.l1 && c.L2 >= m.l2 && c.L3 >= m.l3 && c.L4 >= m.l4 && c.ToolsAtTarget >= m.toolsAt
}

// MilestoneLine is the tracker's computed milestone lines: one "Milestone
// Mn reached" line per reached milestone, then the next one's progress.
func MilestoneLine(c MilestoneCounts) string {
	var lines []string
	for _, m := range milestones {
		if m.reached(c) {
			lines = append(lines, fmt.Sprintf("Milestone %s reached (languages at L1 %d, L1 excellent %d, L2 %d, L3 %d, L4 %d; tool formats at target %d)",
				m.name, c.L1, c.L1Excellent, c.L2, c.L3, c.L4, c.ToolsAtTarget))
			continue
		}
		var parts []string
		if m.l1 > 0 {
			parts = append(parts, fmt.Sprintf("at L1 %d/%d", c.L1, m.l1))
		}
		if m.l1Excellent > 0 {
			parts = append(parts, fmt.Sprintf("at L1 excellent %d/%d", c.L1Excellent, m.l1Excellent))
		}
		parts = append(parts, fmt.Sprintf("at L2 %d/%d", c.L2, m.l2), fmt.Sprintf("at L3 %d/%d", c.L3, m.l3), fmt.Sprintf("at L4 %d/%d", c.L4, m.l4))
		lines = append(lines, fmt.Sprintf("Milestone %s progress: languages %s; tool formats at target %d/%d", m.name, strings.Join(parts, ", "), c.ToolsAtTarget, m.toolsAt))
		break
	}
	return strings.Join(lines, "\n")
}

var planRowRE = regexp.MustCompile(`LB-Q-([a-z0-9-]+)`)

var trackerRowRE = regexp.MustCompile(`^\| (.+?) \| (language|tool format) \| .*? \| (L[1-4]) \| ([^|]+?) \|`)

// CheckTrackerAgainstReadings is I54 over a rendered or committed tracker:
// no generated row's level is above its reading's level_met, and a row
// without a reading claims no level.
func CheckTrackerAgainstReadings(doc string, readings map[string]Reading) []string {
	var errs []string
	inTable := false
	for _, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(line, "| Language or format |") {
			inTable = true
			continue
		}
		if !inTable || !strings.HasPrefix(line, "| ") {
			continue
		}
		m := trackerRowRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		id := ""
		if rm := planRowRE.FindStringSubmatch(line); rm != nil {
			id = rm[1]
		}
		claimed := strings.Fields(m[4])[0]
		if levelIndex(claimed) < 0 {
			continue // tolerated / unsupported / none: no level claimed
		}
		r, ok := readings[id]
		switch {
		case !ok:
			errs = append(errs, fmt.Sprintf("row %q claims %s with no reading", m[1], claimed))
		case levelIndex(claimed) > levelIndex(r.LevelMet):
			errs = append(errs, fmt.Sprintf("row %q claims %s above its reading's level_met %s", m[1], claimed, r.LevelMet))
		}
	}
	sort.Strings(errs)
	return errs
}

// dispatchHeader opens the template's per-language table.
const dispatchHeader = "| Language / dialect | Engine / dispatch |"

// handTypedDispatchRE finds a dispatch fact typed into a cell's prose: an
// extractor source file, a "dispatched" claim, or the old "heuristic" label.
var handTypedDispatchRE = regexp.MustCompile("(?i)extract\\.go|dialect\\.go|\\bdispatched\\b|\\bheuristic\\b|\\bpure Go\\b|tree-sitter \\(wasm")

// CheckDispatchColumn is LB-F357: every "Engine / dispatch" cell of the
// template's table must open with a dispatch placeholder whose keys the
// registry serves, and its prose may not restate which extractor handles the
// language. The same principle as TestMcpToolsDocMatchesRegistry: the doc
// carries no copy of a registry fact that can drift. It returns one message
// per violation, sorted.
func CheckDispatchColumn(tmpl string, table map[string]string) []string {
	var errs []string
	inTable := false
	for _, line := range strings.Split(tmpl, "\n") {
		if strings.HasPrefix(line, dispatchHeader) {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		if strings.HasPrefix(line, "|---") {
			continue
		}
		if !strings.HasPrefix(line, "| ") {
			break
		}
		cells := strings.Split(line, " | ")
		if len(cells) < 2 {
			continue
		}
		name, c := strings.TrimPrefix(cells[0], "| "), cells[1]
		m := dispatchRE.FindStringSubmatchIndex(c)
		if m == nil || m[0] != 0 {
			errs = append(errs, fmt.Sprintf("row %q: the dispatch cell does not open with a placeholder (hand-typed: %q)", name, c))
			continue
		}
		for _, k := range strings.Fields(c[m[2]:m[3]]) {
			if _, ok := table[k]; !ok {
				errs = append(errs, fmt.Sprintf("row %q: key %q is served by no extractor", name, k))
			}
		}
		if prose := c[m[1]:]; handTypedDispatchRE.MatchString(prose) {
			errs = append(errs, fmt.Sprintf("row %q: prose restates a dispatch fact: %q", name, prose))
		}
	}
	sort.Strings(errs)
	return errs
}
