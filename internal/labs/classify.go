package labs

import (
	"sort"
	"strings"
)

// Label is a decision point's eligibility verdict. A point is never
// guessed: a rule that cannot be decided from the retained projection
// leaves the point unclassified, with the reason it could not be read.
type Label string

const (
	ShouldFire   Label = "should-fire"
	ShouldNot    Label = "should-not"
	Unclassified Label = "unclassified"
)

// Reason is the fixed class naming the rule that decided a point, or the
// evidence the retained projection does not hold.
type Reason string

const (
	// ReasonBoundedReread: a Read of an explicit line range is the
	// re-reading of a cited range lab-read-source fires on.
	ReasonBoundedReread Reason = "bounded_reread"
	// ReasonOpenDiscovery: a whole-file Read is lab-read-source's
	// should-not (open discovery), its near miss.
	ReasonOpenDiscovery Reason = "open_discovery"
	// ReasonSubagentSpawn: lab-subagent-brief fires at every spawn, with
	// no rule to read (the card's should-not is deterministic).
	ReasonSubagentSpawn Reason = "subagent_spawn"
	// ReasonNoCompetingSurface: fathom ships no surface for this choice,
	// so no verb should fire at the point.
	ReasonNoCompetingSurface Reason = "no_competing_surface"
	// ReasonNeedsArgument: the rule needs the call's argument text. The
	// decoder keeps JSON shape classes only (I61), so v0 cannot read it.
	ReasonNeedsArgument Reason = "needs_argument"
	// ReasonNeedsDefinitionCheck: the rule needs a declaration scan of the
	// session's tree, which v0 does not run (owner ruling R1).
	ReasonNeedsDefinitionCheck Reason = "needs_definition_check"
	// ReasonNoPromptText: a skill's rule is a request inside its
	// description, and no prompt text is retained.
	ReasonNoPromptText Reason = "no_prompt_text"
	// ReasonHookSeam: the hook labs read AD-20's eligible log, which is
	// not built (owner ruling R3).
	ReasonHookSeam Reason = "hook_seam"
)

// mcpToolPrefix is Claude Code's MCP tool naming: mcp__<server>__<tool>.
const mcpToolPrefix = "mcp__"

// definitionCheckVerbs name the fathom verbs whose should-fire rule reads
// the repository tree's declarations rather than the call's argument.
var definitionCheckVerbs = map[string]bool{"blast_radius": true}

// substituteTools are the tools a missed fathom point is served by
// instead. Bash is unsplit: a `rg` inside a shell command is not
// distinguishable from any other command in the retained projection.
var substituteTools = map[string]bool{"Grep": true, "Glob": true, "Read": true, "Bash": true}

// editTools open the hook seams the obligations and guards labs measure.
var editTools = map[string]bool{"Edit": true, "Write": true, "MultiEdit": true, "NotebookEdit": true}

// ClassifyDecisionPoint reads one decision point against the eligibility
// table v0: the call's name, its input shape class and the calls around
// it. It never asks fathom and never guesses. Most verb points come back
// unclassified, each with the evidence v0 lacks; that is the honest
// reading, not a measured zero.
func ClassifyDecisionPoint(p Point) (Label, Reason) {
	call := p.Call
	if call.Kind == "subagent_start" {
		return ShouldFire, ReasonSubagentSpawn
	}
	if call.Kind == "skill" {
		return Unclassified, ReasonNoPromptText
	}
	verb, isVerb := mcpVerb(call.Name)
	// A file read naming a line range re-reads a cited range; one naming
	// no range is open discovery. Both read_source and Read take an
	// optional numeric window beside the path, and Shape carries the
	// argument types with no keys or values, so the pair is readable
	// without any text. This is the one surface whose own call and its
	// substitute are classified by the same rule, which is what makes
	// its recall a reading rather than an artefact of the framing.
	if call.Name == "Read" || isVerb && verb == "read_source" {
		if boundedRead(call.Shape) {
			return ShouldFire, ReasonBoundedReread
		}
		return ShouldNot, ReasonOpenDiscovery
	}
	if isVerb {
		if definitionCheckVerbs[verb] {
			return Unclassified, ReasonNeedsDefinitionCheck
		}
		return Unclassified, ReasonNeedsArgument
	}
	if editTools[call.Name] {
		return Unclassified, ReasonHookSeam
	}
	if substituteTools[call.Name] {
		return Unclassified, ReasonNeedsArgument
	}
	return ShouldNot, ReasonNoCompetingSurface
}

// boundedRead reports a file read whose arguments name a numeric window
// beside the path: "object{number,number,string}", read_source's
// "object{number,number,string,string}" and their one-bound forms.
func boundedRead(shape string) bool {
	return strings.HasPrefix(shape, "object{number,") && strings.HasSuffix(shape, "string}")
}

// RuleSurface is the shipped surface a decided rule concerns, or "" when
// the rule names no measurable surface. ReasonSubagentSpawn's lab is a
// hook surface, measured from AD-20's log rather than from calls, so it
// has no surface here (owner ruling R3).
func RuleSurface(r Reason) string {
	switch r {
	case ReasonBoundedReread, ReasonOpenDiscovery:
		return "mcp:read_source"
	}
	return ""
}

// UniversalShouldNot reports a should-not reason that holds for every
// shipped surface: at such a point no fathom verb should fire, so the
// point belongs to every surface's quiet margin.
func UniversalShouldNot(r Reason) bool {
	return r == ReasonNoCompetingSurface
}

// SurfaceKey is the shipped surface key a call names, built as the closure
// check builds it: "mcp:<tool>" with the mcp__<server>__ prefix stripped,
// "skill:<name>" with any plugin prefix stripped, else "" for a call that
// is no fathom surface.
func SurfaceKey(c Call) string {
	if c.Kind == "skill" {
		name := c.Name
		if i := strings.LastIndex(name, ":"); i >= 0 {
			name = name[i+1:]
		}
		if name == "" {
			return ""
		}
		return "skill:" + name
	}
	if verb, ok := mcpVerb(c.Name); ok {
		return "mcp:" + verb
	}
	return ""
}

// mcpVerb strips mcp__<server>__ from a tool name. A name with the prefix
// but no server segment is no verb.
func mcpVerb(name string) (string, bool) {
	if !strings.HasPrefix(name, mcpToolPrefix) {
		return "", false
	}
	rest := name[len(mcpToolPrefix):]
	i := strings.Index(rest, "__")
	if i <= 0 || i+2 >= len(rest) {
		return "", false
	}
	return rest[i+2:], true
}

// SubstituteTool is the substitute a missed fathom point was served by,
// or "" when the call is a fathom surface or no substitute at all.
func SubstituteTool(c Call) string {
	if SurfaceKey(c) != "" {
		return ""
	}
	if substituteTools[c.Name] {
		return c.Name
	}
	return ""
}

// ReasonCount is one reason and how many points it decided.
type ReasonCount struct {
	Reason Reason `json:"reason"`
	Count  int    `json:"count"`
}

// Tally is the classified corpus: the three verdicts, the reasons in
// reason order, and the points whose session tree could not be recovered.
// A transcript carries a salted HMAC of its branch, never the branch
// itself, so no point's tree is recoverable in v0 (owner ruling R2).
type Tally struct {
	Points          int           `json:"points"`
	ShouldFire      int           `json:"should_fire"`
	ShouldNot       int           `json:"should_not"`
	Unclassified    int           `json:"unclassified"`
	TreeUnrecovered int           `json:"tree_unrecovered"`
	Reasons         []ReasonCount `json:"reasons,omitempty"`
}

// ClassifyPoints reads every point and counts the verdicts and reasons.
// Output is sorted by reason, never by map order.
func ClassifyPoints(points []Point) Tally {
	tally := Tally{Points: len(points), TreeUnrecovered: len(points)}
	counts := map[Reason]int{}
	for _, p := range points {
		label, reason := ClassifyDecisionPoint(p)
		switch label {
		case ShouldFire:
			tally.ShouldFire++
		case ShouldNot:
			tally.ShouldNot++
		default:
			tally.Unclassified++
		}
		counts[reason]++
	}
	for reason, count := range counts {
		tally.Reasons = append(tally.Reasons, ReasonCount{Reason: reason, Count: count})
	}
	sort.Slice(tally.Reasons, func(i, j int) bool { return tally.Reasons[i].Reason < tally.Reasons[j].Reason })
	return tally
}
