package checkcatalog

import (
	"errors"
	"fmt"
	"strings"
)

// Severity is how badly a finding matters, in a published four-tier
// parser-vendor taxonomy. Its one behavioural property is Blocking.
type Severity string

const (
	// SeverityFatal means the artifact will not be read correctly at all —
	// truncation, an unparseable file, a missing required identity.
	SeverityFatal Severity = "fatal"
	// SeverityMajor means a structural problem that loses information — a
	// record with no dates, a heading a parser will not recognise.
	SeverityMajor Severity = "major"
	// SeverityData means a field that should be extractable and is not.
	SeverityData Severity = "data"
	// SeverityImprovement means it parses and it is true; it just reads worse
	// than it could.
	SeverityImprovement Severity = "improvement"
)

// Severities is the closed vocabulary, most severe first; Report.String
// groups findings in this order.
var Severities = []Severity{SeverityFatal, SeverityMajor, SeverityData, SeverityImprovement}

// Valid reports whether s is in the vocabulary.
func (s Severity) Valid() bool {
	for _, k := range Severities {
		if k == s {
			return true
		}
	}
	return false
}

// Blocking reports whether a finding of this severity should stop a publish.
// Fatal and Major block; Data and Improvement are reported and never gate.
// The line is drawn here, once, rather than per check, so it cannot drift.
func (s Severity) Blocking() bool { return s == SeverityFatal || s == SeverityMajor }

// Means is how a check reaches its verdict. Reports group by it, so "how much
// of this gate is deterministic" has a printed answer.
type Means string

const (
	// MeansDeterministic means the same input always yields the same
	// findings; no model is in the loop.
	MeansDeterministic Means = "deterministic"
	// MeansModel means a model judges; the finding is a proposal, not a proof.
	MeansModel Means = "model"
)

// AllMeans is the closed vocabulary.
var AllMeans = []Means{MeansDeterministic, MeansModel}

// Valid reports whether m is in the vocabulary.
func (m Means) Valid() bool { return m == MeansDeterministic || m == MeansModel }

// Row is one catalog row: the declared shape of a check, mirroring #Check in
// schema.cue. It is what a consumer decodes out of its catalog and what
// Reconcile compares implementations against.
type Row struct {
	// ID is the stable identifier, unique within a catalog. Reconcile
	// enforces the uniqueness — CUE cannot express it across a list.
	ID string `json:"id"`

	// Name is the short human label.
	Name string `json:"name"`

	// Severity is how badly a finding from this check matters.
	Severity Severity `json:"severity"`

	// Means is how the check reaches its verdict.
	Means Means `json:"means"`

	// Rejects is one sentence — what this check refuses — written for a
	// human deciding whether to care. It is the text a catalog prints.
	Rejects string `json:"rejects"`

	// Source names the evidence the check's threshold rests on, so a later
	// reader can re-check it instead of inheriting it. Optional in the shape;
	// a consumer that wants it mandatory asserts so in its own test —
	// "judgement" is an honest value, silence is not.
	Source string `json:"source,omitempty"`
}

// ErrInvalidRow is the sentinel every Row.Validate failure wraps; test for it
// with errors.Is.
var ErrInvalidRow = errors.New("checkcatalog: invalid row")

// Validate reports the first constraint of #Check that r violates, wrapped in
// ErrInvalidRow, or nil. It re-checks what the schema states so a row that
// bypassed CUE still fails closed.
func (r Row) Validate() error {
	switch {
	case strings.TrimSpace(r.ID) == "":
		return fmt.Errorf("%w: id must not be empty", ErrInvalidRow)
	case strings.TrimSpace(r.Name) == "":
		return fmt.Errorf("%w: %s: name must not be empty", ErrInvalidRow, r.ID)
	case !r.Severity.Valid():
		return fmt.Errorf("%w: %s: severity %q is not one of %s", ErrInvalidRow, r.ID, r.Severity, joinSeverities())
	case !r.Means.Valid():
		return fmt.Errorf("%w: %s: means %q is not one of %s", ErrInvalidRow, r.ID, r.Means, joinMeans())
	case strings.TrimSpace(r.Rejects) == "":
		return fmt.Errorf("%w: %s: rejects must say what the check refuses", ErrInvalidRow, r.ID)
	case r.Severity == SeverityFatal && r.Means != MeansDeterministic:
		return fmt.Errorf("%w: %s: a fatal check must be deterministic — a model must not be what blocks a publish", ErrInvalidRow, r.ID)
	}
	return nil
}

// Finding is one problem, attributed as precisely as the check can manage.
type Finding struct {
	// CheckID names the check that produced the finding; Run sets it.
	CheckID string `json:"checkId"`

	// Severity defaults to the producing check's own when a check leaves it
	// empty; a check may lower or raise it per finding.
	Severity Severity `json:"severity"`

	// Subject is what the finding is about when the check can name it — a
	// record id, a path, a field. Empty means the whole input.
	Subject string `json:"subject,omitempty"`

	// Message is the one-line statement of the problem.
	Message string `json:"message"`

	// Detail is optional supporting text.
	Detail string `json:"detail,omitempty"`

	// Skipped marks a finding that reports the check could not run rather
	// than a problem with the input. It is still a finding: it carries the
	// check's severity and blocks when that severity does.
	Skipped bool `json:"skipped,omitempty"`
}

// String renders the finding as one line: [severity] id subject: message — detail.
func (f Finding) String() string {
	loc := f.Subject
	if loc == "" {
		loc = "(input)"
	}
	s := fmt.Sprintf("[%s] %s %s: %s", f.Severity, f.CheckID, loc, f.Message)
	if f.Detail != "" {
		s += " — " + f.Detail
	}
	return s
}

// Skipped is what a check returns when it cannot run: a finding that says so,
// never an empty slice. Run records the check in Report.Skipped and the
// finding inherits the check's severity, so a skipped blocking check blocks.
func Skipped(reason string) []Finding {
	return []Finding{{Message: "skipped: " + reason, Skipped: true}}
}

// Check is one catalog row plus its implementation, generic over the input a
// consumer's checks read. A check takes the whole input and picks what it
// needs, so adding an input later changes no signature.
type Check[In any] struct {
	Row

	// Fn returns findings. A check that cannot run returns Skipped(reason),
	// never nil: a skipped check must not look like a passed one.
	Fn func(In) []Finding
}

// Rows projects the declared shape out of a set of checks, in input order.
func Rows[In any](checks []Check[In]) []Row {
	rows := make([]Row, len(checks))
	for i, c := range checks {
		rows[i] = c.Row
	}
	return rows
}

// Find returns the check with the given id, and whether one exists.
func Find[In any](checks []Check[In], id string) (Check[In], bool) {
	for _, c := range checks {
		if c.ID == id {
			return c, true
		}
	}
	return Check[In]{}, false
}

func joinSeverities() string {
	names := make([]string, len(Severities))
	for i, s := range Severities {
		names[i] = string(s)
	}
	return strings.Join(names, " | ")
}

func joinMeans() string {
	names := make([]string, len(AllMeans))
	for i, m := range AllMeans {
		names[i] = string(m)
	}
	return strings.Join(names, " | ")
}
