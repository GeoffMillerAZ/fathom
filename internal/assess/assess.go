// Package assess is the pure core of fathom's priced judgment lanes
// (docs/design/assess-lanes.md, ADR-046): DO-3's benchmark lane, which
// judges one unit's on-disk work products, and IR-3's intervention lane,
// which judges the firings the mechanical retro cannot decide.
//
// Every judgment is a TYPED decision-port question (agentic-kit
// foundations/decision): noul for a binary property, choice for levels and
// outcomes. There is no free-form prompt anywhere in this package, and no
// transcript ever reaches a state (the refusal is structural: the builders
// take work products, and nothing else, as input).
//
// No I/O except the ledger's append/read; no clock (callers stamp rows).
// The CLI in cmd/fathom gathers evidence from git and the local ledgers,
// gates the run, opens the decision port and prints.
package assess

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/internal/cost"
)

// Lanes and their spend labels (IR-4). The label is what the assess
// ledger meters under and what a judge child's session-ledger row carries.
const (
	LaneUnit          = "unit"
	LaneInterventions = "interventions"

	LabelUnit          = "assess"
	LabelInterventions = "assess-interventions"

	// SpendLabelEnv is stamped into assess's own environment before the
	// decision port opens, so a CLI provider's judge child inherits it and
	// any session-ledger row written inside that child is isolated.
	SpendLabelEnv = "FATHOM_SPEND_LABEL"

	// IsolationRegistration names the pre-registered isolation rule
	// (docs/design/assess-lanes.md §3, registered 2026-09-25 before any
	// assess run). Every ledger row carries it, so a reading can always say
	// which rule its spend was isolated under.
	IsolationRegistration = "IR-4@2026-09-25"
)

// IsolatedSpendLabel reports whether a session-ledger row carrying label is
// judging spend that must never enter an arm or bet unit. ANY non-empty
// label is isolated: a future lane whose label this package has not been
// taught must fail toward exclusion, never leak into a unit total.
func IsolatedSpendLabel(label string) bool { return strings.TrimSpace(label) != "" }

// Caps bound what one run sends. Every cut is named inside the state and
// in Built.Truncations.
type Caps struct {
	MaxStateBytes int // whole state
	MaxFileBytes  int // one IR-3 file's diff
	MaxQuestions  int // IR-3 questions per run
}

// DefaultCaps are the design's v1 caps (§1).
func DefaultCaps() Caps {
	return Caps{MaxStateBytes: 48 * 1024, MaxFileBytes: 8 * 1024, MaxQuestions: 24}
}

// Commit is one commit whose commit time falls inside the unit window.
type Commit struct {
	SHA     string
	Subject string
	Patch   string
}

// Plan delta kinds: a checkbox line a unit's commits changed.
const (
	DeltaFlipped  = "flipped"  // a row now reads [x] that did not before
	DeltaReopened = "reopened" // a row that read [x] no longer does
	DeltaAdded    = "added"    // a new, unchecked row
)

// PlanDelta is one plan row a unit's commits changed.
type PlanDelta struct {
	Plan string
	Kind string
	Row  string
}

// GateResult is one verify-ledger row inside the unit window.
type GateResult struct {
	Plan     string
	Origin   string
	Claimed  int
	Verified int
	Failing  int
}

// UnitEvidence is a unit's on-disk work products — the only inputs DO-3's
// state is ever built from.
type UnitEvidence struct {
	WindowStart, WindowEnd string
	Commits                []Commit
	PlanDeltas             []PlanDelta
	Gates                  []GateResult
	// VerifyCatches is the session-ledger row's verify_catches: nil when
	// verify never ran (unknown, never zero).
	VerifyCatches *int
}

// Item is one IR-3 firing the mechanics could not decide.
type Item struct {
	Channel    string // guard | obligations
	File       string // repo-relative
	Outcome    string // committed | modified-uncommitted
	Guidance   string
	Provenance string
	Diff       string
}

// Built is one ready request plus everything a reader needs to trust it.
type Built struct {
	Lane    string
	Request decision.Request
	// Digest identifies the question SET (templates, not this run's state):
	// readings from different sets are never pooled.
	Digest string
	// Subjects maps each question id to what it judges ("unit" or a file).
	Subjects map[string]string
	// Truncations names every cut; Partial is true when there was one. A
	// partial run never enters a calibration sample.
	Truncations []string
	Partial     bool
	// Redacted is true when the DLP gate changed the state.
	Redacted bool
	// Skipped names questions or items deliberately not asked.
	Skipped []string
}

// ---- DO-3: the unit lane ------------------------------------------------

var qualityLevels = []decision.Option{
	{Name: "harmful", Criteria: "the change introduces a defect, regression or broken build visible in the diffs."},
	{Name: "incomplete", Criteria: "the change visibly stops short of what its commits or plan rows say it does."},
	{Name: "adequate", Criteria: "the change works as described, with visible gaps (untested paths, loose ends)."},
	{Name: "solid", Criteria: "the change is complete and its behaviour is exercised by tests in the diffs."},
	{Name: "exemplary", Criteria: "complete, tested, and it leaves the code simpler than it found it."},
}

func unitQuestions(withClaims bool) []decision.Question {
	qs := []decision.Question{
		{
			ID: "coherent", Kind: decision.KindNoul,
			Instructions:  "The commits in this unit implement one coherent, finished change.",
			TrueCriteria:  "the diffs read as a complete change with a clear purpose.",
			FalseCriteria: "the diffs are partial, abandoned, contradictory, or leave the code broken.",
		},
	}
	if withClaims {
		qs = append(qs, decision.Question{
			ID: "claims_backed", Kind: decision.KindNoul,
			Instructions:  "Every plan row this unit flipped to done ([x]) is backed by code or tests present in the diffs.",
			TrueCriteria:  "each flipped row names something the diffs add or change.",
			FalseCriteria: "at least one flipped row claims work the diffs do not contain.",
		})
	}
	qs = append(qs,
		decision.Question{
			ID: "tests_accompany", Kind: decision.KindNoul,
			Instructions:  "The behaviour changes in the diffs come with test changes that exercise them.",
			TrueCriteria:  "new or changed behaviour has matching new or changed tests.",
			FalseCriteria: "behaviour changed with no test that exercises it.",
		},
		decision.Question{
			ID: "quality", Kind: decision.KindChoice,
			Instructions: "Which level best describes the work in these diffs?",
			Options:      qualityLevels,
		},
	)
	return qs
}

// UnitQuestionSetDigest identifies DO-3's question set (all templates,
// including the conditional one).
func UnitQuestionSetDigest() string { return digestQuestions(LaneUnit, unitQuestions(true)) }

// BuildUnit renders a unit's work products into one typed request. redact
// is the DLP gate (llmsec in production); it sees the whole final state.
func BuildUnit(ev UnitEvidence, caps Caps, redact func(string) (string, bool)) Built {
	b := Built{Lane: LaneUnit, Digest: UnitQuestionSetDigest(), Subjects: map[string]string{}}
	var s strings.Builder
	s.WriteString("Work products of one coding session, read from disk only: commits, plan deltas and gate results. " +
		"By design no transcript is included — judge what shipped, not what was said.\n")
	fmt.Fprintf(&s, "Window: %s to %s\n\n", ev.WindowStart, ev.WindowEnd)

	s.WriteString("== Gate results ==\n")
	if len(ev.Gates) == 0 {
		s.WriteString("no verify run recorded for this repository inside the window\n")
	}
	for _, g := range ev.Gates {
		origin := g.Origin
		if origin == "" {
			origin = "unknown"
		}
		fmt.Fprintf(&s, "verify %s (origin %s): claimed %d, verified %d, failing %d\n", g.Plan, origin, g.Claimed, g.Verified, g.Failing)
	}
	if ev.VerifyCatches != nil {
		fmt.Fprintf(&s, "session verify catches: %d\n", *ev.VerifyCatches)
	} else {
		s.WriteString("session verify catches: unknown (verify did not run at session end)\n")
	}

	s.WriteString("\n== Plan deltas ==\n")
	flipped := 0
	if len(ev.PlanDeltas) == 0 {
		s.WriteString("no plan row changed\n")
	}
	for _, d := range ev.PlanDeltas {
		label := d.Kind
		switch d.Kind {
		case DeltaFlipped:
			label = "flipped to done"
			flipped++
		case DeltaReopened:
			label = "reopened"
		case DeltaAdded:
			label = "added"
		}
		fmt.Fprintf(&s, "%s: %s: %s\n", d.Plan, label, strings.TrimSpace(d.Row))
	}

	fmt.Fprintf(&s, "\n== Commits (%d) ==\n", len(ev.Commits))
	for i, c := range ev.Commits {
		header := fmt.Sprintf("--- commit %s %s\n", shortSHA(c.SHA), c.Subject)
		room := caps.MaxStateBytes - s.Len() - len(header)
		if room <= 0 {
			rest := len(ev.Commits) - i
			note := fmt.Sprintf("[%d commit(s) omitted: cut at the state cap of %d bytes]\n", rest, caps.MaxStateBytes)
			s.WriteString(note)
			b.Truncations = append(b.Truncations, strings.TrimSpace(note))
			break
		}
		s.WriteString(header)
		patch := c.Patch
		if len(patch) > room {
			cut := len(patch) - room
			patch = patch[:room]
			note := fmt.Sprintf("\n[... %d bytes of commit %s cut at the state cap of %d bytes]\n", cut, shortSHA(c.SHA), caps.MaxStateBytes)
			s.WriteString(patch)
			s.WriteString(note)
			b.Truncations = append(b.Truncations, strings.TrimSpace(note))
			continue
		}
		s.WriteString(patch)
		if !strings.HasSuffix(patch, "\n") {
			s.WriteString("\n")
		}
	}

	qs := unitQuestions(flipped > 0)
	if flipped == 0 {
		b.Skipped = append(b.Skipped, "claims_backed: not asked — the unit flipped no plan row")
	}
	for _, q := range qs {
		b.Subjects[q.ID] = "unit"
	}
	b.Partial = len(b.Truncations) > 0
	b.Request = decision.Request{State: s.String(), Questions: qs}
	b.Request.State, b.Redacted = redact(b.Request.State)
	return b
}

// ---- IR-3: the intervention lane ----------------------------------------

var interventionOptions = []decision.Option{
	{Name: "complied", Criteria: "the change to the file follows the guidance, in letter and in spirit."},
	{Name: "departed", Criteria: "the change contradicts or ignores the guidance."},
	{Name: "not_applicable", Criteria: "the guidance does not bear on what this change did."},
	{Name: "cannot_tell", Criteria: "the diff shown is not enough to decide."},
}

func interventionQuestion(id, channel, file string) decision.Question {
	return decision.Question{
		ID: id, Kind: decision.KindChoice,
		Instructions: fmt.Sprintf("Section %s: did the change to %s follow the %s guidance shown in section %s?", id, file, channel, id),
		Options:      interventionOptions,
	}
}

// InterventionQuestionSetDigest identifies IR-3's question template.
func InterventionQuestionSetDigest() string {
	return digestQuestions(LaneInterventions, []decision.Question{interventionQuestion("F00", "<channel>", "<file>")})
}

// BuildInterventions renders the judged firings into one typed request:
// one choice per (channel, file), capped at caps.MaxQuestions with the
// rest named in Skipped. Items are judged in the order given (the caller
// sorts them).
func BuildInterventions(items []Item, caps Caps, redact func(string) (string, bool)) Built {
	b := Built{Lane: LaneInterventions, Digest: InterventionQuestionSetDigest(), Subjects: map[string]string{}}
	var s strings.Builder
	s.WriteString("Changes to files an agent edited after fathom put guidance in front of it, read from disk only. " +
		"By design no transcript is included. Each section names the file, the guidance, where that guidance " +
		"came from, and the file's diff inside the session window.\n")
	var qs []decision.Question
	for i, it := range items {
		if caps.MaxQuestions > 0 && len(qs) >= caps.MaxQuestions {
			b.Skipped = append(b.Skipped, fmt.Sprintf("%s %s: not asked — the per-run cap of %d questions was reached", it.Channel, it.File, caps.MaxQuestions))
			continue
		}
		id := fmt.Sprintf("F%02d", i+1)
		diff := it.Diff
		if caps.MaxFileBytes > 0 && len(diff) > caps.MaxFileBytes {
			cut := len(diff) - caps.MaxFileBytes
			diff = diff[:caps.MaxFileBytes] + fmt.Sprintf("\n[... %d bytes of this diff cut at the per-file cap of %d bytes]\n", cut, caps.MaxFileBytes)
			b.Truncations = append(b.Truncations, fmt.Sprintf("%s %s: %d bytes cut at the per-file cap", id, it.File, cut))
		}
		section := fmt.Sprintf("\n== %s %s: %s (outcome: %s) ==\nguidance (%s):\n%s\ndiff:\n%s\n",
			id, it.Channel, it.File, it.Outcome, it.Provenance, strings.TrimSpace(it.Guidance), strings.TrimRight(diff, "\n"))
		if caps.MaxStateBytes > 0 && s.Len()+len(section) > caps.MaxStateBytes {
			b.Skipped = append(b.Skipped, fmt.Sprintf("%s %s: not asked — cut at the state cap of %d bytes", it.Channel, it.File, caps.MaxStateBytes))
			b.Truncations = append(b.Truncations, fmt.Sprintf("%s %s: section cut at the state cap", id, it.File))
			continue
		}
		s.WriteString(section)
		qs = append(qs, interventionQuestion(id, it.Channel, it.File))
		b.Subjects[id] = it.File
	}
	b.Partial = len(b.Truncations) > 0
	b.Request = decision.Request{State: s.String(), Questions: qs}
	b.Request.State, b.Redacted = redact(b.Request.State)
	return b
}

// ---- Verdicts and the calibrated-band policy ----------------------------

// Verdict is one asked question's outcome. Band is the kit's band
// (proceed ≥ 0.9, unsure < 0.5); a proceed-band answer is the verdict, a
// confirm-band one is "leaning", an unsure one "undecided". Invalid names
// why the provider's answer was refused; an unanswered question says so.
type Verdict struct {
	QuestionID string  `json:"question_id"`
	Subject    string  `json:"subject"`
	Kind       string  `json:"kind"`
	Answer     string  `json:"answer,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
	Band       string  `json:"band,omitempty"`
	Invalid    string  `json:"invalid,omitempty"`
}

// Reading renders the band policy in words for one verdict.
func (v Verdict) Reading() string {
	switch {
	case v.Invalid != "":
		return "invalid (" + v.Invalid + ")"
	case v.Band == string(decision.BandProceed):
		return "verdict"
	case v.Band == string(decision.BandConfirm):
		return "leaning (not counted)"
	case v.Band == string(decision.BandUnsure):
		return "undecided"
	}
	return "unanswered"
}

// Verdicts folds a Response into one Verdict per asked question, in
// question order.
func Verdicts(b Built, resp decision.Response) []Verdict {
	out := make([]Verdict, 0, len(b.Request.Questions))
	for _, q := range b.Request.Questions {
		v := Verdict{QuestionID: q.ID, Subject: b.Subjects[q.ID], Kind: string(q.Kind)}
		if a, ok := resp.Answers[q.ID]; ok {
			v.Confidence = a.Confidence
			v.Band = string(decision.BandOf(a.Confidence))
			switch q.Kind {
			case decision.KindNoul:
				v.Answer = "false"
				if a.Noul >= 0.5 {
					v.Answer = "true"
				}
			case decision.KindChoice:
				v.Answer = a.Choice
			default:
				v.Answer = fmt.Sprintf("%g", a.Score)
			}
		} else if why, bad := resp.Invalid[q.ID]; bad {
			v.Invalid = why
		} else {
			v.Invalid = "not answered by the provider"
		}
		out = append(out, v)
	}
	return out
}

// BandCounts tallies verdicts by band; invalid answers count separately.
func BandCounts(vs []Verdict) (proceed, confirm, unsure, invalid int) {
	for _, v := range vs {
		switch {
		case v.Invalid != "":
			invalid++
		case v.Band == string(decision.BandProceed):
			proceed++
		case v.Band == string(decision.BandConfirm):
			confirm++
		default:
			unsure++
		}
	}
	return
}

// EstimateTokens is compile's pre-flight heuristic (bytes/4, internal/cost)
// over everything the request carries.
func EstimateTokens(req decision.Request) int {
	n := cost.Estimate(req.State)
	for _, q := range req.Questions {
		n += cost.Estimate(q.Instructions) + cost.Estimate(q.TrueCriteria) + cost.Estimate(q.FalseCriteria)
		for _, o := range q.Options {
			n += cost.Estimate(o.Name) + cost.Estimate(o.Criteria)
		}
	}
	return n
}

func digestQuestions(lane string, qs []decision.Question) string {
	h := sha256.New()
	fmt.Fprintf(h, "lane=%s\n", lane)
	for _, q := range qs {
		fmt.Fprintf(h, "%s|%s|%s|%s|%s\n", q.ID, q.Kind, q.Instructions, q.TrueCriteria, q.FalseCriteria)
		opts := make([]string, 0, len(q.Options))
		for _, o := range q.Options {
			opts = append(opts, o.Name+"="+o.Criteria)
		}
		fmt.Fprintf(h, "options=%s\n", strings.Join(opts, ";"))
		fmt.Fprintf(h, "levels=%s\n", strings.Join(q.Levels, ";"))
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func shortSHA(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}
