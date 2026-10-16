// Package intent reads a repository's Project Intent corpus — the router,
// the living documents it names, and the decision records — and reports
// what it found, deterministically and without an LLM.
//
// ADR-026. Three rules shape everything here:
//
//   - Fathom never edits the corpus. It parses, audits, and lists; a
//     proposal is printed for a human to apply (law 15).
//   - Prose is never evaluated. A condition line is human-judged unless its
//     author attached a `{oracle: …}` annotation, and even then the
//     annotation — not the prose — is what is checked.
//   - What could not be parsed is named. A doc that yields nothing is a
//     finding, never a clean empty (the INC-8 class this repo keeps
//     re-learning: a plausible zero reads as health).
//
// docs/vision/ is deliberately never read INTO THIS CORPUS: it is
// aspirational prose, and compiling aspiration into rules is the failure
// the first fleet compile taught (docs/vision/README.md). The vision
// promotion pipeline (visions.go, DP-3, ADR-026 amendment of 2026-09-25) is
// a separate reader that takes only a vision's promotion metadata — its
// status line, plan and decision pointers, links, dates and any condition
// label — and never a condition, an invariant, or a rule from its prose.
package intent

import (
	"path/filepath"
	"regexp"
	"sort"

	readmanifest "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/read-manifest"
)

// Tier says how much of the practice a repository carries. A repo need not
// adopt the whole thing to get something back: nine repos in this fleet have
// decision records and no router, and their `Revisit when` trailers are
// exactly the conditions worth listing.
type Tier string

const (
	TierRouter        Tier = "router"         // docs/intent/README.md exists
	TierDecisionsOnly Tier = "decisions-only" // decision records, no router
	TierNone          Tier = "none"
)

// Rung is an invariant's claimed enforcement, on the practice's evidence
// ladder (stated → reviewed → test → gate → proven), plus the two verdicts
// the ladder itself needs: VIOLATED (the row says so) and unevaluable (the
// claim names something this parser cannot resolve).
type Rung string

const (
	RungGate        Rung = "gate"
	RungTest        Rung = "test"
	RungProven      Rung = "proven"
	RungLint        Rung = "lint"
	RungGuard       Rung = "guard"
	RungCode        Rung = "code"
	RungReviewed    Rung = "reviewed"
	RungStated      Rung = "stated"
	RungViolated    Rung = "violated"
	RungUnevaluable Rung = "unevaluable"
)

// Enforced reports whether a rung is an enforcement claim this tool can
// check. `stated` and `reviewed` are honest rungs, not debt: the practice
// says an invariant nobody checks must SAY so, and pressuring an author to
// upgrade or delete such a row is how honest rows get quietly faked.
func (r Rung) Enforced() bool {
	switch r {
	case RungGate, RungTest, RungProven, RungLint, RungGuard:
		return true
	}
	return false
}

// Condition is one `Done when` / `Promote when` / `Drop if` / `Revisit when`
// line — the practice's expiry conditions, the whole reason the corpus is
// worth reading by machine.
type Condition struct {
	ID     string // "<record>#<n>", e.g. "F9#1", "ADR-014#2", "I5#1"
	Record string // the enclosing record's id, "" at file level
	Label  string // "Promote when"
	Text   string // the condition's prose, annotation removed
	File   string // repo-relative
	Line   int    // 1-based
	Ann    map[string]string
	// Terminal marks a condition that can never fire ("never", "none
	// identified"): listed for completeness, excluded from every total so it
	// cannot inflate a denominator.
	Terminal bool
	// Suspect carries a brace block that looked like an annotation but
	// yielded no keys — reported, never silently dropped (O4).
	Suspect string
}

// Annotated reports whether an author attached an oracle to this condition.
// An unannotated condition is human-judged, always.
//
// A block that carries ONLY bet keys (bets.go: `bet`, `instrument`,
// `threshold`, `review`) is a goal's pre-registration, not an oracle: it
// says how a human will judge the goal on a date, and evaluates nothing.
// Counting it here would let the surface that enumerates bets inflate K0's
// demand signal — the count ADR-026 gates the oracle runtime on — with
// annotations nobody wrote to be evaluated. Any other key still counts, so
// a mistyped oracle block stays visible rather than vanishing.
func (c Condition) Annotated() bool {
	for k := range c.Ann {
		if !betKeys[k] {
			return true
		}
	}
	return false
}

// Invariant is one row of invariants.md.
type Invariant struct {
	ID        string
	Statement string
	RungRaw   string
	Rung      Rung
	Tests     []string // Test names the rung claims; a trailing * is a prefix
	GateSteps []string // "task <target>" / "lefthook <hook> <step>"
	File      string
	Line      int
}

// Surface is one row of the router's sensitive-surface table: the globs a
// change must match, why it is sensitive, and the evidence "done" needs.
type Surface struct {
	Globs     []string
	Why       string
	DoneNeeds string
	Tests     []string
	Line      int
}

// Goal is one `**G<n>. Title.**` record of scope.md. Whether it is a
// pre-registered bet or refused is LoadBets' verdict (bets.go); the corpus
// only records identity, so a refused goal never changes what the audit
// parses or counts.
type Goal struct {
	ID    string // "G2"
	Title string // "The A/B becomes admissible."
	File  string // repo-relative
	Line  int    // 1-based
}

// Unparsed is a line or shape the parser could not read. Every one is
// reported with its file and line: a corpus that silently yields nothing is
// indistinguishable from a healthy one, which is the failure mode this
// repository has now recorded four times.
type Unparsed struct {
	File   string
	Line   int
	Reason string
}

// Corpus is everything Load found.
type Corpus struct {
	Root          string
	Tier          Tier
	RouterPath    string // repo-relative, "" when absent
	DecisionsHome string // repo-relative path or dir the router names
	// VisionsHome is the vision series' directory and VisionsIndex its
	// index file, as the router's docs map names them (the router is the
	// manifest of homes). Only the promotion pipeline reads them.
	VisionsHome  string
	VisionsIndex string
	Conditions   []Condition
	// Goals are scope.md's `**G<n>.**` records, in file order (DP-4).
	Goals      []Goal
	Invariants []Invariant
	Surfaces   []Surface
	Unparsed   []Unparsed
	// NestedRouters are routers below the root (a monorepo's per-product
	// intent dirs). v1 audits the root only and NAMES these as unaudited
	// rather than pretending the repo has one corpus.
	NestedRouters []string
	// Families are the living documents split behind a router (K8):
	// docs/intent/<name>.md lists, docs/intent/<name>/*.md define, and the
	// corpus reads each family as one document (family.go). Sorted by name.
	Families []Family
	// Excluded are lines where a condition label APPEARS but is not a
	// condition: the router's own instructions naming the labels, a
	// decisions-index table header, a question asking about a `Promote
	// when`. The router documents a grep as the registry, and a grep cannot
	// tell prose from a condition — so every difference between the grep and
	// this parser is recorded here with its reason. An unexplained
	// difference is a parser bug; a silent one is the failure this package
	// exists to prevent.
	Excluded []Unparsed
	// Coverage accounts for every corpus file Load considered: read,
	// genuinely absent, deliberately skipped, or UNREADABLE with its errno
	// (ADR-027). Without it an unreadable invariants.md rendered as a repo
	// with zero invariants and half the debt — its most broken state was
	// its healthiest-looking one. Every count derived from this corpus must
	// go through readcoverage.RenderCount.
	Coverage readmanifest.Manifest

	// reads is the live recorder during Load; Coverage is its frozen
	// manifest. Unexported so a caller cannot append to coverage after the
	// fact and claim to have seen more than it did.
	reads *readmanifest.Set
	// famByFile and famParses carry each family's shared parse state
	// during Load (family.go).
	famByFile map[string]*familyParse
	famParses []*familyParse
}

// conditionLabels are the practice's four expiry labels, plus the
// assumption alias domain.md actually uses (`If false:` — the practice's
// §4 says Revisit when covers assumptions, but the template writes them
// this way, and a parser that ignores the corpus in front of it is worse
// than one that names the divergence).
var conditionLabels = []string{"Done when", "Promote when", "Drop if", "Revisit when", "If false"}

var (
	labelRe = regexp.MustCompile(`(?i)\b(Done when|Promote when|Drop if|Revisit when|If false)\b\s*[:.]`)
	// bareLabelRe is the router's own grep: the label with NO punctuation
	// requirement. It is used only to EXPLAIN the difference between that
	// grep and this parser — never to create a condition.
	bareLabelRe  = regexp.MustCompile(`(?i)\b(Done when|Promote when|Drop if|Revisit when)\b`)
	invRowRe     = regexp.MustCompile(`^\|\s*(I\d+)\s*\|(.*)$`)
	goalRe       = regexp.MustCompile(`^\*\*(G\d+)\.`)
	futureRe     = regexp.MustCompile(`^##\s+(F\d+)\.`)
	questionRe   = regexp.MustCompile(`^\*\*(Q\d+)\.`)
	incidentRe   = regexp.MustCompile(`^###\s+(INC-\d+)\b`)
	nonGoalRe    = regexp.MustCompile(`^-\s+\*\*(.+?)\*\*`)
	testNameRe   = regexp.MustCompile(`\bTest\w+\*?`)
	backtickRe   = regexp.MustCompile("`([^`]+)`")
	gateTaskRe   = regexp.MustCompile(`(?i)\btask\s+([a-z0-9:_-]+)`)
	gateHookRe   = regexp.MustCompile(`(?i)\blefthook\s+([a-z-]+)\s+` + "`?([a-z-]+)`?")
	terminalRe   = regexp.MustCompile(`(?i)^\s*(never|none identified|none|n/a)\b`)
	livingDocs   = []string{"principles.md", "invariants.md", "domain.md", "scope.md", "futures.md", "questions.md", "autonomy.md", "incidents.md"}
	routerRelDir = filepath.Join("docs", "intent")
)

// Load reads root's intent corpus. It never fails on a malformed document:
// what cannot be parsed becomes an Unparsed entry, so the caller can report
// the gap instead of an empty success.
func Load(root string) (Corpus, error) {
	c := Corpus{Root: root, Tier: TierNone}
	c.reads = readmanifest.NewSet()
	c.reads.RelativeTo(root)

	routerAbs := filepath.Join(root, routerRelDir, "README.md")
	if body, ok := c.reads.ReadFile(routerAbs); ok {
		c.Tier = TierRouter
		c.RouterPath = filepath.ToSlash(filepath.Join(routerRelDir, "README.md"))
		c.parseRouter(string(body))
		// The corpus is the DIRECTORY, not a hard-coded file list: a repo
		// that adds a living doc, or names its decisions index there, must
		// not have it silently unread.
		docs, _ := filepath.Glob(filepath.Join(root, routerRelDir, "*.md"))
		nested, _ := filepath.Glob(filepath.Join(root, routerRelDir, "*", "*.md"))
		docs = append(docs, nested...)
		sort.Strings(docs)
		// A living doc split behind a router reads as one document: the
		// router sorts before its member directory, and the two share one
		// record and numbering state (family.go, K8).
		var rels []string
		for _, abs := range docs {
			if rp, err := filepath.Rel(root, abs); err == nil {
				rels = append(rels, filepath.ToSlash(rp))
			}
		}
		c.detectFamilies(rels)
		for _, abs := range docs {
			if abs == routerAbs {
				continue
			}
			b, ok := c.reads.ReadFile(abs)
			rp, relErr := filepath.Rel(root, abs)
			if relErr != nil {
				continue
			}
			if !ok {
				// Law 3: a file the glob FOUND and the reader could not
				// open is an instrument failure, recorded on the manifest
				// and named here — never a silently smaller corpus.
				c.Unparsed = append(c.Unparsed, Unparsed{
					File: filepath.ToSlash(rp), Line: 1,
					Reason: "could not be read, so nothing in it is in this corpus",
				})
				continue
			}
			c.parseLivingDoc(filepath.ToSlash(rp), string(b))
		}
		c.finishFamilies()
	}

	c.DecisionsHome = c.decisionsHome()
	c.parseDecisions()
	c.Coverage = c.reads.Manifest()
	if c.Tier == TierNone && len(c.Conditions) > 0 {
		c.Tier = TierDecisionsOnly
	}
	c.findNestedRouters()

	sort.SliceStable(c.Conditions, func(i, j int) bool {
		if c.Conditions[i].File != c.Conditions[j].File {
			return c.Conditions[i].File < c.Conditions[j].File
		}
		return c.Conditions[i].Line < c.Conditions[j].Line
	})
	sort.SliceStable(c.Unparsed, func(i, j int) bool {
		if c.Unparsed[i].File != c.Unparsed[j].File {
			return c.Unparsed[i].File < c.Unparsed[j].File
		}
		return c.Unparsed[i].Line < c.Unparsed[j].Line
	})
	return c, nil
}
