package readout

// policy.go — rich mode's policy table (design docs/design/readouts.md §12):
// one deterministic function over counts and flags, never the output text
// (I61), with provider none (I42). Rules in order:
//
//  1. A failure (PostToolUseFailure or an interrupted call): enriched when a
//     record names a session-edited file or a test in the last run's
//     failing set, else raw.
//  2. A success with protected lines in a diagnostic stream: raw, plus the
//     enrichment of rule 1.
//  3. A success over the rule's byte floor, the rule promoted: lean.
//  4. A whole-file read of a session-edited file: raw.
//  5. Otherwise raw.
//
// Lean is reachable only for a promoted rule, and no caller promotes one
// yet (lean stays deferred). The contested band (rules 1 and 4 and long
// reads of unedited files) is marked on the choice as data; nothing asks
// there. The cut points come from BuiltinPolicy, or from the user's fitted
// policy file (fit.go, `fathom readout fit --write`) when it loads.

import (
	"strings"

	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

// The table's answers. Enriched is the raw output plus one injected line:
// rich mode adds beside the output, never in place of it.
const (
	ChoiceRaw      = "raw"
	ChoiceLean     = "lean"
	ChoiceEnriched = "enriched"
)

// The events the table reads.
const (
	EventSuccess = "success"
	EventFailure = "failure"
)

// DecidedByTable is the receipt's decided_by for a table answer: JG-1's
// table tier (internal/decide DecidedTable; a test holds them equal).
const DecidedByTable = "table"

// BuiltinLongReadLines is the built-in long-read cut: a whole read over this
// many lines is long (design §12: the corpus's reads over 200 lines).
const BuiltinLongReadLines = 200

// leanMaxRecovery is decision 9's demotion bar: a promoted rule whose
// measured lean recovery rate exceeds it serves raw.
const leanMaxRecovery = 0.05

// Features are the inputs every decision sees, and the only ones a model
// would ever see (design §12, I61): counts and flags, never output text.
type Features struct {
	// Event is EventSuccess or EventFailure (PostToolUseFailure, or a call
	// the harness reports interrupted).
	Event string
	// Class is the output's stream class.
	Class Stream
	// Bytes and Lines are the output's size.
	Bytes, Lines int
	// Records counts the call's records by kind.
	Records map[string]int
	// Protected counts the protected lines the call's diagnostic streams
	// carry (a content stdout's lines are data and never count).
	Protected int
	// EditedOverlap counts the records naming a file this session edited or
	// a definition the working tree changed; FailingOverlap, the records
	// naming a test in the last run's failing set.
	EditedOverlap, FailingOverlap int
	// WholeRead is a read printing one file whole; ReadEdited, that the
	// file was edited this session or differs from HEAD.
	WholeRead, ReadEdited bool
	// Promoted is the matched rule's promotion (its report verdict kept);
	// RuleFloor its own byte floor (when.bytes_gt); Recovery its measured
	// lean recovery rate, negative when unmeasured.
	Promoted  bool
	RuleFloor int
	Recovery  float64
	// SubAgent is a sub-agent's call (agent_id set). The table's answer
	// does not depend on it; a model's would see it.
	SubAgent bool
}

// Choice is the table's answer for one call.
type Choice struct {
	// Mode is ChoiceRaw, ChoiceLean or ChoiceEnriched.
	Mode string
	// Rule is the table rule that answered, 1 to 5.
	Rule int
	// Contested marks the band where a model could add anything: rules 1
	// and 4 and long reads of unedited files. Data only.
	Contested bool
	// DecidedBy is DecidedByTable.
	DecidedBy string
}

// Policy is the table's cut points.
type Policy struct {
	// LeanFloorBytes raises every rule's byte floor for rule 3 (0 leaves
	// each rule's own floor).
	LeanFloorBytes int
	// LongReadLines is the long-read cut: a whole read over it is long,
	// contested when its file is unedited, and leans only past it.
	LongReadLines int
}

// BuiltinPolicy is the table's built-in cut points.
func BuiltinPolicy() Policy {
	return Policy{LongReadLines: BuiltinLongReadLines}
}

// Choose is the policy table: rules 1 to 5 in order over f and p's cut
// points. A zero p reads as BuiltinPolicy. The same inputs give the same
// choice; the record map is only summed.
func Choose(f Features, p Policy) Choice {
	if p.LongReadLines <= 0 {
		p.LongReadLines = BuiltinLongReadLines
	}
	if p.LeanFloorBytes < 0 {
		p.LeanFloorBytes = 0
	}
	records := 0
	for _, n := range f.Records {
		records += n
	}
	enrich := records > 0 && (f.EditedOverlap > 0 || f.FailingOverlap > 0)
	editedRead := f.WholeRead && f.ReadEdited
	longRead := f.WholeRead && f.Lines > p.LongReadLines
	floor := max(f.RuleFloor, p.LeanFloorBytes)

	c := Choice{Mode: ChoiceRaw, DecidedBy: DecidedByTable}
	switch {
	case f.Event == EventFailure:
		c.Rule = 1
		if enrich {
			c.Mode = ChoiceEnriched
		}
	case f.Protected > 0:
		c.Rule = 2
		if enrich {
			c.Mode = ChoiceEnriched
		}
	case f.Promoted && f.Bytes > floor && !editedRead && (!f.WholeRead || longRead) && f.Recovery <= leanMaxRecovery:
		// An edited file's whole read never leans: rule 4 holds it raw, as
		// the engine does (ReasonEdited).
		c.Rule, c.Mode = 3, ChoiceLean
	case editedRead:
		c.Rule = 4
	default:
		c.Rule = 5
	}
	c.Contested = c.Rule == 1 || c.Rule == 4 || (longRead && !f.ReadEdited)
	return c
}

// editedWholeRead is rule 4's question as the engine asks it before any
// view: Choose over a success with no protected line and no promotion, so
// rule 4 answers exactly when the read is whole and its file edited.
func editedWholeRead(wholeRead, edited bool) bool {
	return Choose(Features{Event: EventSuccess, WholeRead: wholeRead, ReadEdited: edited}, Policy{}).Rule == 4
}

// StreamOf is the stream class the engine reads c's stdout as, whether or
// not a rule matches it: diagnostic for a merged, piped or unreadable one.
func StreamOf(c *shellcmd.Command, quiet []string) Stream {
	if c == nil || c.Unreadable {
		return StreamDiagnostic
	}
	return streamOf(classifyCommand(c, quiet))
}

// ProtectedLines counts the lines of text a diagnostic stream protects:
// exit, notice, error and frame lines, as the terminal shows them.
func ProtectedLines(text string) int {
	if text == "" {
		return 0
	}
	n := 0
	for _, l := range strings.Split(text, "\n") {
		if c := classify(renderTerminal(l)); c != classPlain && protected(c, StreamDiagnostic) {
			n++
		}
	}
	return n
}

// CompileOverlap counts the compile_error groups that name the session's
// own edits: a group whose root is a definition the working tree changed
// (changed), or whose root or errors sit in a file edited reports.
func CompileOverlap(recs []Record, changed []CompileDef, edited func(file string) bool) int {
	n := 0
	for _, g := range groupCompileErrors(recs, changed, edited) {
		if g.edited {
			n++
			continue
		}
		if g.ident != "" {
			if _, ok := compileDefFor(g.ident, changed); ok {
				n++
			}
		}
	}
	return n
}
