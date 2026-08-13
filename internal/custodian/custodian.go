// Package custodian implements the config custodian's contract layer
// (docs/design/config-custodian.md, vision 22): falsifiable wants —
// "concerns" — about a repository's configuration, loaded from CUE at two
// law-13 scopes (~/.fathom/concerns.cue as user defaults,
// <repo>/.fathom/concerns.cue governing) and audited mechanically — no
// daemon, no LLM, no network. (Named custodian, not concern: the
// concern-scoped QUERY engine already owns internal/concern.)
//
// Fathom is the instrument, never the hand (vision 22 §3b): this package
// reports holds/violated/unable with evidence; an agent in a session, with
// a human present, decides what to edit. Nothing here writes.
package custodian

// Scope names which configuration layer a concern came from. The project
// scope GOVERNS (law 13): a same-ID project entry replaces the user entry
// wholesale.
type Scope string

const (
	ScopeUser    Scope = "user"
	ScopeProject Scope = "project"
)

// UserFileName and ProjectFileName are the two configuration locations,
// mirroring the capability registry's layout.
const (
	UserFileName    = "concerns.cue" // under ~/.fathom/
	ProjectFileName = "concerns.cue" // under <repo>/.fathom/
)

// CheckKind is a machine-evaluable check. A concern whose check kind is
// not one of these is refused at load — an uncheckable concern is a wish,
// and wishes rot into the same misdirection the custodian exists to
// remove.
type CheckKind string

const (
	KindForbiddenPattern           CheckKind = "forbidden_pattern"
	KindCommandDocumentedAndExists CheckKind = "command_documented_and_exists"
	KindMaxLines                   CheckKind = "max_lines"
	KindPointersResolve            CheckKind = "pointers_resolve"
)

// KnownKinds lists the accepted kinds in sorted order, for refusal
// messages that teach.
var KnownKinds = []CheckKind{
	KindCommandDocumentedAndExists,
	KindForbiddenPattern,
	KindMaxLines,
	KindPointersResolve,
}

// CorpusAgentConfig is v1's only `where` corpus: the repo's agent
// configuration files (CLAUDE.md / AGENTS.md at any depth, on git's
// authority of what belongs to the repo). The corpus is a named seam —
// packs for other deterministic-validation configs (lefthook, Taskfile)
// arrive as new corpus resolvers behind the same engine (CC-11/CC-12),
// never as a parallel system.
const CorpusAgentConfig = "agent-config"

// Check is the machine-evaluable half of a concern. Which fields are
// required depends on Kind; the loader refuses an incomplete check rather
// than letting the audit discover the hole later.
type Check struct {
	Kind    CheckKind `json:"kind"`
	Pattern string    `json:"pattern,omitempty"` // forbidden_pattern (Go regexp)
	Where   string    `json:"where,omitempty"`   // corpus name; v1: agent-config
	Command string    `json:"command,omitempty"` // command_documented_and_exists
	File    string    `json:"file,omitempty"`    // max_lines (repo-relative)
	Lines   int       `json:"lines,omitempty"`   // max_lines budget
}

// Concern is a falsifiable want with provenance.
type Concern struct {
	ID    string `json:"id"`
	Want  string `json:"want"`
	Why   string `json:"why,omitempty"`
	Off   bool   `json:"off,omitempty"`
	Scope Scope  `json:"scope"`
	Check Check  `json:"check"`
}

// Status is a finding's outcome. There is no silent state: a concern that
// could not be evaluated says so with a reason (law 3), and a disabled
// concern is named rather than hidden.
type Status string

const (
	StatusHolds    Status = "holds"
	StatusViolated Status = "violated"
	StatusUnable   Status = "unable-to-check"
	StatusOff      Status = "off"
)

// Finding is one concern's audit outcome.
type Finding struct {
	Concern  Concern  `json:"concern"`
	Status   Status   `json:"status"`
	Detail   string   `json:"detail"`             // one line: what held, or what broke, with numbers
	Evidence []string `json:"evidence,omitempty"` // "file:line" (or "file:line → target") per hit
}
