package review

// trace.go — U8's traceability matrix (docs/design/change-and-compliance.md
// §3, plan CX-14 / CX-18): per requirement, the join requirement -> code
// (its anchor) -> test (certified_by: or a reaching test) -> certificate
// (state) -> attested run (digest, commit) -> decision (the ADR or
// invariant the requirement cites). Every cell carries its live state and
// the reading that proved it.
//
// Pure, like the rest of this package: the caller resolves plans, verify
// verdicts and certificates (cmd/fathom trace export); this file joins
// them. Identical input always yields an identical, identically-ordered
// matrix (determinism contract, internal/AGENTS.md) — `fathom trace verify`
// depends on it for byte-identical re-verification.

import (
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// Requirement kinds.
const (
	ReqPlanRow  = "plan_row"
	ReqSpecTask = "spec_task"
	ReqAccept   = "accept"
)

// Cell states. A cell with no evidence at all is CellNone — never omitted,
// so a gap is visible in the pack rather than absent from it.
const (
	CellNone       = "none"
	CellProven     = "proven"
	CellUnproven   = "unproven"
	CellClaimed    = "claimed"
	CellOpen       = "open"
	CellSkipped    = "skipped"
	CellPresent    = "present"
	CellAbsent     = "absent"
	CellReaches    = "reaches"
	CellUnreached  = "unreached"
	CellAttested   = "attested"
	CellCited      = "cited"
	CellNotGating  = "not_gating"
	CellUnverified = "unknown"
)

// TraceSource is one parsed plan the matrix draws requirements from: a
// native fathom plan (Dialect "" or "native") or a foreign spec (spec-kit,
// OpenSpec, ... — SI-), each already parsed into verify.Plan's shape.
type TraceSource struct {
	Path    string
	Dialect string
	Plan    verify.Plan
}

// TraceRequirement is one matrix requirement.
type TraceRequirement struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Source string `json:"source"`
	Text   string `json:"text"`
	// TaskID is the row the requirement belongs to (== ID except for an
	// accept criterion, whose ID is "<row>/accept").
	TaskID      string   `json:"task_id"`
	Anchor      string   `json:"anchor,omitempty"`
	CertifiedBy []string `json:"certified_by,omitempty"`
	Decisions   []string `json:"decisions,omitempty"`
	Claimed     bool     `json:"claimed"`
	Skipped     bool     `json:"skipped"`
}

var decisionRefRe = regexp.MustCompile(`\b(ADR-\d+|I\d+)\b`)

// citedDecisions extracts the ADR and invariant ids a row's text cites,
// sorted and de-duplicated.
func citedDecisions(texts ...string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range texts {
		for _, m := range decisionRefRe.FindAllString(t, -1) {
			if !seen[m] {
				seen[m] = true
				out = append(out, m)
			}
		}
	}
	sort.Strings(out)
	return out
}

func taskAnchor(t verify.Task) string {
	switch {
	case t.Expect.Symbol != "" && t.Expect.Package != "":
		return t.Expect.Package + "." + t.Expect.Symbol
	case t.Expect.Symbol != "":
		return t.Expect.Symbol
	case t.OraclePath != "":
		if t.OraclePattern != "" {
			return t.OraclePath + " ~ " + t.OraclePattern
		}
		return t.OraclePath
	}
	return ""
}

// traceRequirements lists every requirement the sources carry (CX-18): a
// native plan row, a foreign spec task, and — separately — each row's
// `accept:` criterion (SI-8), which is quoted acceptance text and never
// gates by itself, so its own requirement never inherits the row's proof.
// Sorted by (source, requirement id) for determinism.
func traceRequirements(sources []TraceSource) []TraceRequirement {
	var out []TraceRequirement
	for _, src := range sources {
		kind := ReqPlanRow
		if src.Dialect != "" && src.Dialect != "native" {
			kind = ReqSpecTask
		}
		for _, t := range src.Plan.Tasks {
			if t.ID == "" {
				continue
			}
			req := TraceRequirement{
				ID:          t.ID,
				Kind:        kind,
				Source:      src.Path,
				Text:        t.Title,
				TaskID:      t.ID,
				Anchor:      taskAnchor(t),
				CertifiedBy: append([]string(nil), t.Expect.CertifiedBy...),
				Decisions:   citedDecisions(t.Title, t.Reason),
				Claimed:     t.Claimed,
				Skipped:     t.Skipped,
			}
			sort.Strings(req.CertifiedBy)
			out = append(out, req)
			if strings.TrimSpace(t.Accept) != "" {
				out = append(out, TraceRequirement{
					ID:        t.ID + "/accept",
					Kind:      ReqAccept,
					Source:    src.Path,
					Text:      t.Accept,
					TaskID:    t.ID,
					Anchor:    req.Anchor,
					Decisions: req.Decisions,
					Claimed:   t.Claimed,
					Skipped:   t.Skipped,
				})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// TraceCertificate is one compiled certificate the caller resolved
// (internal/certify.GapCertificate flattened with its target).
type TraceCertificate struct {
	TargetKind string
	TargetID   string
	Test       string
	State      string
	RunDigest  string
}

// TraceInput is everything BuildTraceMatrix joins.
type TraceInput struct {
	Sources      []TraceSource
	Reports      map[string]verify.Report // keyed by TraceSource.Path
	Certificates []TraceCertificate
	// Commit is the commit the evidence was read at; an attested run's
	// cell carries it with the run digest.
	Commit string
	// Decisions is the set of decision ids that exist (ADR-N headings,
	// invariant ids) — a cited id absent from it reads "unproven".
	// Nil means "not resolved": cited ids read "cited" unchecked.
	Decisions map[string]bool
}

// TraceCell is one cell: its live state and the reading that proved it.
type TraceCell struct {
	State   string `json:"state"`
	Reading string `json:"reading"`
}

// TraceRow is one requirement's row of the matrix.
type TraceRow struct {
	Requirement TraceRequirement `json:"requirement"`
	Status      TraceCell        `json:"status"`
	Code        TraceCell        `json:"code"`
	Test        TraceCell        `json:"test"`
	Certificate TraceCell        `json:"certificate"`
	AttestedRun TraceCell        `json:"attested_run"`
	Decision    TraceCell        `json:"decision"`
}

// TraceMatrix is the whole matrix.
type TraceMatrix struct {
	Commit string     `json:"commit"`
	Rows   []TraceRow `json:"rows"`
}

func criterion(tv verify.TaskVerdict, name string) (verify.Criterion, bool) {
	for _, c := range tv.Criteria {
		if c.Name == name {
			return c, true
		}
	}
	return verify.Criterion{}, false
}

// BuildTraceMatrix joins requirements to their code, tests, certificates,
// attested runs and decisions (CX-14). Each cell's state is live — read
// from the verify report and certificate states the caller resolved at
// Commit — and carries the reading that proved it.
func BuildTraceMatrix(in TraceInput) TraceMatrix {
	verdicts := map[string]verify.TaskVerdict{}
	for path, rep := range in.Reports {
		for _, tv := range rep.Tasks {
			verdicts[path+"\x00"+tv.TaskID] = tv
		}
	}
	certs := append([]TraceCertificate(nil), in.Certificates...)
	sort.SliceStable(certs, func(i, j int) bool {
		a, b := certs[i], certs[j]
		if a.TargetKind != b.TargetKind {
			return a.TargetKind < b.TargetKind
		}
		if a.TargetID != b.TargetID {
			return a.TargetID < b.TargetID
		}
		return a.Test < b.Test
	})

	m := TraceMatrix{Commit: in.Commit}
	for _, req := range traceRequirements(in.Sources) {
		tv, haveVerdict := verdicts[req.Source+"\x00"+req.TaskID]
		row := TraceRow{Requirement: req}

		// Status: the row's own proof state. An accept criterion never
		// gates by itself (SI-8): it is listed, not proven.
		switch {
		case req.Kind == ReqAccept:
			row.Status = TraceCell{State: CellNotGating, Reading: "accept: criterion (quoted acceptance text, never gating by itself)"}
		case req.Skipped:
			row.Status = TraceCell{State: CellSkipped, Reading: "row disposed [-]"}
		case !haveVerdict:
			row.Status = TraceCell{State: CellUnverified, Reading: "no verify reading for this row"}
		case tv.Status == verify.Pass:
			row.Status = TraceCell{State: CellProven, Reading: "fathom verify: " + tv.Label}
		case tv.Status == verify.Fail:
			row.Status = TraceCell{State: CellUnproven, Reading: "fathom verify: " + tv.Label}
		default:
			row.Status = TraceCell{State: CellUnverified, Reading: "fathom verify: " + tv.Label}
		}
		if req.Kind != ReqAccept && !req.Skipped {
			if req.Claimed {
				row.Status.Reading += " (claimed [x])"
			} else {
				row.Status.Reading += " (open [ ])"
			}
		}

		// Code: the anchor, and whether verify found it.
		switch {
		case req.Anchor == "":
			row.Code = TraceCell{State: CellNone, Reading: "no anchor"}
		case !haveVerdict:
			row.Code = TraceCell{State: CellUnverified, Reading: req.Anchor}
		default:
			if c, ok := criterion(tv, "symbol_present"); ok {
				st := CellAbsent
				if c.Status == verify.Pass {
					st = CellPresent
				}
				row.Code = TraceCell{State: st, Reading: strings.TrimSpace(req.Anchor + " " + c.Detail)}
			} else if tv.Status == verify.Pass {
				row.Code = TraceCell{State: CellPresent, Reading: strings.TrimSpace(req.Anchor + " " + tv.EvidenceFile)}
			} else {
				row.Code = TraceCell{State: CellUnverified, Reading: req.Anchor}
			}
		}

		// Test: certified_by: names, else a reaching test from verify.
		switch {
		case len(req.CertifiedBy) > 0:
			row.Test = TraceCell{State: CellCited, Reading: "certified_by: " + strings.Join(req.CertifiedBy, ", ")}
			if haveVerdict {
				if c, ok := criterion(tv, "certified_by"); ok {
					st := CellUnreached
					if c.Status == verify.Pass {
						st = CellReaches
					}
					row.Test = TraceCell{State: st, Reading: "certified_by: " + strings.Join(req.CertifiedBy, ", ") + "; " + c.Detail}
				}
			}
		case haveVerdict:
			if c, ok := criterion(tv, "test_reaches"); ok {
				st := CellUnreached
				if c.Status == verify.Pass {
					st = CellReaches
				} else if c.Status == verify.NA {
					st = CellNone
				}
				row.Test = TraceCell{State: st, Reading: c.Detail}
			} else {
				row.Test = TraceCell{State: CellNone, Reading: "no test criterion"}
			}
		default:
			row.Test = TraceCell{State: CellNone, Reading: "no test named"}
		}

		// Certificate + attested run: certificates whose test the row
		// names, or whose target is the row's anchor symbol.
		var matched []TraceCertificate
		sym := ""
		if i := strings.LastIndex(req.Anchor, "."); i >= 0 && !strings.Contains(req.Anchor, " ") {
			sym = req.Anchor[i+1:]
		} else if !strings.Contains(req.Anchor, " ") {
			sym = req.Anchor
		}
		named := map[string]bool{}
		for _, n := range req.CertifiedBy {
			named[n] = true
		}
		for _, c := range certs {
			if named[c.Test] || (sym != "" && (c.TargetID == sym || strings.HasSuffix(c.TargetID, "."+sym))) {
				matched = append(matched, c)
			}
		}
		if len(matched) == 0 {
			row.Certificate = TraceCell{State: CellNone, Reading: "no certificate"}
			row.AttestedRun = TraceCell{State: CellNone, Reading: "no attested run"}
		} else {
			state := "live"
			var readings, digests []string
			for _, c := range matched {
				if c.State != "live" {
					state = c.State
				}
				readings = append(readings, c.TargetKind+":"+c.TargetID+" "+c.Test+"="+c.State)
				if c.RunDigest != "" {
					digests = append(digests, c.RunDigest)
				}
			}
			row.Certificate = TraceCell{State: state, Reading: strings.Join(readings, "; ")}
			if len(digests) == 0 {
				row.AttestedRun = TraceCell{State: CellNone, Reading: "certificate carries no run digest"}
			} else {
				sort.Strings(digests)
				row.AttestedRun = TraceCell{State: CellAttested, Reading: strings.Join(digests, ", ") + " @ " + in.Commit}
			}
		}

		// Decision: the ADR/invariant ids the row cites.
		if len(req.Decisions) == 0 {
			row.Decision = TraceCell{State: CellNone, Reading: "no decision cited"}
		} else {
			state := CellCited
			var missing []string
			if in.Decisions != nil {
				for _, d := range req.Decisions {
					if !in.Decisions[d] {
						missing = append(missing, d)
					}
				}
			}
			reading := strings.Join(req.Decisions, ", ")
			if len(missing) > 0 {
				state = CellUnproven
				reading += " (not found: " + strings.Join(missing, ", ") + ")"
			}
			row.Decision = TraceCell{State: state, Reading: reading}
		}
		m.Rows = append(m.Rows, row)
	}
	return m
}
